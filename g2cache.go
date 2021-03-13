package g2cache

import (
	"github.com/gomodule/redigo/redis"
	"github.com/json-iterator/go"
	"github.com/mohae/deepcopy"
	"log"
	"time"
)

const (
	lazyFactor    = 256
	delKeyChannel = "g2cache-delete"
	gcTtl         = 5 * time.Second
)

type G2Cache struct {
	pool  *redis.Pool
	rds   *RedisCache
	mem   *MemCache
	debug bool
}

type LoadFunc func() (interface{}, error)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

func New(redisDsn, redisPwd string, db,maxConn int) *G2Cache {
	g := &G2Cache{}
	g.mem = NewMemCache(gcTtl)
	g.pool = GetRedisPool(redisDsn, redisPwd, db,maxConn)
	g.rds = NewRedisCache(g.pool, g.mem)

	go g.subscribe(delKeyChannel)

	return g
}

func (g *G2Cache) Get(key string, obj interface{}, ttl int, fn LoadFunc) error {
	if g.debug {
		ttl = 1
	}
	return g.getObjectWithExprie(key, obj, ttl, fn)
}

func (g *G2Cache) getObjectWithExprie(key string, obj interface{}, ttl int, fn LoadFunc) error {
	v, ok := g.mem.Get(key)
	if ok {
		if v.OutDated() {
			c := deepcopy.Copy(obj)
			go func() {
				err := g.syncMemCache(key, c, ttl, fn)
				if err != nil {
					log.Printf("syncMemCache key=%v,err=%v",key,err)
				}
			}()
		}
		return clone(v.Value, obj)
	}

	v, ok = g.rds.Get(key, obj)
	if ok {
		if v.OutDated() {
			go func() {
				err:=  g.rds.load(key, nil, ttl, fn, false)
				if err != nil {
					log.Printf("rds load key=%v,err=%v",key,err)
				}
			}()
		}
		return clone(v.Value, obj)
	}
	return g.rds.load(key, obj, ttl, fn, true)
}

func (g *G2Cache) syncMemCache(key string, copy interface{}, ttl int, fn LoadFunc) error {
	e, ok := g.rds.Get(key, copy)
	if !ok || e.OutDated() {
		if err := g.rds.load(key, nil, ttl, fn, false); err != nil {
			return err
		}
	}
	g.mem.Set(key, e)
	return nil
}

func (g *G2Cache) EnableDebug() {
	g.debug = true
	g.mem.StopScan()
	g.mem.SetCleanInterval(2 * time.Second)
	g.mem.StopScan()
}

func (g *G2Cache) Del(key string) error {
	return RedisPublish(delKeyChannel, key, g.pool)
}

func (g *G2Cache) delete(key string) error {
	g.mem.Del(key)
	return g.rds.Del(key)
}

func (g *G2Cache) subscribe(key string) {
	conn := g.pool.Get()
	defer conn.Close()

	psc := redis.PubSubConn{Conn: conn}
	if err := psc.Subscribe(key); err != nil {
		log.Printf("rds subscribe key=%v, err=%v\n", key, err)
		return
	}

	for {
		switch v := psc.Receive().(type) {
		case redis.Message:
			key := string(v.Data)
			if err := g.delete(key); err != nil {
				log.Printf("rds del key=%v, err=%v\n", key, err)
			}
		case error:
			log.Printf("rds receive err, msg=%v\n",v)
			continue
		}
	}
}
