package dlog

import (
	"testing"
)

type NullWriter struct {}

func (NullWriter) Write(data []byte) (int, error) {
	return len(data), nil
}

func TestLogger(t *testing.T) {
	SetOption(OPT_ALTERR|OPT_ALTOUT)
	nw := NullWriter{}
	SetAltWriter(nw)
	Log("ERROR", "%g", 12345.67890)
}

