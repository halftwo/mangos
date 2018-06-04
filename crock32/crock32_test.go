package crock32

import (
	"testing"
	"fmt"
	"bytes"
	"math/rand"
)

func Test1(t *testing.T) {
	var u1 [256]byte
	var u2 [256]byte
	v := make([]byte, EncodeLen(len(u1)+10))
	for k:= 0; k < 1000; k++ {
		i := rand.Intn(len(u1)) + 1
		rand.Read(u1[:i])

		var n int
		switch rand.Intn(2) {
		case 0:
			n = EncodeUpper(v, u1[:i])
		case 1:
			n = EncodeLower(v, u1[:i])
		}

		var j int
		if rand.Intn(2) == 0 {
			j = Decode(u2[:], v[:n])
		} else {
			v2 := bytes.Map(func(x rune)rune{
				switch x {
				case '0':
					x = 'o'
				case '1':
					switch rand.Intn(2) {
					case 0:
						x = 'i'
					case 1:
						x = 'l'
					}
				}

				if !('0' <= x && x <= '9') {
					if rand.Intn(2) == 0{
						if x >= 'a' {
							x -= 32
						} else {
							x += 32
						}
					}
				}
				return x
			}, v[:n])

			p := rand.Intn(len(v2))
			v2 = bytes.Join([][]byte{v2[:p], v2[p:]}, []byte("-"))
			j = DecodeFuzzy(u2[:], v2)
		}

		if !bytes.Equal(u1[:i], u2[:j]) {
			fmt.Println(u1[:i])
			fmt.Println(u2[:j])
			t.Fatal("The decoded data not match the original")
		}
	}
}

