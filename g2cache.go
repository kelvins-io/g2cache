package g2cache

import (
	"fmt"
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
	CacheMonitorSecond          = 5
	DefaultGPoolWorkerNum       = 512
	DefaultGPoolJobQueueChanLen = 1000
)

var (
	HitStatisticsOut HitStatistics
)

type G2Cache struct {
	GID      string
	out      OutCache
	local    LocalCache
	shards   [defaultShards]sync.Mutex
	hash     Harsher
	stop     chan struct{}
	stopOnce sync.Once
	channel  chan *ChannelMeta
	gPool    *Pool
}

func New(out OutCache, local LocalCache) *G2Cache {
	gid, err := NewUUID()
	if err != nil {
		panic(fmt.Sprintf("gen gid err :%v", err))
	}
	LogInfo("gid = ", gid)
	g := &G2Cache{
		GID:     gid,
		hash:    new(fnv64a),
		stop:    make(chan struct{}, 1),
		channel: make(chan *ChannelMeta, defaultShards),
		gPool:   NewPool(DefaultGPoolWorkerNum, DefaultGPoolJobQueueChanLen),
	}
	if local == nil {
		local = NewFreeCache()
	}
	if out == nil {
		out = NewRedisCache(local)
	}
	g.local = local
	g.out = out

	_, ok := g.out.(PubSub)
	if ok {
		g.gPool.SendJob(wrapFuncErr(g.subscribe))
	}

	HitStatisticsOut.AccessGetTotal = 1

	if CacheMonitor {
		g.gPool.SendJob(g.monitor)
	}

	return g
}

func (g *G2Cache) monitor() {
	t := time.NewTicker(time.Duration(CacheMonitorSecond) * time.Second)
	for {
		select {
		case <-t.C:
			HitStatisticsOut.Calculation()
			LogInf("statistics local hit percentage %.4f", HitStatisticsOut.HitLocalStorageTotalRate*100)
			LogInf("statistics out hit percentage %.4f", HitStatisticsOut.HitOutStorageTotalRate*100)
			LogInf("statistics data source hit percentage %.4f", HitStatisticsOut.HitDataSourceTotalRate*100)
		}
	}
}

// ttl is second
func (g *G2Cache) Get(key string, ttl int, obj interface{}, fn LoadDataSourceFunc) error {
	select {
	case <-g.stop:
		return CacheClose
	default:
	}
	return g.get(key, ttl, obj, fn)
}

func (g *G2Cache) get(key string, ttl int, obj interface{}, fn LoadDataSourceFunc) error {
	atomic.AddInt64(&HitStatisticsOut.AccessGetTotal, 1)
	v, ok, err := g.local.Get(key, obj) // sync so not need copy obj
	if err != nil {
		return err
	}
	if ok {
		atomic.AddInt64(&HitStatisticsOut.HitLocalStorageTotal, 1)
		if CacheDebug {
			LogInf("key:%-30s => [\u001B[32m hit local storage \u001B[0m]\n", key)
		}
		if v.Obsoleted() {
			to := deepcopy.Copy(obj) // async so copy obj
			g.gPool.SendJob(func() {
				// Pass a copy in order to explore the internal structure of obj
				err := g.syncLocalCache(key, ttl, to, fn)
				if err != nil {
					LogInf("syncMemCache key=%s,err=%v\n", key, err)
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
				LogInf("key:%-30s => [\u001B[33m hit out storage \u001B[0m]\n", key)
			}
			// Prevent penetration of external storage
			err = g.local.Set(key, v)
			if err != nil {
				return err
			}
			return clone(v.Value, obj)
		}
	}

	o, err := g.syncOutCache(key, ttl, fn)
	if err != nil {
		return err
	}
	return clone(o, obj)
}

func (g *G2Cache) Set(key string, obj interface{}, ttl int, wait bool) (err error) {
	select {
	case <-g.stop:
		return CacheClose
	default:
	}
	return g.set(key, obj, ttl, wait)
}

func (g *G2Cache) set(key string, obj interface{}, ttl int, wait bool) (err error) {
	v := NewEntry(obj, ttl)
	err = g.local.Set(key, v)
	if err != nil {
		return err
	}
	if wait {
		err = g.out.Set(key, v)
		if err != nil {
			LogErr(err)
			return err
		}
		pubsub, ok := g.out.(PubSub)
		if ok {
			return pubsub.Publish(g.GID, key, SetPublishType, v)
		}
		return nil
	}

	g.gPool.SendJob(func() {
		_err := g.out.Set(key, v)
		if _err != nil {
			LogErr(_err)
		}
		pubsub, ok := g.out.(PubSub)
		if ok {
			_err = pubsub.Publish(g.GID, key, SetPublishType, v)
			if _err != nil {
				LogErr(_err)
			}
		}
	})

	return err
}

func (g *G2Cache) syncLocalCache(key string, ttl int, obj interface{}, fn LoadDataSourceFunc) error {
	e, ok, err := g.out.Get(key, obj)
	if err != nil {
		return err
	}
	if !ok || e.Expired() {
		// 从fn里面加载数据
		idx := g.hash.Sum64(key)
		g.shards[idx&defaultShardsAndOpVal].Lock()
		defer g.shards[idx&defaultShardsAndOpVal].Unlock()

		atomic.AddInt64(&HitStatisticsOut.HitDataSourceTotal, 1)
		if CacheDebug {
			LogInf("key:%-30s => [\u001B[31m hit data source \u001B[0m]\n", key)
		}

		v, err := fn()
		if err != nil {
			return err
		}
		if v == nil {
			return DataSourceLoadNil
		}
		e = NewEntry(v, ttl)
		err = g.local.Set(key, e)
		if err != nil {
			return err
		}
		g.gPool.SendJob(func() {
			_err := g.out.Set(key, e)
			if _err != nil {
				LogErr("syncLocalCache out set err ", err)
			}
			pubsub, ok := g.out.(PubSub)
			if ok {
				_err = pubsub.Publish(g.GID, key, SetPublishType, e)
				if _err != nil {
					LogErr("syncLocalCache Publish err", err)
				}
			}
		})
	}

	atomic.AddInt64(&HitStatisticsOut.HitOutStorageTotal, 1)
	if CacheDebug {
		LogInf("key:%-30s => [\u001B[33m hit out storage \u001B[0m]\n", key)
	}

	return g.local.Set(key, e)
}

func (g *G2Cache) syncOutCache(key string, ttl int, fn LoadDataSourceFunc) (interface{}, error) {
	// 从数据源加载
	idx := g.hash.Sum64(key)
	g.shards[idx&defaultShardsAndOpVal].Lock()
	defer g.shards[idx&defaultShardsAndOpVal].Unlock()

	atomic.AddInt64(&HitStatisticsOut.HitDataSourceTotal, 1)
	if CacheDebug {
		LogInf("key:%-30s => [\u001B[31m hit data source \u001B[0m]\n", key)
	}
	o, err := fn()
	if err != nil {
		return nil, err
	}
	if o == nil {
		return nil, DataSourceLoadNil
	}
	e := NewEntry(o, ttl)
	err = g.local.Set(key, e)
	if err != nil {
		return nil, err
	}
	g.gPool.SendJob(func() {
		_err := g.out.Set(key, e)
		pubsub, ok := g.out.(PubSub)
		if ok {
			_err = pubsub.Publish(g.GID, key, SetPublishType, e)
			if _err != nil {
				LogInf("syncOutCache key=%s,val=%+v,,err=%v\n", key, e, err)
			}
		}
	})

	return o, err
}

func (g *G2Cache) Del(key string, wait bool) (err error) {
	select {
	case <-g.stop:
		return CacheClose
	default:
	}
	return g.del(key, wait)
}

func (g *G2Cache) del(key string, wait bool) (err error) {
	err = g.local.Del(key)
	if wait {
		err = g.out.Del(key)
		if err != nil {
			LogErr(err)
		}
		pubsub, ok := g.out.(PubSub)
		if ok {
			return pubsub.Publish(g.GID, key, DelPublishType, nil)
		}
		return nil
	}
	g.gPool.SendJob(func() {
		_err := g.out.Del(key)
		if _err != nil {
			LogErr(_err)
		}
		pubsub, ok := g.out.(PubSub)
		if ok {
			_err = pubsub.Publish(g.GID, key, DelPublishType, nil)
			if _err != nil {
				LogErr(_err)
			}
		}
	})

	return nil
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
			if err = g.out.Del(key); err != nil {
				LogInf("out del key=%v, err=%v\n", key, err)
			}
			g.gPool.SendJob(func() {
				if err := g.local.Del(key); err != nil {
					LogInf("local del key=%v, err=%v\n", key, err)
				}
			})
		case SetPublishType:
			if err = g.local.Set(meta.Key, meta.Data); err != nil {
				LogInf("local set key=%v,val=%+v, err=%v\n", key, *meta.Data, err)
			}
			g.gPool.SendJob(func() {
				if err = g.out.Set(key, meta.Data); err != nil {
					LogInf("out set key=%v,val=%v, err=%v\n", key, *meta.Data, err)
				}
			})
		default:
			continue
		}
	}

	return nil
}

func (g *G2Cache) close() {
	close(g.stop)
	g.out.Close()
	g.local.Close()
	g.gPool.Release()
	g.gPool.WaitAll()
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
