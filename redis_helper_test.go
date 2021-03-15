package g2cache

import (
	"github.com/gomodule/redigo/redis"
	"testing"
)

func TestGetRedisPool(t *testing.T) {
	DefaultRedisConf.DSN = "127.0.0.1:6379"
	DefaultRedisConf.DB = 0
	DefaultRedisConf.Pwd = "07030501310"
	DefaultRedisConf.MaxConn = 20
	pool := GetRedisPool(&DefaultRedisConf)
	_, err := pool.Get().Do("SET", "xxx", "yangq")
	if err != nil {
		t.Fatal(err)
		return
	}
	v, err := redis.String(pool.Get().Do("GET", "xxx"))
	if err != nil {
		t.Fatal(err)
		return
	}
	t.Log("v=", v)
}
