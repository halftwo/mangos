package vbs

import (
	"io"
	"math"
	"reflect"
	"bytes"
	"halftwo/mangos/xerr"
)

const BUFFER_SIZE = 4096

// A Decoder reads and decodes VBS values from an input stream.
type Decoder struct {
	r io.Reader
	size int
	MaxLength int	// max length of the VBS encoded data, default vbs.MaxLength
	MaxStrLen int	// max length of string and blob in the VBS encoded data, default vbs.MaxStringLength
	MaxDepth int16 	// max depth of VBS dict and list, default vbs.MaxDepth
	depth int16
	finished bool
	nocopy bool
	unread bool
	lastByte byte
	err error
	buffer []byte
}

func (dec *Decoder) _read_blob(data []byte) (n int) {
	k := len(data)
	if k > dec.left() {
		dec.finished = true
		dec.err = xerr.Trace(&DataLackError{Expect:int64(k), Got:dec.left()})
		return
	} else if k <= 0 {
		return
	}

	var err error
	if dec.unread {
		dec.unread = false
		data[0] = dec.lastByte
		if k > 1 {
			n, err = dec.r.Read(data[1:])
			if n > 0 {
				dec.lastByte = data[n]
			}
		}
		n += 1
	} else {
		n, err = dec.r.Read(data)
		if n > 0 {
			dec.lastByte = data[n-1]
		}
	}

	if err != nil {
		if err == io.EOF {
			dec.finished = true
		}
		dec.err = xerr.Trace(&DataLackError{Expect:int64(k), Got:n})
	}

	dec.size += n
	if dec.size == dec.MaxLength {
		dec.finished = true
	}
	return
}

func (dec *Decoder) readByte() byte {
	if dec.unread {
		dec.unread = false
		return dec.lastByte
	}

	if dec.err == nil {
		var buf [1]byte
		_, err := dec.r.Read(buf[:1])
		if err == nil {
			dec.lastByte = buf[0]
			dec.size++
			if dec.size == dec.MaxLength {
				dec.finished = true
			}
			return dec.lastByte
		}

		if err == io.EOF {
			dec.finished = true
		}
		dec.err = xerr.Trace(&DataLackError{Expect:1, Got:0})
	}
	return 0
}

func (dec *Decoder) unreadByte() {
	if dec.unread {
		panic("unreadByte been called twice")
	}

	dec.unread = true
}

// Unmarshal decodes the VBS-encoded data and stores the result in the value pointed to by v.
// Return consumed number of bytes
// If v is nil or not a pointer, Unmarshal returns an InvalidUnmarshalError.
func Unmarshal(buf []byte, v any) (n int, err error) {
	r := bytes.NewBuffer(buf)
	dec := NewDecoderLength(r, len(buf))
	dec.Decode(v)
	return dec.size, dec.err
}

// NewDecoder returns a Decoder that decodes VBS from input stream r
func NewDecoder(r io.Reader) *Decoder {
	if b, ok := r.(*bytes.Buffer); ok {
		return NewDecoderLength(r, b.Len())
	}
	return NewDecoderLength(r, MaxLength)
}

// If owner is true, the Decoder owns the buf, the decoded []byte (vbs blob) 
// points to the buf (i.e. not copied from the buf).
func NewDecoderBytes(buf []byte, owner bool) *Decoder {
	r := bytes.NewBuffer(buf)
	return &Decoder{r:r, MaxLength:len(buf), MaxStrLen:MaxStringLength, MaxDepth:MaxDepth, nocopy:owner}
}

func NewDecoderLength(r io.Reader, maxLength int) *Decoder {
	return &Decoder{r:r, MaxLength:maxLength, MaxStrLen:MaxStringLength, MaxDepth:MaxDepth}
}


// Decode reads and decode the next VBS-encoded value from its input and 
// stores it into the value pointed to by out.
// out must be a non-nil pointer or map.
func (dec *Decoder) Decode(out any) error {
	valid := false
	v := reflect.ValueOf(out)
	switch v.Kind() {
	case reflect.Pointer:
		if !v.IsNil() {
			valid = true
			v = v.Elem()
		}
	case reflect.Map:
		valid = !v.IsNil()
	}

	if !valid {
		panic("out must be a pointer or a map, and not nil.")
	}

	dec.decodeReflectValue(v)
	return dec.err
}

func (dec *Decoder) Err() error {
	return dec.err
}

func (dec *Decoder) More() bool {
	return !dec.finished && dec.left() > 0
}

func (dec *Decoder) Size() int {
	return dec.size
}

func (dec *Decoder) left() int {
	return (dec.MaxLength - dec.size)
}

func (dec *Decoder) _bytesbuffer_next(num int) []byte {
	if num > dec.left() {
		dec.err = xerr.Trace(&DataLackError{Expect:int64(num), Got:dec.left()})
		return nil
	}
	if dec.unread {
		dec.unread = false
		err := dec.r.(*bytes.Buffer).UnreadByte()
		if err != nil {
			panic("Can't reach here")
		}
	}

	dec.size += num
	return dec.r.(*bytes.Buffer).Next(num)
}

func (dec *Decoder) _get_bytes(number int64, take bool) []byte {
	if number > int64(dec.MaxStrLen) {
		dec.err = xerr.Trace(&StringTooLongError{Got:number, Max:dec.MaxStrLen})
		return nil
	}

	num := int(number)
	if num <= 0 {
		return nil
	}

	if dec.nocopy {
		return dec._bytesbuffer_next(num)
	}

	if take {
		buf := make([]byte, 0, num)
		k := dec._read_blob(buf[:num])
		return buf[:k]
	}

	if _, ok := dec.r.(*bytes.Buffer); ok {
		return dec._bytesbuffer_next(num)
	}

	if cap(dec.buffer) < num {
		dec.buffer = make([]byte, 0, num)
	}
	k := dec._read_blob(dec.buffer[:num])
	return dec.buffer[:k]
}

func min[T ~int|~int64](a, b T) T {
	if a <= b {
		return a
	}
	return b
}

func (dec *Decoder) discardBytes(number int64) {
	if number > int64(dec.MaxStrLen) {
		dec.err = xerr.Trace(&StringTooLongError{Got:number, Max:dec.MaxStrLen})
	}

	num := int(number)
	if num <= 0 {
		return
	}

	if _, ok := dec.r.(*bytes.Buffer); ok {
		dec._bytesbuffer_next(num)
		return
	}

	bufcap := min(BUFFER_SIZE, num)
	if cap(dec.buffer) < bufcap {
		dec.buffer = make([]byte, 0, bufcap)
	}
	for num > 0 && dec.err == nil {
		k := min(bufcap, num)
		num -= k
		dec._read_blob(dec.buffer[:k])
	}
}

func (dec *Decoder) getBytes(number int64) []byte {
	return dec._get_bytes(number, false)
}

func (dec *Decoder) takeBytes(number int64) []byte {
	return dec._get_bytes(number, true)
}

func (dec *Decoder) decodeReflectValue(v reflect.Value) {
	if dec.err != nil {
		return
	}

	var decode _DecodeFunc
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

	case reflect.Interface, reflect.Pointer:
		for v.IsValid() {
			elem := v.Elem()
			ek := elem.Kind()
			if ek != reflect.Pointer && ek != reflect.Interface {
				break
			}
			v = elem
		}

		if v.Kind() == reflect.Pointer {
			if v.IsNil() {
				p := reflect.New(v.Type().Elem())
				v.Set(p)
			}
			dec.decodeReflectValue(v.Elem())
		} else if v.Kind() == reflect.Interface {
			if v.IsNil() {
				if v.NumMethod() == 0 {
					x := dec.decodeAny()
					if dec.err == nil {
						v.Set(reflect.ValueOf(x))
					}
				} else {
					dec.err = xerr.Trace(&NonEmptyInterfaceError{})
				}
			} else {
				dec.decodeReflectValue(v.Elem())
			}
		}
		return

	// reflect.Func, reflect.Chan, reflect.UnsafePointer:
	default:
		dec.err = xerr.Trace(&UnsupportedTypeError{v.Type()})
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

type _HeadInfo struct {
	vbsKind VbsKind
	descriptor uint16
	num int64
}

func (dec *Decoder) unpackHeadKind(vbsKind VbsKind, permitDescriptor bool) (head _HeadInfo) {
	head = dec.unpackHead()
	if dec.err == nil {
		if head.vbsKind != vbsKind {
			dec.err = xerr.Trace(&MismatchedKindError{Expect:vbsKind, Got:head.vbsKind})
		} else if (!permitDescriptor && head.descriptor != 0) {
			dec.err = xerr.Trace(&InvalidVbsError{})
		}
	}
	return
}

func (dec *Decoder) unpackHead() (head _HeadInfo) {
	if dec.err != nil {
		return
	}

	negative := false
	vbsKind := byte(0)
	descriptor := uint16(0)
	num := uint64(0)
again:
	for {
		x := dec.readByte()
		if dec.err != nil {
			return
		}

		if x < 0x80 {
			vbsKind = x
			if x >= VBS_STRING {
				vbsKind = (x & 0x60)
                                num = uint64(x & 0x1F)
				if vbsKind == 0x60 {
					vbsKind = VBS_INTEGER
					negative = true
				}
			} else if x >= VBS_BOOL {
				if x != VBS_BLOB {
					vbsKind = (x & 0xFE)
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
						dec.err = xerr.Trace(&InvalidVbsError{})
						return
					}
				} else {
					if ((descriptor & VBS_DESCRIPTOR_MAX) == 0) {
						descriptor |= uint16(num)
					} else {
						dec.err = xerr.Trace(&InvalidVbsError{})
						return
					}
				}
				goto again
			} else if !bitmapTestSingle(x) {
				dec.err = xerr.Trace(&InvalidVbsError{})
				return
			}
		} else {
			shift := 0
			num = uint64(x & 0x7F)
			for {
				x = dec.readByte()
				if dec.err != nil {
					return
				}

				shift += 7
				if x < 0x80 {
					break
				}

				x &= 0x7F
				left := 64 - shift
				if left <= 0 || (left < 7 && x >= (1 << uint(left))) {
					dec.err = xerr.Trace(&NumberOverflowError{64})
                                        return
                                }
				num |= uint64(x) << uint(shift)
			}

			vbsKind = x
			if x >= VBS_STRING {
				vbsKind = (x & 0x60)
				x &= 0x1F
				if x != 0 {
					left := 64 - shift
					if left <= 0 || (left < 7 && x >= (1 << uint(left))) {
						dec.err = xerr.Trace(&NumberOverflowError{64})
						return
					}
					num |= uint64(x) << uint(shift)
				}
				if vbsKind == 0x60 {
					vbsKind = VBS_INTEGER
					negative = true
				}
			} else if x >= VBS_DECIMAL {
                                vbsKind = (x & 0xFE)
                                negative = (x & 0x01) != 0
			} else if x >= VBS_DESCRIPTOR && x < VBS_BOOL {
				x &= 0x07
				if x != 0 {
					left := 64 - shift
					if left <= 0 || (left < 7 && x >= (1 << uint(left))) {
						dec.err = xerr.Trace(&NumberOverflowError{64})
						return
					}
					num |= uint64(x) << uint(shift)
				}

				if num == 0 || num > VBS_DESCRIPTOR_MAX {
					dec.err = xerr.Trace(&InvalidVbsError{})
					return
				}

				if (descriptor & VBS_DESCRIPTOR_MAX) == 0 {
					descriptor |= uint16(num)
				} else {
					dec.err = xerr.Trace(&InvalidVbsError{})
					return
				}
				goto again
			} else if !bitmapTestMulti(x) {
				dec.err = xerr.Trace(&InvalidVbsError{})
				return
			}

			if num > math.MaxInt64 {
				/* overflow */
				if !(vbsKind == VBS_INTEGER && negative && int64(num) == math.MinInt64) {
					dec.err = xerr.Trace(&NumberOverflowError{64})
					return
				}
			}
		}

		head.vbsKind = VbsKind(vbsKind)
		head.descriptor = descriptor
		head.num = int64(num)
		if negative {
			head.num = -head.num
		}
		return
	}

	dec.err = xerr.Trace(&InvalidVbsError{})
	return
}

func (dec *Decoder) unpackHeadOfList() (head _HeadInfo) {
	head = dec.unpackHeadKind(VBS_LIST, true)
	if dec.err == nil {
		dec.enterCompound()
	}
	return
}

func (dec *Decoder) unpackHeadOfDict() (head _HeadInfo) {
	head = dec.unpackHeadKind(VBS_DICT, true)
	if dec.err == nil {
		dec.enterCompound()
	}
	return
}

func (dec *Decoder) unpackTail() {
	if !dec.unpackIfTail() {
		if dec.err == nil {
			dec.err = xerr.Trace(&InvalidVbsError{})
		}
	}
}

func (dec *Decoder) unpackIfTail() bool {
	if dec.err == nil {
		x := dec.readByte()
		if dec.err != nil {
			return false
		}

		if dec.depth > 0 && x == byte(VBS_TAIL) {
			dec.depth--
			return true
		}
		dec.unreadByte()
	}
	return false
}

func (dec *Decoder) enterCompound() bool {
	if dec.depth >= dec.MaxDepth {
		dec.err = xerr.Trace(&DepthOverflowError{dec.MaxDepth})
		return false
	}
	dec.depth++
	return true
}

type _DecodeFunc func (dec *Decoder, v reflect.Value)

func (dec *Decoder) decodeIntValue(v reflect.Value) {
	head := dec.unpackHeadKind(VBS_INTEGER, true)
	if dec.err == nil {
		v.SetInt(head.num)
		if v.Int() != head.num {
			dec.err = xerr.Trace(&IntegerOverflowError{kind:v.Kind(), value:head.num})
		}
	}
}

func (dec *Decoder) decodeUintValue(v reflect.Value) {
	head := dec.unpackHeadKind(VBS_INTEGER, true)
	if dec.err == nil {
		v.SetUint(uint64(head.num))
		if v.Uint() != uint64(head.num) {
			dec.err = xerr.Trace(&IntegerOverflowError{kind:v.Kind(), value:head.num})
		}
	}
}

func (dec *Decoder) decodeStringValue(v reflect.Value) {
	head := dec.unpackHeadKind(VBS_STRING, true)
	if dec.err != nil {
		return
	}

	buf := dec.getBytes(head.num)
	if dec.err == nil {
		v.SetString(string(buf))
	}
}

func (dec *Decoder) decodeBoolValue(v reflect.Value) {
	head := dec.unpackHeadKind(VBS_BOOL, true)
	if dec.err == nil {
		v.SetBool(head.num != 0)
	}
}

func (dec *Decoder) unpackFloat() (r float64) {
	head := dec.unpackHead()
	if dec.err != nil {
		return
	}

	if head.vbsKind == VBS_INTEGER {
		r = float64(head.num)
	} else if head.vbsKind == VBS_FLOATING {
		head2 := dec.unpackHeadKind(VBS_INTEGER, false)
		if dec.err == nil {
			mantissa := head.num
			expo := int(head2.num)
			r = makeFloat(mantissa, expo)
		}
	} else {
		dec.err = xerr.Trace(&MismatchedKindError{Expect:VBS_FLOATING, Got:head.vbsKind})
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
	if v.Type().Elem().Kind() == reflect.Uint8 {	// VBS_BLOB
		head := dec.unpackHeadKind(VBS_BLOB, true)
		if dec.err != nil {
			return
		}
		buf := v.Slice(0, v.Len()).Interface().([]byte)
		if head.num != int64(len(buf)) {
			dec.err = xerr.Trace(&ArrayLengthError{len(buf)})
			return
		}
		dec._read_blob(buf)
		return
	}

	dec.unpackHeadOfList()
	for i := 0; dec.err == nil; i++ {
		if dec.unpackIfTail() {
			if i != v.Len() {
				dec.err = xerr.Trace(&ArrayLengthError{v.Len()})
			}
			break
		}

		if i >= v.Len() {
			dec.err = xerr.Trace(&ArrayLengthError{v.Len()})
			return
		}

		dec.decodeReflectValue(v.Index(i))
	}
}

func (dec *Decoder) decodeSliceValue(v reflect.Value) {
	if v.Type().Elem().Kind() == reflect.Uint8 {	// VBS_BLOB
		head := dec.unpackHead()
		if dec.err != nil {
			return
		}

		if head.vbsKind != VBS_BLOB && head.vbsKind != VBS_STRING {
			dec.err = xerr.Trace(&MismatchedKindError{Expect:VBS_BLOB, Got:head.vbsKind})
			return
		}

		buf := dec.takeBytes(head.num)
		if dec.err == nil {
			v.SetBytes(buf)
		}
		return
	}

	head := dec.unpackHeadOfList()
	if dec.err != nil {
		if head.vbsKind == VBS_NULL {
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
		if head.vbsKind == VBS_NULL {
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
	fields := GetStructFieldInfos(v.Type())

	for dec.err == nil {
		if dec.unpackIfTail() {
			break
		}

		head := dec.unpackHead()
		if dec.err != nil {
			return
		}

		var f *_FieldInfo
		switch head.vbsKind {
		case VBS_INTEGER:
			f = fields.FindInt(head.num)

		case VBS_STRING:
			buf := dec.getBytes(head.num)
			f = fields.FindNameBlob(buf)
		default:
			dec.err = xerr.Trace(&MismatchedKindError{Expect:VBS_STRING, Got:head.vbsKind})
			return
		}

		if f == nil {
			// unknown field, discard value
			dec.discardAny()
			continue
		}

		dec.decodeReflectValue(v.Field(int(f.Idx)))
	}
}

func (dec *Decoder) discardAny() {
	head := dec.unpackHead()
	if dec.err != nil {
		return
	}

	switch head.vbsKind {
	case VBS_STRING, VBS_BLOB:
		dec.discardBytes(head.num)

	case VBS_FLOATING, VBS_DECIMAL:
		dec.unpackHeadKind(VBS_INTEGER, false)

	case VBS_LIST:
		dec.discardAnySlice()

	case VBS_DICT:
		dec.discardAnyMap()

	case VBS_INTEGER, VBS_BOOL, VBS_NULL:
		/* Do nothing */

	default:
		dec.err = xerr.Trace(&InvalidVbsError{})
	}

	return
}

func (dec *Decoder) decodeAny() (x any) {
	head := dec.unpackHead()
	if dec.err != nil {
		return
	}

	switch head.vbsKind {
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
		x = dec.decodeAnySlice()

	case VBS_DICT:
		x = dec.decodeAnyMap()

	case VBS_NULL:
		/* Do nothing */

	default:
		dec.err = xerr.Trace(&InvalidVbsError{})
	}

	return
}

func (dec *Decoder) discardAnySlice() {
	if !dec.enterCompound() {
		return
	}

	for dec.err == nil {
		if dec.unpackIfTail() {
			break
		}

		dec.discardAny()
	}
}

func (dec *Decoder) decodeAnySlice() (r any) {
	if !dec.enterCompound() {
		return
	}

	s := make([]any, 0)
	for i := 0; dec.err == nil; i++ {
		if dec.unpackIfTail() {
			break
		}

		x := dec.decodeAny()
		if dec.err != nil {
			return
		}

		s = append(s, x)
	}
	r = s
	return
}

func (dec *Decoder) discardAnyMap() {
	if !dec.enterCompound() {
		return
	}

	for dec.err == nil {
		if dec.unpackIfTail() {
			break
		}

		dec.discardAny()
		dec.discardAny()
	}
}

func (dec *Decoder) decodeAnyMap() (r any) {
	if !dec.enterCompound() {
		return
	}

	var mi map[int64]any
	var ms map[string]any

	kind := reflect.Invalid
	for dec.err == nil {
		if dec.unpackIfTail() {
			break
		}

		k := dec.decodeAny()
		v := dec.decodeAny()
		if dec.err != nil {
			return
		}

		kk := reflect.ValueOf(k)
		if kind == reflect.Invalid {
			kind = kk.Kind()
			switch kind {
			case reflect.Int64:
				mi = make(map[int64]any)
				r = mi
			case reflect.String:
				ms = make(map[string]any)
				r = ms
			case reflect.Bool, reflect.Float64, reflect.Slice, reflect.Map:
				dec.err = xerr.Trace(&InvalidUnmarshalError{kk.Type()})	// TODO
				return
			default:
				panic("vbs: can't reach here!")
			}
		} else if kk.Kind() != kind {
			dec.err = xerr.Trace(&InvalidUnmarshalError{kk.Type()})	// TODO
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

