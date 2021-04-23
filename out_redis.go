package g2cache

import (
	"github.com/gomodule/redigo/redis"
	jsoniter "github.com/json-iterator/go"
	"sync"
)

var DefaultPubSubRedisChannel = "g2cache-pubsub-channel"
var DefaultRedisConf RedisConf

func init() {
	DefaultRedisConf.DSN = "127.0.0.1:6379"
	DefaultRedisConf.MaxConn = 10
}

type RedisCache struct {
	pool     *redis.Pool
	stop     chan struct{}
	stopOnce sync.Once
}

type RedisConf struct {
	DSN     string
	Pwd     string
	DB      int
	MaxConn int
}

func NewRedisCache() *RedisCache {
	c := &RedisCache{
		pool:  GetRedisPool(&DefaultRedisConf),
		stop:  make(chan struct{}, 1),
	}
	return c
}

func (r *RedisCache) Del(key string) error {
	select {
	case <-r.stop:
		return OutStorageClose
	default:
	}
	return RedisDelKey(key, r.pool)
}

func (r *RedisCache) Close() {
	r.stopOnce.Do(r.close)
}

func (r *RedisCache) close() {
	close(r.stop)
	r.pool.Close()
}

func (r *RedisCache) Set(key string, obj *Entry) error {
	select {
	case <-r.stop:
		return OutStorageClose
	default:
	}
	str, err := jsoniter.MarshalToString(obj)
	if err != nil {
		return err
	}
	// out storage should set Expiration time
	rdsTtl := obj.GetExpireTTL()
	return RedisSetString(key, str, int(rdsTtl), r.pool)
}

func (r *RedisCache) DistributedEnable() bool {
	return true
}

func (r *RedisCache) Subscribe(ch chan *ChannelMeta) error {
	select {
	case <-r.stop:
		return OutStorageClose
	default:
	}
	conn := r.pool.Get()
	defer conn.Close()

	psc := redis.PubSubConn{Conn: conn}
	if err := psc.Subscribe(DefaultPubSubRedisChannel); err != nil {
		LogErrF("rds subscribe key=%v, err=%v\n", DefaultPubSubRedisChannel, err)
		return err
	}

LOOP:
	for {
		select {
		case <-r.stop:
			return OutStorageClose
		default:
		}
		switch v := psc.Receive().(type) {
		case redis.Message:
			meta := &ChannelMeta{}
			err := jsoniter.Unmarshal(v.Data, meta)
			if err != nil || meta.Key == "" {
				LogErrF("rds Subscribe Unmarshal data: %+v,err:%v",v.Data,err)
				continue
			}
			ch <- meta
		case error:
			LogErrF("rds receive error, msg=%v\n", v)
			break LOOP
		}
	}
	return nil
}

func (r *RedisCache) Get(key string, obj interface{}) (*Entry, bool, error) {
	select {
	case <-r.stop:
		return nil, false, OutStorageClose
	default:
	}
	str, err := RedisGetString(key, r.pool)
	if err != nil  {
		if err == redis.ErrNil {
			return nil, false, nil
		}
		return nil, false, err
	}
	if str == "" {
		return nil, false, nil
	}
	var e Entry
	e.Value = obj // Save the reflection structure of obj
	err = jsoniter.UnmarshalFromString(str, &e)
	if err != nil {
		return nil, false, err
	}

	return &e, true, err
}

func (r *RedisCache) Publish(gid, key string, action int8, value *Entry) error {
	select {
	case <-r.stop:
		return OutStorageClose
	default:
	}
	meta := ChannelMeta{
		Gid:    gid,
		Key:    key,
		Action: action,
		Data:   value,
	}
	s, err := jsoniter.MarshalToString(meta)
	if err != nil {
		return err
	}
	return RedisPublish(DefaultPubSubRedisChannel, s, r.pool)
}

func (r *RedisCache) ThreadSafe() {}
