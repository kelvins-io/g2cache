package g2cache

// Local memory cache，Local memory cache with high access speed
type LocalCache interface {
	Get(key string, obj interface{}) (*Entry, bool, error) // obj represents the internal structure of the real object
	Set(key string, e *Entry) error                        // local storage should set Entry.Obsolete
	Del(key string) error
	ThreadSafe() // Need to ensure thread safety
	Close()
}

// External cache has faster access speed, such as Redis
type OutCache interface {
	Get(key string, obj interface{}) (*Entry, bool, error) // obj represents the internal structure of the real object
	Set(key string, e *Entry) error                        // out storage should set Entry.Expiration
	Del(key string) error
	ThreadSafe() // Need to ensure thread safety
	Close()
}

// only out storage pub sub
type PubSub interface {
	Subscribe(data chan *ChannelMeta) error
	Publish(gid, key string, action int8, data *Entry) error
}

// Shouldn't throw a panic, please return an error
type LoadDataSourceFunc func() (interface{}, error)

// Entry expire is UnixNano
const (
	SetPublishType int8 = iota
	DelPublishType
)

type ChannelMeta struct {
	Key    string `json:"key"` // cache key
	Gid    string `json:"gid"` // Used to identify working groups
	Action int8   `json:"action"` // SetPublishType,DelPublishType
	Data   *Entry `json:"data"`
}
