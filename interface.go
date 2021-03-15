package g2cache

// Local memory cache，Local memory cache with high access speed
type LocalCache interface {
	Get(key string, obj interface{}) (*Entry, bool, error)
	Set(key string, e *Entry) error
	Del(key string) error
	ThreadSafe() // Need to ensure thread safety
	Close()
}

// External cache has faster access speed, such as Redis
type OutCache interface {
	Get(key string, obj interface{}) (*Entry, bool, error)
	Set(key string, e *Entry) error
	Subscribe(data chan *ChannelMeta) error
	Publish(key string, action int8, data *Entry) error
	Del(key string) error
	Close()
	ThreadSafe() // Need to ensure thread safety
}

const (
	SetPublishType int8 = iota
	DelPublishType
)

type ChannelMeta struct {
	Key    string `json:"key"`
	Action int8   `json:"action"`
	Data   *Entry `json:"data"`
}
