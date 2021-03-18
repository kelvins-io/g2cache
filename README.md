# g2cache

#### 介绍
分布式多级缓存方案g2cache

#### 软件架构
软件架构说明   
主要内容：   

模块 | 功能 |  特点 | 注意  
---|------|------|---
local interface | 本机内存高速缓存 | 纳秒，毫秒级别响应速度；有效期较短 | 实现LocalCache接口，注意控制内存
out interface |外部高速缓存，如Redis | 较快的访问速度，容量很足 | 实现OutCache接口
LoadDataSourceFunc | 外部数据源 | 外部加载函数，灵活，并发保护 | 需要自行处理panic

local 本地内存高速缓存接口
```go
// Local memory cache，Local memory cache with high access speed
type LocalCache interface {
	Get(key string, obj interface{}) (*Entry, bool, error) // obj代表LoadDataSourceFunc返回的object类型
	Set(key string, e *Entry) error
	Del(key string) error
	ThreadSafe() // 接口实例对外方法都是线程安全
	Close()
}
```

out 外部缓存接口，值得一提的是外部缓存需要实现发布订阅接口，这是因为g2cache支持分布式多级缓存
```go
// External cache has faster access speed, such as Redis
type OutCache interface {
	Get(key string, obj interface{}) (*Entry, bool, error)
	Set(key string, e *Entry) error
	Subscribe(data chan *ChannelMeta) error
	Publish(key string, action int8, data *Entry) error
	Del(key string) error
	Close()
	ThreadSafe() // 接口实例对外方法都是线程安全
}
```

Usage   
go get gitee.com/kelvins-io/g2cache
```go
func TestG2Cache_Get(t *testing.T) {
	DefaultRedisConf.DSN = "127.0.0.1:6379"
	DefaultRedisConf.DB = 0
	DefaultRedisConf.Pwd = "xxx"
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
			Address: []string{"g2cache 基地🌲✨", time.Now().String()},
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
```

#### 分支说明

1. copyobj分支支持返回特定object，为最新版
2.  dev分支Get返回object序列化值，过渡版
3.  master分支只提供sync.Map的local实现，早期版

#### 参与贡献

1.  Fork 本仓库
2.  新建 Feat_xxx 分支
3.  提交代码
4.  新建 Pull Request


#### 特技

1.  使用 Readme\_XXX.md 来支持不同的语言，例如 Readme\_en.md, Readme\_zh.md
2.  Gitee 官方博客 [blog.gitee.com](https://blog.gitee.com)
3.  你可以 [https://gitee.com/explore](https://gitee.com/explore) 这个地址来了解 Gitee 上的优秀开源项目
4.  [GVP](https://gitee.com/gvp) 全称是 Gitee 最有价值开源项目，是综合评定出的优秀开源项目
5.  Gitee 官方提供的使用手册 [https://gitee.com/help](https://gitee.com/help)
6.  Gitee 封面人物是一档用来展示 Gitee 会员风采的栏目 [https://gitee.com/gitee-stars/](https://gitee.com/gitee-stars/)
