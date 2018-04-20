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
	MaxDepth int
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


type MismatchedKindError struct {
	Expect, Got Kind
}

func (e *MismatchedKindError) Error() string {
        return fmt.Sprintf("vbs: mismatched kind: expect %s, got %s", e.Expect.String(), e.Got.String())
}

