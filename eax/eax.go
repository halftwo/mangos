/*
   Package eax implements cryptographic block ciphers' EAX mode
*/
package eax

import (
	"crypto/cipher"
	"errors"
)

const BLOCK_SIZE = 16

type cmacCtx struct {
	cipher cipher.Block
	X [16]byte
	count int
}

func NewCMAC(cipher cipher.Block) (*cmacCtx, error) {
	if cipher.BlockSize() != BLOCK_SIZE {
		return nil, errors.New("eax: NewCMAC requires 128-bit block cipher")
	}
	return &cmacCtx{cipher:cipher}, nil
}

func (ca *cmacCtx) Start() {
	ca.count = 0
	for i := len(ca.X) - 1; i >= 0; i-- {
		ca.X[i] = 0
	}
}

func blockXOR(d, s []byte) {
	for i := 0; i < BLOCK_SIZE; i++ {
		d[i] ^= s[i]
	}
}

func (ca *cmacCtx) Update(buf []byte) {
	k := ca.count % BLOCK_SIZE
	if k == 0 && ca.count > 0 {
		ca.cipher.Encrypt(ca.X[:], ca.X[:])
	}

	i := 0
	num := len(buf)
	for i < num && k < BLOCK_SIZE {
                ca.X[k] ^= buf[i]
		i++
		k++
	}

	num -= i
        ca.count += i
	for num >= BLOCK_SIZE {
                ca.cipher.Encrypt(ca.X[:], ca.X[:])
                blockXOR(ca.X[:], buf[i:])
                i += BLOCK_SIZE
                num -= BLOCK_SIZE
                ca.count += BLOCK_SIZE
        }

	if num > 0 {
                ca.cipher.Encrypt(ca.X[:], ca.X[:])
                for k = 0; k < num; k++ {
                        ca.X[k] ^= buf[i]
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

func (ca *cmacCtx) Finish(mac []byte) {
	var pad [BLOCK_SIZE]byte
	ca.cipher.Encrypt(pad[:], pad[:])

        k := ca.count % BLOCK_SIZE
        if ca.count == 0 || k > 0 {
                ca.X[k] ^= 0x80
                gf_mulx2(pad[:])
        } else {
                gf_mulx1(pad[:])
        }

        blockXOR(pad[:], ca.X[:])
	ca.cipher.Encrypt(pad[:], pad[:])

	copy(mac, pad[:])
}



type eaxCtx struct {
	cmacCtx
        encrypt bool
        pos int
	S [16]byte
	C [16]byte
	N [16]byte
	H [16]byte
}

func NewEAX(cipher cipher.Block) (*eaxCtx, error) {
	if cipher.BlockSize() != BLOCK_SIZE {
		return nil, errors.New("eax: NewEAX requires 128-bit block cipher")
	}
	x := &eaxCtx{}
	x.cipher = cipher
	return x, nil
}

func (ax *eaxCtx) Start(encrypt bool, nonce, header []byte) {
        ax.encrypt = encrypt
        ax.pos = 0

	var block [BLOCK_SIZE]byte

        block[BLOCK_SIZE - 1] = 0
	ax.cmacCtx.Start()
	ax.cmacCtx.Update(block[:])
	ax.cmacCtx.Update(nonce)
	ax.cmacCtx.Finish(ax.N[:])

        block[BLOCK_SIZE - 1] = 1
	ax.cmacCtx.Start()
	ax.cmacCtx.Update(block[:])
	ax.cmacCtx.Update(header)
	ax.cmacCtx.Finish(ax.H[:])

        block[BLOCK_SIZE - 1] = 2
	ax.cmacCtx.Start()
	ax.cmacCtx.Update(block[:])

	copy(ax.C[:], ax.N[:])
}

func (ax *eaxCtx) increaseCounter() {
        for i := BLOCK_SIZE - 1; i >= 0; i-- {
                ax.C[i]++
                if (ax.C[i] != 0) {
                        break
		}
        }
}

func (ax *eaxCtx) Update(out, in []byte) {
	if !ax.encrypt {
		ax.cmacCtx.Update(in)
	}

	n := 0
	num := len(in)
	if ax.pos > 0 {
		for n < num && ax.pos < BLOCK_SIZE {
			out[n] = ax.S[ax.pos] ^ in[n]
			n++
			ax.pos++
		}
		num -= n
		if ax.pos == BLOCK_SIZE {
			ax.pos = 0
		}
	}

	for num >= BLOCK_SIZE {
		ax.cipher.Encrypt(ax.S[:], ax.C[:])
		ax.increaseCounter()

		dst := out[n:]
		src := in[n:]
		for i := 0; i < BLOCK_SIZE; i++ {
			dst[i] = ax.S[i] ^ src[i]
		}
		n += BLOCK_SIZE
                num -= BLOCK_SIZE
	}

        if num > 0 {
		ax.cipher.Encrypt(ax.S[:], ax.C[:])
		ax.increaseCounter()

                for ax.pos < num {
                        out[n] = ax.S[ax.pos] ^ in[n]
			n++
			ax.pos++
                }
        }

        if (ax.encrypt) {
                ax.cmacCtx.Update(out[:n])
	}
}

func (ax *eaxCtx) Finish(mac []byte) {
	ax.cmacCtx.Finish(mac)

        for i := 0; i < BLOCK_SIZE; i++ {
                mac[i] ^= ax.N[i] ^ ax.H[i]
        }
}

