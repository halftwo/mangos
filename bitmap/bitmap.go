package bitmap

import (
	"bytes"
)


func Set(bm []byte, pos uint) {
	bm[pos/8] |= (0x80 >> (pos % 8))
}

func Clear(bm []byte, pos uint) {
	bm[pos/8] &^= (0x80 >> (pos % 8))
}

func Flip(bm []byte, pos uint) {
	bm[pos/8] ^= (0x80 >> (pos % 8))
}

func Test(bm []byte, pos uint) bool {
	return (bm[pos/8] & (0x80 >> (pos % 8))) != 0
}


func EqualPrefix(bm1, bm2 []byte, prefix uint) bool {
        n := prefix / 8
        if n > 0 {
		if !bytes.Equal(bm1[:n], bm2[:n]) {
			return false
		}
        }

        bits := prefix % 8
        if bits > 0 {
		b1 := bm1[n] >> (8 - bits)
		b2 := bm2[n] >> (8 - bits)
                return b1 == b2
        }

        return true
}

