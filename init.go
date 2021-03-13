package g2cache

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/mohae/deepcopy"
	"reflect"
	"strconv"
)

func clone(src, dst interface{}) (err error)  {
	defer func() {
		if e := recover();e != nil {
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

func GenKey(args ...interface{}) string  {
	var buf bytes.Buffer
	for i := range args {
		addBuf(args[i],&buf)
		if i < len(args) -1 {
			addBuf(":",&buf)
		}
	}
	return buf.String()
}

func addBuf(i interface{}, buf *bytes.Buffer)  {
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