/**     
 * Package srp6a implements SRP6a authentication algorithm
 * The routines comply with RFC 5054 (SRP for TLS), with the following exceptions:
 * The evidence messages 'M1' and 'M2' are computed according to Tom Wu's paper 
 * "SRP-6: Improvements and refinements to the Secure Remote Password protocol",
 * table 5, from 2002. 
**/
package srp6a

import (
	"fmt"
	"hash"
	"math/big"
	crand "crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"strings"
)

const randomSize = 512/2/8
const MinSaltSize = 16

type srp6aBase struct {
	err error
	hashName string
	hasher hash.Hash
	bits int
	byteLen int
	iN *big.Int
	ig *big.Int
	ik *big.Int
	_N []byte
	_g []byte
	_A []byte
	_B []byte
	_S []byte
	_u []byte	// SHA1(PAD(A) | PAD(B))
	_M1 []byte	// SHA1(PAD(A) | PAD(B) | PAD(S))
	_M2 []byte	// SHA1(PAD(A) | M1 | PAD(S))
	_K []byte	// SHA1(PAD(S))
}

func GenerateSalt() []byte {
	salt := make([]byte, MinSaltSize)
	err := RandomBytes(salt)
	if err != nil {
		return nil
	}
	return salt
}

func padCopy(dst, src []byte) {
	if len(dst) < len(src) {
		panic("Can't reach here")
	}

	n := len(dst) - len(src)
	copy(dst[n:], src)
	for n--; n >= 0; n-- {
		dst[n] = 0
	}
}

func RandomBytes(buf []byte) error {
	_, err := crand.Read(buf)
	if err != nil {
		return err
	}
	return nil
}

func isModZero(x *big.Int, m *big.Int) bool {
	i := &big.Int{}
	i.Mod(x, m)
	return i.Sign() == 0
}

func (b *srp6aBase) Err() error {
	return b.err
}

func (b *srp6aBase) setHash(hash string) {
	if strings.EqualFold(hash, "SHA1") {
		b.hashName = "SHA1"
		b.hasher = sha1.New()
	} else if strings.EqualFold(hash, "SHA256") {
		b.hashName = "SHA256"
		b.hasher = sha256.New()
	} else {
		b.err = fmt.Errorf("Unsupported hash \"%s\"", hash)
	}
}

func (b *srp6aBase) setParameter(g int, N []byte, bits int) {
	if b.err != nil {
		return
	}
	if bits < 512 && bits < len(N) * 8 {
		b.err = fmt.Errorf("bits must be 512 or above, and be len(N)*8 or above")
		return
	}

	b.bits = bits
	b.byteLen = (bits + 7) / 8

	b.ig = big.NewInt(int64(g))

	b.iN = &big.Int{}
	b.iN.SetBytes(N)

	b._N = make([]byte, b.byteLen)
	padCopy(b._N, b.iN.Bytes())

	b._g = make([]byte, b.byteLen)
	padCopy(b._g, b.ig.Bytes())

	// Compute: k = SHA1(N | PAD(g)) 
	b.hasher.Reset()
	b.hasher.Write(b._N)
	b.hasher.Write(b._g)
	buf := b.hasher.Sum(nil)
	b.ik = &big.Int{}
	b.ik.SetBytes(buf)
}

func (b *srp6aBase) Bits() int {
	return b.bits
}

func (b *srp6aBase) G() []byte {
	return b._g
}

func (b *srp6aBase) N() []byte {
	return b._N
}

func computeU(hasher hash.Hash, bufLen int, A, B []byte) []byte {
	if len(A) == 0 || len(B) == 0 {
		return nil
	}

	// Compute: u = SHA1(PAD(A) | PAD(B))
	buf := make([]byte, bufLen)
	hasher.Reset()
	padCopy(buf, A); hasher.Write(buf)
	padCopy(buf, B); hasher.Write(buf)
	u := hasher.Sum(nil)
	for i := len(u) - 1; i >= 0; i-- {
		if u[i] != 0 {
			return u
		}
	}
	return nil
}

func (b *srp6aBase) compute_u() {
	if len(b._u) == 0 && b.err == nil {
		if len(b._A) == 0 || len(b._B) == 0 {
			b.err = fmt.Errorf("A or B not set yet")
			return
		}

		b._u = computeU(b.hasher, b.byteLen, b._A, b._B)
		if len(b._u) == 0 {
			b.err = fmt.Errorf("u can't be 0")
		}
	}
}

func (b *srp6aBase) ComputeM1() []byte {
	if len(b._M1) == 0 && b.err == nil {
		if len(b._A) == 0 || len(b._B) == 0 {
			b.err = fmt.Errorf("A or B is not set yet")
			return nil
		}

		if len(b._S) == 0 {
			b.err = fmt.Errorf("S must be computed before M1 and M2")
			return nil
		}

		// Compute: M1 = SHA1(PAD(A) | PAD(B) | PAD(S))
		buf := make([]byte, b.byteLen)
		b.hasher.Reset()
		padCopy(buf, b._A); b.hasher.Write(buf)
		padCopy(buf, b._B); b.hasher.Write(buf)
		padCopy(buf, b._S); b.hasher.Write(buf)
		b._M1 = b.hasher.Sum(nil)
	}
	return b._M1
}

func (b *srp6aBase) ComputeM2() []byte {
	if len(b._M2) == 0 && b.err == nil {
                b.ComputeM1()
		if b.err != nil {
			return nil
		}

                // Compute: M2 = SHA1(PAD(A) | M1 | PAD(S)) 
		buf := make([]byte, b.byteLen)
		b.hasher.Reset()
		padCopy(buf, b._A); b.hasher.Write(buf)
		b.hasher.Write(b._M1)
		padCopy(buf, b._S); b.hasher.Write(buf)
		b._M2 = b.hasher.Sum(nil)
	}
	return b._M2
}

func (b *srp6aBase) ComputeK() []byte {
	if len(b._K) == 0 && b.err == nil {
		if len(b._S) == 0 {
			b.err = fmt.Errorf("S must be computed before K")
			return nil
		}

		// Compute: K = SHA1(PAD(S))  
		buf := make([]byte, b.byteLen)
		b.hasher.Reset()
		padCopy(buf, b._S); b.hasher.Write(buf)
		b._K = b.hasher.Sum(nil)
	}
	return b._K
}


type Srp6aServer struct {
	srp6aBase
	iv *big.Int
        ib *big.Int
	iA *big.Int
}

func NewServer(g int, N []byte, bits int, hash string) *Srp6aServer {
	srv := &Srp6aServer{}
	srv.setHash(hash)
	srv.setParameter(g, N, bits)
	return srv
}

func (srv *Srp6aServer) SetV(v []byte)  {
	if srv.iv == nil && srv.err == nil {
		srv.iv = &big.Int{}
		srv.iv.SetBytes(v)
	}
}

func (srv *Srp6aServer) GenerateB() []byte {
	if len(srv._B) == 0 && srv.err == nil {
		var buf [randomSize]byte
		for len(srv._B) == 0 {
			err := RandomBytes(buf[:])
			if err != nil {
				srv.err = err
				return nil
			}
			srv.set_b(buf[:])

			if len(srv._A) > 0 {
				u := computeU(srv.hasher, srv.byteLen, srv._A, srv._B)
				if len(u) == 0 {
					srv._B = nil
				} else {
					srv._u = u
				}
			}
		}
	}
	return srv._B
}

func (srv *Srp6aServer) set_b(b []byte) []byte {
	srv.ib = &big.Int{}
	srv.ib.SetBytes(b)

	// Compute: B = k*v + g^b % N
	i1 := &big.Int{}
	i1.Mul(srv.ik, srv.iv)

	i2 := &big.Int{}
	i2.Exp(srv.ig, srv.ib, srv.iN)

	i1.Add(i1, i2)
	i1.Mod(i1, srv.iN)
	if i1.Sign() == 0 {
		return nil
	}
	srv._B = make([]byte, srv.byteLen)
	padCopy(srv._B, i1.Bytes())
	return srv._B
}

func (srv *Srp6aServer) SetA(A []byte) {
	if srv.err == nil {
		if len(A) > srv.byteLen {
			srv.err = fmt.Errorf("Invalid A, too large")
		} else {
			srv.iA = &big.Int{}
			srv.iA.SetBytes(A)
			if isModZero(srv.iA, srv.iN) {
				srv.err = fmt.Errorf("Invalid A, A%%N == 0")
				return
			}
			srv._A = make([]byte, srv.byteLen)
			padCopy(srv._A, A)
		}
	}
}

func (srv *Srp6aServer) ComputeS() []byte {
	if len(srv._S) == 0 && srv.err == nil {
		if len(srv._A) == 0 || srv.iv == nil {
			srv.err = fmt.Errorf("A or v is not set yet")
			return nil
		}

		srv.GenerateB()
                srv.compute_u()
		if srv.err != nil {
			return nil
		}

		// Compute: S_host = (A * v^u) ^ b % N
		srv._S = make([]byte, srv.byteLen)
		iu := &big.Int{}
		iu.SetBytes(srv._u)

		i1 := &big.Int{}
		i1.Exp(srv.iv, iu, srv.iN)
		i1.Mul(srv.iA, i1)
		i1.Mod(i1, srv.iN)
		i1.Exp(i1, srv.ib, srv.iN)
		padCopy(srv._S, i1.Bytes())
	}
	return srv._S
}


type Srp6aClient struct {
	srp6aBase
	identity string
	pass string
	salt []byte
	ix *big.Int
	ia *big.Int
	iB *big.Int
	_v []byte
}

func NewClient(g int, N []byte, bits int, hash string) *Srp6aClient {
	cli := &Srp6aClient{}
	cli.setHash(hash)
	cli.setParameter(g, N, bits)
	return cli
}

func (cli *Srp6aClient) SetIdentity(id string, pass string) {
	cli.identity = id
	cli.pass = pass
}

func (cli *Srp6aClient) SetSalt(salt []byte) bool {
	if len(cli.salt) == 0 && cli.err == nil {
		cli.salt = make([]byte, len(salt))
		copy(cli.salt, salt)
		return true
	}
	return false
}

func (cli *Srp6aClient) compute_x() {
	if cli.ix == nil && cli.err == nil {
		if len(cli.identity) == 0 || len(cli.pass) == 0 || len(cli.salt) == 0 {
			cli.err = fmt.Errorf("id, pass or salt not set yet")
			return
		}

		// Compute: x = SHA1(salt | SHA1(Id | ":" | pass)) 
		cli.ix = &big.Int{}
		cli.hasher.Reset()
		cli.hasher.Write([]byte(cli.identity))
		cli.hasher.Write([]byte{':'})
		cli.hasher.Write([]byte(cli.pass))
		buf := cli.hasher.Sum(nil)

		cli.hasher.Reset()
		cli.hasher.Write(cli.salt)
		cli.hasher.Write(buf)
		buf = cli.hasher.Sum(nil)
		cli.ix.SetBytes(buf)
	}
}

func (cli *Srp6aClient) ComputeV() []byte {
	if len(cli._v) == 0 && cli.err == nil {
		if cli.iN == nil {
			cli.err = fmt.Errorf("Parameters (g,N) not set yet")
			return nil
		}

                cli.compute_x()
		if cli.err != nil {
			return nil
		}

		// Compute: v = g^x % N 
		cli._v = make([]byte, cli.byteLen)
		i1 := &big.Int{}
		i1.Exp(cli.ig, cli.ix, cli.iN)
		padCopy(cli._v, i1.Bytes())
	}
	return cli._v
}

func (cli *Srp6aClient) GenerateA() []byte {
	if len(cli._A) == 0 && cli.err == nil {
		if cli.iN == nil {
			cli.err = fmt.Errorf("Parameters (g,N) not set yet")
			return nil
		}

		var buf [randomSize]byte
		for len(cli._A) == 0 {
			err := RandomBytes(buf[:])
			if err != nil {
				cli.err = err
				return nil
			}
			cli.set_a(buf[:])
		}
        }
	return cli._A
}

func (cli *Srp6aClient) set_a(a []byte) []byte {
	cli.ia = &big.Int{}
	cli.ia.SetBytes(a)

	// Compute: A = g^a % N 
	i1 := &big.Int{}
	i1.Exp(cli.ig, cli.ia, cli.iN)
	if i1.Sign() == 0 {
		return nil
	}
	cli._A = make([]byte, cli.byteLen)
	padCopy(cli._A, i1.Bytes())
	return cli._A
}

func (cli *Srp6aClient) SetB(B []byte) {
	if cli.err == nil {
		if len(B) > cli.byteLen {
			cli.err = fmt.Errorf("Invalid B, too large")
		} else {
			cli.iB = &big.Int{}
			cli.iB.SetBytes(B)
			if isModZero(cli.iB, cli.iN) {
				cli.err = fmt.Errorf("Invalid B, B%%N == 0")
				return
			}
			cli._B = make([]byte, cli.byteLen)
			padCopy(cli._B, B)
		}
	}
}

func (cli *Srp6aClient) ComputeS() []byte {
	if len(cli._S) == 0 && cli.err == nil {
		if len(cli._B) == 0 {
			cli.err = fmt.Errorf("B is not set yet")
			return nil
		}
                cli.GenerateA()
                cli.compute_x()
                cli.compute_u()
		if cli.err != nil {
			return nil
		}

                // Compute: S_user = (B - (k * g^x)) ^ (a + (u * x)) % N 
		cli._S = make([]byte, cli.byteLen)
		iu := &big.Int{}
		iu.SetBytes(cli._u)

		i1 := &big.Int{}
		i1.Exp(cli.ig, cli.ix, cli.iN)
		i1.Mul(cli.ik, i1)
		i1.Mod(i1, cli.iN)
		i1.Sub(cli.iB, i1)
		if i1.Sign() < 0 {
			i1.Add(i1, cli.iN)
		}

		i2 := &big.Int{}
		i2.Mul(iu, cli.ix)
		i2.Add(cli.ia, i2)
		i2.Mod(i2, cli.iN)

		i1.Exp(i1, i2, cli.iN)
		padCopy(cli._S, i1.Bytes())
	}
	return cli._S
}

