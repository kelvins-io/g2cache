package g2cache

import (
	"bytes"
	"encoding/gob"
	"testing"
	"time"
)

type Object struct {
	Value string `json:"value"`
	Foo   *Foo   `json:"foo"`
}

type Foo struct {
	Value   string `json:"value"`
	Version int    `json:"version"`
}

//func (o *Object) DeepCopy() interface{} {
//	return &(*o)
//}

func BenchmarkG2Cache_Get(b *testing.B) {
	g2 := New("127.0.0.1:6379", "07030501310", 0, 10)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		key := GenKey("g2cache-key", i)
		var o Object
		err := g2.Get(key, &o, 5, func() (interface{}, error) {
			return &Object{
				Value: "g2cache some val",
				Foo: &Foo{
					Value:   time.Now().String(),
					Version: i,
				}}, nil
		})
		if err != nil {
			b.Fatal(err)
			return
		}
		buf := &bytes.Buffer{}
		_ = gob.NewEncoder(buf).Encode(o)
		b.Logf("%+v", buf.String())

		s, _ := json.MarshalToString(o)
		b.Logf("%+v", s)
	}
}
