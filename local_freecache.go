package g2cache

import (
	"github.com/coocood/freecache"
	jsoniter "github.com/json-iterator/go"
	"sync"
)

var (
	DefaultFreeCacheSize = 50 * 1024 * 1024 // 50MB
)

type FreeCache struct {
	storage  *freecache.Cache
	stop     chan struct{}
	stopOnce sync.Once
}

func NewFreeCache() *FreeCache {
	f := &FreeCache{
		storage: freecache.NewCache(DefaultFreeCacheSize),
		stop:    make(chan struct{}, 1),
	}
	return f
}

func (c *FreeCache) Set(key string, e *Entry) error {
	select {
	case <-c.stop:
		return LocalStorageClose
	default:
	}
	s, _ := jsoniter.Marshal(e)
	return c.storage.Set([]byte(key), s, DefaultGcTTL)
}

func (c *FreeCache) Del(key string) error {
	select {
	case <-c.stop:
		return LocalStorageClose
	default:
	}
	c.storage.Del([]byte(key))
	return nil
}

func (c *FreeCache) Get(key string) (*Entry, bool, error) {
	select {
	case <-c.stop:
		return nil, false, LocalStorageClose
	default:
	}
	b, err := c.storage.Get([]byte(key))
	if err != nil {
		if err == freecache.ErrNotFound {
			return nil, false, nil
		}
		return nil, false, err
	}
	e := new(Entry)
	err = jsoniter.Unmarshal(b,e)
	if err != nil {
		return nil, false, err
	}
	return e, true, nil
}

func (c *FreeCache) close() {
	close(c.stop)
	c.storage.Clear()
}

func (c *FreeCache) Close() {
	c.stopOnce.Do(c.close)
}

func (c *FreeCache) ThreadSafe() {}
