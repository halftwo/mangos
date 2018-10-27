package xstr

import (
	"bytes"
	"unicode"
	"unicode/utf8"
)

type BytesCutter struct {
	buf []byte
	current int
}

// NewBytesCutter initialize a new BytesCutter object
func NewBytesCutter(buf []byte) *BytesCutter {
	return &BytesCutter{buf, 0}
}

// HasMore return true if some of the buffer are not scaned.
func (bc *BytesCutter) HasMore() bool {
	return bc.current >= 0 && bc.current < len(bc.buf)
}

// Current return the current position already scaned in the buffer.
// -1 is returned if all the buffer are scaned.
func (bc *BytesCutter) Current() int {
	return bc.current
}

// Remain return the not-consumed part in the buffer
func (bc *BytesCutter) Remain() []byte {
	if bc.current < 0 {
		return nil
	}
	return bc.buf[bc.current:]
}

func indexAny(s []byte, chars string, truth bool) (int, int) {
	start := 0
	for start < len(s) {
		width := 1
		r := rune(s[start])
		if r >= utf8.RuneSelf {
			r, width = utf8.DecodeRune(s[start:])
		}

		found := false
		for _, ch := range chars {
			if r == ch {
				found = true
				break
			}
		}

		if found == truth {
			return start, width
		}
		start += width
	}
	return -1, 0

}

func indexFunc(s []byte, f func(rune) bool, truth bool) (int, int) {
	start := 0
	for start < len(s) {
		width := 1
		r := rune(s[start])
		if r >= utf8.RuneSelf {
			r, width = utf8.DecodeRune(s[start:])
		}
		if f(r) == truth {
			return start, width
		}
		start += width
	}
	return -1, 0
}

// NextToken is a shorthand for (*BytesCutter).NextTokenFunc(unicode.IsSpace)
func (bc *BytesCutter) NextTokenSpace() []byte {
	return bc.NextTokenFunc(unicode.IsSpace)
}

// NextTokenAny return next part seperated by consecutive runes which is contained in chars.
// It may return nil even if HasMore() returns true.
func (bc *BytesCutter) NextTokenAny(chars string) []byte {
	if bc.current < 0 {
		return nil
	}

	i, w1 := indexAny(bc.buf[bc.current:], chars, false)
	if i < 0 {
		bc.current = -1
		return nil
	}
	start := bc.current + i

	j, w2 := indexAny(bc.buf[start+w1:], chars, true)
	if j < 0 {
		bc.current = -1
		return bc.buf[start:]
	}
	end := start + w1 + j
	bc.current = end + w2
	return bc.buf[start:end]
}

// NextTokenFunc return next part seperated by consecutive runes which satisfies func f.
// It may return nil even if HasMore() returns true.
func (bc *BytesCutter) NextTokenFunc(f func(rune) bool) []byte {
	if bc.current < 0 {
		return nil
	}

	i, w1 := indexFunc(bc.buf[bc.current:], f, false)
	if i < 0 {
		bc.current = -1
		return nil
	}
	start := bc.current + i

	j, w2 := indexFunc(bc.buf[start+w1:], f, true)
	if j < 0 {
		bc.current = -1
		return bc.buf[start:]
	}
	end := start + w1 + j
	bc.current = end + w2
	return bc.buf[start:end]
}

// SkipSpace skips consecutive spaces
func (bc *BytesCutter) SkipSpace() {
	bc.SkipFunc(unicode.IsSpace)
}

// SkipAny skips consecutive runes which are contained in chars
func (bc *BytesCutter) SkipAny(chars string) {
	if bc.current >= 0 {
		i, _ := indexAny(bc.buf[bc.current:], chars, false)
		if i < 0 {
			bc.current = -1
		} else {
			bc.current += i
		}
	}
}

// SkipFunc skips consecutive runes which satisfy func f
func (bc *BytesCutter) SkipFunc(f func(rune) bool) {
	if bc.current >= 0 {
		i, _ := indexFunc(bc.buf[bc.current:], f, false)
		if i < 0 {
			bc.current = -1
		} else {
			bc.current += i
		}
	}
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

// NextPart return next part seperated by one sep
func (bc *BytesCutter) NextPart(sep []byte) []byte {
	if bc.current < 0 {
		return nil
	}
	i, j := bc.NextIndex(sep)
	return bc.buf[i:j]
}

// NextIndex return [start, end) indexes of next part seperated by one sep
func (bc *BytesCutter) NextIndex(sep []byte) (int, int) {
	if bc.current < 0 {
		return -1, -1
	}
	return bc._next(func(buf []byte) (int, int) {
		return bytes.Index(buf, sep), len(sep)
	})
}

// NextPartByte return next part seperated by one byte b
func (bc *BytesCutter) NextPartByte(b byte) []byte {
	if bc.current < 0 {
		return nil
	}
	i, j := bc.NextIndexByte(b)
	return bc.buf[i:j]
}

// NextIndexByte return [start, end) indexes of next part seperated by one byte b
func (bc *BytesCutter) NextIndexByte(b byte) (int, int) {
	if bc.current < 0 {
		return -1, -1
	}
	return bc._next(func(buf []byte) (int, int) {
		return bytes.IndexByte(buf, b), 1
	})
}

// NextPartRune return next part seperated by one rune r
func (bc *BytesCutter) NextPartRune(r rune) []byte {
	if bc.current < 0 {
		return nil
	}
	i, j := bc.NextIndexRune(r)
	return bc.buf[i:j]
}

// NextIndexRune return [start, end) indexes of next part seperated by one rune r
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

// NextPartAny return next part seperated by one of rune in chars
func (bc *BytesCutter) NextPartAny(chars string) []byte {
	if bc.current < 0 {
		return nil
	}
	i, j := bc.NextIndexAny(chars)
	return bc.buf[i:j]
}

// NextIndexAny return [start, end) indexes of next part seperated by one rune in chars
func (bc *BytesCutter) NextIndexAny(chars string) (int, int) {
	if bc.current < 0 {
		return -1, -1
	}
	return bc._next(func(buf []byte) (int, int) {
		i := bytes.IndexAny(buf, chars)
		if i < 0 {
			return -1, 0
		}
		_, n := utf8.DecodeRune(buf[i:])
		return i, n
	})
}

// NextPartFunc return next part seperated by one rune that satisfies func f
func (bc *BytesCutter) NextPartFunc(f func(rune) bool) []byte {
	if bc.current < 0 {
		return nil
	}
	i, j := bc.NextIndexFunc(f)
	return bc.buf[i:j]
}

// NextIndexFunc return [start, end) indexes of next part seperated by one rune that satisfies func f
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

