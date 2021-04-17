package g2cache

import (
	"github.com/mohae/deepcopy"
	"log"
	"sync"
)

// The value can be changed by external assignment
// It does not need to be too large, only one area needs to be locked, and it is very suitable if there are not many concurrent keys.
const (
	defaultShards         = 256
	defaultShardsAndOpVal = 255
)

var (
	CacheDebug bool
)

type G2Cache struct {
	gid      string
	out      OutCache
	local    LocalCache
	shards   [defaultShards]sync.Mutex
	hash     Harsher
	stop     chan struct{}
	stopOnce sync.Once
	channel  chan *ChannelMeta
}

// Shouldn't throw a panic, please return an error
type LoadDataSourceFunc func() (interface{}, error)

func New(out OutCache, local LocalCache) *G2Cache {
	g := &G2Cache{
		hash:    new(fnv64a),
		stop:    make(chan struct{}, 1),
		channel: make(chan *ChannelMeta, defaultShards),
	}
	gid, err := NewUUID()
	if err != nil {
		log.Fatalln(err)
	}
	g.gid = gid
	if local == nil {
		local = NewFreeCache()
	}
	if out == nil {
		out = NewRedisCache(local)
	}
	g.local = local
	g.out = out

	go g.Subscribe()

	return g
}

func (g *G2Cache) Get(key string, ttl int, obj interface{}, fn LoadDataSourceFunc) error {
	select {
	case <-g.stop:
		return CacheClose
	default:
	}
	return g.get(key, ttl, obj, fn)
}

func (g *G2Cache) get(key string, ttl int, obj interface{}, fn LoadDataSourceFunc) error {
	v, ok, err := g.local.Get(key, obj)
	if err != nil {
		return err
	}
	if ok {
		if CacheDebug {
			log.Printf("key:%-20s ==>hit local storage\n", key)
		}
		if v.Obsoleted() {
			to := deepcopy.Copy(obj)
			go func() {
				// Pass a copy in order to explore the internal structure of obj
				_err := g.syncLocalCache(key, ttl, to, fn)
				if _err != nil {
					log.Printf("syncMemCache key=%v,err=%v", key, _err)
				}
			}()
		}
		return clone(v.Value, obj)
	}
	v, ok, err = g.out.Get(key, obj)
	if err != nil {
		return err
	}
	if ok {
		if CacheDebug {
			log.Printf("key:%-20s ==>hit out storage\n", key)
		}
		err = g.local.Set(key, v)
		return clone(v.Value, obj)
	}

	// 从数据源加载
	idx := g.hash.Sum64(key)
	g.shards[idx&defaultShardsAndOpVal].Lock()
	defer g.shards[idx&defaultShardsAndOpVal].Unlock()

	if CacheDebug {
		log.Printf("key:%-20s ==>hit data source\n", key)
	}
	o, err := fn()
	if err != nil {
		return err
	}
	e := NewEntry(o, ttl)
	err = g.local.Set(key, e)
	if err != nil {
		return err
	}

	go func() {
		_err := g.out.Set(key, e)
		if _err != nil {
			log.Printf("syncMemCache key=%v,val=%+v,,err=%v", key, e, _err)
		}
	}()

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
		err = g.out.Publish(g.gid, key, SetPublishType, v)
		return err
	}

	go func() {
		_err := g.out.Set(key, v)
		_err = g.out.Publish(g.gid, key, SetPublishType, v)
		if _err != nil {
			log.Println("g2cache set err", _err)
		}
	}()

	return err
}

func (g *G2Cache) syncLocalCache(key string, ttl int, obj interface{}, fn LoadDataSourceFunc) error {
	e, ok, err := g.out.Get(key, obj)
	if err != nil {
		return err
	}
	if !ok || e == nil || e.Expired() {
		// 从fn里面加载数据
		idx := g.hash.Sum64(key)
		g.shards[idx&defaultShardsAndOpVal].Lock()
		defer g.shards[idx&defaultShardsAndOpVal].Unlock()

		v, err := fn()
		if err != nil {
			return err
		}
		if v == nil {
			return nil
		}
		e = NewEntry(v, ttl)
		err = g.local.Set(key, e)
		err = g.out.Set(key, e)
		return err
	}
	err = g.local.Set(key, e)
	return err
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
	if err != nil {
		return err
	}
	if wait {
		err = g.out.Del(key)
		err = g.out.Publish(g.gid, key, DelPublishType, nil)
		return err
	}
	go func() {
		_err := g.out.Del(key)
		_err = g.out.Publish(g.gid, key, DelPublishType, nil)
		log.Println("g2cache del err ", _err)
	}()

	return nil
}

func (g *G2Cache) Subscribe() error {
	select {
	case <-g.stop:
		return CacheClose
	default:
	}
	return g.subscribe()
}

func (g *G2Cache) subscribe() error {
	err := g.out.Subscribe(g.channel)
	if err != nil {
		return err
	}

	go func() {
		for meta := range g.channel {
			select {
			case <-g.stop:
				return
			default:
			}
			if meta.Gid == g.gid {
				continue
			}
			key := meta.Key
			switch meta.Action {
			case DelPublishType:
				if err := g.local.Del(key); err != nil {
					log.Printf("local del key=%v, err=%v\n", key, err)
				}
				go func() {
					if _err := g.out.Del(key); _err != nil {
						log.Printf("out del key=%v, err=%v\n", key, _err)
					}
				}()
			case SetPublishType:
				if err = g.local.Set(meta.Key, meta.Data); err != nil {
					log.Printf("local set key=%v,val=%+v, err=%v\n", key, *meta.Data, err)
				}
				go func() {
					if _err := g.out.Set(key, meta.Data); _err != nil {
						log.Printf("out set key=%v,val=%v, err=%v\n", key, *meta.Data, _err)
					}
				}()
			default:
				continue
			}
		}
	}()

	return nil
}

func (g *G2Cache) close() {
	close(g.stop)
	g.out.Close()
	g.local.Close()
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
