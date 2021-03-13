package g2cache

import (
	"github.com/gomodule/redigo/redis"
	"sync"
	"time"
)

type RedisCache struct {
	pool  *redis.Pool
	mutex sync.Map
	mem   *MemCache
}

func NewRedisCache(p *redis.Pool, m *MemCache) *RedisCache {
	c := &RedisCache{
		pool:  p,
		mutex: sync.Map{},
		mem:   m,
	}
	return c
}

func (r *RedisCache) Del(key string) error  {
	return RedisDelKey(key,r.pool)
}

func (r *RedisCache) load(key string, obj interface{}, ttl int, fn LoadFunc, sync bool) error {
	mux := r.getMutex(key)
	mux.RLock()
	defer mux.RUnlock()

	if e, fresh := r.mem.Load(key); fresh {
		if sync {
			return clone(e.Value, obj)
		}
		return nil
	}

	v, err := fn()
	if err != nil {
		return err
	}

	if sync {
		if err = clone(v, obj); err != nil {
			return err
		}
	}

	e := NewEntry(v, ttl)
	rdsTtl := (e.Expiration - time.Now().UnixNano()) / int64(time.Second)
	bs, err := json.Marshal(e)
	if err != nil {
		return err
	}
	err = RedisSetString(key, string(bs), int(rdsTtl), r.pool)
	if err != nil {
		return err
	}
	r.mem.Set(key, e)
	return nil
}

func (r *RedisCache) Get(key string, obj interface{}) (*Entry, bool) {
	mux := r.getMutex(key)
	mux.RLock()
	defer mux.RUnlock()

	if v, fresh := r.mem.Load(key); fresh {
		return v, true
	}
	str, err := RedisGetString(key, r.pool)
	if err != nil && err != redis.ErrNil {
		return nil, false
	}
	if str == "" {
		return nil, false
	}
	var e Entry
	e.Value = obj
	if json.UnmarshalFromString(str, &e) != nil {
		return nil, false
	}
	r.mem.Set(key, &e)
	return &e, true
}

func (r *RedisCache) getMutex(key string) *sync.RWMutex {
	var mux *sync.RWMutex
	nMux := new(sync.RWMutex)
	if oMux, ok := r.mutex.LoadOrStore(key, nMux); ok {
		mux = oMux.(*sync.RWMutex)
	} else {
		mux = nMux
	}
	return mux
}
