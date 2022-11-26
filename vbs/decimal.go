package vbs

import (
	"reflect"
)

var ReflectTypeOfDecimal64 = reflect.TypeOf(Decimal64{})

type Decimal64 struct {
}

func breakDecimal64(v Decimal64) (mantissa int64, expo int) {
	// TODO
	return 0, 0
}

func makeDecimal64(mantissa int64, expo int) Decimal64 {
	// TODO
	return Decimal64{}
}



