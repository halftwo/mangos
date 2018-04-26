package vbs

import (
	"io"
	"math"
	"reflect"
	"bytes"
)


// A Decoder reads and decodes VBS values from an input stream.
type Decoder struct {
	r io.Reader
	pos int
	maxLength int
	maxStrLength int
	maxDepth int16
	depth int16
	eof bool
	finished bool
	err error
	bytes []byte
	hStart int16
	hEnd int16
	hBuffer [32]byte
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
	return &Decoder{r:r, maxLength:MaxLength, maxStrLength:MaxStringLength, maxDepth:MaxDepth}
}

func NewDecoderLength(r io.Reader, maxLength int) *Decoder {
	maxString := MaxStringLength
	if maxLength > 0 && maxString >= maxLength {
		maxString = maxLength - 1
	}

	return &Decoder{r:r, maxLength:maxLength, maxStrLength:maxString, maxDepth:MaxDepth}
}

func NewDecoderBytes(buf []byte) *Decoder {
	r := bytes.NewBuffer(buf)
	return NewDecoderLength(r, len(buf))
}

// SetMaxLength sets the max length of string and blob in the VBS encoded data
func (dec *Decoder) SetMaxLength(length int) {
	dec.maxLength = length
}

// SetMaxDepth sets the max depth of VBS dict and list
func (dec *Decoder) SetMaxDepth(depth int) {
	if depth < 0 {
		dec.maxDepth = MaxDepth
	} else if depth > math.MaxInt16 {
		dec.maxDepth = math.MaxInt16
	} else {
		dec.maxDepth = int16(depth)
	}
}

func (dec *Decoder) SetMaxStringLength(length int) {
	if length < 0 {
		dec.maxStrLength = MaxStringLength
	} else {
		dec.maxStrLength = length
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

func (dec *Decoder) More() bool {
	if dec.hEnd != dec.hStart {
		return true
	}

	if dec.eof || dec.finished {
		return false
	}

	dec.headBuffer()
	return !((dec.eof || dec.finished) && dec.hEnd == dec.hStart)
}

func (dec *Decoder) left() int {
	if dec.maxLength > 0 {
		return (dec.maxLength - dec.pos) + int(dec.hEnd - dec.hStart)
	}
	return math.MaxInt64
}

func (dec *Decoder) headBuffer() []byte {
	hsize := dec.hEnd - dec.hStart
	if hsize < 16 && !dec.eof && !dec.finished && dec.err == nil {
		if hsize > 0 {
			copy(dec.hBuffer[:], dec.hBuffer[dec.hStart:dec.hEnd])
		}
		dec.hStart = 0
		dec.hEnd = hsize

		need := dec.left()
		if need > len(dec.hBuffer) {
			need = len(dec.hBuffer)
		}

		k, err := dec.r.Read(dec.hBuffer[dec.hEnd:need])
		if k > 0 {
			dec.hEnd += int16(k)
			dec.pos += k
			if dec.maxLength > 0 && dec.pos >= dec.maxLength {
				dec.finished = true
			}
		}
		if err != nil {
			if err == io.EOF {
				dec.eof = true
			} else {
				dec.err = err
			}
		}
	}
	return dec.hBuffer[dec.hStart:dec.hEnd]
}

func (dec *Decoder) mustRead(buf []byte) (n int) {
	num := len(buf)
	hsize := int(dec.hEnd - dec.hStart)
	if num <= hsize {
		copy(buf, dec.hBuffer[dec.hStart:dec.hEnd])
		dec.hStart += int16(num)
		n += num
	} else {
		left := dec.left()
		if num > left {
			num = left
		}

		copy(buf, dec.hBuffer[dec.hStart:dec.hEnd])
		dec.hStart = dec.hEnd
		n += hsize

		k, err := dec.r.Read(buf[hsize:num])
		if k > 0 {
			n += k
			dec.pos += k
			if dec.maxLength > 0 && dec.pos >= dec.maxLength {
				dec.finished = true
				if n < len(buf) {
					dec.err = &InvalidVbsError{}
				}
			}
		}

		if err != nil {
			if err == io.EOF {
				dec.eof = true
				if n < len(buf) {
					dec.err = &InvalidVbsError{}
				}
			} else {
				dec.err = err
			}
		}
	}
	return
}

func (dec *Decoder) getBytes(number int64) []byte {
	num := int(number)
	if num > dec.left() || num > dec.maxStrLength {
		dec.err = &InvalidVbsError{}
		return dec.bytes[:0]
	}

	if cap(dec.bytes) < num {
		dec.bytes = make([]byte, num)
	}
	dec.bytes = dec.bytes[:num]
	k := dec.mustRead(dec.bytes)
	return dec.bytes[:k]
}

func (dec *Decoder) takeBytes(number int64) []byte {
	num := int(number)
	if num > dec.left() || num > dec.maxStrLength {
		dec.err = &InvalidVbsError{}
		return []byte{}
	}

	buf := make([]byte, num)
	k := dec.mustRead(buf)
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
		for v.IsValid() {
			elem := v.Elem()
			k := elem.Kind()
			if k != reflect.Ptr && k != reflect.Interface {
				break
			}
			v = elem
		}

		if v.Kind() == reflect.Ptr {
			p := reflect.New(v.Type().Elem())
			v.Set(p)
			dec.decodeReflectValue(p.Elem())
		} else if v.Kind() == reflect.Interface {
			if v.NumMethod() == 0 {
				x := dec.decodeInterface()
				if dec.err == nil {
					v.Set(reflect.ValueOf(x))
				}
			} else {
				dec.err = &NonEmptyInterfaceError{}
			}
		}
		return

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
		dec.hStart += int16(i)
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

func (dec *Decoder) unpackString() string {
	head := dec.unpackHeadKind(VBS_STRING, true)
	if dec.err == nil {
		buf := dec.getBytes(head.num)
		if dec.err == nil {
			return string(buf)
		}
	}
	return ""
}

func (dec *Decoder) decodeStringValue(v reflect.Value) {
	s := dec.unpackString()
	if dec.err == nil {
		v.SetString(s)
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
			if len(buf) != v.Len() {
				dec.err = &ArrayLengthError{v.Len()}
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
			if i != v.Len() {
				dec.err = &ArrayLengthError{v.Len()}
			}
			break
		}

		if i >= v.Len() {
			dec.err = &ArrayLengthError{v.Len()}
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
			// No error, leave the v alone
			dec.err = nil
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
			// No error, leave the v alone
			dec.err = nil
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
	fields := cachedTypeFields(v.Type())

	for dec.err == nil {
		if dec.unpackIfTail() {
			break
		}

		name := dec.unpackString()
		if dec.err != nil {
			break
		}

		i, j := 0, len(fields)
		for i < j {
			m := int(uint(i+j) >> 1) // avoid overflow
			if fields[m].name >= name {
				j = m
			} else {
				i = m + 1
			}
		}

		if i >= len(fields) || fields[i].name != name {
			continue
		}

		f := &fields[i]
		dec.decodeReflectValue(v.Field(f.index))
	}
}

func (dec *Decoder) decodeInterface() (x interface{}) {
	head := dec.unpackHead()
	if dec.err != nil {
		return
	}

	switch head.kind {
	case VBS_INTEGER:
		x = head.num

	case VBS_STRING:
		buf := dec.getBytes(head.num)
		if dec.err == nil {
			x = string(buf)
		}

	case VBS_FLOATING:
		head2 := dec.unpackHeadKind(VBS_INTEGER, false)
		if dec.err == nil {
			x = makeFloat(head.num, int(head2.num))
		}
		
	case VBS_DECIMAL:
		head2 := dec.unpackHeadKind(VBS_INTEGER, false)
		if dec.err == nil {
			x = makeDecimal64(head.num, int(head2.num))
		}

	case VBS_BLOB:
		buf := dec.takeBytes(head.num)
		if dec.err == nil {
			x = buf
		}

	case VBS_BOOL:
		x = bool(head.num != 0)

	case VBS_LIST:
		x = dec.decodeInterfaceSlice()

	case VBS_DICT:
		x = dec.decodeInterfaceMap()

	case VBS_NULL:
		/* Do nothing */

	default:
		dec.err = &InvalidVbsError{}
	}

	return
}

func (dec *Decoder) decodeInterfaceSlice() (r interface{}) {
	dec.depth++
	if dec.depth > dec.maxDepth {
		dec.err = &DepthOverflowError{dec.maxDepth}
		return
	}

	s := make([]interface{}, 0)
	for i := 0; dec.err == nil; i++ {
		if dec.unpackIfTail() {
			break
		}

		x := dec.decodeInterface()
		if dec.err != nil {
			return
		}

		s = append(s, x)
	}
	r = s
	return
}

func (dec *Decoder) decodeInterfaceMap() (r interface{}) {
	dec.depth++
	if dec.depth > dec.maxDepth {
		dec.err = &DepthOverflowError{dec.maxDepth}
		return
	}

	var mi map[int64]interface{}
	var ms map[string]interface{}

	kind := reflect.Invalid
	for dec.err == nil {
		if dec.unpackIfTail() {
			break
		}

		k := dec.decodeInterface()
		v := dec.decodeInterface()
		if dec.err != nil {
			return
		}

		kk := reflect.ValueOf(k)
		if kind == reflect.Invalid {
			kind = kk.Kind()
			switch kind {
			case reflect.Int64:
				mi = make(map[int64]interface{})
				r = mi
			case reflect.String:
				ms = make(map[string]interface{})
				r = ms
			case reflect.Bool, reflect.Float64, reflect.Slice, reflect.Map:
				dec.err = &InvalidUnmarshalError{kk.Type()}	// TODO
				return
			default:
				panic("vbs: can't reach here!")
			}
		} else if kk.Kind() != kind {
			dec.err = &InvalidUnmarshalError{kk.Type()}	// TODO
			return
		}

		switch kind {
		case reflect.Int64:
			mi[kk.Int()] = v
		case reflect.String:
			ms[kk.String()] = v
		default:
			panic("vbs: can't reach here!")
		}
	}
	return
}

