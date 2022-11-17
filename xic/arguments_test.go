package xic

import (
	"testing"
)

func TestArguments(t *testing.T) {
	args := NewArguments()
	m := make(map[string]any)
	m["a"] = 1.2345
	m["b"] = "faint"
	m["c"] = 666
	args.Set("hello", 5.4321)
	args.Set("world", m)

	t.Log(args.Get("hello"))

	type Params struct {
		A float32 `vbs:"a"`
		B string  `vbs:"b"`
		C any     `vbs:"c"`
	}
	var p Params
	err := args.GetStruct("world", &p)
	if err != nil {
		t.Error(err)
	}
	t.Logf("%+v\n", p)
}

func TestArgumentsCopy(t *testing.T) {
	a := NewArguments()
	m := make(map[string]any)
	m["a"] = 1.2345
	m["b"] = "faint"
	m["c"] = 666
	a.Set("hello", 5.4321)
	a.Set("world", m)

	b := NewArguments()
	b.CopyFrom(a)
	t.Logf("a: %v\n", a)
	t.Logf("b: %v\n", b)
}
