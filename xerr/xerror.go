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
	Is(target error) bool
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
	xe, ok := cause.(*_XErr)
	if !ok {
		xe = newXErr(cause)
	}
	xe.trace_msg(2, msg)
	return xe
}

type _XErr struct {
	cause       any
	stacktrace []uintptr      // first stack trace
	msgtrace   []_TraceItem   // all messages traced
}

func newXErr(cause any) *_XErr {
	return &_XErr{cause:cause}
}

func (xe *_XErr) Error() string {
	return fmt.Sprintf("%v", xe)
}

func (xe *_XErr) Is(target error) bool {
	return xe.cause == target
}

func (xe *_XErr) Unwrap() error {
	e, _ := xe.cause.(error)
	return e
}

// Return the "cause" of this error.
// Cause could be used for error handling/switching,
// or for holding general error/debug information.
func (xe *_XErr) Cause() any {
	return xe.cause
}

// Add tracing information with msg.
// Set n=0 unless wrapped with some function, then n > 0.
func (xe *_XErr) trace_msg(n int, msg string) {
	if xe.stacktrace == nil {
		var pcs = make([]uintptr, 16)
		n := runtime.Callers(n + 3, pcs)
		xe.stacktrace = pcs[:n]
	}

	pc, _, _, _ := runtime.Caller(n + 1)
	xe.msgtrace = append(xe.msgtrace, _TraceItem{pc:pc, msg:msg})
}

type _TraceItem struct {
	pc  uintptr
	msg string
}

// Each line of trace begins with one of the letters: T, X, C
// X means where the XErr object is first created (by xerr.Trace or xerr.Errorf)
// T means the following xerr.Trace()'s that call on the XErr object
// C means the call stacks to the X point
func (xe *_XErr) PrintTrace(w io.Writer) {
	for i := len(xe.msgtrace) - 1; i >= 0; i-- {
		ti := &xe.msgtrace[i]
		if i > 0 {
			w.Write([]byte(" T "))
		} else {
			w.Write([]byte(" X "))
		}
		printLocus(w, ti.pc)
		if len(ti.msg) > 0 {
			fmt.Fprintf(w, ": %s", ti.msg)
		}
		w.Write([]byte("\n"))
	}
	if len(xe.stacktrace) > 0 {
		for _, pc := range xe.stacktrace {
			w.Write([]byte(" C "))
			printLocus(w, pc)
			w.Write([]byte("\n"))
		}
	}
}

func (xe *_XErr) Format(s fmt.State, verb rune) {
	switch verb {
	case 'p':
		s.Write([]byte(fmt.Sprintf("%p", &xe)))
	default:
		if s.Flag('#') {
			s.Write([]byte("<XErr>\n"))
			if xe.cause != nil {
				fmt.Fprintf(s, "Cause: %#v\n", xe.cause)
			}
			s.Write([]byte("Trace:\n"))
			xe.PrintTrace(s)
			s.Write([]byte("</XErr>\n"))
		} else {
			ti := &xe.msgtrace[0]
			s.Write([]byte("XErr "))
			printLocus(s, ti.pc)
			if len(ti.msg) > 0 {
				fmt.Fprintf(s, ": %s", ti.msg)
			}
			if xe.cause != nil {
				s.Write([]byte(" --- "))
				fmt.Fprintf(s, "%v", xe.cause)
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

