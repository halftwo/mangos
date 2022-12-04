package dlog

import (
	"time"
)

func timeBuf(t time.Time, buf []byte) int {
	b := t.AppendFormat(buf[:0], "060102+150405-0700")
	b[6] = "umtwrfs"[t.Weekday()]

	// Remove trailing "00"
	n := len(b)
	if (b[n-2] == '0' && b[n-1] == '0') {
		b = b[:n-2]
		n -= 2
	}
	return n
}

func TimeString(t time.Time) string {
	var buf [24]byte
	n := timeBuf(t, buf[:])
	return string(buf[:n])
}

