package xic

import (
	"bytes"
	"crypto/aes"
	"crypto/sha1"

	"halftwo/mangos/eax"
	"halftwo/mangos/xerr"
)

type _CipherSuite int

const CipherMacSize = 16

const (
	CIPHER_UNKNOWN _CipherSuite  = iota
	CLEARTEXT
	AES128_EAX
	AES192_EAX
	AES256_EAX
)

func (c _CipherSuite) String() string {
	switch c {
	case CIPHER_UNKNOWN:
		return "UNKNOWN"
	case CLEARTEXT:
		return "CLEARTEXT"
	case AES128_EAX:
		return "AES128-EAX"
	case AES192_EAX:
		return "AES192-EAX"
	case AES256_EAX:
		return "AES256-EAX"
	}
	return "INVALID"
}

func String2CipherSuite(s string) _CipherSuite {
	suite := CIPHER_UNKNOWN
	switch s {
	case "CLEARTEXT":
		suite = CLEARTEXT
	case "AES128-EAX":
		suite = AES128_EAX
	case "AES192-EAX":
		suite = AES192_EAX
	case "AES256-EAX":
		suite = AES256_EAX
	}
	return suite
}

type _Cipher struct {
	ox *eax.EaxCtx
	ix *eax.EaxCtx

	oNonce [20]byte
	iNonce [20]byte
}


func newXicCipher(suite _CipherSuite, keyInfo []byte, isServer bool) (*_Cipher, error) {
	keyLen := 0
	switch suite {
	case AES128_EAX:
		keyLen = 16
	case AES192_EAX:
		keyLen = 24
	case AES256_EAX:
		keyLen = 32
	default:
		return nil, xerr.Errorf("Unsupported CipherSuite %s", suite)
	}

	c := &_Cipher{}

	var key [32]byte
	copy(key[:keyLen], keyInfo)

	blockCipher, err := aes.NewCipher(key[:keyLen])
	if err != nil {
		return nil, xerr.Trace(err)
	}

	c.ox, err = eax.NewEax(blockCipher)
	if err != nil {
		panic("Can't reach here")
	}

	c.ix, err = eax.NewEax(blockCipher)
	if err != nil {
		panic("Can't reach here")
	}

	c.oNonce = sha1.Sum(keyInfo)
	copy(c.iNonce[:], c.oNonce[:])
	if (isServer) {
		c.oNonce[len(c.oNonce)-1] |= 0x01;
		c.iNonce[len(c.iNonce)-1] &^= 0x01;
	} else {
		c.iNonce[len(c.iNonce)-1] |= 0x01;
		c.oNonce[len(c.oNonce)-1] &^= 0x01;
	}
	return c, nil
}

func counterAdd2(counter []byte) {
	for i := len(counter)-1; i >= 0; i-- {
		counter[i]++
                if counter[i] != 0 {
			break
		}
	}
	for i := len(counter)-1; i >= 0; i-- {
		counter[i]++
                if counter[i] != 0 {
			break
		}
	}
}

func (c *_Cipher) OutputStart(header []byte) {
	counterAdd2(c.oNonce[:])
	c.ox.Start(true, c.oNonce[:], header)
}

func (c *_Cipher) OutputUpdate(out, in []byte) {
	c.ox.Update(out, in)
}

func (c *_Cipher) OutputFinish(MAC []byte) {
	c.ox.Finish(MAC)
}


func (c *_Cipher) InputStart(header []byte) {
	counterAdd2(c.iNonce[:])
	c.ix.Start(false, c.iNonce[:], header)
}

func (c *_Cipher) InputUpdate(out, in []byte) {
	c.ix.Update(out, in)
}

func (c *_Cipher) InputFinish(MAC []byte) bool {
	var mac [CipherMacSize]byte
	c.ix.Finish(mac[:])
	return bytes.Equal(MAC, mac[:])
}

