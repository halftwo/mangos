package xic

import (
	"testing"
	"bytes"
)

func TestCipher(t *testing.T) {
	keyInfo := make([]byte, 64)
	getRandomBytes(keyInfo)
	srv, _ := newXicCipher(AES192_EAX, keyInfo, true)
	cli, _ := newXicCipher(AES192_EAX, keyInfo, false)

	var header [15]byte
	var IV [16]byte
	var cipher, plain, out [100]byte

	getRandomBytes(header[:])
	getRandomBytes(plain[:])

	var ok = true
	if !srv.OutputGetIV(IV[:]) {
		ok = false
	}
	if !cli.InputSetIV(IV[:]) {
		ok = false
	}

	srv.OutputStart(header[:])
	cli.InputStart(header[:])

	srv.OutputUpdate(cipher[:], plain[:])
	cli.InputUpdate(out[:], cipher[:])

	MAC := srv.OutputMakeMAC()
	if !cli.InputCheckMAC(MAC) {
		ok = false
	}

	if !ok || !bytes.Equal(out[:], plain[:]) {
		t.Errorf("test cipher failed")
	}
}
