package main

import (
	"gitee.com/kelvins-io/g2cache"
	"log"
	"math"
	"math/rand"
	"net/http"
	_ "net/http/pprof"
	"sync"
	"time"
)

func main() {
	g2cache.CacheDebug = true
	g2cache.CacheMonitor = false
	g2cache.CacheMonitorSecond = 5
	g2cache.EntryLazyFactor = 256
	g2cache.DefaultGPoolWorkerNum = 5000
	g2cache.DefaultGPoolJobQueueChanLen = 5000
	g2cache.DefaultFreeCacheSize = 100 * 1024 * 1024 // 100MB
	g2cache.DefaultPubSubRedisChannel = "g2cache-pubsub-channel"
	g2cache.DefaultRedisConf.DSN = "127.0.0.1:6379"
	g2cache.DefaultRedisConf.DB = 1
	g2cache.DefaultRedisConf.Pwd = "07030501310"
	g2cache.DefaultRedisConf.MaxConn = 30
	g2 := g2cache.New(nil, nil)
	go func() {
		http.HandleFunc("/statics", func(writer http.ResponseWriter, request *http.Request) {
			m := g2cache.HitStatisticsOut.String()
			_, _ = writer.Write([]byte(m))
		})
		err := http.ListenAndServe("0.0.0.0:6060", nil)
		if err != nil {
			log.Println("server ", err)
		}
	}()
	defer g2.Close()
	wg := &sync.WaitGroup{}
	wg.Add(20)
	for i := 0; i < 10; i++ {
		go getKey(g2, wg)
	}
	for i := 0; i < 5; i++ {
		go setKey(g2, wg)
	}
	for i := 0; i < 5; i++ {
		go delKey(g2, wg)
	}
	wg.Wait()
}

func delKey(g2 *g2cache.G2Cache, wg *sync.WaitGroup) {
	defer wg.Done()
	for i := 0; i < math.MaxInt16; i++ {
		key := g2cache.GenKey("g2cache-key", rand.Intn(math.MaxUint8))
		err := g2.Del(key, false)
		if err != nil {
			log.Println(err)
			return
		}
		time.Sleep(30 * time.Second)
	}
}

func setKey(g2 *g2cache.G2Cache, wg *sync.WaitGroup) {
	defer wg.Done()
	for i := 0; i < math.MaxInt16; i++ {
		key := g2cache.GenKey("g2cache-key", rand.Intn(math.MaxUint8))
		obj := &Object{
			ID:      i,
			Value:   key,
			Address: []string{"example setKey 未来星球🌲✨", time.Now().String()},
			Car: &Car{
				Name:  "概念🚗，✈，🚚️",
				Price: float64(i) / 100,
			},
		}
		err := g2.Set(key, obj, 30, true) // ttl is second
		if err != nil {
			log.Println(err)
			return
		}
		time.Sleep(15 * time.Second)
	}
}

func getKey(g2 *g2cache.G2Cache, wg *sync.WaitGroup) {
	defer wg.Done()
	for i := 0; i < math.MaxInt16; i++ {
		key := g2cache.GenKey("g2cache-key", rand.Intn(math.MaxUint8))
		var o Object
		// ttl is second
		err := g2.Get(key, 30, &o, func() (interface{}, error) {
			time.Sleep(50 * time.Millisecond)
			return &Object{
				ID:      i,
				Value:   key,
				Address: []string{"example getKey 未来星球🌲✨", time.Now().String()},
				Car: &Car{
					Name:  "概念🚗，✈，🚚️",
					Price: float64(i) / 100,
				},
			}, nil
		})
		if err != nil {
			log.Println(err)
			return
		}
		//out,err := jsoniter.MarshalToString(o)
		//if err != nil {
		//	log.Println(err)
		//	return
		//}
		//fmt.Printf("%s\n",out)
		time.Sleep(990 * time.Millisecond)
	}
}

type Object struct {
	ID      int      `json:"id"`
	Value   string   `json:"value"`
	Address []string `json:"address"`
	Car     *Car     `json:"car"`
}

type Car struct {
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

//func (o *Object) DeepCopy() interface{} {
//	return &(*o)
//}
