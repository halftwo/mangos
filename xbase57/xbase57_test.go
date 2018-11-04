package xbase57

import (
        "testing"
        "bytes"
        "math/rand"
	"io"
	"os"
	"time"
)

func TestEncodeDecode(t *testing.T) {
        var u1 [256]byte
        var u2 [256]byte
        v := make([]byte, StdEncoding.EncodedLen(len(u1)+10))
        for k:= 0; k < 1000; k++ {
                i := rand.Intn(len(u1)) + 1
                rand.Read(u1[:i])

		n := StdEncoding.Encode(v, u1[:i])
		j, err := StdEncoding.Decode(u2[:], v[:n])

		if err != nil || !bytes.Equal(u1[:i], u2[:j]) {
                        t.Log(u1[:i])
                        t.Log(u2[:j])
                        t.Fatal("The decoded data not match the original")
                }
	}
}

type _LineWriter struct {
	w io.Writer
	k int
}

func (lw *_LineWriter) Write(buf []byte) (int, error) {
	width := 66
	total := len(buf)
	if lw.k > 0 {
		m := width - lw.k
		if m > len(buf) {
			m = len(buf)
		}
		lw.w.Write(buf[:m])
		lw.k += m
		if lw.k >= width {
			lw.k = 0
			lw.w.Write([]byte("\n"))
		}
		buf = buf[m:]
	}

	for len(buf) >= width {
		lw.w.Write(buf[:width])
		lw.w.Write([]byte("\n"))
		buf = buf[width:]
	}

	if len(buf) > 0 {
		lw.k = len(buf)
		lw.w.Write(buf)
	}

	return total, nil
}

func TestEncoder(t *testing.T) {
	if testing.Verbose() {
		rand.Seed(time.Now().Unix())
		w := &_LineWriter{w:os.Stdout}
		enc := NewEncoder(StdEncoding, w)
		for i := 0; i < 10; i++ {
			var buf [128]byte
			n := rand.Intn(len(buf)) + 1
			rand.Read(buf[:n])
			enc.Write(buf[:n])
		}
		enc.Close()
		w.w.Write([]byte("\n"))
	}
}

type _FreakReader struct {
	bb *bytes.Buffer
}

func (fr *_FreakReader) Read(buf []byte) (int, error) {
	n := rand.Intn(len(buf) + 1)
	return fr.bb.Read(buf[:n])
}

func TestDecoder(t *testing.T) {
	var buf [16]byte
	bb := &bytes.Buffer{}
	fr := &_FreakReader{bb}
        for k := 0; k < 1000; k++ {
		bb.Reset()
		enc := NewEncoder(StdEncoding, bb)
		rawlen := 0
		for i := 0; i < 10; i++ {
			n := rand.Intn(len(buf)) + 1
			rand.Read(buf[:n])
			rawlen += n
			enc.Write(buf[:n])
		}
		enc.Close()

		dec := NewDecoder(StdEncoding, fr)
		rawlen2 := 0
		for {
			n := rand.Intn(len(buf)) + 1
			m, err := dec.Read(buf[:n])
			rawlen2 += m
			if err != nil {
				if err == io.EOF {
					break
				}
				t.Fatal("The decoder failed")
			}
		}

		if rawlen != rawlen2 {
			t.Fatal("The decoded data not match the original")
		}
	}
}

