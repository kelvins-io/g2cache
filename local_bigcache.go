package g2cache

import (
	"github.com/allegro/bigcache/v3"
	jsoniter "github.com/json-iterator/go"
	"sync"
	"time"
)

// The value can be changed by external assignment
var DefaultBigCacheEntryLife = 5 * time.Second
var DefaultBigCacheConf = bigcache.DefaultConfig(DefaultBigCacheEntryLife) // 可以删除元素的时间，元素的生存时间
var DefaultBigCacheMaxSize = 50

type BigCache struct {
	storage  *bigcache.BigCache
	stop     chan struct{}
	stopOnce sync.Once
}

func initBigCacheConf() {
	if CacheDebug {
		DefaultBigCacheConf.StatsEnabled = true
		DefaultBigCacheConf.Verbose = true
	}
	DefaultBigCacheConf.CleanWindow = 2 * time.Second             // 删除过期条目之间的间隔时间
	DefaultBigCacheConf.HardMaxCacheSize = DefaultBigCacheMaxSize // 限制内存大小，单位MB
	DefaultBigCacheConf.Shards = 256                              // 哈希分配大小，必须是2的倍数
	DefaultBigCacheConf.MaxEntrySize = 500                        // 最大entry大小，单位字节，仅用于计算初始大小
}

func NewBigCache() *BigCache {

	initBigCacheConf()

	storage, err := bigcache.NewBigCache(DefaultBigCacheConf)
	if err != nil {
		panic(err)
	}
	b := BigCache{storage: storage, stop: make(chan struct{}, 1)}

	go b.startScan()

	return &b
}

func (b *BigCache) Set(key string, e *Entry) error {
	select {
	case <-b.stop:
		return LocalStorageClose
	default:
	}
	v, _ := jsoniter.Marshal(e)
	return b.storage.Set(key, v)
}

func (b *BigCache) Del(key string) error {
	select {
	case <-b.stop:
		return LocalStorageClose
	default:
	}
	return b.storage.Delete(key)
}

func (b *BigCache) setCleanInterval(t time.Duration) {
	DefaultBigCacheConf.LifeWindow = t
}

func (b *BigCache) close() {
	close(b.stop)
}

func (b *BigCache) Close() {
	b.stopOnce.Do(b.close)
}

func (b *BigCache) startScan() {
	b.stop <- struct{}{}
}

func (b *BigCache) stopScan() {
	<-b.stop
}

func (b *BigCache) Get(key string) (*Entry, bool, error) {
	select {
	case <-b.stop:
		return nil, false, LocalStorageClose
	default:
	}
	str, err := b.storage.Get(key)
	if err != nil {
		if err == bigcache.ErrEntryNotFound {
			return nil, false, nil
		}
		return nil, false, err
	}
	e := new(Entry)
	err = jsoniter.Unmarshal(str, e)
	if err != nil {
		return nil, false, err
	}
	if e.Expired() {
		return nil, false, nil
	}
	return e, true, nil
}

func (b *BigCache) ThreadSafe() {}
