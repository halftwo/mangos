package xerr

import (
	"testing"
	"io"
)

type MyError struct{}

func (MyError) Error() string { return "#MyError#" }


func foo() error {
	x := MyError{}
	return Trace(x, "my message")
}

func Test1(t *testing.T) {
	xx := Trace(io.EOF)
	t.Logf("%v", xx)

	err1 := foo()
	err2 := Tracef(err1, "another message")

	if err1 != err2 {
		t.Fatal("should be the same")
	}

	switch err2.Cause().(type) {
		case MyError:
			break
		default:
			t.Fatal("can't be here")
	}

	t.Logf("%#v", err1)
}
