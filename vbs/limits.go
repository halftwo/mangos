package vbs

import (
	"math"
)

var MaxLength int = math.MaxInt

var MaxStringLength int = math.MaxInt32

var MaxDepth int16 = math.MaxInt16


const MaxInt64Length = 1 + (63-5+6)/7		// 10

const MaxUint32Length = 1 + (32-5+6)/7		// 5
const MaxInt32Length = 1 + (31-5+6)/7		// 5

const MaxUint16Length = 1 + (16-5+6)/7		// 3
const MaxInt16Length = 1 + (15-5+6)/7		// 3

const MaxUint8Length = 1 + (8-5+6)/7		// 2
const MaxInt8Length = 1 + (7-5+6)/7		// 2

