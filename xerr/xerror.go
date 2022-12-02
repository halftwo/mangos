package xerr

import (
	"io"
	"fmt"
	"strings"
	"runtime"
)

type XErr interface {
	error
	Cause() any
	Unwrap() error
	PrintTrace(w io.Writer)
}

// If cause is nil, return nil,
// else add the msg to the msgtrace
func Trace(cause error, args ...any) XErr {
	if cause == nil {
		return nil
	}
	msg := fmt.Sprint(args...)
	return _traceOrNew(cause, msg)
}

// If cause is nil, return nil,
// else add the msg to the msgtrace
func Tracef(cause error, format string, args ...any) XErr {
	if cause == nil {
		return nil
	}
	msg := fmt.Sprintf(format, args...)
	return _traceOrNew(cause, msg)
}

// Make a brand new XErr
func Errorf(format string, args ...any) XErr {
	msg := fmt.Sprintf(format, args...)
	return _traceOrNew(nil, msg)
}

// If cause == nil or not *_XErr, new a *_XErr
func _traceOrNew(cause any, msg string) *_XErr {
	err, ok := cause.(*_XErr)
	if !ok {
		err = newXErr(cause)
	}
	err.trace_msg(2, msg)
	return err
}

type _XErr struct {
	cause       any
	stacktrace []uintptr      // first stack trace
	msgtrace   []_TraceItem   // all messages traced
}

func newXErr(cause any) *_XErr {
	return &_XErr{cause:cause}
}

func (err *_XErr) Error() string {
	return fmt.Sprintf("%v", err)
}

func (err *_XErr) Unwrap() error {
	e, _ := err.cause.(error)
	return e
}

// Return the "cause" of this error.
// Cause could be used for error handling/switching,
// or for holding general error/debug information.
func (err *_XErr) Cause() any {
	return err.cause
}

// Add tracing information with msg.
// Set n=0 unless wrapped with some function, then n > 0.
func (err *_XErr) trace_msg(n int, msg string) {
	if err.stacktrace == nil {
		var pcs = make([]uintptr, 16)
		n := runtime.Callers(n + 3, pcs)
		err.stacktrace = pcs[:n]
	}

	pc, _, _, _ := runtime.Caller(n + 1)
	err.msgtrace = append(err.msgtrace, _TraceItem{pc:pc, msg:msg})
}

type _TraceItem struct {
	pc  uintptr
	msg string
}

func (err *_XErr) PrintTrace(w io.Writer) {
	for i := len(err.msgtrace) - 1; i >= 0; i-- {
		ti := &err.msgtrace[i]
		if i > 0 {
			w.Write([]byte(" v "))
		} else {
			w.Write([]byte(" O "))
		}
		printLocus(w, ti.pc)
		if len(ti.msg) > 0 {
			fmt.Fprintf(w, ": %s", ti.msg)
		}
		w.Write([]byte("\n"))
	}
	if len(err.stacktrace) > 0 {
		for _, pc := range err.stacktrace {
			w.Write([]byte(" ^ "))
			printLocus(w, pc)
			w.Write([]byte("\n"))
		}
	}
}

func (err *_XErr) Format(s fmt.State, verb rune) {
	switch verb {
	case 'p':
		s.Write([]byte(fmt.Sprintf("%p", &err)))
	default:
		if s.Flag('#') {
			s.Write([]byte("<XErr>\n"))
			if err.cause != nil {
				fmt.Fprintf(s, "Cause: %#v\n", err.cause)
			}
			s.Write([]byte("Trace:\n"))
			err.PrintTrace(s)
			s.Write([]byte("</XErr>\n"))
		} else {
			ti := &err.msgtrace[0]
			s.Write([]byte("XErr "))
			printLocus(s, ti.pc)
			if len(ti.msg) > 0 {
				fmt.Fprintf(s, ": %s", ti.msg)
			}
			if err.cause != nil {
				s.Write([]byte(" --- "))
				fmt.Fprintf(s, "%#v", err.cause)
			}
		}
	}
}

func printLocus(w io.Writer, pc uintptr) {
	fun := runtime.FuncForPC(pc)
	name := getFuncName(fun)
	file, line := fun.FileLine(pc)
	file = TrimFileName(file, 3)
	fmt.Fprintf(w, "%s:%d:%s", file, line, name)
}

func getFuncName(f *runtime.Func) string {
	name := f.Name()
	i := strings.LastIndexByte(name, '.')
	if i >= 0 {
		return name[i+1:]
	}
	return name
}

func TrimFileName(name string, n int) string {
	var k int
	s := name
	for i := 0; i < n; i++ {
		k = strings.LastIndexByte(s, '/')
		if k < 0 {
			return name
		}
		s = name[:k]
	}
	k++
	return name[k:]
}

