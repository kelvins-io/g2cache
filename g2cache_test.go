package g2cache

import (
	"testing"
)

type Object struct {
	Value string
}

func (o *Object) DeepCopy() interface{} {
	return &(*o)
}

func BenchmarkG2Cache_Get(b *testing.B) {
	g2 := New("127.0.0.1:6379", "07030501310", 0, 10)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		key := GenKey("g2cache-key", i)
		var o Object
		err := g2.Get(key, &o, 2, func() (interface{}, error) {
			return &Object{Value: "ssfadf"}, nil
		})
		if err != nil {
			b.Fatal(err)
			return
		}
	}
}
