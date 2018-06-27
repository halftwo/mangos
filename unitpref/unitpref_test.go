package unitpref

import (
	"testing"
	"math/big"
	"fmt"
)

func TestUnitpref(t *testing.T) {
	mm := []string{"X", "ki", "Mi", "Gi", "Ti", "Pi", "Ei", "Zi", "Yi" }
	dd := []string{"X", "m",  "u",  "n",  "p",  "f",  "a",  "z",  "y" }

	x := uint64(1)
	for i, s := range mm {
		m, n := Multiplier(s, true)
		if i == 0 {
			if m != 1 && n != 0 {
				t.Fatalf("Unknown char not detected")
			}
		} else if i <= 6 {
			x *= 1024
			if m != x && n != 2 {
				t.Fatalf("Unexpected number")
			}
		} else {
			if m != 0 && n != 2 {
				t.Fatalf("Overflow not detected")
			}
		}
	}

	x = 1
	for i, s := range dd {
		m, n := Divider(s)
		if i == 0 {
			if m != 1 && n != 0 {
				t.Fatalf("Unknown char not detected")
			}
		} else if i <= 6 {
			x *= 1000
			if m != x && n != 1 {
				t.Fatalf("Unexpected number")
			}
		} else {
			if m != 0 && n != 1 {
				t.Fatalf("Overflow not detected")
			}
		}
	}
}


func TestParse(t *testing.T) {
	s := "123456789YiB"
	x, _ := ParseBigIntWithMultiplier(s, true)
	y, _ := new(big.Int).SetString("123456789", 10)
	z := new(big.Int).Exp(new(big.Int).SetInt64(1024), new(big.Int).SetInt64(8), nil)
	z.Mul(y, z)

	if x.Cmp(z) != 0 {
		fmt.Println(x, y)
		t.Fatalf("ParseBigIntWithMultiplier() failed")
	}
}

