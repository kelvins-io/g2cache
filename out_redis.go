package g2cache

import (
	"github.com/gomodule/redigo/redis"
	jsoniter "github.com/json-iterator/go"
	"log"
	"sync"
	"time"
)

var DefaultPubSubRedisChannel = "g2cache-pubsub-channel"
var DefaultRedisConf RedisConf

type RedisCache struct {
	pool     *redis.Pool
	local    LocalCache
	stop     chan struct{}
	stopOnce sync.Once
}

type RedisConf struct {
	DSN     string
	Pwd     string
	DB      int
	MaxConn int
}

func NewRedisCache(local LocalCache) *RedisCache {
	c := &RedisCache{
		pool:  GetRedisPool(&DefaultRedisConf),
		local: local,
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
	rdsTtl := (obj.Expiration - time.Now().UnixNano()) / int64(time.Second)
	return RedisSetString(key, str, int(rdsTtl), r.pool)
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
		log.Printf("rds subscribe key=%v, err=%v\n", DefaultPubSubRedisChannel, err)
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
			err := jsoniter.Unmarshal(v.Data,meta)
			if err != nil || meta.Key == "" {
				continue
			}
			ch <- meta
		case error:
			log.Printf("rds receive err, msg=%v\n", v)
			break LOOP
		}
	}
	close(ch)
	return nil
}

func (r *RedisCache) Get(key string, obj interface{}) (*Entry, bool, error) {
	select {
	case <-r.stop:
		return nil, false, OutStorageClose
	default:
	}
	str, err := RedisGetString(key, r.pool)
	if err != nil && err != redis.ErrNil {
		return nil, false, nil
	}
	if str == "" {
		return nil, false, nil
	}
	var e Entry
	e.Value = obj // Save the reflection structure of obj
	if jsoniter.UnmarshalFromString(str, &e) != nil {
		return nil, false, nil
	}

	return &e, true, err
}

func (r *RedisCache) Publish(key string, action int8, value *Entry) error {
	select {
	case <-r.stop:
		return OutStorageClose
	default:
	}
	meta := ChannelMeta{
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
