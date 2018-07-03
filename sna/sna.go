/* Package sna deals with Serial Number Arithmetic
 */
package sna

import (
	"fmt"
)


func Add64(number *uint64, delta uint64, bits uint) error {
	if bits <= 0 || bits > 64 {
		bits = 64
	}

	mask := (uint64(1) << bits) - 1
	max := mask >> 1
	if delta > max {
		return fmt.Errorf("sna: delta is out of range")
	}

	*number += delta
	*number &= mask
	return nil
}

func Distance64(a uint64, b uint64, bits uint) int64 {
	if bits > 0 && bits < 64 {
		shift := 64 - bits
		distance := (int64)((a << shift) - (b << shift))
		return distance >> shift
	}
	return (int64)(a - b)
}

func Compare64(a uint64, b uint64, bits uint) int {
	x := Distance64(a, b, bits)
	if x > 0 {
		return 1
	} else if x < 0 {
		return -1
	}
	return 0
}


func Add32(number *uint32, delta uint32, bits uint) error {
	if bits <= 0 || bits > 32 {
		bits = 32
	}

	mask := (uint32(1) << bits) - 1
	max := mask >> 1
	if delta > max {
		return fmt.Errorf("sna: delta is out of range")
	}

	*number += delta
	*number &= mask
	return nil
}

func Distance32(a uint32, b uint32, bits uint) int32 {
	if bits > 0 && bits < 32 {
		shift := 32 - bits
		distance := (int32)((a << shift) - (b << shift))
		return distance >> shift
	}
	return (int32)(a - b)
}

func Compare32(a uint32, b uint32, bits uint) int {
	x := Distance32(a, b, bits)
	if x > 0 {
		return 1
	} else if x < 0 {
		return -1
	}
	return 0
}


