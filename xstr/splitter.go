package xstr

import (
	"strings"
	"unicode"
	"unicode/utf8"
)


type Splitter interface {
	Remain() string
	HasMore() bool
	Next() string
	Count() int
}

type _IndexFunction func(string) (idx int, n int)

// _StrSplitter implements Splitter interface
type _StrSplitter struct {
	remain string
	count int
	done bool
	iFun _IndexFunction
}

func (sp *_StrSplitter) Count() int {
	return sp.count
}

func (sp *_StrSplitter) HasMore() bool {
	return !sp.done
}

func (sp *_StrSplitter) Remain() string {
	return sp.remain
}

func (sp *_StrSplitter) Next() string {
	if sp.done {
		return ""
	}

	sp.count++
	i, n := sp.iFun(sp.remain)
	if i < 0 {
		sp.done = true
		token := sp.remain
		sp.remain = ""
		return token
	}

	token := sp.remain[:i]
	sp.remain = sp.remain[i+n:]
	return token
}

func emptySepIndexFun(s string) (int, int) {
	if s == "" {
		return -1, 0
	}
	_, n := utf8.DecodeRuneInString(s)
	return n, 0
}

func makeIndexFunSeparator(sep string) _IndexFunction {
	if len(sep) == 0 {
		return emptySepIndexFun
	}

	return func(s string) (int, int) {
		i := strings.Index(s, sep)
		if i < 0 {
			return i, 0
		}
		return i, len(sep)
	}
}

func makeIndexFunAny(chars string) _IndexFunction {
	if len(chars) == 0 {
		return emptySepIndexFun
	}

	return func(s string) (int, int) {
		i := strings.IndexAny(s, chars)
		if i < 0 {
			return i, 0
		}
		_, n := utf8.DecodeRuneInString(s[i:])
		return i, n
	}
}

func makeIndexFunPredicate(f func(rune) bool) _IndexFunction {
	return func(s string) (int, int) {
		for i, r := range s {
			if f(r) {
				_, n := utf8.DecodeRuneInString(s[i:])
				return i, n
			}
		}
		return -1, 0
	}
}

func NewSplitter(s string, sep string) Splitter {
	fn := makeIndexFunSeparator(sep)
	sp := &_StrSplitter{remain:s, iFun:fn}
	return sp
}

func NewSplitterAny(s string, chars string) Splitter {
	fn := makeIndexFunAny(chars)
	sp := &_StrSplitter{remain:s, iFun:fn}
	return sp
}

func NewSplitterFunc(s string, f func(rune) bool) Splitter {
	fn := makeIndexFunPredicate(f)
	sp := &_StrSplitter{remain:s, iFun:fn}
	return sp
}

func NewSplitterSpace(s string) Splitter {
	return NewSplitterFunc(s, unicode.IsSpace)
}



// _StrTokenizer implements Splitter interface
// Two or more contiguous delimiter chars are considered to be one single delimiter.
type _StrTokenizer struct {
	_StrSplitter
	i int
	n int
}

func (tk *_StrTokenizer) prepare() {
	for {
		i, n := tk.iFun(tk.remain)
		if i != 0 {
			tk.i = i
			tk.n = n
			break
		}
		tk.remain = tk.remain[n:]
	}

	if tk.remain == "" {
		tk.done = true
	}
}

func (tk *_StrTokenizer) Next() string {
	if tk.done {
		return ""
	}

	tk.count++
	if tk.i < 0 {
		tk.done = true
		token := tk.remain
		tk.remain = ""
		return token
	}

	token := tk.remain[:tk.i]
	tk.remain = tk.remain[tk.i+tk.n:]
	tk.prepare()
	return token
}

func newTokenizer(s string, fn _IndexFunction) *_StrTokenizer {
	tk := &_StrTokenizer{_StrSplitter{remain:s, iFun:fn}, 0, 0}
	tk.prepare()
	return tk
}

func NewTokenizer(s string, sep string) Splitter {
	return newTokenizer(s, makeIndexFunSeparator(sep))
}

func NewTokenizerAny(s string, chars string) Splitter {
	return newTokenizer(s, makeIndexFunAny(chars))
}

func NewTokenizerFunc(s string, f func(rune) bool) Splitter {
	return newTokenizer(s, makeIndexFunPredicate(f))
}

func NewTokenizerSpace(s string) Splitter {
	return NewTokenizerFunc(s, unicode.IsSpace)
}

