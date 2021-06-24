package g2cache

import (
	"github.com/gomodule/redigo/redis"
	"testing"
)

func TestGetRedisPool(t *testing.T) {
	DefaultRedisConf.DSN = "127.0.0.1:6379"
	DefaultRedisConf.DB = 0
	DefaultRedisConf.Pwd = ""
	DefaultRedisConf.MaxConn = 3
	pool, err := GetRedisPool(&DefaultRedisConf)
	if err != nil {
		t.Fatal(err)
		return
	}
	DefaultPubSubRedisConf = DefaultRedisConf
	pubsubPool, err := GetRedisPool(&DefaultPubSubRedisConf)
	if err != nil {
		t.Fatal(err)
		return
	}
	_, err = pool.Get().Do("SET", "surprise", "g2cache")
	if err != nil {
		t.Fatal(err)
		return
	}
	v, err := redis.String(pool.Get().Do("GET", "surprise"))
	if err != nil {
		t.Fatal(err)
		return
	}
	t.Log("GET surprise=", v)
	err = RedisPublish(DefaultPubSubRedisChannel, "set surprise g2cache", pubsubPool)
	if err != nil {
		t.Fatal(err)
		return
	}
	t.Logf("channel %s publish ok", DefaultPubSubRedisChannel)
}
