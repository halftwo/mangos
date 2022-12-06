package dlog

import (
	"testing"
	"time"
)

type NullWriter struct {}

func (NullWriter) Write(data []byte) (int, error) {
	return len(data), nil
}

func TestLogger(t *testing.T) {
	SetOption(OPT_ALTERR|OPT_ALTOUT)
	nw := NullWriter{}
	SetAltWriter(nw)
	Log(ERROR, "%g", 12345.67890)
	SetAltWriter(nil)
	Log(ERROR, "%g", 12345.67890)
}

func BenchmarkLog(b *testing.B) {
	nw := NullWriter{}
	SetAltWriter(nw)
	SetOption(OPT_NONET|OPT_ALTOUT)
	for i := 0; i < b.N; i++ {
		Log(ERROR, "%g", 12345.67890)
	}
}

func BenchmarkTimeString(b *testing.B) {
	t := time.Now()
	for i := 0; i < b.N; i++ {
		TimeString(t)
	}
}

func BenchmarkTimeBuffer(b *testing.B) {
	t := time.Now()
	for i := 0; i < b.N; i++ {
		var buf [24]byte
		TimeBuffer(buf[:], t, true)
	}
}

func BenchmarkTimeBufferNoZone(b *testing.B) {
	t := time.Now()
	for i := 0; i < b.N; i++ {
		var buf [24]byte
		TimeBuffer(buf[:], t, false)
	}
}

