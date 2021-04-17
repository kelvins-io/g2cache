package g2cache

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/mohae/deepcopy"
	"reflect"
	"strconv"
)

var (
	LocalStorageClose = errors.New("local storage close !!! ")
	OutStorageClose   = errors.New("out storage close !!! ")
	CacheClose        = errors.New("g2cache close !!! ")
)

func clone(src, dst interface{}) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = errors.New(fmt.Sprint(e))
			return
		}
	}()

	v := deepcopy.Copy(src)
	if reflect.ValueOf(v).IsValid() {
		reflect.ValueOf(dst).Elem().Set(reflect.Indirect(reflect.ValueOf(v)))
	}
	return err
}

func GenKey(args ...interface{}) string {
	var buf bytes.Buffer
	for i := range args {
		addBuf(args[i], &buf)
		if i < len(args)-1 {
			addBuf(":", &buf)
		}
	}
	return buf.String()
}

func addBuf(i interface{}, buf *bytes.Buffer) {
	switch v := i.(type) {
	case int:
		buf.WriteString(strconv.Itoa(v))
	case int8:
		buf.WriteString(strconv.Itoa(int(v)))
	case int16:
		buf.WriteString(strconv.Itoa(int(v)))
	case int32:
		buf.WriteString(strconv.Itoa(int(v)))
	case int64:
		buf.WriteString(strconv.Itoa(int(v)))
	case uint8:
		buf.WriteString(strconv.Itoa(int(v)))
	case uint16:
		buf.WriteString(strconv.Itoa(int(v)))
	case uint32:
		buf.WriteString(strconv.Itoa(int(v)))
	case uint64:
		buf.WriteString(strconv.Itoa(int(v)))
	case string:
		buf.WriteString(v)
	}
}

func encodeUUID(u []byte) string {
	buf := make([]byte, 36)
	hex.Encode(buf[0:8], u[0:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], u[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], u[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], u[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:], u[10:])
	return string(buf)
}

// NewUUID create v4 uuid
// More powerful UUID libraries can be used: https://github.com/google/uuid
func NewUUID() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return encodeUUID(b), nil
}