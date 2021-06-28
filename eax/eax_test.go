package eax_test

import (
	"testing"
	"fmt"
	"crypto/aes"
	"encoding/hex"
	"bytes"
	"strings"

	"halftwo/mangos/eax"
)

func TestCmac(t *testing.T) {
	// from https://datatracker.ietf.org/doc/html/rfc4493
	keyHex	:= "2b7e151628aed2a6abf7158809cf4f3c"
	msgHex	:= "6bc1bee22e409f96e93d7e117393172aae2d8a571e03ac9c9eb76fac45af8e5130c81c46a35ce411"
	macHex	:= "dfa66747de9ae63030ca32611497c827"

	key, _ := hex.DecodeString(keyHex)
	msg, _ := hex.DecodeString(msgHex)
	mac, _ := hex.DecodeString(macHex)

	blockCipher, _ := aes.NewCipher(key)
	cmac, _ := eax.NewCmac(blockCipher)
	cmac.Start()
	cmac.Update(msg)
	mac2 := make([]byte, 16)
	cmac.Finish(mac2[:])
	if !bytes.Equal(mac, mac2) {
		fmt.Println("MAC", macHex)
		fmt.Println("MAC2", hex.EncodeToString(mac2))
		t.Errorf("Test failed")
	}
}

func TestEax(t *testing.T) {
	keyHex	  := "8395FCF1E95BEBD697BD010BC766AAC3"
        nonceHex  := "22E7ADD93CFC6393C57EC0B3C17D6B44"
	headerHex := "126735FCC320D25A"
        msgHex    := "CA40D7446E545FFAED3BD12A740A659FFBBB3CEAB7"
	cipherHex := "CB8920F87A6C75CFF39627B56E3ED197C552D295A7CFC46AFC253B4652B1AF3795B124AB6E"

	key, _ := hex.DecodeString(keyHex)
	nonce, _ := hex.DecodeString(nonceHex)
	header, _ := hex.DecodeString(headerHex)
	msg, _ := hex.DecodeString(msgHex)
	cipher, _ := hex.DecodeString(cipherHex)

	out := make([]byte, 128)
	blockCipher, _ := aes.NewCipher(key)
	ax, _ := eax.NewEax(blockCipher)

	ax.Start(true, nonce, header)
	ax.Update(out, msg)
	ax.Finish(out[len(msg):])

	n := len(msg) + eax.BLOCK_SIZE
	if !bytes.Equal(out[:n], cipher) {
		fmt.Println("C1", cipherHex)
		fmt.Println("C2", strings.ToUpper(hex.EncodeToString(out[:n])))
		t.Errorf("Test failed")
	}
}

