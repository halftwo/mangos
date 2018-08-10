package vbs

import (
	"fmt"
	"reflect"
)


type UnsupportedTypeError struct {
        Type reflect.Type
}

func (e *UnsupportedTypeError) Error() string {
        return "vbs: unsupported type: " + e.Type.String()
}


type DepthOverflowError struct {
	MaxDepth int16
}

func (e *DepthOverflowError) Error() string {
        return fmt.Sprintf("vbs: depth exceeds max (%d)", e.MaxDepth)
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

type DataLackError struct {
	Err error
}

func (e *DataLackError) Error() string {
        return fmt.Sprintf("vbs: Need more data when decoding: %#v", e.Err)
}

type InvalidVbsError struct {
}

func (e *InvalidVbsError) Error() string {
        return "vbs: Invalid vbs-encoded bytes"
}

type IntegerOverflowError struct {
	kind reflect.Kind
	value int64
}

func (e *IntegerOverflowError) Error() string {
        return fmt.Sprintf("vbs: integer %d can't be stored in %v", e.value, e.kind)
}

type NumberOverflowError struct {
	MaxBits int
}

func (e *NumberOverflowError) Error() string {
        return fmt.Sprintf("vbs: allowed max bits: %d", e.MaxBits)
}


type ArrayLengthError struct {
	Len int
}

func (e *ArrayLengthError) Error() string {
        return fmt.Sprintf("vbs: array length must be %d", e.Len)
}


type MismatchedKindError struct {
	Expect, Got Kind
}

func (e *MismatchedKindError) Error() string {
        return fmt.Sprintf("vbs: mismatched kind: expect %s, got %s", e.Expect.String(), e.Got.String())
}

type NonEmptyInterfaceError struct {
}

func (e *NonEmptyInterfaceError) Error() string {
	return "vbs: can't decode into non empty interface variable"
}

// An UnmarshalTypeError describes a VBS value that was
// not appropriate for a value of a specific Go type.
type UnmarshalTypeError struct {
	Value  string       // description of VBS value - "INTEGER", "STRING", etc
	Type   reflect.Type // type of Go value it could not be assigned to
	Struct string       // name of the struct type containing the field
	Field  string       // name of the field holding the Go value
}

func (e *UnmarshalTypeError) Error() string {
	if e.Struct != "" || e.Field != "" {
		return "vbs: cannot unmarshal " + e.Value + " into Go struct field " + e.Struct + "." + e.Field + " of type " + e.Type.String()
	}
	return "vbs: cannot unmarshal " + e.Value + " into Go value of type " + e.Type.String()
}

