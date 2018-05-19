package srp6a

import (
	"testing"
	"fmt"
	"bytes"
	"strings"
	"math/rand"
	"encoding/hex"
)

func getRandomBytes(buf []byte) {
	rand.Read(buf)
}

func TestSrp6a(t *testing.T) {
	hexN := "EEAF0AB9ADB38DD69C33F80AFA8FC5E86072618775FF3C0B9EA2314C" +
                "9C256576D674DF7496EA81D3383B4813D692C6E0E0D5D8E250B98BE4" +
                "8E495C1D6089DAD15DC7D7B46154D6B6CE8EF4AD69B15D4982559B29" +
                "7BCF1885C529F566660E57EC68EDBC3C05726CC02FD4CBF4976EAA9A" +
                "FD5138FE8376435B9FC61D2FC0EB06E3"
	N, _ := hex.DecodeString(hexN)

	id := "alice"
	pass := "password123"
	salt, _ := hex.DecodeString("BEB25379D1A8581EB5A727673A2441EE")
	a, _ := hex.DecodeString("60975527035CF2AD1989806F0407210BC81EDC04E2762A56AFD529DDDA2D4393")
	b, _ := hex.DecodeString("E487CB59D31AC550471E81F00F6928E01DDA08E974A004F49E61F5D105284D20")

	hexS := "B0DC82BA BCF30674 AE450C02 87745E79 90A3381F 63B387AA F271A10D" +
		"233861E3 59B48220 F7C4693C 9AE12B0A 6F67809F 0876E2D0 13800D6C" +
		"41BB59B6 D5979B5C 00A172B4 A2A5903A 0BDCAF8A 709585EB 2AFAFA8F" +
		"3499B200 210DCC1F 10EB3394 3CD67FC8 8A2F39A4 BE5BEC4E C0A3212D" +
		"C346D7E4 74B29EDE 8A469FFE CA686E5A"
	S, _ := hex.DecodeString(strings.Replace(hexS, " ", "", -1))

	const BITS = 1024
	srv := NewServer(2, N, BITS, "SHA1")
	cli := NewClient(2, N, BITS, "SHA1")

	cli.SetIdentity(id, pass)
	cli.set_salt(salt)

	v := cli.ComputeV()
	srv.SetV(v)

	A := cli.set_a(a)
	srv.SetA(A)

	B := srv.set_b(b)
	cli.SetB(B)

	S1 := cli.ComputeS()
	S2 := srv.ComputeS()

	if !bytes.Equal(S1, S2) || !bytes.Equal(S1, S) {
		fmt.Println("S1", hex.EncodeToString(S1))
		fmt.Println("S2", hex.EncodeToString(S2))
		t.Fatalf("S1 and S2 differ")
	}
}

func BenchmarkSrp6a(b *testing.B) {
        hexN := "ac6bdb41324a9a9bf166de5e1389582faf72b6651987ee07fc3192943db56050" +
		"a37329cbb4a099ed8193e0757767a13dd52312ab4b03310dcd7f48a9da04fd50" +
		"e8083969edb767b0cf6095179a163ab3661a05fbd5faaae82918a9962f0b93b8" +
		"55f97993ec975eeaa80d740adbf4ff747359d041d5c33ea71d281e446b14773b" +
		"ca97b43a23fb801676bd207a436c6481f1d2b9078717461a5b9d32e688f87748" +
		"544523b524b0d57d5ea77a2775d2ecfa032cfbdbf52fb3786160279004e57ae6" +
		"af874e7303ce53299ccc041c7bc308d82a5698f3a8d0c38271ae35f8e9dbfbb6" +
		"94b5c803d89f7ae435de236d525f54759b65e372fcd68ef20fa7111f9e4aff73"
	N, _ := hex.DecodeString(hexN)

	id := make([]byte, 16)
	pass := make([]byte, 16)

	const BITS = 2048
	for i := 0; i < b.N; i++ {
		srv := NewServer(2, N, BITS, "SHA256")
		cli := NewClient(2, N, BITS, "SHA256")

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
			fmt.Println("S1", hex.EncodeToString(S1))
			fmt.Println("S2", hex.EncodeToString(S2))
			b.Fatalf("S1 and S2 differ")
		}
	}
}

