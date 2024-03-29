package vbs

import (
	"testing"
	"reflect"
	"bytes"
	"fmt"
)


func equal(v1 any, v2 any) bool {
	x1 := reflect.ValueOf(v1)
	x2 := reflect.ValueOf(v2)
	if x1.Kind() != x2.Kind() {
		return false
	}

	switch x1.Kind() {
	case reflect.Map, reflect.Slice:
		if x1.Len() == 0 && x2.Len() == 0 {
			return true
		}
	}
	return reflect.DeepEqual(v1, v2)
}

func testMarshal(t *testing.T, u any) {
	buf, err := Marshal(u)
	if err != nil {
		t.Fatalf("error encoding %T: %#v", u, err)
	}

	pv := reflect.New(reflect.TypeOf(u))
	n, err := Unmarshal(buf, pv.Interface())
	if err != nil || n != len(buf) {
		t.Fatalf("error decoding %T: %#v", u, err)
	}

	v := pv.Elem().Interface()
	if !equal(u, v) {
		fmt.Println(u)
		fmt.Println(v)
		t.Fatal("The unmarshaled data does not match the original")
	}
}

func benchmark(b *testing.B, u any) {
	pv := reflect.New(reflect.TypeOf(u))
	pi := pv.Interface()

	buf := &bytes.Buffer{}
	for i := 0; i < b.N; i++ {
		buf.Reset()

		enc := NewEncoder(buf)
		err := enc.Encode(u)
		if err != nil {
			b.Fatalf("enc.Encode() failed, %#v", err)
		}

		dec := NewDecoder(buf)
		err = dec.Decode(pi)
		if err != nil {
			b.Fatalf("dec.Decode() failed, %#v", err)
		}
	}

	v := pv.Elem().Interface()
	if !equal(u, v) {
		fmt.Println(u)
		fmt.Println(v)
		b.Fatalf("The unmarshaled data does not match the original")
	}
}

func TestNil(t *testing.T) {
	defer func() {
		if x := recover(); x == nil {
			t.Fatal("TestNil: failed")
		}
	}()

	var x any
	Marshal(x)
}

func TestDiscard(t *testing.T) {
	type Q struct {
		Con string `vbs:"con"`
		Strftime map[string]string `vbs:"strftime"`
		Time int `vbs:"time"`
	}

	type A struct {
		Con string `vbs:"con"`
		Strftime map[string]string
		Time int `vbs:"time"`
	}

	q := Q{Con:"tcp/::1+5555/::1+58879", Strftime:map[string]string{"ctime":"Sun Nov 27 10:43:57 2022", "local":"221127u104357+08"}, Time:1669517037}
	a := A{}

	buf, err := Marshal(&q)
	if err != nil {
		t.Fatalf("error encoding %T: %#v", q, err)
	}
	n, err := Unmarshal(buf, &a)
	if err != nil || n != len(buf) {
		t.Fatalf("error decoding %T: %#v", a, err)
	}
}

func TestMarshalFloat(t *testing.T) {
	var u float64 = -1.25
	testMarshal(t, u)
}

func BenchmarkFloat(b *testing.B) {
	var u float64 = -0.1
	benchmark(b, u)
}

func TestMarshalBlob(t *testing.T) {
	u := [6]byte{1,2,3,4,5,6}
	testMarshal(t, u)
}

func TestMarshalSlice(t *testing.T) {
	u1 := [][3]float64{[3]float64{0.1,0.2,0.3}, [3]float64{0.4,0.5,0.6}, [3]float64{0.7,0.8,0.9}}
	testMarshal(t, u1)

	u2 := [3][]float64{[]float64{0.1,0.2,0.3}, []float64{0.4,0.5,0.6}, []float64{0.7,0.8,0.9}}
	testMarshal(t, u2)

	u3 := []int{}	// empty 
	testMarshal(t, u3)

	var u4 []int	// nil 
	testMarshal(t, u4)
}

func TestMarshalSliceBytes(t *testing.T) {
	u := [...][]byte{[]byte{1,2,3,4,5}, []byte{4,5,6,7,8}, []byte{7,8,9,10,11}}
	testMarshal(t, u)
}

func BenchmarkSliceBytes(b *testing.B) {
	u := [...][]byte{[]byte{1,2,3,4,5}, []byte{4,5,6,7,8}, []byte{7,8,9,10,11}}
	benchmark(b, u)
}

func BenchmarkSliceString(b *testing.B) {
	u := []string{"hello", "world", "faint"}
	benchmark(b, u)
}

func TestMarshalMap(t *testing.T) {
	u1 := map[int]string{1:"hello", 3:"world", -1:"faint"}
	testMarshal(t, u1)

	u2 := map[string]int{"hello":1, "world":3, "faint":-1}
	testMarshal(t, u2)

	u3 := map[int]string{}	// empty 
	testMarshal(t, u3)

	var u4 map[int]string	// nil 
	testMarshal(t, u4)
}

func BenchmarkMap(b *testing.B) {
	u := map[int]string{1:"hello", 3:"world", -1:"faint"}
	benchmark(b, u)
}

var st1 = struct {
	Alpha int	`vbs:"alpha"`
	Bravo int	`vbs:"b,omitempty"`
	Charlie string	`json:"charlie,omitempty"`
	Delta string	`json:"d"`
	Echo []byte
	Foxtrot float64
	Golf [4]byte
}{1234567890, 0, "hello,world!", "你好，世界！", []byte{1,2,3,4,5,6,7}, -1.1, [4]byte{'a','b','c','d'},}

var st2 = struct {
	Alpha int	`vbs:"1"`
	Bravo int	`vbs:"2,omitempty"`
	Charlie string	`json:"3,omitempty"`
	Delta string	`json:"4"`
	Echo []byte	`vbs:"5"`
	Foxtrot float64	`vbs:"6"`
	Golf [4]byte	`vbs:"7"`
}{st1.Alpha, st1.Bravo, st1.Charlie, st1.Delta, st1.Echo, st1.Foxtrot, st1.Golf,}

func TestMarshalStruct(t *testing.T) {
	testMarshal(t, st1)
	testMarshal(t, st2)
}

func BenchmarkStructNameKey(b *testing.B) {
	benchmark(b, st1)
}

func BenchmarkStructIntKey(b *testing.B) {
	benchmark(b, st2)
}

func testUnmarshalInterface(t *testing.T, u any) {
	buf, err := Marshal(u)
	if err != nil {
		t.Fatalf("error encoding %T: %v:", u, err)
	}

	var v any
	n, err := Unmarshal(buf, &v)
	if err != nil || n != len(buf) {
		t.Fatalf("error decoding %T: %v:", u, err)
	}
}

func TestUnmarshalInterface(t *testing.T) {
	u1 := -1.25
	testUnmarshalInterface(t, u1)

	u2 := []int{1,2,3}
	testUnmarshalInterface(t, u2)

	u3 := []any{666, "hello", "world", 0.999}
	testUnmarshalInterface(t, u3)

	u4 := map[int]string{1:"hello", 5:"world", -2:"faint"}
	testUnmarshalInterface(t, u4)

	u5 := map[string]any{"hello":1.25, "world":"ok", "faint":123456789}
	testUnmarshalInterface(t, u5)
}

