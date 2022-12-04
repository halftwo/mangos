package dlog

import (
	"time"
)

func n00(buf []byte, n int) {
	buf[1] = byte('0' + n % 10)
	buf[0] = byte('0' + n / 10)
}

func timeNoZone(buf []byte, t time.Time) int {
	year, month, day := t.Date()
	hour, min, sec := t.Clock()
	wday := t.Weekday()

	n00(buf[0:2], year%100)
	n00(buf[2:4], int(month))
	n00(buf[4:6], day)
	buf[6] = "umtwrfs"[wday]
	n00(buf[7:9], hour)
	n00(buf[9:11], min)
	n00(buf[11:13], sec)
	return 13
}

func timeZone(buf []byte, t time.Time) int {
	_, offset := t.Zone()
	if offset < 0 {
		offset = -offset
		buf[0] = '-'
	} else {
		buf[0] = '+'
	}

	zone := offset / 60
	hour := zone / 60
	n00(buf[1:3], hour)
	min := zone % 60
	if min == 0 {
		return 3
	}
	n00(buf[3:5], min)
	return 5
}

// len(buf) must be at least 18 if zone is true
// and 13 if it is not
func TimeBuffer(buf []byte, t time.Time, zone bool) int {
	n := timeNoZone(buf, t)
	if zone {
		n += timeZone(buf[n:], t)
	}
	return n
}

func TimeString(t time.Time) string {
	var buf [20]byte
	n := TimeBuffer(buf[:], t, true)
	return string(buf[:n])
}

