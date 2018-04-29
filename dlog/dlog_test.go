package dlog_test

import (
	"testing"

	"mangos/dlog"
)

func Test1(t *testing.T) {
	dlog.SetOption(dlog.OPT_STDERR)
	dlog.Log("XXX", "hello, %s!\r\n\r\n", "world")
}

