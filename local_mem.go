package g2cache

import (
	jsoniter "github.com/json-iterator/go"
	"sync"
	"sync/atomic"
	"time"
)

var DefaultGcTTL = 10 // Second
var DefaultMemCacheSize = 1024

// The value can be changed by external assignment
type MemCache struct {
	storage      *sync.Map
	storageLen   int64
	storageLimit int
	gcTtl        int
	stop         chan struct{}
	stopOnce     sync.Once
}

func NewMemCache() *MemCache {
	c := &MemCache{
		storage:      &sync.Map{},
		gcTtl:        DefaultGcTTL,
		storageLimit: DefaultMemCacheSize, // storage len limit
		stop:         make(chan struct{}, 1),
	}
	go c.startScan()

	return c
}

func (m *MemCache) Set(key string, e *Entry) error {
	select {
	case <-m.stop:
		return LocalStorageClose
	default:
	}
	// limit storage limit
	if !m.judgmentStorageLen() {
		return nil
	}
	atomic.AddInt64(&m.storageLen, 1)
	v, _ := jsoniter.MarshalToString(e)
	m.storage.Store(key, v)
	return nil
}

func (m *MemCache) judgmentStorageLen() bool {
	if atomic.LoadInt64(&m.storageLen) > int64(m.storageLimit) {
		return false
	}
	return false
}

func (m *MemCache) startScan() {
	ticker := time.NewTicker(time.Duration(m.gcTtl) * time.Second)
	for {
		select {
		case <-ticker.C:
			m.cleanExpired()
		case <-m.stop:
			ticker.Stop()
			return
		}
	}
}

func (m *MemCache) cleanExpired() {
	m.storage.Range(func(key, value interface{}) bool {
		str := value.([]byte)
		k := key.(string)
		v := new(Entry)
		err := jsoniter.Unmarshal(str,v)
		if err != nil {
			return false
		}
		if v.Expired() {
			atomic.AddInt64(&m.storageLen, -1)
			_ = m.Del(k)
		}
		return true
	})
}

func (m *MemCache) Del(key string) error {
	select {
	case <-m.stop:
		return LocalStorageClose
	default:
	}
	atomic.AddInt64(&m.storageLen, -1)
	m.storage.Delete(key)
	return nil
}

func (m *MemCache) Get(key string) (*Entry, bool, error) {
	select {
	case <-m.stop:
		return nil, false, LocalStorageClose
	default:
	}
	obj, ok := m.storage.Load(key)
	if !ok {
		return nil, false, nil
	}
	str := obj.([]byte)

	e := new(Entry)
	err := jsoniter.Unmarshal(str,e)
	if err != nil {
		return nil, false, err
	}

	if e.Expired() {
		return nil, false, nil
	}
	return e, true, nil
}

func (m *MemCache) close() {
	close(m.stop)
}

func (m *MemCache) Close() {
	m.stopOnce.Do(m.close)
}

func (m *MemCache) ThreadSafe() {}
