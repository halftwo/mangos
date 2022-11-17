package xic

import (
	"reflect"
)

// Do not be confused with context.Context
type Context map[string]any

func NewContext() Context {
	return Context{}
}

func (ctx Context) Has(name string) bool {
	_, ok := ctx[name]
	return ok
}

func (ctx Context) Get(name string) any {
	return ctx[name]
}

func (ctx Context) GetString(name string, dft string) string {
	v, ok := ctx[name]
	if ok {
		s, ok := v.(string)
		if ok && len(s) > 0 {
			return s
		}
	}
	return dft
}

func (ctx Context) GetInt(name string, dft int64) int64 {
	v, ok := ctx[name]
	if ok {
		i, ok := v.(int64)
		if ok {
			return i
		}
	}
	return dft
}

func (ctx Context) GetUint(name string, dft uint64) uint64 {
	v, ok := ctx[name]
	if ok {
		i, ok := v.(uint64)
		if ok {
			return i
		}
	}
	return dft
}

func (ctx Context) GetBool(name string, dft bool) bool {
	v, ok := ctx[name]
	if ok {
		t, ok := v.(bool)
		if ok {
			return t
		}
	}
	return dft
}

func (ctx Context) GetFloat(name string, dft float64) float64 {
	v, ok := ctx[name]
	if ok {
		f, ok := v.(float64)
		if ok {
			return f
		}
	}
	return dft
}

func (ctx Context) GetBlob(name string, dft []byte) []byte {
	v, ok := ctx[name]
	if ok {
		b, ok := v.([]byte)
		if ok {
			return b
		}
	}
	return dft
}

func (ctx Context) Set(name string, x any) {
	v := reflect.ValueOf(x)
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		x = v.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		x = int64(v.Uint())
	case reflect.Float32:
		x = v.Float()
	case reflect.String:
		x = v.String()
	case reflect.Bool:
		x = v.Bool()
	case reflect.Array:
		if v.Elem().Kind() != reflect.Uint8 {
			return
		}
		buf := make([]byte, v.Len())
		reflect.Copy(reflect.ValueOf(buf), v)
		x = buf
	case reflect.Slice:
		if v.Elem().Kind() != reflect.Uint8 {
			return
		}
	case reflect.Complex64, reflect.Complex128, reflect.Struct, reflect.Ptr, reflect.Interface, reflect.Map:
		return
	}
	ctx[name] = x
}
