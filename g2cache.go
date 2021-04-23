package g2cache

import (
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"github.com/mohae/deepcopy"
	"sync"
	"sync/atomic"
	"time"
)

// The value can be changed by external assignment
// It does not need to be too large, only one area needs to be locked, and it is very suitable if there are not many concurrent keys.
const (
	defaultShards         = 256
	defaultShardsAndOpVal = 255
)

var (
	CacheDebug                  bool
	CacheMonitor                bool
	OutCachePubSub              bool
	CacheMonitorSecond          = 5
	DefaultGPoolWorkerNum       = 200
	DefaultGPoolJobQueueChanLen = 1000
)

var (
	HitStatisticsOut HitStatistics
)

type G2Cache struct {
	GID      string // Identifies the number of an instance
	out      OutCache
	local    LocalCache
	shards   [defaultShards]sync.Mutex
	hash     Harsher
	stop     chan struct{}
	stopOnce sync.Once
	channel  chan *ChannelMeta
	gPool    *Pool
}

func New(out OutCache, local LocalCache) (g *G2Cache, err error) {
	if local == nil {
		local = NewFreeCache()
	}
	if out == nil {
		out, err = NewRedisCache()
		if err != nil {
			return nil, fmt.Errorf("NewRedisCache err: %v", err)
		}
	}

	gid, err := NewUUID()
	if err != nil {
		return nil, fmt.Errorf("gen G2Cache.gid err :%v", err)
	}
	LogInfo("[g2cache.gid] = ", gid)
	g = &G2Cache{
		GID:     gid,
		hash:    new(fnv64a),
		stop:    make(chan struct{}, 1),
		channel: make(chan *ChannelMeta, defaultShards),
		gPool:   NewPool(DefaultGPoolWorkerNum, DefaultGPoolJobQueueChanLen),
	}
	g.local = local
	g.out = out

	_, ok := g.out.(PubSub)
	if ok && OutCachePubSub {
		g.gPool.SendJob(wrapFuncErr(g.subscribe))
	}

	HitStatisticsOut.AccessGetTotal = 1

	if CacheMonitor {
		g.gPool.SendJob(g.monitor)
	}

	return g, nil
}

func (g *G2Cache) monitor() {
	t := time.NewTicker(time.Duration(CacheMonitorSecond) * time.Second)
	for {
		select {
		case <-g.stop:
			return
		case <-t.C:
			HitStatisticsOut.Calculation()
			LogDebugF("statistics [\u001B[32mlocal\u001B[0m] hit percentage %.4f", HitStatisticsOut.HitLocalStorageTotalRate*100)
			LogDebugF("statistics [\u001B[33mout\u001B[0m] hit percentage %.4f", HitStatisticsOut.HitOutStorageTotalRate*100)
			LogDebugF("statistics [\u001B[31mdata source\u001B[0m] hit percentage %.4f", HitStatisticsOut.HitDataSourceTotalRate*100)
		}
	}
}

// Get return err DataSourceLoadNil if fn exec return nil
func (g *G2Cache) Get(key string, ttlSecond int, obj interface{}, fn LoadDataSourceFunc) error {
	select {
	case <-g.stop:
		return CacheClose
	default:
	}
	if key == "" {
		return CacheKeyEmpty
	}
	if obj == nil {
		return CacheObjNil
	}
	if fn == nil {
		return LoadDataSourceFuncNil
	}
	if ttlSecond <= 0 {
		ttlSecond = 5
	}
	return g.get(key, ttlSecond, obj, fn)
}

func (g *G2Cache) get(key string, ttlSecond int, obj interface{}, fn LoadDataSourceFunc) error {
	atomic.AddInt64(&HitStatisticsOut.AccessGetTotal, 1)
	v, ok, err := g.local.Get(key, obj) // sync so not need copy obj
	if err != nil {
		return err
	}
	if ok {
		atomic.AddInt64(&HitStatisticsOut.HitLocalStorageTotal, 1)
		if CacheDebug {
			LogDebugF("key:%-30s => [\u001B[32m hit local storage \u001B[0m]\n", key)
		}
		if v.Obsoleted() {
			to := deepcopy.Copy(obj) // async so copy obj
			g.gPool.SendJob(func() {
				// Pass a copy in order to explore the internal structure of obj
				err := g.syncLocalCache(key, ttlSecond, to, fn)
				if err != nil {
					LogErrF("syncMemCache key=%s,err=%v\n", key, err)
				}
			})
		}
		return clone(v.Value, obj)
	}
	v, ok, err = g.out.Get(key, obj)
	if err != nil {
		return err
	}
	if ok {
		if !v.Expired() {
			atomic.AddInt64(&HitStatisticsOut.HitOutStorageTotal, 1)
			if CacheDebug {
				LogDebugF("key:%-30s => [\u001B[33m hit out storage \u001B[0m]\n", key)
			}
			// Prevent penetration of external storage
			err = g.local.Set(key, v)
			if err != nil {
				return err
			}
			return clone(v.Value, obj)
		}
	}

	if fn != nil {
		o, err := g.syncOutCache(key, ttlSecond, fn)
		if err != nil {
			return err
		}
		return clone(o, obj)
	}

	return OutStorageLoadNil
}

func (g *G2Cache) syncLocalCache(key string, ttlSecond int, obj interface{}, fn LoadDataSourceFunc) error {
	e, ok, err := g.out.Get(key, obj)
	if err != nil {
		return err
	}
	if !ok || e.Expired() {
		atomic.AddInt64(&HitStatisticsOut.HitDataSourceTotal, 1)
		if CacheDebug {
			LogDebugF("key:%-30s => [\u001B[31m hit data source \u001B[0m]\n", key)
		}
		// 从fn里面加载数据
		g.getShardsLock(key)
		defer g.releaseShardsLock(key)

		v, err := fn()
		if err != nil {
			return err
		}
		if v == nil {
			return DataSourceLoadNil
		}
		e = NewEntry(v, ttlSecond)
		err = g.local.Set(key, e)
		if err != nil {
			return err
		}
		g.gPool.SendJob(func() {
			_err := g.out.Set(key, e)
			if _err != nil {
				LogErr("syncLocalCache out set err:", err)
				return
			}
			pubsub, ok := g.out.(PubSub)
			if ok && OutCachePubSub {
				_err = pubsub.Publish(g.GID, key, SetPublishType, e)
				if _err != nil {
					LogErr("syncLocalCache Publish err:", err)
				}
			}
		})
	}

	atomic.AddInt64(&HitStatisticsOut.HitOutStorageTotal, 1)
	if CacheDebug {
		LogDebugF("key:%-30s => [\u001B[33m hit out storage \u001B[0m]\n", key)
	}

	return g.local.Set(key, e)
}

func (g *G2Cache) syncOutCache(key string, ttlSecond int, fn LoadDataSourceFunc) (interface{}, error) {

	atomic.AddInt64(&HitStatisticsOut.HitDataSourceTotal, 1)
	if CacheDebug {
		LogDebugF("key:%-30s => [\u001B[31m hit data source \u001B[0m]\n", key)
	}
	// 从数据源加载
	g.getShardsLock(key)
	defer g.releaseShardsLock(key)

	o, err := fn()
	if err != nil {
		return nil, err
	}
	if o == nil {
		return nil, DataSourceLoadNil
	}
	e := NewEntry(o, ttlSecond)
	err = g.local.Set(key, e)
	if err != nil {
		return nil, err
	}
	g.gPool.SendJob(func() {
		_err := g.out.Set(key, e)
		if _err != nil {
			return
		}
		pubsub, ok := g.out.(PubSub)
		if ok && OutCachePubSub {
			_err = pubsub.Publish(g.GID, key, SetPublishType, e)
			if _err != nil {
				eS, _ := jsoniter.MarshalToString(e)
				LogErrF("syncOutCache key=%s,val=%s,err=%v\n", key, eS, err)
			}
		}
	})

	return o, err
}

// This function may block
func (g *G2Cache) getShardsLock(key string) {
	idx := g.hash.Sum64(key)
	g.shards[idx&defaultShardsAndOpVal].Lock()
}

func (g *G2Cache) releaseShardsLock(key string) {
	idx := g.hash.Sum64(key)
	g.shards[idx&defaultShardsAndOpVal].Unlock()
}

func (g *G2Cache) Set(key string, obj interface{}, ttlSecond int, wait bool) (err error) {
	select {
	case <-g.stop:
		return CacheClose
	default:
	}
	if key == "" {
		return CacheKeyEmpty
	}
	if obj == nil {
		return CacheObjNil
	}
	if ttlSecond <= 0 {
		ttlSecond = 5
	}
	return g.set(key, obj, ttlSecond, wait)
}

func (g *G2Cache) set(key string, obj interface{}, ttlSecond int, wait bool) (err error) {
	v := NewEntry(obj, ttlSecond)
	if wait {
		return g.setInternal(key, v)
	}

	g.gPool.SendJob(func() {
		_err := g.setInternal(key, v)
		if _err != nil {
			objS, _ := jsoniter.MarshalToString(v)
			LogErrF("setInternal key: %s,obj: %s ,err: %v", key, objS, err)
		}
	})

	return err
}

func (g *G2Cache) setInternal(key string, e *Entry) (err error) {
	err = g.local.Set(key, e)
	if err != nil {
		return err
	}
	err = g.out.Set(key, e)
	if err != nil {
		return err
	}
	pubsub, ok := g.out.(PubSub)
	if ok && OutCachePubSub {
		return pubsub.Publish(g.GID, key, SetPublishType, e)
	}
	return nil
}

func (g *G2Cache) Del(key string, wait bool) (err error) {
	select {
	case <-g.stop:
		return CacheClose
	default:
	}
	if key == "" {
		return CacheKeyEmpty
	}
	return g.del(key, wait)
}

func (g *G2Cache) del(key string, wait bool) (err error) {
	if wait {
		return g.delInternal(key)
	}
	g.gPool.SendJob(func() {
		_err := g.delInternal(key)
		if _err != nil {
			LogErrF("delInternal key: %s,err: %v", key, err)
		}
	})

	return nil
}

func (g *G2Cache) delInternal(key string) (err error) {
	defer func() {
		if err == nil {
			err = g.local.Del(key)
		}
	}()
	err = g.out.Del(key)
	if err != nil {
		return err
	}
	pubsub, ok := g.out.(PubSub)
	if ok && OutCachePubSub {
		return pubsub.Publish(g.GID, key, DelPublishType, nil)
	}
	return err
}

func (g *G2Cache) subscribe() error {
	select {
	case <-g.stop:
		return CacheClose
	default:
	}
	return g.subscribeInternal()
}

func (g *G2Cache) subscribeInternal() error {
	pubsub, ok := g.out.(PubSub)
	if !ok {
		return CacheNotImplementPubSub
	}
	err := pubsub.Subscribe(g.channel)
	if err != nil {
		return err
	}

	for meta := range g.channel {
		select {
		case <-g.stop:
			return OutStorageClose
		default:
		}
		if meta.Gid == g.GID {
			continue
		}
		key := meta.Key
		switch meta.Action {
		case DelPublishType:
			g.gPool.SendJob(func() {
				if err = g.out.Del(key); err != nil {
					LogErrF("out del key=%s, err=%v\n", key, err)
				}
				if err := g.local.Del(key); err != nil {
					LogErrF("local del key=%s, err=%v\n", key, err)
				}
			})
		case SetPublishType:
			g.gPool.SendJob(func() {
				if err = g.local.Set(meta.Key, meta.Data); err != nil {
					dataS, _ := jsoniter.MarshalToString(meta.Data)
					LogErrF("local set key=%s,val=%s, err=%v\n", key, dataS, err)
				}
				if err = g.out.Set(key, meta.Data); err != nil {
					dataS, _ := jsoniter.MarshalToString(meta.Data)
					LogErrF("out set key=%s,val=%s, err=%v\n", key, dataS, err)
				}
			})
		default:
			continue
		}
	}

	return nil
}

func (g *G2Cache) close() {
	if g.stop != nil {
		close(g.stop)
	}
	if g.out != nil {
		g.out.Close()
	}
	if g.out != nil {
		g.local.Close()
	}
	if g.channel != nil {
		close(g.channel)
	}
	if g.gPool != nil {
		g.gPool.Release()
	}
}

func (g *G2Cache) Close() {
	g.stopOnce.Do(g.close)
}

type Harsher interface {
	Sum64(string) uint64
}

type fnv64a struct{}

const (
	// offset64 FNVa offset basis. See https://en.wikipedia.org/wiki/Fowler–Noll–Vo_hash_function#FNV-1a_hash
	offset64 = 14695981039346656037
	// prime64 FNVa prime value. See https://en.wikipedia.org/wiki/Fowler–Noll–Vo_hash_function#FNV-1a_hash
	prime64 = 1099511628211
)

// Sum64 gets the string and returns its uint64 hash value.
func (f fnv64a) Sum64(key string) uint64 {
	var hash uint64 = offset64
	for i := 0; i < len(key); i++ {
		hash ^= uint64(key[i])
		hash *= prime64
	}

	return hash
}
