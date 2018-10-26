package xstr

import (
	"bytes"
	"unicode/utf8"
)

type BytesCutter struct {
	buf []byte
	current int
}

func NewBytesCutter(buf []byte) *BytesCutter {
	return &BytesCutter{buf, 0}
}

func (bc *BytesCutter) HasMore() bool {
	return bc.current >= 0
}

func (bc *BytesCutter) Current() int {
	return bc.current
}

func (bc *BytesCutter) Remain() []byte {
	if bc.current < 0 {
		return nil
	}
	return bc.buf[bc.current:]
}

func (bc *BytesCutter) _next(f func([]byte)(int, int)) (int, int) {
	start := bc.current
	i, n := f(bc.buf[start:])
	if i < 0 {
		bc.current = -1
		return start, len(bc.buf)
	}
	end := start + i
	bc.current = end + n
	return start, end
}

func (bc *BytesCutter) NextPart(sep []byte) []byte {
	if bc.current < 0 {
		return nil
	}
	i, j := bc.NextIndex(sep)
	return bc.buf[i:j]
}


func (bc *BytesCutter) NextIndex(sep []byte) (int, int) {
	if bc.current < 0 {
		return -1, -1
	}
	return bc._next(func(buf []byte) (int, int) {
		return bytes.Index(buf, sep), len(sep)
	})
}

func (bc *BytesCutter) NextPartByte(b byte) []byte {
	if bc.current < 0 {
		return nil
	}
	i, j := bc.NextIndexByte(b)
	return bc.buf[i:j]
}

func (bc *BytesCutter) NextIndexByte(b byte) (int, int) {
	if bc.current < 0 {
		return -1, -1
	}
	return bc._next(func(buf []byte) (int, int) {
		return bytes.IndexByte(buf, b), 1
	})
}

func (bc *BytesCutter) NextPartRune(r rune) []byte {
	if bc.current < 0 {
		return nil
	}
	i, j := bc.NextIndexRune(r)
	return bc.buf[i:j]
}

func (bc *BytesCutter) NextIndexRune(r rune) (int, int) {
	if bc.current < 0 {
		return -1, -1
	}
	return bc._next(func(buf []byte) (int, int) {
		i := bytes.IndexRune(buf, r)
		if i < 0 {
			return -1, 0
		}
		return i, utf8.RuneLen(r)
	})
}

func (bc *BytesCutter) NextPartFunc(f func(rune) bool) []byte {
	if bc.current < 0 {
		return nil
	}
	i, j := bc.NextIndexFunc(f)
	return bc.buf[i:j]
}

func (bc *BytesCutter) NextIndexFunc(f func(rune) bool) (int, int) {
	if bc.current < 0 {
		return -1, -1
	}
	return bc._next(func(buf []byte) (int, int) {
		i := bytes.IndexFunc(buf, f)
		if i < 0 {
			return i, 0
		}
		_, n := utf8.DecodeRune(buf[i:])
		return i, n
	})
}

