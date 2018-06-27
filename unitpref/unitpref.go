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

func Multiplier(s string, binary bool) (uint64, int) {
	m, n := MultiplierBigInt(s, binary)
	if m.IsUint64() {
		return m.Uint64(), n
	}
	return 0, n	// Overflow
}

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

func Divider(s string) (uint64, int) {
	m, n := DividerBigInt(s)
	if m.IsUint64() {
		return m.Uint64(), n
	}
	return 0, n	// Overflow
}

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

