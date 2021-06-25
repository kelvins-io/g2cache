package g2cache

import (
	jsoniter "github.com/json-iterator/go"
	"time"
)

var (
	EntryLazyFactor = 32
)

type Entry struct {
	Value      interface{} `json:"value"`
	TtlSecond  int         `json:"ttl"`
	Obsolete   int64       `json:"obsolete"`
	Expiration int64       `json:"expiration"`
}

// Outdated data means that the data is still available, but not up-to-date
func (e *Entry) Obsoleted() bool {
	if e.Obsolete <= 0 {
		return true
	}
	if e.Obsolete < time.Now().Unix() {
		return true
	}
	return false
}

func (e *Entry) GetObsoleteTTL() (second int64) {
	return e.Obsolete - time.Now().Unix()
}

// Expired means that the data is unavailable and data needs to be synchronized
func (e *Entry) Expired() bool {
	if e.Expiration <= 0 {
		return true
	}
	if e.Expiration < time.Now().Unix() {
		return true
	}
	return false
}

func (e *Entry) GetExpireTTL() (second int64) {
	return e.Expiration - time.Now().Unix()
}

func (e *Entry) String() string {
	s, _ := jsoniter.MarshalToString(e.Value)
	return s
}

func NewEntry(v interface{}, second int) *Entry {
	ttl := second
	var od, e int64
	if second > 0 {
		od = time.Now().Add(time.Duration(second) * time.Second).Unix()
		e = time.Now().Add(time.Duration(second*EntryLazyFactor) * time.Second).Unix()
	}
	return &Entry{
		Value:      v,
		TtlSecond:  ttl,
		Obsolete:   od,
		Expiration: e,
	}
}
