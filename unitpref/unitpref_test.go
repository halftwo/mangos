package unitpref

import (
	"testing"
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

