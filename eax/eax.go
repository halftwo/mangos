/*
   Package eax implements cryptographic block ciphers' EAX mode
*/
package eax

import (
	"crypto/cipher"
	"errors"
)

const BLOCK_SIZE = 16

type CmacCtx struct {
	cipher cipher.Block
	_X [16]byte
	count int
}

func NewCMAC(cipher cipher.Block) (*CmacCtx, error) {
	if cipher.BlockSize() != BLOCK_SIZE {
		return nil, errors.New("eax: NewCMAC requires 128-bit block cipher")
	}
	return &CmacCtx{cipher:cipher}, nil
}

func (ca *CmacCtx) Start() {
	ca.count = 0
	for i := len(ca._X) - 1; i >= 0; i-- {
		ca._X[i] = 0
	}
}

func blockXOR(d, s []byte) {
	for i := 0; i < BLOCK_SIZE; i++ {
		d[i] ^= s[i]
	}
}

func (ca *CmacCtx) Update(buf []byte) {
	k := ca.count % BLOCK_SIZE
	if k == 0 && ca.count > 0 {
		ca.cipher.Encrypt(ca._X[:], ca._X[:])
	}

	i := 0
	num := len(buf)
	for i < num && k < BLOCK_SIZE {
                ca._X[k] ^= buf[i]
		i++
		k++
	}

	num -= i
        ca.count += i
	for num >= BLOCK_SIZE {
                ca.cipher.Encrypt(ca._X[:], ca._X[:])
                blockXOR(ca._X[:], buf[i:])
                i += BLOCK_SIZE
                num -= BLOCK_SIZE
                ca.count += BLOCK_SIZE
        }

	if num > 0 {
                ca.cipher.Encrypt(ca._X[:], ca._X[:])
                for k = 0; k < num; k++ {
                        ca._X[k] ^= buf[i]
			i++
		}
                ca.count += num
        }
}

var cXor = [4]byte { 0x00, 0x87, 0x0E, 0x89 }

func gf_mulx1(pad []byte) {
        t := pad[0] >> 7

        for i := 0; i < BLOCK_SIZE - 1; i++ {
                pad[i] = (pad[i] << 1) | (pad[i + 1] >> 7)
	}

        pad[BLOCK_SIZE - 1] = (pad[BLOCK_SIZE - 1] << 1) ^ cXor[t]
}

func gf_mulx2(pad []byte) {
        t := pad[0] >> 6

        for i := 0; i < BLOCK_SIZE - 1; i++ {
                pad[i] = (pad[i] << 2) | (pad[i + 1] >> 6)
	}

        pad[BLOCK_SIZE - 2] ^= (t >> 1)
        pad[BLOCK_SIZE - 1] = (pad[BLOCK_SIZE - 1] << 2) ^ cXor[t]
}

func (ca *CmacCtx) Finish(mac []byte) {
	var pad [BLOCK_SIZE]byte
	ca.cipher.Encrypt(pad[:], pad[:])

        k := ca.count % BLOCK_SIZE
        if ca.count == 0 || k > 0 {
                ca._X[k] ^= 0x80
                gf_mulx2(pad[:])
        } else {
                gf_mulx1(pad[:])
        }

        blockXOR(pad[:], ca._X[:])
	ca.cipher.Encrypt(pad[:], pad[:])

	copy(mac, pad[:])
}



type EaxCtx struct {
	cmac CmacCtx
        encrypt bool
        pos int
	_S [16]byte
	_C [16]byte
	_N [16]byte
	_H [16]byte
}

func NewEAX(cipher cipher.Block) (*EaxCtx, error) {
	if cipher.BlockSize() != BLOCK_SIZE {
		return nil, errors.New("eax: NewEAX requires 128-bit block cipher")
	}
	x := &EaxCtx{cmac:CmacCtx{cipher:cipher}}
	return x, nil
}

func (ax *EaxCtx) Start(encrypt bool, nonce, header []byte) {
        ax.encrypt = encrypt
        ax.pos = 0

	var block [BLOCK_SIZE]byte

        block[BLOCK_SIZE - 1] = 0
	ax.cmac.Start()
	ax.cmac.Update(block[:])
	ax.cmac.Update(nonce)
	ax.cmac.Finish(ax._N[:])

        block[BLOCK_SIZE - 1] = 1
	ax.cmac.Start()
	ax.cmac.Update(block[:])
	ax.cmac.Update(header)
	ax.cmac.Finish(ax._H[:])

        block[BLOCK_SIZE - 1] = 2
	ax.cmac.Start()
	ax.cmac.Update(block[:])

	copy(ax._C[:], ax._N[:])
}

func (ax *EaxCtx) increaseCounter() {
        for i := BLOCK_SIZE - 1; i >= 0; i-- {
                ax._C[i]++
                if (ax._C[i] != 0) {
                        break
		}
        }
}

func (ax *EaxCtx) Update(out, in []byte) {
	if !ax.encrypt {
		ax.cmac.Update(in)
	}

	n := 0
	num := len(in)
	if ax.pos > 0 {
		for n < num && ax.pos < BLOCK_SIZE {
			out[n] = ax._S[ax.pos] ^ in[n]
			n++
			ax.pos++
		}
		num -= n
		if ax.pos == BLOCK_SIZE {
			ax.pos = 0
		}
	}

	for num >= BLOCK_SIZE {
		ax.cmac.cipher.Encrypt(ax._S[:], ax._C[:])
		ax.increaseCounter()

		dst := out[n:]
		src := in[n:]
		for i := 0; i < BLOCK_SIZE; i++ {
			dst[i] = ax._S[i] ^ src[i]
		}
		n += BLOCK_SIZE
                num -= BLOCK_SIZE
	}

        if num > 0 {
		ax.cmac.cipher.Encrypt(ax._S[:], ax._C[:])
		ax.increaseCounter()

                for ax.pos < num {
                        out[n] = ax._S[ax.pos] ^ in[n]
			n++
			ax.pos++
                }
        }

        if (ax.encrypt) {
                ax.cmac.Update(out[:n])
	}
}

func (ax *EaxCtx) Finish(mac []byte) {
	ax.cmac.Finish(mac)

        for i := 0; i < BLOCK_SIZE; i++ {
                mac[i] ^= ax._N[i] ^ ax._H[i]
        }
}

