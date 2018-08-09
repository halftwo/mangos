package xerr

import (
	"testing"
)

type MyError struct{}


func foo() error {
	return Trace(MyError{}, "my message")
}

func Test1(t *testing.T) {
	err1 := foo()
	err2 := Trace(err1, "another message")

	if err1 != err2 {
		panic("should be the same")
	}

	switch err2.Cause().(type) {
		case MyError:
			break
		default:
			panic("can't be here")
	}

	t.Logf("%#v", err1)
}
