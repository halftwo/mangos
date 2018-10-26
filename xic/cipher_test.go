package xic

import (
	"testing"
	"bytes"
	"math/rand"
)

func getRandomBytes(buf []byte) error {
	_, err := rand.Read(buf)
	return err
}

func TestCipher(t *testing.T) {
	keyInfo := make([]byte, 17)
	getRandomBytes(keyInfo)
	srv, _ := newXicCipher(AES192_EAX, keyInfo, true)
	cli, _ := newXicCipher(AES192_EAX, keyInfo, false)

	var header [15]byte
	var cipher, plain, out [100]byte

	getRandomBytes(header[:])
	getRandomBytes(plain[:])

	var ok = true

	srv.OutputStart(header[:])
	cli.InputStart(header[:])

	srv.OutputUpdate(cipher[:], plain[:])
	cli.InputUpdate(out[:], cipher[:])

	var mac [16]byte
	srv.OutputFinish(mac[:])
	if !cli.InputFinish(mac[:]) {
		ok = false
	}

	if !ok || !bytes.Equal(out[:], plain[:]) {
		t.Errorf("test cipher failed")
	}
}

