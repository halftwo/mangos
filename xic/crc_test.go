package xic

import (
	"testing"
)

const in = "123456789"
const crc32_IEEE = 0xcbf43926
const crc64_XIC = 0xe9c6d914c4b8d9ca

func TestCrc32(t *testing.T) {
	if crc := Crc32Checksum([]byte(in)); crc != crc32_IEEE {
		t.Errorf("%#x", crc)
	}
}

func TestCrc64(t *testing.T) {
	if crc := Crc64Checksum([]byte(in)); crc != crc64_XIC {
		t.Errorf("%#x", crc)
	}
}

