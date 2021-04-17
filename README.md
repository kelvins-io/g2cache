# g2cache

[![MinIO](logo.png)](https://gitee.com/kelvins-io)
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
PubSub | 外部缓存发布订阅 | 外部缓存可以自选实现 | 未实现就不支持分布式

local 本地内存高速缓存接口
```go
// Local memory cache，Local memory cache with high access speed
type LocalCache interface {
	Get(key string, obj interface{}) (*Entry, bool, error) // obj represents the internal structure of the real object
	Set(key string, e *Entry) error                        // local storage should set Entry.Obsolete
	Del(key string) error
	ThreadSafe() // Need to ensure thread safety
	Close()
}
```

out 外部缓存接口，值得一提的是外部缓存需要实现发布订阅接口，这是因为g2cache支持分布式多级缓存
```go
// External cache has faster access speed, such as Redis
type OutCache interface {
	Get(key string, obj interface{}) (*Entry, bool, error) // obj represents the internal structure of the real object
	Set(key string, e *Entry) error                        // out storage should set Entry.Expiration
	Subscribe(data chan *ChannelMeta) error
	Publish(gid ,key string, action int8, data *Entry) error
	Del(key string) error
	ThreadSafe() // Need to ensure thread safety
	Close()
}
```

外部缓存发布订阅
```go
// only out storage pub sub
type PubSub interface {
	Subscribe(data chan *ChannelMeta) error
	Publish(gid, key string, action int8, data *Entry) error
}
```


LoadDataSourceFunc 原始数据加载函数，需要自行处理panic并以error形式返回    
加载函数支持返回string，map，slice，struct，ptr类型   
```go
// Shouldn't throw a panic, please return an error
type LoadDataSourceFunc func() (interface{}, error)

```


Usage   
go get gitee.com/kelvins-io/g2cache   
cd example   
go run main.go

#### 分支说明

1. copyobj分支支持返回特定object，为不稳定版本
2.  dev分支Get返回object序列化值，过渡版
3.  master分支只提供sync.Map的local实现，早期版
4.  release分支提供发布版，与copyobj有较大变化

#### 参与贡献

1.  Fork 本仓库
2.  新建 Feat_xxx 分支
3.  提交代码
4.  新建 Pull Request

#### 合作交流
1225807605@qq.com