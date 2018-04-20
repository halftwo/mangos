package vbs

import (
	"math"
	"math/bits"
)

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


