package eax_test

import (
	"testing"
	"fmt"
	"crypto/aes"
	"encoding/hex"
	"bytes"
	"strings"

	"mangos/eax"
)

func TestEAX(t *testing.T) {
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
	ax, _ := eax.NewEAX(blockCipher)

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

