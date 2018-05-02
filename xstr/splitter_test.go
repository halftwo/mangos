package xstr

import (
	"testing"
)

func TestSplitter(t *testing.T) {
	s := ",Hello.,World.."
	sp := NewSplitterAny(s, ",.")
	sp.Next()
	sp.Next()
	sp.Next()
	w := sp.Next()
	sp.Next()
	sp.Next()
	sp.Next()
	if sp.Count() != 6 || w != "World" {
		t.Error("Splitter testing failed")
	}
}


func TestTokenizerAny(t *testing.T) {
	s := ",Hello.,World.."
	tk := NewTokenizerAny(s, ",.")
	tk.Next()
	w := tk.Next()
	tk.Next()
	if tk.Count() != 2 || w != "World" {
		t.Error("TokenizerAny testing failed")
	}
}

func TestTokenizerEmptySep(t *testing.T) {
	s := ",Hello.,World.."
	tk := NewTokenizerAny(s, "")
	for tk.HasMore() {
		tk.Next()
	}
	if tk.Count() != len(s) {
		t.Errorf("TokenizerEmptySep testing failed, %d, %d", tk.Count(), len(s))
	}
}


