package vbs

import (
	"testing"
	"reflect"
	"bytes"
	"fmt"
)

func testMarshal(t *testing.T, u interface{}) {
	got, err := Marshal(u)
	if err != nil {
		t.Fatalf("error encoding %T: %v:", u, err)
	}

	fmt.Printf("Marshal %T\t\t%v\n", u, len(got))

	pv := reflect.New(reflect.TypeOf(u))
	err = Unmarshal(got, pv.Interface())
	if err != nil {
		t.Fatalf("error decoding %T: %v:", u, err)
	}

	v := pv.Elem().Interface()
	if !reflect.DeepEqual(u, v) {
		fmt.Println(u)
		fmt.Println(v)
		t.Fatal("The unmarshaled data does not match the original")
	}
}

func benchmark(b *testing.B, u interface{}) {
	buf := &bytes.Buffer{}
	enc := NewEncoder(buf)
	dec := NewDecoder(buf)

	pv := reflect.New(reflect.TypeOf(u))
	pi := pv.Interface()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		enc.Encode(u)
		dec.Decode(pi)
	}

	v := pv.Elem().Interface()
	if !reflect.DeepEqual(u, v) {
		fmt.Println(u)
		fmt.Println(v)
		b.Fatalf("The unmarshaled data does not match the original")
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

	u3 := []int{} 	// empty 
	testMarshal(t, u3)

	var u4 []int	// nil 
	testMarshal(t, u4)
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

