package main

import (
	"fmt"
	"gitee.com/kelvins-io/g2cache"
	"log"
	"math"
	"math/rand"
	"net/http"
	_ "net/http/pprof"
	"sync"
	"time"
)

var (
	testTotal = math.MaxInt16
)

var appName = "example-"

func main() {
	g2cache.CacheDebug = true
	g2cache.CacheMonitor = true
	g2cache.CacheMonitorSecond = 10
	g2cache.OutCachePubSub = true
	g2cache.EntryLazyFactor = 18
	g2cache.DefaultGPoolWorkerNum = 20
	g2cache.DefaultGPoolJobQueueChanLen = 300
	g2cache.DefaultFreeCacheSize = 100 * 1024 * 1024 // 100MB
	g2cache.DefaultPubSubRedisChannel = "g2cache-pubsub-channel"
	g2cache.DefaultRedisConf.DSN = "127.0.0.1:6379"
	g2cache.DefaultRedisConf.DB = 1
	g2cache.DefaultRedisConf.Pwd = ""
	g2cache.DefaultRedisConf.MaxConn = 30
	g2cache.DefaultPubSubRedisConf = g2cache.DefaultRedisConf
	g2,err := g2cache.New(nil, nil)
	if err != nil {
		log.Println("g2cache New",err)
		return
	}
	go func() {
		http.HandleFunc("/statics", func(writer http.ResponseWriter, request *http.Request) {
			m := g2cache.HitStatisticsOut.String()
			_, _ = writer.Write([]byte(m))
		})
		port := 6000+rand.Intn(1000)
		addr := fmt.Sprintf("0.0.0.0:%d",port)
		log.Println("g2cache-example run at",addr)
		err := http.ListenAndServe(addr, nil)
		if err != nil {
			log.Println("server ", err)
		}
	}()

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
	g2.Close()
}

func delKey(g2 *g2cache.G2Cache, wg *sync.WaitGroup) {
	defer wg.Done()
	for i := 0; i < testTotal; i++ {
		key := g2cache.GenKey(appName, rand.Intn(math.MaxInt8))
		err := g2.Del(key, false)
		if err != nil {
			log.Fatalln(err)
		}
		time.Sleep(5 * time.Second)
	}
}

func setKey(g2 *g2cache.G2Cache, wg *sync.WaitGroup) {
	defer wg.Done()
	for i := 0; i < testTotal; i++ {
		key := g2cache.GenKey(appName, rand.Intn(math.MaxInt8))
		obj := &Object{
			ID:      i,
			Value:   key,
			Address: []string{"example setKey 未来星球🌲✨", appName},
			Car: &Car{
				Name:  "概念🚗，✈，🚚️",
				Price: float64(i) / 100,
			},
		}
		err := g2.Set(key, obj, 30, true) // ttl is second
		if err != nil {
			log.Fatalln(err)
		}
		time.Sleep(5 * time.Second)
	}
}

func getKey(g2 *g2cache.G2Cache, wg *sync.WaitGroup) {
	defer wg.Done()
	for i := 0; i < testTotal; i++ {
		key := g2cache.GenKey(appName, rand.Intn(math.MaxInt8))
		var o Object
		// ttl is second
		err := g2.Get(key, 30, &o, func() (interface{}, error) {
			time.Sleep(50 * time.Millisecond)
			return &Object{
				ID:      i,
				Value:   key,
				Address: []string{"example getKey 未来星球🌲✨", appName},
				Car: &Car{
					Name:  "概念🚗，✈，🚚️",
					Price: float64(i) / 100,
				},
			}, nil
		})
		if err != nil {
			log.Fatalln(err)
		}
		//out,err := jsoniter.MarshalToString(o)
		//if err != nil {
		//	log.Println(err)
		//	return
		//}
		//fmt.Printf("%s\n",out)
		time.Sleep(3 * time.Second)
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

func init()  {
	rand.Seed(time.Now().UnixNano())
	appName += fmt.Sprintf("%d",rand.Intn(100))
}