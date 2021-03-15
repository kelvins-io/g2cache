package g2cache

import (
	jsoniter "github.com/json-iterator/go"
	"testing"
	"time"
)

type Object struct {
	ID      int      `json:"id"`
	Value   string   `json:"value"`
	Address []string `json:"address"`
	Car     *Car     `json:"car"`
}

type Car struct {
	Name  string    `json:"name"`
	Price float64   `json:"price"`
}

//func (o *Object) DeepCopy() interface{} {
//	return &(*o)
//}

func TestG2Cache_Get(t *testing.T) {
	DefaultRedisConf.DSN = "127.0.0.1:6379"
	DefaultRedisConf.DB = 0
	DefaultRedisConf.Pwd = "07030501310"
	DefaultRedisConf.MaxConn = 20
	g2 := New(nil, nil)
	defer g2.Close()
	key := GenKey("g2cache-key", 1)
	var o Object
	err := g2.Get(key, 5, &o, func() (interface{}, error) {
		time.Sleep(1 * time.Second)
		return &Object{
			ID:      1,
			Value:   "😄",
			Address: []string{"test get 未来星球🌲✨", time.Now().String()},
			Car: &Car{
				Name:  "概念🚗，✈，🚚️",
				Price: float64(1) / 100,
			},
		}, nil
	})
	if err != nil {
		t.Fatal(err)
		return
	}
	t.Log(o.Car.Price)
	t.Log(o.Car.Name)
	str, _ := jsoniter.MarshalToString(o)
	t.Log(str)
	time.Sleep(2*time.Second)
}

func TestG2Cache_Set(t *testing.T) {
	DefaultRedisConf.DSN = "127.0.0.1:6379"
	DefaultRedisConf.DB = 0
	DefaultRedisConf.Pwd = "07030501310"
	DefaultRedisConf.MaxConn = 20
	g2 := New(nil, nil)
	defer g2.Close()
	key := GenKey("g2cache-key", 1)
	var o = Object{
		ID:      1,
		Value:   "😄",
		Address: []string{"test set 未来星球🌲✨", time.Now().String()},
		Car: &Car{
			Name:  "概念🚗，✈，🚚️",
			Price: float64(1) / 100,
		},
	}
	err := g2.Set(key, &o, 5, true)
	if err != nil {
		t.Fatal(err)
		return
	}
}

func TestG2Cache_Del(t *testing.T) {
	DefaultRedisConf.DSN = "127.0.0.1:6379"
	DefaultRedisConf.DB = 0
	DefaultRedisConf.Pwd = "07030501310"
	DefaultRedisConf.MaxConn = 20
	g2 := New(nil, nil)
	defer g2.Close()
	key := GenKey("g2cache-key", 1)
	err := g2.Del(key, true)
	if err != nil {
		t.Fatal(err)
		return
	}
}

func BenchmarkG2Cache_Get(b *testing.B) {
	DefaultRedisConf.DSN = "127.0.0.1:6379"
	DefaultRedisConf.DB = 0
	DefaultRedisConf.Pwd = "07030501310"
	DefaultRedisConf.MaxConn = 20
	g2 := New(nil, nil)
	defer g2.Close()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		key := GenKey("g2cache-key", i)
		var o Object
		err := g2.Get(key, 5, &o, func() (interface{}, error) {
			time.Sleep(1 * time.Second)
			return &Object{
				ID:      i,
				Value:   "😄",
				Address: []string{"test get 未来星球🌲✨", time.Now().String()},
				Car: &Car{
					Name:  "概念🚗，✈，🚚️",
					Price: float64(i) / 100,
				},
			}, nil
		})
		if err != nil {
			b.Fatal(err)
		}
		b.Log(o)
	}
}

func BenchmarkG2Cache_Set(b *testing.B) {
	DefaultRedisConf.DSN = "127.0.0.1:6379"
	DefaultRedisConf.DB = 0
	DefaultRedisConf.Pwd = "07030501310"
	DefaultRedisConf.MaxConn = 20
	g2 := New(nil, nil)
	defer g2.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := GenKey("g2cache-key", i)
		var o = Object{
			ID:      i,
			Value:   "😄",
			Address: []string{"test set 未来星球🌲✨", time.Now().String()},
			Car: &Car{
				Name:  "概念🚗，✈，🚚️",
				Price: float64(i) / 100,
			},
		}
		err := g2.Set(key, &o, 5, false)
		if err != nil {
			b.Fatal(err)
			return
		}
	}
	b.ReportAllocs()
}

func BenchmarkG2Cache_Del(b *testing.B) {
	DefaultRedisConf.DSN = "127.0.0.1:6379"
	DefaultRedisConf.DB = 0
	DefaultRedisConf.Pwd = "07030501310"
	DefaultRedisConf.MaxConn = 20
	g2 := New(nil, nil)
	defer g2.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := GenKey("g2cache-key", i)
		err := g2.Del(key, true)
		if err != nil {
			b.Fatal(err)
			return
		}
	}
	b.ReportAllocs()
}
