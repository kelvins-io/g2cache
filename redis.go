package g2cache

import (
	"github.com/gomodule/redigo/redis"
	"time"
)

func RedisPublish(channel, message string, pool *redis.Pool) error {
	conn, err := getRedisConn(pool)
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = conn.Do("PUBLISH", channel, message)
	return err
}

func RedisSetString(key, value string, ttl int, pool *redis.Pool) error {
	conn, err := getRedisConn(pool)
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = conn.Do("SETEX", key, ttl, value)
	return err
}

func RedisGetString(key string, pool *redis.Pool) (string, error) {
	conn, err := getRedisConn(pool)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	v, err := redis.String(conn.Do("GET", key))
	if err != nil {
		return "", err
	}
	return v, nil
}

func RedisDelKey(key string, pool *redis.Pool) error {
	conn, err := getRedisConn(pool)
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = conn.Do("DEL", key)
	return err
}

func getRedisConn(pool *redis.Pool) (redis.Conn, error) {
	conn := pool.Get()
	if err := conn.Err(); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

func GetRedisPool(dsn, pwd string, db, maxConn int) *redis.Pool {
	pool := &redis.Pool{
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", dsn)
			if err != nil {
				return nil, err
			}
			if pwd != "" {
				if _, err := c.Do("AUTH", pwd); err != nil {
					err = c.Close()
					return nil, err
				}
			}
			if db > 0 {
				if _, err := c.Do("SELECT", db); err != nil {
					err = c.Close()
					return nil, err
				}
			}
			return c, err
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
		MaxIdle:         maxConn,
		MaxActive:       maxConn,
		IdleTimeout:     300 * time.Second,
		Wait:            true,
		MaxConnLifetime: 30 * time.Minute,
	}
	return pool
}
