package vbs

import (
	"io"
	"reflect"
	"bytes"
	"math"
	"halftwo/mangos/xerr"
)

type BytesPacker [16]byte

func (bp *BytesPacker) packIntOrStringHead(n *int, kind VbsKind, num uint64) {
	for ; num >= 0x20; *n++ {
		bp[*n] = 0x80 | byte(num)
		num >>= 7
	}
	bp[*n] = byte(kind) | byte(num)
	*n++
}

func (bp *BytesPacker) PackStringHead(n *int, len int) {
	bp.packIntOrStringHead(n, VBS_STRING, uint64(len))
}

func (bp *BytesPacker) PackInteger(n *int, v int64) {
	if v < 0 {
		bp.packIntOrStringHead(n, VBS_INTEGER + 0x20, uint64(-v))
	} else {
		bp.packIntOrStringHead(n, VBS_INTEGER, uint64(v))
	}
}

func (bp *BytesPacker) PackDescriptor(n *int, descriptor uint16) {
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

func (bp *BytesPacker) PackKind(n *int, kind VbsKind, num uint64) {
	for ; num > 0; *n++ {
		bp[*n] = 0x80 | byte(num)
		num >>= 7
	}
	bp[*n] = byte(kind)
	*n++
}

func (bp *BytesPacker) PackHeadOfList(n *int, variety int) {
	if variety > 0 {
		bp.PackKind(n, VBS_LIST, uint64(variety))
	} else {
		bp[*n] = byte(VBS_LIST)
		*n++
	}
}

func (bp *BytesPacker) PackHeadOfDict(n *int, variety int) {
	if variety > 0 {
		bp.PackKind(n, VBS_DICT, uint64(variety))
	} else {
		bp[*n] = byte(VBS_DICT)
		*n++
	}
}

func (bp *BytesPacker) PackTail(n *int) {
	bp[*n] = byte(VBS_TAIL)
	*n++
}

func (bp *BytesPacker) PackBool(n *int, v bool) {
	if v {
		bp[*n] = byte(VBS_BOOL + 1)
	} else {
		bp[*n] = byte(VBS_BOOL)
	}
	*n++
}

func (bp *BytesPacker) PackFloat(n *int, v float64) {
	mantissa, expo := breakFloat(v)
	if mantissa < 0 {
		bp.PackKind(n, VBS_FLOATING + 1, uint64(-mantissa))
	} else {
		bp.PackKind(n, VBS_FLOATING, uint64(mantissa))
	}
	bp.PackInteger(n, int64(expo))
}

func (bp *BytesPacker) PackDecimal64(n *int, v Decimal64) {
	mantissa, expo := breakDecimal64(v)
	if mantissa < 0 {
		bp.PackKind(n, VBS_DECIMAL + 1, uint64(-mantissa))
	} else {
		bp.PackKind(n, VBS_DECIMAL, uint64(mantissa))
	}
	bp.PackInteger(n, int64(expo))
}

// Marshal returns the VBS encoding of v.
func Marshal(data any) ([]byte, error) {
	buf := bytes.NewBuffer([]byte{})
	enc := NewEncoder(buf)
	enc.Encode(data)
	return buf.Bytes(), enc.err
}


// An Encoder writes VBS values to an output stream.
type Encoder struct {
	BytesPacker
	w io.Writer
	size int
	maxDepth int16
	depth int16
	err error
	buffer []byte
}

// NewEncoder returns a new Encoder that writes to w
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w:w, maxDepth:MaxDepth}
}

func (enc *Encoder) Err() error {
	return enc.err
}

func (enc *Encoder) Size() int {
	return enc.size
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

// Encode writes the VBS encoding of data to the Encoder
func (enc *Encoder) Encode(data any) error {
	if data == nil {
		panic("No data given")
	}
	v := reflect.ValueOf(data)
	enc.encodeReflectValue(v)
	return enc.err
}

func (enc *Encoder) write(buf []byte) {
	if enc.err == nil {
		var k int
		k, enc.err = enc.w.Write(buf)
		enc.size += k
	}
}

func (enc *Encoder) writeByte(b byte) {
	if enc.err == nil {
		var k int
		buf := [1]byte{b}
		k, enc.err = enc.w.Write(buf[:1])
		enc.size += k
	}
}

func (enc *Encoder) encodeDescriptor(descriptor uint16) {
	n := 0
	enc.PackDescriptor(&n, descriptor)
	enc.write(enc.BytesPacker[0:n])
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

	case reflect.Interface, reflect.Pointer:
		if v.IsNil() {
			enc.encodeNil()
		} else {
			enc.encodeReflectValue(v.Elem())
		}

	// reflect.Func, reflect.Chan, reflect.UnsafePointer:
	default:
		enc.err = xerr.Trace(&UnsupportedTypeError{v.Type()})
	}
}

func (enc *Encoder) encodeInteger(v int64) {
	n := 0
	enc.PackInteger(&n, v)
	enc.write(enc.BytesPacker[0:n])
}

func (enc *Encoder) encodeFloat(v float64) {
	n := 0
	enc.PackFloat(&n, v)
	enc.write(enc.BytesPacker[0:n])
}

func (enc *Encoder) encodeDecimal64(v Decimal64) {
	n := 0
	enc.PackDecimal64(&n, v)
	enc.write(enc.BytesPacker[0:n])
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
	enc.PackStringHead(&n, len(v))
	enc.write(enc.BytesPacker[0:n])
	enc.write([]byte(v))
}

func (enc *Encoder) encodeBlob(v []byte) {
	n := 0
	enc.PackKind(&n, VBS_BLOB, uint64(len(v)))
	enc.write(enc.BytesPacker[0:n])
	enc.write(v)
}

func (enc *Encoder) encodeNil() {
	enc.writeByte(byte(VBS_NULL))
}

func (enc *Encoder) enterCompound(kind VbsKind) bool {
	if enc.depth >= enc.maxDepth {
		enc.err = xerr.Trace(&DepthOverflowError{enc.maxDepth})
		return false
	}
	enc.depth++
	enc.writeByte(byte(kind))
	return true
}

func (enc *Encoder) leaveCompound() {
	if enc.depth <= 0 {
		panic("Can't reach here")
	}
	enc.depth--
	enc.writeByte(byte(VBS_TAIL))
}

func (enc *Encoder) encodeList(v reflect.Value) {
	isSlice := (v.Kind() == reflect.Slice)
	if v.Type().Elem().Kind() == reflect.Uint8 {	// VBS_BLOB
		var buf []byte
		if isSlice {
			buf = v.Bytes()
		} else if v.CanAddr() {
			buf = v.Slice(0, v.Len()).Interface().([]byte)
		} else {
			n := v.Len()
			if cap(enc.buffer) < n {
				enc.buffer = make([]byte, 0, n)
			}
			buf = enc.buffer[:n]
			reflect.Copy(reflect.ValueOf(buf), v)
		}
		enc.encodeBlob(buf)
		return
	}

	if !enc.enterCompound(VBS_LIST) {
		return
	}

	n := v.Len()
	for i := 0; i < n; i++ {
		enc.encodeReflectValue(v.Index(i))
		if enc.err != nil {
			return
		}
	}

	enc.leaveCompound()
}

func (enc *Encoder) encodeMap(v reflect.Value) {
	if !enc.enterCompound(VBS_DICT) {
		return
	}

	iter := v.MapRange()
	for iter.Next() {
		enc.encodeReflectValue(iter.Key())
		enc.encodeReflectValue(iter.Value())
		if enc.err != nil {
			return
		}
	}

	enc.leaveCompound()
}

func (enc *Encoder) encodeStruct(v reflect.Value) {
	if !enc.enterCompound(VBS_DICT) {
		return
	}

	fields := GetStructFieldInfos(v.Type())
	for _, f := range fields {
		value := v.Field(int(f.Idx))
		if f.OmitEmpty && IsEmptyValue(value) {
			continue
		}

		if f.NameInt != 0 {
			enc.encodeInteger(int64(f.NameInt))
		} else {
			enc.encodeString(f.Name)
		}
		enc.encodeReflectValue(value)
		if enc.err != nil {
			return
		}
	}

	enc.leaveCompound()
}


