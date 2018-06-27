/**
 * Package unitpref calculates the unit prefix factors
 * see: https://en.wikipedia.org/wiki/Unit_prefix
**/
package unitpref

import (
	"math/big"
	"strings"
	"unicode/utf8"
)

const multiplier = "kKMGTPEZY"
const divider =     "munpfazy"

// Multiplier return the factor that will multiply the original number
func Multiplier(s string, binary bool) (uint64, int) {
	m, n := MultiplierBigInt(s, binary)
	if m.IsUint64() {
		return m.Uint64(), n
	}
	return 0, n	// Overflow
}

// Multiplier return the factor (in *big.Int) that will multiply the original number
func MultiplierBigInt(s string, binary bool) (*big.Int, int) {
	m := new(big.Int).SetUint64(1)
	r, n := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return m, 0
	}

	k := strings.IndexRune(multiplier, r)
	if k < 0 {
		return m, 0
	}

	x := new(big.Int).SetUint64(1000)
	if binary && strings.HasPrefix(s[n:], "i") {
		x.SetUint64(1024)
		n++
	}

	switch k {
	case 8: m.Mul(m, x); fallthrough
	case 7: m.Mul(m, x); fallthrough
	case 6: m.Mul(m, x); fallthrough
	case 5: m.Mul(m, x); fallthrough
	case 4: m.Mul(m, x); fallthrough
	case 3: m.Mul(m, x); fallthrough
	case 2: m.Mul(m, x); fallthrough
	case 1: m.Mul(m, x)
	case 0: m.Mul(m, x)
	}

	return m, n
}

// ParseIntWithMultiplier return the multiplier-adjusted number
func ParseIntWithMultiplier(s string, binary bool) (int64, int) {
	x, n := ParseBigIntWithMultiplier(s, binary)
	if x.IsInt64() {
		return x.Int64(), n
	}

	return 0, n	// Overflow

}

// ParseBigIntWithMultiplier return the multiplier-adjusted number in *big.Int
func ParseBigIntWithMultiplier(s string, binary bool) (*big.Int, int) {
	i := strings.IndexFunc(s, func(x rune) bool {
		return strings.IndexRune("0123456789-", x) < 0
	})

	x := new(big.Int)
	if i == 0 {
		return x, 0
	}

	number := s
	if i > 0 {
		number = s[:i]
	}

	if _, ok := x.SetString(number, 10); !ok {
		panic("unitpref: (*big.Int).SetString() failed")
	}

	if i < len(s) {
		m, n := MultiplierBigInt(s[i:], binary)
		x.Mul(x, m)
		i += n
	}
	return x, i
}


// Divider return the factor that will divide the original number
func Divider(s string) (uint64, int) {
	m, n := DividerBigInt(s)
	if m.IsUint64() {
		return m.Uint64(), n
	}
	return 0, n	// Overflow
}

// Divider return the factor (in *big.Int) that will divide the original number
func DividerBigInt(s string) (*big.Int, int) {
	m := new(big.Int).SetUint64(1)
	r, n := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return m, 0
	}

	k := strings.IndexRune(divider, r)
	if k < 0 {
		return m, 0
	}
	k++

	x := new(big.Int).SetUint64(1000)
	switch k {
	case 8: m.Mul(m, x); fallthrough
	case 7: m.Mul(m, x); fallthrough
	case 6: m.Mul(m, x); fallthrough
	case 5: m.Mul(m, x); fallthrough
	case 4: m.Mul(m, x); fallthrough
	case 3: m.Mul(m, x); fallthrough
	case 2: m.Mul(m, x); fallthrough
	case 1: m.Mul(m, x)
	}

	return m, n
}

// ParseBigRatWithMultiplier return the multiplier-adjusted number in *big.Rat
func ParseBigRatWithMultiplier(s string, binary bool) (*big.Rat, int) {
	i := strings.IndexFunc(s, func(x rune) bool {
		return strings.IndexRune("0123456789.-", x) < 0
	})

	x := new(big.Rat)
	if i == 0 {
		return x, 0
	}

	number := s
	if i > 0 {
		number = s[:i]
	}

	if _, ok := x.SetString(number); !ok {
		panic("unitpref: (*big.Int).SetString() failed")
	}

	if i < len(s) {
		m, n := MultiplierBigInt(s[i:], binary)
		if n > 0 {
			multi := new(big.Rat).SetInt(m)
			x.Mul(x, multi)
			i += n
		}
	}
	return x, i
}

// ParseBigRatWithDivider return the divider-adjusted number in *big.Rat
func ParseBigRatWithDivider(s string) (*big.Rat, int) {
	i := strings.IndexFunc(s, func(x rune) bool {
		return strings.IndexRune("0123456789.-", x) < 0
	})

	x := new(big.Rat)
	if i == 0 {
		return x, 0
	}

	number := s
	if i > 0 {
		number = s[:i]
	}

	if _, ok := x.SetString(number); !ok {
		panic("unitpref: (*big.Int).SetString() failed")
	}

	if i < len(s) {
		m, n := DividerBigInt(s[i:])
		if n > 0 {
			multi := new(big.Rat).SetInt(m)
			multi.Inv(multi)
			x.Mul(x, multi)
			i += n
		}
	}
	return x, i
}

