package srp6a_test

import (
	"testing"
	"bytes"
	"math/rand"
	"encoding/hex"

	"mangos/srp6a"
)

const BITS = 2048

func getRandomBytes(buf []byte) {
	rand.Read(buf)
}

func TestSrp6a(t *testing.T) {
        hexN := "ac6bdb41324a9a9bf166de5e1389582faf72b6651987ee07fc3192943db56050" +
		"a37329cbb4a099ed8193e0757767a13dd52312ab4b03310dcd7f48a9da04fd50" +
		"e8083969edb767b0cf6095179a163ab3661a05fbd5faaae82918a9962f0b93b8" +
		"55f97993ec975eeaa80d740adbf4ff747359d041d5c33ea71d281e446b14773b" +
		"ca97b43a23fb801676bd207a436c6481f1d2b9078717461a5b9d32e688f87748" +
		"544523b524b0d57d5ea77a2775d2ecfa032cfbdbf52fb3786160279004e57ae6" +
		"af874e7303ce53299ccc041c7bc308d82a5698f3a8d0c38271ae35f8e9dbfbb6" +
		"94b5c803d89f7ae435de236d525f54759b65e372fcd68ef20fa7111f9e4aff73"
	N := make([]byte, hex.DecodedLen(len(hexN)))
	hex.Decode(N, []byte(hexN))

	id := make([]byte, 16)
	pass := make([]byte, 16)

	for i := 0; i < 100; i++ {
		srv := srp6a.NewServer(2, N, BITS, "SHA256")
		cli := srp6a.NewClient(2, N, BITS, "SHA256")

		getRandomBytes(id)
		getRandomBytes(pass)
		cli.SetIdentity(string(id), string(pass))

		v := cli.ComputeV()
                srv.SetV(v)

		A := cli.GenerateA()
		srv.SetA(A)

		B := srv.GenerateB()
		cli.SetB(B)

		S1 := cli.ComputeS()
		S2 := srv.ComputeS()

		if !bytes.Equal(S1, S2) {
			t.Errorf("S1 and S2 differ")
		}
	}
}

