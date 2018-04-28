// Package crock32 implements base32 encoding using an alphabet specified by
// Douglas Crockford in http://www.crockford.com/wrmg/base32.html
package crock32

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
        22, 23, 24, 25, 26, 59, 27, 28, 29, 30, 31, -1, -1, -1, -1, -1,
        -1, 10, 11, 12, 13, 14, 15, 16, 17, 33, 18, 19, 33, 20, 21, 32,
        22, 23, 24, 25, 26, 59, 27, 28, 29, 30, 31, -1, -1, -1, -1, -1,
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
		c1 = 0
		c2 = 0
		c3 = 0
		var tmp [8]byte
		var m int
		switch i {
		case 4:
			c3 = in[3]
			tmp[6] = alphabet[(c3 << 3) & 0x1F]      /* C4 == 0 */
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
// It will treat 'i' 'l' as '1', 'o' as '0', 'u' as 'v', all case-insensitive.
func DecodeFuzzy(out []byte, in []byte) int {
	return decode(out, in, true)
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

