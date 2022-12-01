package xbase57

import (
	"io"
	"strconv"
	"math"
	"encoding/binary"
)

const StdAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"

var StdEncoding = NewEncoding(StdAlphabet)

type Encoding struct {
	alphabet [57]byte
	detab [128]int8
}

// NewEncoding returns a new Encoding defined by the given alphabet,
// which must be a 57-byte string without space characters.
func NewEncoding(alphabet string) *Encoding {
	if len(alphabet) != 57 {
		panic("encoding alphabet is not 57-bytes long")
	}

	enc := new(Encoding)
	copy(enc.alphabet[:], alphabet)

	for i := 0; i < len(enc.detab); i++ {
		enc.detab[i] = -1
	}
	enc.detab['\t'] = -2
	enc.detab['\n'] = -2
	enc.detab['\v'] = -2
	enc.detab['\f'] = -2
	enc.detab['\r'] = -2
	enc.detab[' '] = -2

	for i := 0; i < len(enc.alphabet); i++ {
		k := enc.alphabet[i]
		if enc.detab[k] != -1 {
			if enc.detab[k] >= 0 {
				panic("encoding alphabet has duplicate characters")
			} else if enc.detab[k] == -2 {
				panic("encoding alphabet has space characters")
			}
		}
		enc.detab[k] = int8(i)
	}

	return enc
}

// Alphabet returns the alphabet of this xbase57 encoding.
func (enc *Encoding) Alphabet() []byte {
	return enc.alphabet[:]
}

// EncodedLen returns the length src bytes of the xbase57 encoding
// of an input buffer of length n.
func (enc *Encoding) EncodedLen(n int) int {
        return (n * 11 + 7) / 8
}

// EncodeToString returns the xbase57 encoding of src.
func (enc *Encoding) EncodeToString(src []byte) string {
	buf := make([]byte, enc.EncodedLen(len(src)))
	n := enc.Encode(buf, src)
	return string(buf[:n])
}

// Encode encodes src using the encoding enc, writing EncodedLen(len(src)) bytes to dst.
// If Encode is used on individual blocks of a large data stream, len(src) must be multiple of 8.
// Use NewEncoder() otherwise.
func (enc *Encoding) Encode(dst []byte, src []byte) int {
	total := 0
	n := len(src)
	k := len(dst)
        for n >= 8 && k >= 11 {
		acc := (uint64(src[0]) << 56) |
			(uint64(src[1]) << 48) |
			(uint64(src[2]) << 40) |
			(uint64(src[3]) << 32) |
			(uint64(src[4]) << 24) |
			(uint64(src[5]) << 16) |
			(uint64(src[6]) << 8) |
			uint64(src[7])

		for i := 10; i > 0; i-- {
                        dst[i] = enc.alphabet[acc % 57]
                        acc /= 57
                }
                dst[0] = enc.alphabet[acc]

		src = src[8:]
                dst = dst[11:]
		n -= 8
		k -= 11
		total += 11
	}

	if n > 0 && k > 0 {
		if n > 8 {
			n = 8
		}

                var acc uint64
                for i := 0; i < n; i++ {
                        shift := uint(7 - i) * 8
                        acc |= (uint64(src[i]) << shift)
                }

                m := enc.EncodedLen(n)
		if k > m {
			k = m
		}

                for i := 11 - k; i > 0; i-- {
                        acc /= 57
                }

                for i := k - 1; i > 0; i-- {
                        dst[i] = enc.alphabet[acc % 57]
                        acc /= 57
                }
                dst[0] = enc.alphabet[acc]
		total += k
	}
	return total
}

// DecodedLen returns the maximum length in bytes of the decoded data
// corresponding to n bytes of xbase57-encoded data.
func (enc *Encoding) DecodedLen(n int) int {
        return (n * 8) / 11
}

// DecodeString returns the bytes represented by the xbase57 string src.
func (enc *Encoding) DecodeString(src string) ([]byte, error) {
	buf := make([]byte, enc.DecodedLen(len(src)))
	n, err := enc.Decode(buf, []byte(src))
	return buf[:n], err
}


var _Dlens = []int8{ 0, -1, 1, 2, -1, 3, 4, 5, -1, 6, 7, }
var _Maxes = []uint64{ 0,
		0x047944da05d3ad0c,   /* "3t999999999" / 57 */
		0x047dbddd16ed96b0,   /* "37S99999999" / 57 */
		0x047dc11aff23a734,   /* "37Ux5999999" / 57 */
		0x047dc11f6c4df39d,   /* "37Uydj99999" / 57 */
		0x047dc11f70446bdc,   /* "37UydrS9999" / 57 */
		0x047dc11f7047d7ca,   /* "37UydrUNA99" / 57 */
		0x047dc11f7047dc0d,   /* "37UydrUNWH9" / 57 */
}

// Decode decodes src using the encoding enc. 
// It writes at most DecodedLen(len(src)) bytes to dst and returns the number of bytes written. 
// If src contains invalid xbase57 data, it will return the number of bytes successfully written and CorruptInputError. 
// Space characters (\t\r\n\f\v) are ignored.
func (enc *Encoding) Decode(dst []byte, src []byte) (int, error) {
	var last int
	var buf [8]byte
	slen := len(src)
        dlen := len(dst)
	acc := uint64(0)
	cnt := 0
	n := 0
        for k := 0; k < slen && n < dlen; k++ {
		ch := src[k]
		var x int
		if ch < 128 {
			x = int(enc.detab[ch])
		} else {
			x = -1
		}

		if x < 0 {
			if x == -2 {
                                continue
                        }
                        return n, CorruptInputError(k+1)
                }

		if cnt < 10 {
			last = k
			acc = acc * 57 + uint64(x)
                        cnt++
		} else {
			if acc > 0x047dc11f7047dc11 {
                                return n, CorruptInputError(k+1)
			}

                        acc *= 57
                        if acc > 0xffffffffffffffff - uint64(x) {
                                return n, CorruptInputError(k+1)
			}
                        acc += uint64(x)

                        buf[0] = byte(acc >> 56)
                        buf[1] = byte(acc >> 48)
                        buf[2] = byte(acc >> 40)
                        buf[3] = byte(acc >> 32)
                        buf[4] = byte(acc >> 24)
                        buf[5] = byte(acc >> 16)
                        buf[6] = byte(acc >> 8)
                        buf[7] = byte(acc)

			m := copy(dst[n:], buf[:])
			n += m

			acc = 0
			cnt = 0
		}
	}

	if cnt > 0 && n < dlen {
		m := _Dlens[cnt]
		if m < 0 {
			return n, CorruptInputError(last+1)
		}

		for i := cnt; i < 10; i++ {
			acc = acc * 57 + 56
		}

		if acc > _Maxes[m] {
			return n, CorruptInputError(last+1)
		}

		acc = acc * 57 + 56
		for i := 0; i < int(m) && n < dlen; i++ {
			shift := uint(7 - i) * 8
			dst[n] = byte(acc >> shift)
			n++
		}

		shift := uint(8 - m) * 8
		acc >>= shift
		acc <<= shift

		for i := 11 - cnt; i > 0; i-- {
			acc /= 57
		}

		if src[last] != enc.alphabet[acc%57] {
			return n, CorruptInputError(last+1)
		}
	}

	return n, nil
}


type CorruptInputError int64

func (e CorruptInputError) Error() string {
	return "illegal xbase57 data at input byte " + strconv.FormatInt(int64(e), 10)
}


// NewEncoder returns a new xbase57 stream encoder. Data written to
// the returned writer will be encoded using enc and then written to w.
// xbase57 encodings operate in 8-byte blocks; when finished
// writing, the caller must Close the returned encoder to flush any
// partially written blocks.
func NewEncoder(enc *Encoding, w io.Writer) io.WriteCloser {
	return &_Encoder{enc:enc, w:w}
}


type _Encoder struct {
	err error
	enc *Encoding
	w io.Writer
	buf [8]byte    // buffered data waiting to be encoded
	nb int         // number of bytes in buf
	out [1024 / 11 * 11]byte // output buffer
}

func (e *_Encoder) Write(p []byte) (n int, err error) {
	if e.err != nil {
		return 0, e.err
	}

	// Leading fringe.
	if e.nb > 0 {
		var i int
		for i = 0; i < len(p) && e.nb < 8; i++ {
			e.buf[e.nb] = p[i]
			e.nb++
		}
		n += i
		if e.nb < 8 {
			return
		}
		p = p[i:]

		e.enc.Encode(e.out[:], e.buf[:])
		if _, e.err = e.w.Write(e.out[:11]); e.err != nil {
			return n, e.err
		}
		e.nb = 0
	}

	// Large interior chunks.
	for len(p) >= 8 {
		m := len(e.out) / 11 * 8
		if m > len(p) {
			m = len(p)
			m -= m % 8
		}
		e.enc.Encode(e.out[:], p[:m])
		if _, e.err = e.w.Write(e.out[:m/8*11]); e.err != nil {
			return n, e.err
		}
		n += m
		p = p[m:]
	}

	// Trailing fringe.
	for i := 0; i < len(p); i++ {
		e.buf[i] = p[i]
	}
	e.nb = len(p)
	n += len(p)
	return
}

func (e *_Encoder) Close() error {
	if e.err == nil && e.nb > 0 {
		e.enc.Encode(e.out[0:], e.buf[:e.nb])
		k := e.enc.EncodedLen(e.nb)
		e.nb = 0
		_, e.err = e.w.Write(e.out[:k])
	}
	return e.err
}


// NewDecoder constructs a new xbase57 stream decoder.
func NewDecoder(enc *Encoding, r io.Reader) io.Reader {
	return &_Decoder{enc:enc, r:&_SpaceFilteringReader{r},}

}

type _SpaceFilteringReader struct {
	wrapped io.Reader
}

func (r *_SpaceFilteringReader) Read(p []byte) (int, error) {
	n, err := r.wrapped.Read(p)
	for n > 0 {
		offset := 0
		for i, b := range p[0:n] {
			if b < 9 || (b > 13 && b != 0x20) {
				if i != offset {
					p[offset] = b
				}
				offset++
			}
		}
		if err != nil || offset > 0 {
			return offset, err
		}
		// Previous buffer entirely whitespace, read again
		n, err = r.wrapped.Read(p)
	}
	return n, err
}


type _Decoder struct {
	err error
	enc *Encoding
	r io.Reader
	buf [1024 / 11 * 11]byte // leftover input
	nb int
	out []byte     // leftover decoded output
	outbuf [1024 / 11 * 8]byte
}

func (d *_Decoder) Read(p []byte) (n int, err error) {
	// Use leftover decoded output from last read.
	if len(d.out) > 0 {
		n = copy(p, d.out)
		d.out = d.out[n:]
		if len(d.out) == 0 {
			return n, d.err
		}
		return n, nil
	}

	if d.err != nil {
		return 0, d.err
	}

	// Read a chunk.
	m := len(p) / 8 * 11
	if m < 11 {
		m = 11
	} else if m > len(d.buf) {
		m = len(d.buf)
	}

	m, d.err = d.r.Read(d.buf[d.nb:m])
	d.nb += m

	var k int
	if d.err == io.EOF {
		k = d.nb
	} else {
		k = d.nb / 11 * 11
		if k < 11 {
			return 0, d.err
		}
	}

	// Decode chunk into p, or d.out and then p if p is too small.
	outlen := d.enc.DecodedLen(k)
	if outlen > len(p) {
		outlen, err = d.enc.Decode(d.outbuf[:], d.buf[:k])
		d.out = d.outbuf[:outlen]
		n = copy(p, d.out)
		d.out = d.out[n:]
	} else {
		n, err = d.enc.Decode(p, d.buf[:k])
	}
	d.nb -= k
	for i := 0; i < d.nb; i++ {
		d.buf[i] = d.buf[k+i]
	}

	if err != nil && (d.err == nil || d.err == io.EOF) {
		d.err = err
	}

	if len(d.out) > 0 {
		// We cannot return all the decoded bytes to the caller in this
		// invocation of Read, so we return a nil error to ensure that Read
		// will be called again.  The error stored in d.err, if any, will be
		// returned with the last set of decoded bytes.
		return n, nil
	}

	return n, d.err
}

type RandomSourceFunction func ([]byte)(int, error)

// The first char of the returned string is always a letter instead of digit
func RandomId(n int, rnd RandomSourceFunction) string {
        if n < 1 {
                panic("length of id must be greater than 1")
        }

        m := StdEncoding.DecodedLen(n) + 1
        if m < 4 {
                m = 4
        }

        src := make([]byte, m)
	k := 0
	for k < m {
		i, _ := rnd(src[k:])
		k += i
	}

        dst := make([]byte, n)
        k = StdEncoding.Encode(dst, src)
        u32 := binary.BigEndian.Uint32(src[:4])
        dst[0] = StdAlphabet[u32/(math.MaxUint32/49+1)]
        return string(dst[:k])
}

