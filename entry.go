package g2cache

import (
	"time"
)

const (
	lazyFactor = 256
)

type Entry struct {
	Value      interface{} `json:"value"`
	Ttl        int         `json:"ttl"`
	Obsolete   int64       `json:"obsolete"`
	Expiration int64       `json:"expiration"`
}
// Outdated data means that the data is still available, but not up-to-date
func (e *Entry) Obsoleted() bool {
	if e.Obsolete == 0 {
		return false
	}
	if e.Obsolete < time.Now().UnixNano() {
		return true
	}
	return false
}
// Expired means that the data is unavailable and data needs to be synchronized
func (e *Entry) Expired() bool {
	if e.Expiration > 0 {
		if time.Now().UnixNano() > e.Expiration {
			return false
		}
	}
	return true
}

func NewEntry(v interface{}, d int) *Entry {
	ttl := d
	var od, e int64
	if d > 0 {
		od = time.Now().Add(time.Duration(d) * time.Second).UnixNano()
		e = time.Now().Add(time.Duration(d*lazyFactor) * time.Second).UnixNano()
	}
	return &Entry{
		Value:      v,
		Ttl:        ttl,
		Obsolete:   od,
		Expiration: e,
	}
}
