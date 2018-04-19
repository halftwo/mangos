package vbs

import (
	"testing"
	"reflect"
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
	for i := 0; i < b.N; i++ {
		got, _ := Marshal(u)
		Unmarshal(got, &v)
	}

	if u != v {
		b.Fatalf("The unmarshaled float (%v) does not match the marshaled data (%v)", u, v)
	}
}

func TestMarshalSlice(t *testing.T) {
	u := []string{"hello", "world", "faint"}
	got, err := Marshal(u)
	if err != nil {
		t.Fatal(err)
	}

	v := []string{}
	err = Unmarshal(got, &v)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(u, v) {
		t.Fatal("The unmarshaled slice does not match the original")
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

	for i := 0; i < b.N; i++ {
		got, _ := Marshal(u)
		Unmarshal(got, &v)
	}

	if !reflect.DeepEqual(u, v) {
		b.Fatal("The unmarshaled map does not match the original")
	}
}

