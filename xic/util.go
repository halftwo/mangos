package xic

import (
	"math"
	"math/rand"
	crand "crypto/rand"
	"encoding/hex"
	"encoding/binary"
	"sync"
	"halftwo/mangos/xbase57"
)

var once sync.Once

// Generate random uuid's 16 bytes, see RFC4122
func GenerateRandomUuidBytes() []byte {
	once.Do(func() {
		var b [8]byte
		_, err := crand.Read(b[:])
		if err == nil {
			seed := int64(binary.BigEndian.Uint64(b[:]))
			rand.Seed(seed)
		}
	})

	buf := make([]byte, 16)
	rand.Read(buf)
	b1 := buf[6]
	buf[6] = 0x40 + (b1 & 0x0F)
	b2 := buf[8]
	buf[8] = 0x80 + (b2 & 0x3F)
	return buf
}

// Generate random uuid string, see RFC4122
func GenerateRandomUuid() string {
	buf := GenerateRandomUuidBytes()
	return (hex.EncodeToString(buf[:4]) + "-" + hex.EncodeToString(buf[4:6]) + 
		"-" + hex.EncodeToString(buf[6:8]) + "-" + hex.EncodeToString(buf[8:10]) + 
		"-" + hex.EncodeToString(buf[10:]))
}

func GenerateRandomBase57Id(n int) string {
	if n < 1 {
		panic("length of id must be greater than 1")
	}

	m := xbase57.StdEncoding.DecodedLen(n) + 1
	if m < 4 {
		m = 4
	}
	src := make([]byte, m)
	dst := make([]byte, n)
	crand.Read(src)
	k := xbase57.StdEncoding.Encode(dst, src)
	u32 := binary.BigEndian.Uint32(src[:4])
	dst[0] = xbase57.StdAlphabet[u32/(math.MaxUint32/49+1)]
	return string(dst[:k])
}

