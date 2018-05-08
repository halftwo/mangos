package carp

import (
	"testing"
	"fmt"
)

var members = []uint64{9,8,7,6,5,4,3,2,1}
var cp = NewCarp(members, nil)
var key = uint32(123454321)

func TestCarp(t *testing.T) {

	i := cp.Which(key)

	seqs := make([]int,100)
	seqs = cp.Sequence(key, seqs)

	if i != seqs[0] {
		t.Errorf("Which() not equal Sequence()[0]")
	}

	fmt.Printf("which=%d seqs=%v\n", i, seqs)
	for i, m := range members {
		x := myCombine(m, key)
		fmt.Println(i, m, x)
	}
}

func BenchmarkWhich(b *testing.B) {
	for i := 0; i < b.N; i++ {
		cp.Which(key)
	}
}

func BenchmarkSequence(b *testing.B) {
	seqs := make([]int, 3)
	for i := 0; i < b.N; i++ {
		cp.Sequence(key, seqs)
	}
}

