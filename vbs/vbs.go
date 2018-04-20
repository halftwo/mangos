package vbs

import (
	"io"
	"math"
	"math/bits"
	"reflect"
	"fmt"
	"bytes"
)

const VBS_DESCRIPTOR_MAX 	= 0x7fff
const VBS_SPECIAL_DESCRIPTOR 	= 0x8000

type Kind int8

const (
	VBS_TAIL Kind 	= 0x01         // Used to terminate list or dict.

	VBS_LIST        = 0x02

	VBS_DICT        = 0x03

	// RESERVED       0x04
	// RESERVED       0x05
	// RESERVED       0x06

	// DONT USE       0x07
	// DONT USE       ....
	// DONT USE       0x0D

	// RESERVED       0x0E

	VBS_NULL        = 0x0F

	VBS_DESCRIPTOR  = 0x10         // 0001 0xxx

	VBS_BOOL        = 0x18         // 0001 100x 	0=F 1=T

	// DONT USE       0x1A

	VBS_BLOB        = 0x1B

	VBS_DECIMAL     = 0x1C         // 0001 110x 	0=+ 1=-

	VBS_FLOATING    = 0x1E         // 0001 111x 	0=+ 1=-

	VBS_STRING      = 0x20         // 001x xxxx

	VBS_INTEGER     = 0x40  
)

type Decimal64 struct {
}

var kindNames = [...]string{
        "INVALID",      /*  0 */
        "TAIL",         /*  1 */
        "LIST",         /*  2 */
        "DICT",         /*  3 */
        "NULL",         /*  4 */
        "FLOATING",     /*  5 */
        "DECIMAL",      /*  6 */
        "BOOL",         /*  7 */
        "STRING",       /*  8 */
        "INTEGER",      /*  9 */
        "BLOB",         /* 10 */
        "DESCRIPTOR",   /* 11 */
}

var kindIdx = [VBS_INTEGER + 1]uint8{
         0,  1,  2,  3,  0,  0,  0, 0,  0,  0,  0,  0,  0,  0,  0,  4,
        11,  0,  0,  0,  0,  0,  0, 0,  7,  0,  0, 10,  6,  0,  5,  0,
         8,  0,  0,  0,  0,  0,  0, 0,  0,  0,  0,  0,  0,  0,  0,  0,
         0,  0,  0,  0,  0,  0,  0, 0,  0,  0,  0,  0,  0,  0,  0,  0,
         9,
}

func (k Kind) String() string {
	if k >= 0 && k <= VBS_INTEGER {
		return kindNames[kindIdx[k]]
	}
	return kindNames[0]
}


type InvalidVbsError struct {
}

func (e *InvalidVbsError) Error() string {
        return "vbs: Invalid vbs-encoded bytes"
}

type NumberOverflowError struct {
	MaxBits int
}

func (e *NumberOverflowError) Error() string {
        return fmt.Sprintf("vbs: allowed max bits: %d", e.MaxBits)
}

type ArrayOverflowError struct {
	Len int
}

func (e *ArrayOverflowError) Error() string {
        return fmt.Sprintf("vbs: allowed array len %d", e.Len)
}

type UnsupportedTypeError struct {
        Type reflect.Type
}

func (e *UnsupportedTypeError) Error() string {
        return "vbs: unsupported type: " + e.Type.String()
}

type DepthOverflowError struct {
	MaxDepth int
}

func (e *DepthOverflowError) Error() string {
        return fmt.Sprintf("vbs: depth exceeds max (%d)", e.MaxDepth)
}


type bufPacker [16]byte

func (bp *bufPacker) packIntOrStringHead(n *int, kind Kind, num uint64) {
	for ; num >= 0x20; *n++ {
		bp[*n] = 0x80 | byte(num)
		num >>= 7
	}
	bp[*n] = byte(kind) | byte(num)
	*n++
}

func (bp *bufPacker) packStringHead(n *int, len int) {
	bp.packIntOrStringHead(n, VBS_STRING, uint64(len))
}

func (bp *bufPacker) packInteger(n *int, v int64) {
	if v < 0 {
		bp.packIntOrStringHead(n, VBS_INTEGER + 0x20, uint64(-v))
	} else {
		bp.packIntOrStringHead(n, VBS_INTEGER, uint64(v))
	}
}

func (bp *bufPacker) packDescriptor(n *int, descriptor uint16) {
	v := descriptor
	if v > 0 && v < (VBS_SPECIAL_DESCRIPTOR | VBS_DESCRIPTOR_MAX) {
		if (v & VBS_SPECIAL_DESCRIPTOR) != 0 {
			bp[*n] = VBS_DESCRIPTOR
			*n++
			v &= VBS_DESCRIPTOR_MAX
		}

		if v > 0 {
			for ; v >= 0x08; *n++ {
                                bp[*n] = 0x80 | byte(v)
                                v >>= 7
                        }
                        bp[*n] = VBS_DESCRIPTOR | byte(v)
			*n++
		}
	}
}

func (bp *bufPacker) packKind(n *int, kind Kind, num uint64) {
	for ; num > 0; *n++ {
		bp[*n] = 0x80 | byte(num)
		num >>= 7
	}
	bp[*n] = byte(kind)
	*n++
}

func (bp *bufPacker) packHeadOfList(n *int, variety int) {
	if variety > 0 {
		bp.packKind(n, VBS_LIST, uint64(variety))
	} else {
		bp[*n] = byte(VBS_LIST)
		*n++
	}
}

func (bp *bufPacker) packHeadOfDict(n *int, variety int) {
	if variety > 0 {
		bp.packKind(n, VBS_DICT, uint64(variety))
	} else {
		bp[*n] = byte(VBS_DICT)
		*n++
	}
}

func (bp *bufPacker) packTail(n *int) {
	bp[*n] = byte(VBS_TAIL)
	*n++
}

func (bp *bufPacker) packBool(n *int, v bool) {
	if v {
		bp[*n] = byte(VBS_BOOL + 1)
	} else {
		bp[*n] = byte(VBS_BOOL)
	}
	*n++
}

func (bp *bufPacker) packFloat(n *int, v float64) {
	mantissa, expo := breakFloat(v)
	if mantissa < 0 {
		bp.packKind(n, VBS_FLOATING + 1, uint64(-mantissa))
	} else {
		bp.packKind(n, VBS_FLOATING, uint64(mantissa))
	}
	bp.packInteger(n, int64(expo))
}

func (bp *bufPacker) packDecimal64(n *int, v Decimal64) {
	mantissa, expo := breakDecimal64(v)
	if mantissa < 0 {
		bp.packKind(n, VBS_DECIMAL + 1, uint64(-mantissa))
	} else {
		bp.packKind(n, VBS_DECIMAL, uint64(mantissa))
	}
	bp.packInteger(n, int64(expo))
}

func findMsbSet(v uint64) int {
	return 63 - bits.LeadingZeros64(v)
}

func findLsbSet(v uint64) int {
	return bits.TrailingZeros64(v)
}

const (
	flt_ZERO_ZERO	= 0	// 0.0
	flt_ZERO	= 1	// +0.0 or -0.0
	flt_INF		= 2	// +inf or -inf
	flt_NAN		= 3	// nan
)

const (
	mask_SIGN = 0x8000000000000000
	mask_EXPO = 0x7FF0000000000000
	mask_MANT = 0x000FFFFFFFFFFFFF
)


func breakFloat(v float64) (mantissa int64, expo int) {
	bits := math.Float64bits(v)
	negative := (bits & mask_SIGN) != 0
	e := (bits & mask_EXPO) >> 52
	m := (bits & mask_MANT)
	switch e {
	case 0x7FF:
		if m == 0 {
			expo = flt_INF
		} else {
			expo = flt_NAN
		}
		m = 0
	case 0:
		if m == 0 {
			expo = flt_ZERO_ZERO
		} else {
			expo = 1
		}
	default:
		m |= (1 << 52)
		expo = int(e)
	}

	if m == 0 {
		if negative {
			expo = -expo
		}
	} else {
		shift := findLsbSet(m)
		m >>= uint(shift)
		expo = expo - 52 + shift - 1023
	}

	if negative {
		mantissa = -int64(m)
	} else {
		mantissa = int64(m)
	}

	return
}

func makeFloat(mantissa int64, expo int) float64 {
	var v float64
	negative := false
	if mantissa == 0 {
		if expo < 0 {
			expo = -expo
			negative = true
		}

		if expo < flt_ZERO {
			if negative {
				v = -0.0
			} else {
				v = +0.0
			}
		} else if expo == flt_INF {
			if negative {
				v = math.Inf(-1)
			} else {
				v = math.Inf(1)
			}
		} else {
			v = math.NaN()
		}
	} else {
		if mantissa < 0 {
			mantissa = -mantissa
			negative = true
		}

		point := findMsbSet(uint64(mantissa))
		expo += point + 1023
		if expo >= 0x7FF {
			if negative {
				v = math.Inf(-1) 
			} else {
				v = math.Inf(1) 
			}
		} else {
			shift := 0
			if expo <= 0 {
				shift = 52 - (point + 1) + expo
				expo = 0
			} else {
				shift = 52 - point
			}

			var bits uint64 = 0
			if shift >= 0 {
				bits = (uint64(mantissa) << uint(shift)) & mask_MANT
			} else {
				bits = (uint64(mantissa) >> uint(-shift)) & mask_MANT
			}

			bits |= (uint64(expo) << 52) & mask_EXPO
			if negative {
				bits |= mask_SIGN
			}
			v = math.Float64frombits(bits)
		}
	}
	return v
}

func breakDecimal64(v Decimal64) (mantissa int64, expo int) {
	// TODO
	return 0, 0
}

func makeDecimal64(mantissa int64, expo int) Decimal64 {
	// TODO
	return Decimal64{}
}

// Marshal returns the VBS encoding of v.
func Marshal(data interface{}) ([]byte, error) {
	buf := bytes.NewBuffer([]byte{})
	enc := NewEncoder(buf)
	enc.Encode(data)
	return buf.Bytes(), enc.err
}


// An Encoder writes VBS values to an output stream.
type Encoder struct {
	bufPacker
	w io.Writer
	maxDepth int
	depth int
	err error
}

// NewEncoder returns a new Encoder that writes to w
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w:w, maxDepth:math.MaxInt32}
}

// SetMaxDepth set the max depth of VBS dict and list
func (enc *Encoder) SetMaxDepth(depth int) {
	if depth < 0 {
		enc.maxDepth = math.MaxInt32
	} else {
		enc.maxDepth = depth
	}
}

// Encode writes the VBS encoding of v to the Encoder
func (enc *Encoder) Encode(data interface{}) error {
	v := reflect.ValueOf(data)
	enc.encodeReflectValue(v)
	return enc.err
}

func (enc *Encoder) write(buf []byte) {
	if enc.err == nil {
		_, enc.err = enc.w.Write(buf)
	}
}

func (enc *Encoder) writeByte(b byte) {
	if enc.err == nil {
		buf := [1]byte{b}
		_, enc.err = enc.w.Write(buf[:1])
	}
}

func (enc *Encoder) encodeDescriptor(descriptor uint16) {
	n := 0
	enc.packDescriptor(&n, descriptor)
	enc.write(enc.bufPacker[0:n])
}

func (enc *Encoder) encodeInteger(v int64) {
	n := 0
	enc.packInteger(&n, v)
	enc.write(enc.bufPacker[0:n])
}

func (enc *Encoder) encodeFloat(v float64) {
	n := 0
	enc.packFloat(&n, v)
	enc.write(enc.bufPacker[0:n])
}

func (enc *Encoder) encodeDecimal64(v Decimal64) {
	n := 0
	enc.packDecimal64(&n, v)
	enc.write(enc.bufPacker[0:n])
}

func (enc *Encoder) encodeComplex(v complex128) {
	enc.writeByte(byte(VBS_LIST))
	enc.encodeFloat(real(v))
	enc.encodeFloat(imag(v))
	enc.writeByte(byte(VBS_TAIL))
}

func (enc *Encoder) encodeBool(v bool) {
	b := byte(VBS_BOOL)
	if v {
		b += 1
	}
	enc.writeByte(b)
}

func (enc *Encoder) encodeString(v string) {
	n := 0
	enc.packStringHead(&n, len(v))
	enc.write(enc.bufPacker[0:n])
	enc.write([]byte(v))
}

func (enc *Encoder) encodeBlob(v []byte) {
	n := 0
	enc.packKind(&n, VBS_BLOB, uint64(len(v)))
	enc.write(enc.bufPacker[0:n])
	enc.write(v)
}

func (enc *Encoder) encodeNil() {
	enc.writeByte(byte(VBS_NULL))
}

func (enc *Encoder) encodeList(v reflect.Value) {
	if v.Kind() != reflect.Array && v.IsNil() {
		enc.encodeNil()
		return
	}

	elemKind := v.Type().Elem().Kind()
	if elemKind == reflect.Uint8 {	// VBS_BLOB
		var buf []byte
		if v.Kind() == reflect.Slice {
			buf = v.Interface().([]byte)
		} else {
			n := v.Len()
			buf = make([]byte, n)
			for i := 0; i < n; i++ {
				buf[i] = byte(v.Index(i).Uint())
			}
		}
		enc.encodeBlob(buf)
		return
	}

	enc.depth++
	if enc.depth > enc.maxDepth {
		enc.err = &DepthOverflowError{enc.maxDepth}
		return
	}

	enc.writeByte(byte(VBS_LIST))

	n := v.Len()
	for i := 0; i < n; i++ {
		enc.encodeReflectValue(v.Index(i))
		if enc.err != nil {
			return
		}
	}

	enc.writeByte(byte(VBS_TAIL))
	enc.depth--
} 

func (enc *Encoder) encodeMap(v reflect.Value) {
	if v.IsNil() {
		enc.encodeNil()
		return
	}

	enc.depth++
	if enc.depth > enc.maxDepth {
		enc.err = &DepthOverflowError{enc.maxDepth}
		return
	}

	enc.writeByte(byte(VBS_DICT))

	keys := v.MapKeys()
	for _, key := range(keys) {
		val := v.MapIndex(key)
		enc.encodeReflectValue(key)
		enc.encodeReflectValue(val)
		if enc.err != nil {
			return
		}
	}

	enc.writeByte(byte(VBS_TAIL))
	enc.depth--
}

func (enc *Encoder) encodeStruct(v reflect.Value) {
	if enc.depth >= enc.maxDepth {
		enc.err = &DepthOverflowError{enc.maxDepth}
		return
	}

	enc.depth++
	enc.writeByte(byte(VBS_DICT))

	tp := v.Type()
	n := v.NumField()
	for i := 0; i < n; i++ {
		tf := tp.Field(i)
		enc.encodeString(tf.Name)

		f := v.Field(i)
		enc.encodeReflectValue(f)
		if enc.err != nil {
			return
		}
	}

	enc.writeByte(byte(VBS_TAIL))
	enc.depth--
}

func (enc *Encoder) encodePointer(v reflect.Value) {
	if v.IsNil() {
		enc.encodeNil()
		return
	}

	enc.encodeReflectValue(v.Elem())
}

func (enc *Encoder) encodeReflectValue(v reflect.Value) {
	if enc.err != nil {
		return
	}

	if d, ok := v.Interface().(Decimal64); ok {
		enc.encodeDecimal64(d)
		return
	} 

	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		enc.encodeInteger(v.Int())

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		enc.encodeInteger(int64(v.Uint()))

	case reflect.String:
		enc.encodeString(v.String())

	case reflect.Bool:
		enc.encodeBool(v.Bool())

	case reflect.Float32, reflect.Float64:
		enc.encodeFloat(v.Float())

	case reflect.Complex64, reflect.Complex128:
		enc.encodeComplex(v.Complex())

	case reflect.Array, reflect.Slice:
		enc.encodeList(v)

	case reflect.Map:
		enc.encodeMap(v)

	case reflect.Struct:
		enc.encodeStruct(v)

	case reflect.Interface, reflect.Ptr:
		enc.encodePointer(v)

	// reflect.Func, reflect.Chan, reflect.UnsafePointer:
	default:
		enc.err = &UnsupportedTypeError{v.Type()}
	}
}

// An InvalidUnmarshalError describes an invalid argument passed to Unmarshal.
// (The argument to Unmarshal must be a non-nil pointer.)
type InvalidUnmarshalError struct {
	Type reflect.Type
}

func (e *InvalidUnmarshalError) Error() string {
	if e.Type == nil {
		return "vbs: Unmarshal(nil)"
	}

	if e.Type.Kind() != reflect.Ptr {
		return "vbs: Unmarshal(non-pointer " + e.Type.String() + ")"
	}
	return "vbs: Unmarshal(nil " + e.Type.String() + ")"
}


var MaxLength = math.MaxInt32
var MaxDepth = math.MaxInt32

// A Decoder reads and decodes VBS values from an input stream.
type Decoder struct {
	r io.Reader
	maxLength int
	maxDepth int
	depth int
	err error
	eof bool
	bytes []byte
	hStart int
	hEnd int
	hBuffer [64]byte
}

// Unmarshal decodes the VBS-encoded data and stores the result in the value pointed to by v.
// If v is nil or not a pointer, Unmarshal returns an InvalidUnmarshalError.
func Unmarshal(buf []byte, v interface{}) error {
	dec := NewDecoderBytes(buf)
	dec.Decode(v)
	return dec.err
}

// NewDecoder returns a Decoder that decodes VBS from input stream r
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r:r, maxLength:MaxLength, maxDepth:MaxDepth}
}

func NewDecoderBytes(buf []byte) *Decoder {
	r := bytes.NewBuffer(buf)
	return &Decoder{r:r, maxLength:len(buf), maxDepth:len(buf)/2}
}

// SetMaxLength sets the max length of string and blob in the VBS encoded data
func (dec *Decoder) SetMaxLength(length int) {
	if length < 0 {
		dec.maxLength = MaxLength
	} else {
		dec.maxLength = length
	}
}

// SetMaxDepth sets the max depth of VBS dict and list
func (dec *Decoder) SetMaxDepth(depth int) {
	if depth < 0 {
		dec.maxDepth = MaxDepth
	} else {
		dec.maxDepth = depth
	}
}


// Decode reads the next VBS-encoded value from its input and stores it in the value pointed to by v.
func (dec *Decoder) Decode(data interface{}) error {
	v := reflect.ValueOf(data)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		dec.err = &InvalidUnmarshalError{v.Type()}
		return dec.err
	}
	dec.decodeReflectValue(v.Elem())
	return dec.err
}

func (dec *Decoder) headBuffer() []byte {
	left := dec.hEnd - dec.hStart
	if left < 16 && !dec.eof && dec.err == nil {
		if left > 0 {
			copy(dec.hBuffer[:], dec.hBuffer[dec.hStart:dec.hEnd])
		}
		dec.hStart = 0
		dec.hEnd = left

		n, err := dec.r.Read(dec.hBuffer[dec.hEnd:])
		if n > 0 {
			dec.hEnd += n
		} else if err != nil {
			if err == io.EOF {
				dec.eof = true
			} else {
				dec.err = err
			}
		}
	}
	return dec.hBuffer[dec.hStart:dec.hEnd]
}

func (dec *Decoder) read(buf []byte) (n int) {
	num := len(buf)
	left := dec.hEnd - dec.hStart
	if num <= left {
		copy(buf, dec.hBuffer[dec.hStart:dec.hEnd])
		dec.hStart += num
		n += num
	} else {
		copy(buf, dec.hBuffer[dec.hStart:dec.hEnd])
		dec.hStart = dec.hEnd
		n += left
		k, err := dec.r.Read(buf[left:num])
		if err != nil {
			if err == io.EOF {
				dec.err = &InvalidVbsError{}
			} else {
				dec.err = err
			}
		}
		n += k
	}
	return
}

func (dec *Decoder) getBytes(number int64) []byte {
	num := int(number)
	if num > dec.maxLength {
		dec.err = &InvalidVbsError{}
		return dec.bytes[:0]
	}

	if cap(dec.bytes) < num {
		dec.bytes = make([]byte, num)
	}
	dec.bytes = dec.bytes[:num]
	k := dec.read(dec.bytes)
	return dec.bytes[:k]
}

func (dec *Decoder) takeBytes(number int64) []byte {
	if number > int64(dec.maxLength) {
		dec.err = &InvalidVbsError{}
		return []byte{}
	}

	buf := make([]byte, number)
	k := dec.read(buf)
	return buf[:k]
}

func (dec *Decoder) decodeReflectValue(v reflect.Value) {
	if dec.err != nil {
		return
	}

	var decode decodeFunc
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		decode = (*Decoder).decodeIntValue

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		decode = (*Decoder).decodeUintValue

	case reflect.String:
		decode = (*Decoder).decodeStringValue

	case reflect.Bool:
		decode = (*Decoder).decodeBoolValue

	case reflect.Float32, reflect.Float64:
		decode = (*Decoder).decodeFloatValue

	case reflect.Complex64, reflect.Complex128:
		decode = (*Decoder).decodeComplexValue

	case reflect.Array:
		decode = (*Decoder).decodeArrayValue

	case reflect.Slice:
		decode = (*Decoder).decodeSliceValue

	case reflect.Map:
		decode = (*Decoder).decodeMapValue

	case reflect.Struct:
		decode = (*Decoder).decodeStructValue

	case reflect.Interface, reflect.Ptr:
		if v.IsNil() {
			dec.err = &InvalidUnmarshalError{v.Type()}
			return
		}
		v = v.Elem()
		decode = (*Decoder).decodeReflectValue

	// reflect.Func, reflect.Chan, reflect.UnsafePointer:
	default:
		dec.err = &UnsupportedTypeError{v.Type()}
		return
	}

	decode(dec, v)
}


var bitmapSingle = [8]uint32 {
        0xFB00C00E, /* 1111 1011 1111 1111  1000 0000 0000 1110 */

                    /* ?>=< ;:98 7654 3210  /.-, +*)( '&%$ #"!  */
        0xFFFFFFFF, /* 1111 1111 1111 1111  1111 1111 1111 1111 */

                    /* _^]\ [ZYX WVUT SRQP  ONML KJIH GFED CBA@ */
        0xFFFFFFFF, /* 1111 1111 1111 1111  1111 1111 1111 1111 */

                    /*  ~}| {zyx wvut srqp  onml kjih gfed cba` */
        0xFFFFFFFF, /* 1111 1111 1111 1111  1111 1111 1111 1111 */

        0x00000000, /* 0000 0000 0000 0000  0000 0000 0000 0000 */
        0x00000000, /* 0000 0000 0000 0000  0000 0000 0000 0000 */
        0x00000000, /* 0000 0000 0000 0000  0000 0000 0000 0000 */
        0x00000000, /* 0000 0000 0000 0000  0000 0000 0000 0000 */
}

var bitmapMulti = [8]uint32{
        0xF800400C, /* 1111 1000 1111 1111  0000 0000 0000 1100 */

                    /* ?>=< ;:98 7654 3210  /.-, +*)( '&%$ #"!  */
        0xFFFFFFFF, /* 1111 1111 1111 1111  1111 1111 1111 1111 */

                    /* _^]\ [ZYX WVUT SRQP  ONML KJIH GFED CBA@ */
        0xFFFFFFFF, /* 1111 1111 1111 1111  1111 1111 1111 1111 */

                    /*  ~}| {zyx wvut srqp  onml kjih gfed cba` */
        0xFFFFFFFF, /* 1111 1111 1111 1111  1111 1111 1111 1111 */

        0x00000000, /* 0000 0000 0000 0000  0000 0000 0000 0000 */
        0x00000000, /* 0000 0000 0000 0000  0000 0000 0000 0000 */
        0x00000000, /* 0000 0000 0000 0000  0000 0000 0000 0000 */
        0x00000000, /* 0000 0000 0000 0000  0000 0000 0000 0000 */
}

func bitmapTestSingle(x uint8) bool {
	return (bitmapSingle[x>>3] & (1 << (x & 0x1F))) != 0
}

func bitmapTestMulti(x uint8) bool {
	return (bitmapMulti[x>>3] & (1 << (x & 0x1F))) != 0
}

type vbsHead struct {
	kind Kind
	descriptor uint16
	num int64
}

func (dec *Decoder) unpackHeadKind(kind Kind, permitDescriptor bool) (head vbsHead) {
	head = dec.unpackHead()
	if dec.err == nil {
		if head.kind != kind {
			dec.err = &MismatchedKindError{Expect:kind, Got:head.kind}
		} else if (!permitDescriptor && head.descriptor != 0) {
			dec.err = &InvalidVbsError{}
		}
	}
	return
}

func (dec *Decoder) unpackHead() (head vbsHead) {
	if dec.err != nil {
		return
	}

	buf := dec.headBuffer()
	n := len(buf)
	negative := false
	kd := byte(0)
	descriptor := uint16(0)
	num := uint64(0)
	i := 0
again:
	for i < n {
		x := buf[i]
		i++
		if x < 0x80 {
			kd = x
			if x >= VBS_STRING {
				kd = (x & 0x60)
                                num = uint64(x & 0x1F)
				if kd == 0x60 {
					kd = VBS_INTEGER
					negative = true
				}
			} else if x >= VBS_BOOL {
				if x != VBS_BLOB {
					kd = (x & 0xFE)
				}
				if x <= VBS_BOOL + 1 {
					num = uint64(x & 0x01)
				}
				// For VBS_DECIMAL and VBS_FLOATING, the negative bit
				// has no effect when num == 0. So we ignore it.
                                // negative = (x & 0x01) != 0
			} else if x >= VBS_DESCRIPTOR {
				num = uint64(x & 0x07)
				if (num == 0) {
					if ((descriptor & VBS_SPECIAL_DESCRIPTOR) == 0) {
						descriptor |= VBS_SPECIAL_DESCRIPTOR
					} else {
						dec.err = &InvalidVbsError{}
						return
					}
				} else {
					if ((descriptor & VBS_DESCRIPTOR_MAX) == 0) {
						descriptor |= uint16(num)
					} else {
						dec.err = &InvalidVbsError{}
						return
					}
				}
				goto again
			} else if !bitmapTestSingle(x) {
				dec.err = &InvalidVbsError{}
				return
			}
		} else {
			shift := 0
			num = uint64(x & 0x7F)
			for {
				if i >= n {
					dec.err = &InvalidVbsError{}
					return
				}

				shift += 7
				x = buf[i]
				i++
				if x < 0x80 {
					break
				}

				x &= 0x7F
				left := 64 - shift
				if left <= 0 || (left < 7 && x >= (1 << uint(left))) {
					dec.err = &NumberOverflowError{64}
                                        return
                                }
				num |= uint64(x) << uint(shift)
			}

			kd = x
			if x >= VBS_STRING {
				kd = (x & 0x60)
				x &= 0x1F
				if x != 0 {
					left := 64 - shift
					if left <= 0 || (left < 7 && x >= (1 << uint(left))) {
						dec.err = &NumberOverflowError{64}
						return
					}
					num |= uint64(x) << uint(shift)
				}
				if kd == 0x60 {
					kd = VBS_INTEGER
					negative = true
				}
			} else if x >= VBS_DECIMAL {
                                kd = (x & 0xFE)
                                negative = (x & 0x01) != 0
			} else if x >= VBS_DESCRIPTOR && x < VBS_BOOL {
				x &= 0x07
				if x != 0 {
					left := 64 - shift
					if left <= 0 || (left < 7 && x >= (1 << uint(left))) {
						dec.err = &NumberOverflowError{64}
						return
					}
					num |= uint64(x) << uint(shift)
				}

				if num == 0 || num > VBS_DESCRIPTOR_MAX {
					dec.err = &InvalidVbsError{}
					return
				}

				if (descriptor & VBS_DESCRIPTOR_MAX) == 0 {
					descriptor |= uint16(num)
				} else {
					dec.err = &InvalidVbsError{}
					return
				}
				goto again
			} else if !bitmapTestMulti(x) {
				dec.err = &InvalidVbsError{}
				return
			}

			if num > math.MaxInt64 {
				/* overflow */
				if !(kd == VBS_INTEGER && negative && int64(num) == math.MinInt64) {
					dec.err = &NumberOverflowError{64}
					return
				}
			}
		}

		head.kind = Kind(kd)
		head.descriptor = descriptor
		head.num = int64(num)
		if negative {
			head.num = -head.num
		}
		dec.hStart += i
		return
	}

	dec.err = &InvalidVbsError{}
	return
}

func (dec *Decoder) unpackHeadOfList() (head vbsHead) {
	head = dec.unpackHeadKind(VBS_LIST, true)
	if dec.err == nil {
		dec.depth++
		if dec.depth > dec.maxDepth {
			dec.err = &DepthOverflowError{dec.maxDepth}
		}
	}
	return
}

func (dec *Decoder) unpackHeadOfDict() (head vbsHead) {
	head = dec.unpackHeadKind(VBS_DICT, true)
	if dec.err == nil {
		dec.depth++
		if dec.depth > dec.maxDepth {
			dec.err = &DepthOverflowError{dec.maxDepth}
		}
	}
	return
}

func (dec *Decoder) unpackTail() {
	if !dec.unpackIfTail() {
		if dec.err == nil {
			dec.err = &InvalidVbsError{}
		}
	}
}

func (dec *Decoder) unpackIfTail() bool {
	if dec.err == nil {
		buf := dec.headBuffer()
		if len(buf) > 0 && dec.depth > 0 && buf[0] == byte(VBS_TAIL) {
			dec.hStart++
			dec.depth--
			return true
		}
	}
	return false
}

type MismatchedKindError struct {
	Expect, Got Kind
}

func (e *MismatchedKindError) Error() string {
        return fmt.Sprintf("vbs: mismatched kind: expect %s, got %s", e.Expect.String(), e.Got.String())
}

type BadDataError struct {
}

func (e *BadDataError) Error() string {
        return "vbs: bad data"
}

type decodeFunc func (dec *Decoder, v reflect.Value)

func (dec *Decoder) decodeIntValue(v reflect.Value) {
	head := dec.unpackHeadKind(VBS_INTEGER, true)
	if dec.err == nil {
		v.SetInt(head.num)
	}
}

func (dec *Decoder) decodeUintValue(v reflect.Value) {
	head := dec.unpackHeadKind(VBS_INTEGER, true)
	if dec.err == nil {
		v.SetUint(uint64(head.num))
	}
}

func (dec *Decoder) decodeStringValue(v reflect.Value) {
	head := dec.unpackHeadKind(VBS_STRING, true)
	if dec.err == nil {
		buf := dec.getBytes(head.num)
		if dec.err == nil {
			v.SetString(string(buf))
		}
	}
}

func (dec *Decoder) unpackByteArray() []byte {
	head := dec.unpackHeadKind(VBS_BLOB, true)
	if dec.err == nil {
		buf := dec.getBytes(head.num)
		if dec.err == nil {
			return buf
		}
	}
	return []byte{}
}

func (dec *Decoder) unpackByteSlice() []byte {
	head := dec.unpackHeadKind(VBS_BLOB, true)
	if dec.err == nil {
		buf := dec.takeBytes(head.num)
		if dec.err == nil {
			return buf
		}
	}
	return []byte{}
}

func (dec *Decoder) decodeBoolValue(v reflect.Value) {
	head := dec.unpackHeadKind(VBS_BOOL, true)
	if dec.err == nil {
		v.SetBool(head.num != 0)
	}
}

func (dec *Decoder) unpackFloat() (r float64) {
	head := dec.unpackHeadKind(VBS_FLOATING, true)
	head2 := dec.unpackHeadKind(VBS_INTEGER, false)
	if dec.err == nil {
		mantissa := head.num
		expo := int(head2.num)
		r = makeFloat(mantissa, expo)
	}
	return 
}

func (dec *Decoder) unpackDecimal64() (r Decimal64) {
	head := dec.unpackHeadKind(VBS_DECIMAL, true)
	head2 := dec.unpackHeadKind(VBS_INTEGER, false)
	if dec.err == nil {
		mantissa := head.num
		expo := int(head2.num)
		r = makeDecimal64(mantissa, expo)
	}
	return 
}

func (dec *Decoder) decodeFloatValue(v reflect.Value) {
	f := dec.unpackFloat()
	if dec.err == nil {
		v.SetFloat(f)
	}
}

func (dec *Decoder) decodeComplexValue(v reflect.Value) {
	dec.unpackHeadOfList()
	real := dec.unpackFloat()
	img := dec.unpackFloat()
	dec.unpackTail()
	if dec.err == nil {
		v.SetComplex(complex(real, img))
	}
}

func (dec *Decoder) decodeArrayValue(v reflect.Value) {
	if v.Type().Elem().Kind() == reflect.Uint8 {
		buf := dec.unpackByteArray()
		if dec.err == nil {
			if len(buf) > v.Len() {
				dec.err = &ArrayOverflowError{v.Len()}
			} else {
				for i := 0; i < len(buf); i++ {
					v.Index(i).SetUint(uint64(buf[i]))
				}
			}
		}
		return
	}

	dec.unpackHeadOfList()
	for i := 0; dec.err == nil; i++ {
		if dec.unpackIfTail() {
			break
		}

		if i >= v.Len() {
			dec.err = &ArrayOverflowError{v.Len()}
			return
		}

		dec.decodeReflectValue(v.Index(i))
	}
}

func (dec *Decoder) decodeSliceValue(v reflect.Value) {
	if v.Type().Elem().Kind() == reflect.Uint8 {	// VBS_BLOB
		buf := dec.unpackByteSlice()
		if dec.err == nil {
			v.SetBytes(buf)
		}
		return
	}

	head := dec.unpackHeadOfList()
	if dec.err != nil {
		if head.kind == VBS_NULL {
			dec.err = nil
			if !v.IsNil() {
				v.Set(reflect.Zero(v.Type()))
			}
		}
		return
	}

	for i := 0; dec.err == nil; i++ {
		if dec.unpackIfTail() {
			if i == 0 && v.IsNil() {
				v.Set(reflect.MakeSlice(v.Type(), 0, 0))
			}
			break
		}

		if i >= v.Cap() {
			newcap := v.Cap() + v.Cap()/2
			if newcap < 4 {
				newcap = 4
			}
			newv := reflect.MakeSlice(v.Type(), v.Len(), newcap)
			reflect.Copy(newv, v)
			v.Set(newv)
		}
		if i >= v.Len() {
			v.SetLen(i + 1)
		}

		dec.decodeReflectValue(v.Index(i))
	}
}

func (dec *Decoder) decodeMapValue(v reflect.Value) {
	head := dec.unpackHeadOfDict()
	if dec.err != nil {
		if head.kind == VBS_NULL {
			dec.err = nil
			if !v.IsNil() {
				v.Set(reflect.Zero(v.Type()))
			}
		}
		return
	}

	tp := v.Type()
	if v.IsNil() {
		v.Set(reflect.MakeMap(tp))
	}
	keyType := tp.Key()
	elemType := tp.Elem()
	for dec.err == nil {
		if dec.unpackIfTail() {
			break
		}

		key := reflect.New(keyType)
		key = key.Elem()
		dec.decodeReflectValue(key)
		if dec.err != nil {
			break
		}

		elem := reflect.New(elemType)
		elem = elem.Elem()
		dec.decodeReflectValue(elem)
		if dec.err != nil {
			break
		}

		v.SetMapIndex(key, elem)
	}
}

func (dec *Decoder) decodeStructValue(v reflect.Value) {
	dec.unpackHeadOfDict()
	for dec.err == nil {
		if dec.unpackIfTail() {
			break
		}
		// TODO
	}
}

