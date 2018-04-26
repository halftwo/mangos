package vbs

import (
	"io"
	"reflect"
	"bytes"
	"math"
)

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
	maxDepth int16
	depth int16
	err error
}

// NewEncoder returns a new Encoder that writes to w
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w:w, maxDepth:MaxDepth}
}

// SetMaxDepth set the max depth of VBS dict and list
func (enc *Encoder) SetMaxDepth(depth int) {
	if depth < 0 {
		enc.maxDepth = MaxDepth
	} else if depth > math.MaxInt16 {
		enc.maxDepth = math.MaxInt16
	} else {
		enc.maxDepth = int16(depth)
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
		if v.IsNil() {
			enc.encodeNil()
		} else {
			enc.encodeReflectValue(v.Elem())
		}

	// reflect.Func, reflect.Chan, reflect.UnsafePointer:
	default:
		enc.err = &UnsupportedTypeError{v.Type()}
	}
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
	isSlice := (v.Kind() == reflect.Slice)
	if v.Type().Elem().Kind() == reflect.Uint8 {	// VBS_BLOB
		var buf []byte
		if isSlice {
			buf = v.Bytes()
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

	if isSlice && v.IsNil() {
		enc.encodeNil()
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

func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	}
	return false
}

func (enc *Encoder) encodeStruct(v reflect.Value) {
	enc.depth++
	if enc.depth >= enc.maxDepth {
		enc.err = &DepthOverflowError{enc.maxDepth}
		return
	}

	enc.writeByte(byte(VBS_DICT))

	fields := cachedTypeFields(v.Type())
	for _, f := range fields {
		value := v.Field(f.index)
		if f.omitEmpty && isEmptyValue(value) {
			continue
		}

		enc.encodeString(f.name)
		enc.encodeReflectValue(value)
		if enc.err != nil {
			return
		}
	}

	enc.writeByte(byte(VBS_TAIL))
	enc.depth--
}


