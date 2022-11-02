package dlog_test

import (
	"testing"

	"time"
	"halftwo/mangos/dlog"
)

func Test1(t *testing.T) {
	now := time.Now()
	loc := time.FixedZone("TEST", -30*60)
	t.Log(dlog.TimeString(now))
	t.Log(dlog.TimeString(now.In(loc)))
	dlog.SetOption(dlog.OPT_STDERR)
	dlog.Log("XXX", "hello, %s!\r\n\r\n", "world")
}

func BenchmarkTimeString(b *testing.B) {
	tm := time.Now()
	for i := 0; i < b.N; i++ {
		dlog.TimeString(tm)
	}
}

