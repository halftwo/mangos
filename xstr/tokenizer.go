package xstr

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

type Tokenizer interface {
	Count() int
	HasMore() bool
	Next() string
}

type strTokenizer struct {
	s string
	count int
	done bool
	indexFun func(s string) int
}

func (tk *strTokenizer) Count() int {
	return tk.count
}

func (tk *strTokenizer) HasMore() bool {
	return !tk.done
}

func (tk *strTokenizer) Next() string {
	if tk.done {
		return ""
	}

	tk.count++
	i := tk.indexFun(tk.s)
	if i < 0 {
		tk.done = true
		return tk.s
	}

	token := tk.s[:i]
	_, n := utf8.DecodeRuneInString(tk.s[i:])
	tk.s = tk.s[i+n:]
	return token
}


func NewTokenizer(s string, sep string) Tokenizer {
	fn := func(s string) int {
		return strings.Index(s, sep)
	}
	tk := &strTokenizer{s:s, indexFun:fn}
	return tk
}

func NewTokenizerAny(s string, chars string) Tokenizer {
	fn := func(s string) int {
		return strings.IndexAny(s, chars)
	}
	tk := &strTokenizer{s:s, indexFun:fn}
	return tk
}

func NewTokenizerFunc(s string, f func(rune) bool) Tokenizer {
	fn := func(s string) int {
		return strings.IndexFunc(s, f)
	}
	tk := &strTokenizer{s:s, indexFun:fn}
	return tk
}

func NewTokenizerSpace(s string) Tokenizer {
	return NewTokenizerFunc(s, unicode.IsSpace)
}

