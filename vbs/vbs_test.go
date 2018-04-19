package vbs

import (
	"testing"
	"reflect"
	"bytes"
	"fmt"
)

func TestMarshalFloat(t *testing.T) {
	u := -1.25
	got, err := Marshal(u)
	if err != nil {
		t.Fatal(err)
	}

	var v float64
	err = Unmarshal(got, &v)
	if err != nil {
		t.Fatal(err)
	}

	if u != v {
		t.Fatalf("The unmarshaled float (%v) does not match the marshaled data (%v)", u, v)
	}
}

func BenchmarkFloat(b *testing.B) {
	u := -0.1
	var v float64
	buf := &bytes.Buffer{}
	for i := 0; i < b.N; i++ {
		buf.Reset()
		Encode(buf, u)
		Decode(buf, &v)
	}

	if u != v {
		b.Fatalf("The unmarshaled float (%v) does not match the marshaled data (%v)", u, v)
	}
}

func TestMarshalBlob(t *testing.T) {
	u := [6]byte{1,2,3,4,5,6}
	v := [6]byte{}

	got, err := Marshal(u)
	if err != nil {
		t.Fatal(err)
	}

	err = Unmarshal(got, &v)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(got)

	if !reflect.DeepEqual(u, v) {
		t.Fatal("The unmarshaled blob does not match the original")
	}
}

func TestMarshalSlice(t *testing.T) {
	u := [][]byte{[]byte{1,2,3}, []byte{4,5,6}, []byte{7,8,9}}
	got, err := Marshal(u)
	if err != nil {
		t.Fatal(err)
	}

	v := [][]byte{}
	err = Unmarshal(got, &v)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(u, v) {
		fmt.Println(u)
		fmt.Println(v)
		t.Fatal("The unmarshaled slice does not match the original")
	}
}

func BenchmarkSlice(b *testing.B) {
	u := []string{"hello", "world", "faint"}
	v := []string{}

	buf := &bytes.Buffer{}
	for i := 0; i < b.N; i++ {
		buf.Reset()
		Encode(buf, u)
		Decode(buf, &v)
	}

	if !reflect.DeepEqual(u, v) {
		b.Fatal("The unmarshaled slice does not match the original")
	}
}

func TestMarshalMap(t *testing.T) {
	u := map[int]string{1:"hello", 3:"world"}
	u[-1] = "faint"
	got, err := Marshal(u)
	if err != nil {
		t.Fatal(err)
	}

	v := map[int]string{}
	err = Unmarshal(got, &v)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(u, v) {
		t.Fatal("The unmarshaled map does not match the original")
	}
}

func BenchmarkMap(b *testing.B) {
	u := map[int]string{1:"hello", 3:"world", -1:"faint"}
	v := map[int]string{}

	buf := &bytes.Buffer{}
	for i := 0; i < b.N; i++ {
		buf.Reset()
		Encode(buf, u)
		Decode(buf, &v)
	}

	if !reflect.DeepEqual(u, v) {
		b.Fatal("The unmarshaled map does not match the original")
	}
}

