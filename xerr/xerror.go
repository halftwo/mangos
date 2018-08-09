package xerr

import (
	"fmt"
	"runtime"
)

/*
Usage with arbitrary error data:

```go
	type MyError struct{}
	err1 := Trace(MyError{}, "my message")
	...
	// Wrapping
	err2 := Trace(err1, "another message")
	if (err1 != err2) { panic("should be the same")
	...
	// Error handling
	switch err2.Cause().(type){
		case MyError: ...
		default: ...
	}
```
*/

func Trace(cause interface{}) Xerror {
	err, ok := cause.(*_Xerror)
	if !ok {
		err = newXerror(cause)
	}
	return err.withStacktrace().trace(1, "")
}

func TraceMessage(cause interface{}, format string, args ...interface{}) Xerror {
	err, ok := cause.(*_Xerror)
	if !ok {
		err = newXerror(cause)
	}
	msg := fmt.Sprintf(format, args...)
	return err.withStacktrace().trace(1, msg)
}

type Xerror interface {
	Error() string
	Cause() interface{}

	trace(skip int, format string, args ...interface{}) Xerror
}

type _Xerror struct {
	cause       interface{}
	stacktrace []uintptr      // first stack trace
	msgtrace   []_TraceItem   // all messages traced
}


func newXerror(cause interface{}) *_Xerror {
	return &_Xerror{cause:cause}
}

func (err *_Xerror) Error() string {
	return fmt.Sprintf("%#v", err)
}

// Return the "cause" of this error.
// Cause could be used for error handling/switching,
// or for holding general error/debug information.
func (err *_Xerror) Cause() interface{} {
	return err.cause
}

func (err *_Xerror) withStacktrace() Xerror {
	if err.stacktrace == nil {
		var offset = 4
		var depth = 32
		err.stacktrace = captureStacktrace(offset, depth)
	}
	return err
}

// Add tracing information with msg.
// Set n=0 unless wrapped with some function, then n > 0.
func (err *_Xerror) trace(n int, format string, args ...interface{}) Xerror {
	msg := fmt.Sprintf(format, args...)
	pc, _, _, _ := runtime.Caller(n + 1)
	err.msgtrace = append(err.msgtrace, _TraceItem{pc:pc, msg:msg})
	return err
}

func (err *_Xerror) Format(s fmt.State, verb rune) {
	switch verb {
	case 'p':
		s.Write([]byte(fmt.Sprintf("%p", &err)))
	default:
		if s.Flag('#') {
			s.Write([]byte("--= Xerror =--\n"))
			s.Write([]byte(fmt.Sprintf("Cause: %#v\n", err.cause)))
			s.Write([]byte(fmt.Sprintf("Msg-Traces:\n")))
			for i, mt := range err.msgtrace {
				s.Write([]byte(fmt.Sprintf(" %3d  %s\n", i, mt.String())))
			}
			if err.stacktrace != nil {
				s.Write([]byte(fmt.Sprintf("Stack-Trace:\n")))
				for i, pc := range err.stacktrace {
					fun := runtime.FuncForPC(pc)
					file, line := fun.FileLine(pc)
					s.Write([]byte(fmt.Sprintf(" %3d  %s:%d\n", i, file, line)))
				}
			}
			s.Write([]byte("--= /Xerror =--\n"))
		} else {
			s.Write([]byte(fmt.Sprintf("Xerror{%#v}", err.cause)))
		}
	}
}

//----------------------------------------
// stacktrace & _TraceItem

func captureStacktrace(offset int, depth int) []uintptr {
	var pcs = make([]uintptr, depth)
	n := runtime.Callers(offset, pcs)
	return pcs[0:n]
}

type _TraceItem struct {
	pc  uintptr
	msg string
}

func (ti _TraceItem) String() string {
	fun := runtime.FuncForPC(ti.pc)
	file, line := fun.FileLine(ti.pc)
	if len(ti.msg) == 0 {
		return fmt.Sprintf("%s:%d", file, line)
	}
	return fmt.Sprintf("%s:%d  %s", file, line, ti.msg)
}

