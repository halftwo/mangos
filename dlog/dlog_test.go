package dlog_test

import (
	"testing"

	"mangos/dlog"
)

func Test1(t *testing.T) {
	dlog.Log("XXX", "hello, %s!", "world")
}

