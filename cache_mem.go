package g2cache

import (
	"sync"
	"time"
)

type MemCache struct {
	storage sync.Map
	gcTtl   time.Duration
	stop    chan struct{}
}

func NewMemCache(gcTtl time.Duration) *MemCache {
	c := &MemCache{
		storage: sync.Map{},
		gcTtl:   gcTtl,
		stop:    make(chan struct{}),
	}
	go c.StartScan()
	return c
}

func (m *MemCache) Set(key string, e *Entry) {
	m.storage.Store(key, e)
}

func (m *MemCache) StartScan() {
	ticker := time.NewTicker(m.gcTtl)
	for {
		select {
		case <-ticker.C:
			m.CleanExpired()
		case <-m.stop:
			ticker.Stop()
			return
		}
	}
}

func (m *MemCache) CleanExpired() {
	m.storage.Range(func(key, value interface{}) bool {
		v := value.(*Entry)
		k := key.(string)
		if v.OutDated() {
			m.Del(k)
		}
		return true
	})
}

func (m *MemCache) Del(key string) {
	m.storage.Delete(key)
}

func (m *MemCache) Get(key string) (*Entry, bool) {
	obj, ok := m.storage.Load(key)
	if !ok {
		return nil, false
	}
	e := obj.(*Entry)
	if e.Expired() {
		return nil, false
	}
	return e, true
}

func (m *MemCache) Load(key string) (*Entry, bool) {
	e, ok := m.Get(key)
	if !ok {
		return nil, false
	}
	return e, !e.OutDated()
}

func (m *MemCache) StopScan() {
	m.stop <- struct{}{}
}

func (m *MemCache) SetCleanInterval(t time.Duration) {
	m.gcTtl = t
}
