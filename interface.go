package g2cache

import "time"

type StoreInterface interface {
	Get(key string) (value string,err error)
	Put(key string,value string,ttl time.Duration) error
	Del(key string) error
}
