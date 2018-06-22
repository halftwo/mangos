// Package crock32 implements base32 encoding using an alphabet specified by
// Douglas Crockford in http://www.crockford.com/wrmg/base32.html
package crock32

import "fmt"

const AlphabetUpper = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

const AlphabetLower = "0123456789abcdefghjkmnpqrstvwxyz"


const _S = -2	// space
const _P = -3	// '-' partition separator

var detab = [128]int8{
        -1, -1, -1, -1, -1, -1, -1, -1, -1, _S, _S, _S, _S, _S, -1, -1,
        -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
        _S, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, _P, -1, -1,
         0,  1,  2,  3,  4,  5,  6,  7,  8,  9, -1, -1, -1, -1, -1, -1,
        -1, 10, 11, 12, 13, 14, 15, 16, 17, 33, 18, 19, 33, 20, 21, 32,
        22, 23, 24, 25, 26, -1, 27, 28, 29, 30, 31, -1, -1, -1, -1, -1,
        -1, 10, 11, 12, 13, 14, 15, 16, 17, 33, 18, 19, 33, 20, 21, 32,
        22, 23, 24, 25, 26, -1, 27, 28, 29, 30, 31, -1, -1, -1, -1, -1,
}

// EncodedLen returns the length in bytes of the crock32 encoding
// of an input buffer of length n.
func EncodeLen(n int) int {
	return (n * 8 + 4) / 5
}

// EncodeUpper return the number of characters placed in out. 
func EncodeUpper(out []byte, in []byte) int {
	return encode([]byte(AlphabetUpper), out, in)
}

// EncodeLower return the number of characters placed in out. 
func EncodeLower(out []byte, in []byte) int {
	return encode([]byte(AlphabetLower), out, in)
}

func EncodeToUpperString(in []byte) string {
	n := EncodeLen(len(in))
	buf := make([]byte, n)
	EncodeUpper(buf, in)
	return string(buf)
}

func EncodeToLowerString(in []byte) string {
	n := EncodeLen(len(in))
	buf := make([]byte, n)
	EncodeLower(buf, in)
	return string(buf)
}

func encode(alphabet []byte, out, in []byte) int {
	var c0, c1, c2, c3, c4 byte
	n := 0
	i := len(in)
	k := len(out)
	for i >= 5 && k >= 8 {
		c0 = in[0]
		c1 = in[1]
		c2 = in[2]
		c3 = in[3]
		c4 = in[4]

		out[0] = alphabet[c0 >> 3]
		out[1] = alphabet[(c0 << 2 | c1 >> 6) & 0x1F]
		out[2] = alphabet[(c1 >> 1) & 0x1F]
		out[3] = alphabet[(c1 << 4 | c2 >> 4) & 0x1F]
		out[4] = alphabet[(c2 << 1 | c3 >> 7) & 0x1F]
		out[5] = alphabet[(c3 >> 2) & 0x1F]
		out[6] = alphabet[(c3 << 3 | c4 >> 5) & 0x1F]
		out[7] = alphabet[c4 & 0x1F]

		in = in[5:]
		out = out[8:]
		i -= 5
		k -= 8
		n += 8
	}

	if i > 0 && k > 0 {
		if i > 5 {
			i = 5
		}
		c1 = 0
		c2 = 0
		c3 = 0
		c4 = 0
		var tmp [8]byte
		var m int
		switch i {
		case 5:
			c4 = in[4]
			tmp[7] = alphabet[c4 & 0x1F]
			fallthrough
		case 4:
			c3 = in[3]
			tmp[6] = alphabet[(c3 << 3 | c4 >> 5) & 0x1F]
			tmp[5] = alphabet[(c3 >> 2) & 0x1F]
			m += 2
			fallthrough
		case 3:
			c2 = in[2]
			tmp[4] = alphabet[(c2 << 1 | c3 >> 7) & 0x1F]
			m += 1
			fallthrough
		case 2:
			c1 = in[1]
			tmp[3] = alphabet[(c1 << 4 | c2 >> 4) & 0x1F]
			tmp[2] = alphabet[(c1 >> 1) & 0x1F]
			m += 2
			fallthrough
		case 1:
			c0 = in[0]
			tmp[1] = alphabet[(c0 << 2 | c1 >> 6) & 0x1F]
			tmp[0] = alphabet[c0 >> 3]
			m += 2
		}

		n += copy(out, tmp[:m])
	}
	return n
}

// DecodedLen returns the maximum length in bytes of the decoded data 
// corresponding to n bytes of crock32-encoded data.
func DecodeLen(n int) int {
	return (n * 5 + 7) / 8
}

// Decode return the number of bytes placed in out. 
// On error, return a negative number, the absolute value of the number
// equals to the consumed size of the input string.
func Decode(out []byte, in []byte) int {
	return decode(out, in, false)
}

// DecodeFuzzy do the same thing as Decode except it will ignore '-'.
// It will treat 'i' 'l' as '1', 'o' as '0', all case-insensitive.
func DecodeFuzzy(out []byte, in []byte) int {
	return decode(out, in, true)
}

func DecodeString(s string) ([]byte, error) {
	n := DecodeLen(len(s))
	buf := make([]byte, n)
	k := Decode(buf, []byte(s))
	if k < 0 {
		return nil, fmt.Errorf("decode error at %d", -(k + 1))
	}
	return buf[:k], nil
}

func DecodeFuzzyString(s string) ([]byte, error) {
	n := DecodeLen(len(s))
	buf := make([]byte, n)
	k := DecodeFuzzy(buf, []byte(s))
	if k < 0 {
		return nil, fmt.Errorf("decode error at %d", -(k + 1))
	}
	return buf[:k], nil
}

func decode(out []byte, in []byte, fuzzy bool) int {
	var r, r2 byte
	var n, k int
	ilen := len(in)
	olen := len(out)
	for i := 0; i < ilen && k < olen; i++ {
		c := in[i]
		x := -1
		if c < 128 {
			x = int(detab[c])
		}

		if x < 0 {
			if fuzzy && x <= _P {
				continue
			}
			return -(i + 1)
		} else if x >= 32 {
			x -= 32
			if !fuzzy {
				return -(i + 1)
			}
		}

		n++
		switch n {
		case 1:
			r = byte(x << 3)
		case 2:
			out[k] = r + byte(x >> 2)
			k++
			r = byte(x << 6)
		case 3:
			r2 = r
			r = byte(x << 1)
		case 4:
			out[k] = r2 + r + byte(x >> 4)
			k++
			r = byte(x << 4)
		case 5:
			out[k] = r + byte(x >> 1)
			k++
			r = byte(x << 7)
		case 6:
			r2 = r
			r = byte(x << 2)
		case 7:
			out[k] = r2 + r + byte(x >> 3)
			k++
			r = byte(x << 5)
		case 8:
			out[k] = r + byte(x)
			k++
			n = 0
		}
	}

	if (n == 1 || n == 3 || n == 6) || (n != 0 && (r & 0xff) != 0) {
		return -len(in)
	}

	return k
}

var dammTable32 = [32][32]byte {
	[32]byte{ 0, 2, 4, 6, 8,10,12,14,16,18,20,22,24,26,28,30, 3, 1, 7, 5,11, 9,15,13,19,17,23,21,27,25,31,29},
	[32]byte{ 2, 0, 6, 4,10, 8,14,12,18,16,22,20,26,24,30,28, 1, 3, 5, 7, 9,11,13,15,17,19,21,23,25,27,29,31},
	[32]byte{ 4, 6, 0, 2,12,14, 8,10,20,22,16,18,28,30,24,26, 7, 5, 3, 1,15,13,11, 9,23,21,19,17,31,29,27,25},
	[32]byte{ 6, 4, 2, 0,14,12,10, 8,22,20,18,16,30,28,26,24, 5, 7, 1, 3,13,15, 9,11,21,23,17,19,29,31,25,27},
	[32]byte{ 8,10,12,14, 0, 2, 4, 6,24,26,28,30,16,18,20,22,11, 9,15,13, 3, 1, 7, 5,27,25,31,29,19,17,23,21},
	[32]byte{10, 8,14,12, 2, 0, 6, 4,26,24,30,28,18,16,22,20, 9,11,13,15, 1, 3, 5, 7,25,27,29,31,17,19,21,23},
	[32]byte{12,14, 8,10, 4, 6, 0, 2,28,30,24,26,20,22,16,18,15,13,11, 9, 7, 5, 3, 1,31,29,27,25,23,21,19,17},
	[32]byte{14,12,10, 8, 6, 4, 2, 0,30,28,26,24,22,20,18,16,13,15, 9,11, 5, 7, 1, 3,29,31,25,27,21,23,17,19},
	[32]byte{16,18,20,22,24,26,28,30, 0, 2, 4, 6, 8,10,12,14,19,17,23,21,27,25,31,29, 3, 1, 7, 5,11, 9,15,13},
	[32]byte{18,16,22,20,26,24,30,28, 2, 0, 6, 4,10, 8,14,12,17,19,21,23,25,27,29,31, 1, 3, 5, 7, 9,11,13,15},
	[32]byte{20,22,16,18,28,30,24,26, 4, 6, 0, 2,12,14, 8,10,23,21,19,17,31,29,27,25, 7, 5, 3, 1,15,13,11, 9},
	[32]byte{22,20,18,16,30,28,26,24, 6, 4, 2, 0,14,12,10, 8,21,23,17,19,29,31,25,27, 5, 7, 1, 3,13,15, 9,11},
	[32]byte{24,26,28,30,16,18,20,22, 8,10,12,14, 0, 2, 4, 6,27,25,31,29,19,17,23,21,11, 9,15,13, 3, 1, 7, 5},
	[32]byte{26,24,30,28,18,16,22,20,10, 8,14,12, 2, 0, 6, 4,25,27,29,31,17,19,21,23, 9,11,13,15, 1, 3, 5, 7},
	[32]byte{28,30,24,26,20,22,16,18,12,14, 8,10, 4, 6, 0, 2,31,29,27,25,23,21,19,17,15,13,11, 9, 7, 5, 3, 1},
	[32]byte{30,28,26,24,22,20,18,16,14,12,10, 8, 6, 4, 2, 0,29,31,25,27,21,23,17,19,13,15, 9,11, 5, 7, 1, 3},
	[32]byte{ 3, 1, 7, 5,11, 9,15,13,19,17,23,21,27,25,31,29, 0, 2, 4, 6, 8,10,12,14,16,18,20,22,24,26,28,30},
	[32]byte{ 1, 3, 5, 7, 9,11,13,15,17,19,21,23,25,27,29,31, 2, 0, 6, 4,10, 8,14,12,18,16,22,20,26,24,30,28},
	[32]byte{ 7, 5, 3, 1,15,13,11, 9,23,21,19,17,31,29,27,25, 4, 6, 0, 2,12,14, 8,10,20,22,16,18,28,30,24,26},
	[32]byte{ 5, 7, 1, 3,13,15, 9,11,21,23,17,19,29,31,25,27, 6, 4, 2, 0,14,12,10, 8,22,20,18,16,30,28,26,24},
	[32]byte{11, 9,15,13, 3, 1, 7, 5,27,25,31,29,19,17,23,21, 8,10,12,14, 0, 2, 4, 6,24,26,28,30,16,18,20,22},
	[32]byte{ 9,11,13,15, 1, 3, 5, 7,25,27,29,31,17,19,21,23,10, 8,14,12, 2, 0, 6, 4,26,24,30,28,18,16,22,20},
	[32]byte{15,13,11, 9, 7, 5, 3, 1,31,29,27,25,23,21,19,17,12,14, 8,10, 4, 6, 0, 2,28,30,24,26,20,22,16,18},
	[32]byte{13,15, 9,11, 5, 7, 1, 3,29,31,25,27,21,23,17,19,14,12,10, 8, 6, 4, 2, 0,30,28,26,24,22,20,18,16},
	[32]byte{19,17,23,21,27,25,31,29, 3, 1, 7, 5,11, 9,15,13,16,18,20,22,24,26,28,30, 0, 2, 4, 6, 8,10,12,14},
	[32]byte{17,19,21,23,25,27,29,31, 1, 3, 5, 7, 9,11,13,15,18,16,22,20,26,24,30,28, 2, 0, 6, 4,10, 8,14,12},
	[32]byte{23,21,19,17,31,29,27,25, 7, 5, 3, 1,15,13,11, 9,20,22,16,18,28,30,24,26, 4, 6, 0, 2,12,14, 8,10},
	[32]byte{21,23,17,19,29,31,25,27, 5, 7, 1, 3,13,15, 9,11,22,20,18,16,30,28,26,24, 6, 4, 2, 0,14,12,10, 8},
	[32]byte{27,25,31,29,19,17,23,21,11, 9,15,13, 3, 1, 7, 5,24,26,28,30,16,18,20,22, 8,10,12,14, 0, 2, 4, 6},
	[32]byte{25,27,29,31,17,19,21,23, 9,11,13,15, 1, 3, 5, 7,26,24,30,28,18,16,22,20,10, 8,14,12, 2, 0, 6, 4},
	[32]byte{31,29,27,25,23,21,19,17,15,13,11, 9, 7, 5, 3, 1,28,30,24,26,20,22,16,18,12,14, 8,10, 4, 6, 0, 2},
	[32]byte{29,31,25,27,21,23,17,19,13,15, 9,11, 5, 7, 1, 3,30,28,26,24,22,20,18,16,14,12,10, 8, 6, 4, 2, 0},
}

func DammChecksum(in []byte) int {
	interim := 0
	for i, c := range in {
		x := -1
		if c < 128 {
			x = int(detab[c])
			if x >= 32 {
				x -= 32
			}
		}

		if x < 0 {
			if x <= _P {
				continue
			}
			return -(i + 1)
		}

		interim = int(dammTable32[interim][x])
	}
	return interim
}

func DammValidate(in []byte) bool {
	return DammChecksum(in) == 0
}

