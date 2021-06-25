package g2cache

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/mohae/deepcopy"
	"reflect"
	"strconv"
	"sync/atomic"
)

var (
	CacheKeyEmpty           = errors.New("cache key is empty")
	CacheObjNil             = errors.New("cache object is nil")
	LoadDataSourceFuncNil   = errors.New("cache load func is nil")
	LocalStorageClose       = errors.New("local storage close !!! ")
	OutStorageClose         = errors.New("out storage close !!! ")
	CacheClose              = errors.New("g2cache close !!! ")
	DataSourceLoadNil       = errors.New("data source load nil")
	OutStorageLoadNil       = errors.New("out storage load nil")
	CacheNotImplementPubSub = errors.New("cache not implement pubsub interface")
)

func clone(src, dst interface{}) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = errors.New(fmt.Sprint(e))
			return
		}
	}()

	v := deepcopy.Copy(src)
	if reflect.ValueOf(v).IsValid() {
		reflect.ValueOf(dst).Elem().Set(reflect.Indirect(reflect.ValueOf(v)))
	}
	return err
}

func GenKey(args ...interface{}) string {
	var buf bytes.Buffer
	for i := range args {
		addBuf(args[i], &buf)
		if i < len(args)-1 {
			addBuf(":", &buf)
		}
	}
	return buf.String()
}

func addBuf(i interface{}, buf *bytes.Buffer) {
	switch v := i.(type) {
	case int:
		buf.WriteString(strconv.Itoa(v))
	case int8:
		buf.WriteString(strconv.Itoa(int(v)))
	case int16:
		buf.WriteString(strconv.Itoa(int(v)))
	case int32:
		buf.WriteString(strconv.Itoa(int(v)))
	case int64:
		buf.WriteString(strconv.Itoa(int(v)))
	case uint8:
		buf.WriteString(strconv.Itoa(int(v)))
	case uint16:
		buf.WriteString(strconv.Itoa(int(v)))
	case uint32:
		buf.WriteString(strconv.Itoa(int(v)))
	case uint64:
		buf.WriteString(strconv.Itoa(int(v)))
	case string:
		buf.WriteString(v)
	}
}

func wrapFuncErr(f func() error) func() {
	return func() {
		err := f()
		if err != nil {
			LogErr(err)
		}
	}
}

type HitStatistics struct {
	HitOutStorageTotalRate   float64 `json:"hit_out_storage_total_rate"`
	HitDataSourceTotalRate   float64 `json:"hit_data_source_total_rate"`
	HitLocalStorageTotalRate float64 `json:"hit_local_storage_total_rate"`
	HitDataSourceTotal       int64   `json:"hit_data_source_total"`
	HitLocalStorageTotal     int64   `json:"hit_local_storage_total"`
	HitOutStorageTotal       int64   `json:"hit_out_storage_total"`
	AccessGetTotal           int64   `json:"access_get_total"`
}

func (h *HitStatistics) String() string {
	v, _ := json.MarshalToString(h)
	return v
}

func (h *HitStatistics) Calculation() {
	h.StatisticsDataSource()
	h.StatisticsOutStorage()
	h.StatisticsLocalStorage()
}

func (h *HitStatistics) StatisticsDataSource() {
	h.HitDataSourceTotalRate = float64(atomic.LoadInt64(&h.HitDataSourceTotal)) / float64(atomic.LoadInt64(&h.AccessGetTotal))
}

func (h *HitStatistics) StatisticsOutStorage() {
	h.HitOutStorageTotalRate = float64(atomic.LoadInt64(&h.HitOutStorageTotal)) / float64(atomic.LoadInt64(&h.AccessGetTotal))
}

func (h *HitStatistics) StatisticsLocalStorage() {
	h.HitLocalStorageTotalRate = float64(atomic.LoadInt64(&h.HitLocalStorageTotal)) / float64(atomic.LoadInt64(&h.AccessGetTotal))
}

func NewUUID() (string, error) {
	return uuid.New().String(), nil
}
