package xic

import (
	"testing"
	"fmt"
)

func TestArguments(t *testing.T) {
	args := NewArguments()
	m := make(map[string]interface{})
	m["a"] = 1.2345
	m["b"] = "faint"
	m["c"] = 666
	args.Set("hello", 5.4321)
	args.Set("world", m)

	fmt.Println(args.GetString("hello"))

	type Params struct {
		A float32	`vbs:"a"`
		B string	`vbs:"b"`
		C interface{}	`vbs:"c"`
	}
	var p Params
	err := args.GetStruct("world", &p)
	if err != nil {
		fmt.Println("ERROR:", err)
	}
	fmt.Printf("%+v\n", p)
}

func TestArgumentsCopy(t *testing.T) {
	a := NewArguments()
	m := make(map[string]interface{})
	m["a"] = 1.2345
	m["b"] = "faint"
	m["c"] = 666
	a.Set("hello", 5.4321)
	a.Set("world", m)

	b := NewArguments()
	b.CopyFrom(a)
	fmt.Printf("a: %v\n", a)
	fmt.Printf("b: %v\n", b)
}
