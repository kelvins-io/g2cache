package main

import (
	"gitee.com/kelvins-io/g2cache"
	"log"
	"math"
	"math/rand"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

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

func (o *Object) DeepCopy() interface{} {
	return &(*o)
}

var (
	channel = make(chan os.Signal, 2)
	sleep   = 800 * time.Millisecond
)

func main() {
	g2cache.CacheDebug = true
	g2cache.DefaultPubSubRedisChannel = "g2cache-pubsub-channel"
	g2cache.DefaultRedisConf.DSN = "127.0.0.1:6379"
	g2cache.DefaultRedisConf.DB = 1
	g2cache.DefaultRedisConf.Pwd = "07030501310"
	g2cache.DefaultRedisConf.MaxConn = 30
	g2 := g2cache.New(nil, nil)
	signal.Notify(channel, syscall.SIGUSR1, syscall.SIGUSR2)
	go func() {
		err := http.ListenAndServe("0.0.0.0:6060", nil)
		if err != nil{
			log.Fatalln(err)
		}
	}()
	defer g2.Close()
	wg := &sync.WaitGroup{}
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go example(g2, wg)
	}
	wg.Wait()
}

func example(g2 *g2cache.G2Cache, wg *sync.WaitGroup) {
	defer wg.Done()
	for i := 0; i < math.MaxInt64; i++ {
		select {
		case v := <-channel:
			switch v {
			case syscall.SIGUSR1:
				sleep = 3 * time.Second
			case syscall.SIGUSR2:
				sleep = 2 * time.Millisecond
			default:
			}
		default:
		}

		key := g2cache.GenKey("g2cache-key", rand.Intn(math.MaxUint8))
		var o Object
		err := g2.Get(key, 60, &o, func() (interface{}, error) {
			time.Sleep(1 * time.Second)
			return &Object{
				ID:      i,
				Value:   "😄",
				Address: []string{"example 未来星球🌲✨",time.Now().String()},
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
		//fmt.Println(o.Car.Name)
		//s, _ := jsoniter.MarshalToString(o)
		//fmt.Println(s)
		time.Sleep(sleep)
	}
}
