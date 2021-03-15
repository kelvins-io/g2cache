package g2cache

import (
	jsoniter "github.com/json-iterator/go"
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

func (g *G2Cache) Get(key string, ttl int, fn LoadDataSourceFunc) (interface{}, error) {
	select {
	case <-g.stop:
		return nil, CacheClose
	default:
	}
	return g.get(key, ttl, fn)
}

func (g *G2Cache) get(key string, ttl int, fn LoadDataSourceFunc) (interface{}, error) {
	v, ok, err := g.local.Get(key)
	if err != nil {
		return nil, err
	}
	if ok {
		if CacheDebug {
			log.Printf("key:%-20s ==>hit local storage\n",key)
		}
		if v.Obsoleted() {
			go func() {
				err := g.syncLocalCache(key, ttl, fn)
				if err != nil {
					log.Printf("syncMemCache key=%v,err=%v", key, err)
				}
			}()
		}
		return jsoniter.MarshalToString(v.Value)
	}
	v, ok, err = g.out.Get(key)
	if err != nil {
		return nil, err
	}
	if ok {
		if CacheDebug {
			log.Printf("key:%-20s ==>hit out storage\n",key)
		}
		err = g.local.Set(key, v)
		return jsoniter.MarshalToString(v.Value)
	}

	// 从数据源加载
	idx := g.hash.Sum64(key)
	g.shards[idx&defaultShardsAndOpVal].Lock()
	defer g.shards[idx&defaultShardsAndOpVal].Unlock()

	if CacheDebug {
		log.Printf("key:%-20s ==>hit data source\n",key)
	}
	o, err := fn()
	if err != nil {
		return nil, err
	}
	e := NewEntry(o, ttl)
	err = g.local.Set(key, e)
	if err != nil {
		return nil, err
	}

	go func() {
		err = g.out.Set(key, e)
		if err != nil {
			log.Printf("syncMemCache key=%v,val=%+v,,err=%v", key, e, err)
		}
	}()

	return jsoniter.MarshalToString(o)
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
		err = g.out.Publish(key, SetPublishType, v)
		return err
	}

	go func() {
		_ = g.out.Publish(key, SetPublishType, v)
	}()

	return err
}

func (g *G2Cache) syncLocalCache(key string, ttl int, fn LoadDataSourceFunc) error {
	e, ok, err := g.out.Get(key)
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
		err = g.out.Set(key, e)
		err = g.local.Set(key, e)
		return err
	}
	err = g.local.Set(key, e)
	err = g.out.Set(key, e)
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

	if wait {
		err = g.out.Publish(key, DelPublishType, nil)
		return err
	}
	go func() {
		_ = g.out.Publish(key, DelPublishType, nil)
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
			key := meta.Key
			switch meta.Action {
			case DelPublishType:
				if err := g.local.Del(key); err != nil {
					log.Printf("local del key=%v, err=%v\n", key, err)
				}
				if err = g.out.Del(key); err != nil {
					log.Printf("rds del key=%v, err=%v\n", key, err)
				}
			case SetPublishType:
				if err = g.local.Set(meta.Key, meta.Data); err != nil {
					log.Printf("local set key=%v,val=%+v, err=%v\n", key, *meta.Data, err)
				}
				if err = g.out.Set(key, meta.Data); err != nil {
					log.Printf("rds set key=%v,val=%v, err=%v\n", key, *meta.Data, err)
				}
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
