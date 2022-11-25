package xerr

import (
	"fmt"
	"strings"
	"runtime"
)

func Trace(cause any, args ...any) Xerror {
	msg := fmt.Sprint(args...)
	return doTrace(cause, msg)
}

func Tracef(cause any, format string, args ...any) Xerror {
	msg := fmt.Sprintf(format, args...)
	return doTrace(cause, msg)
}

func doTrace(cause any, msg string) Xerror {
	err, ok := cause.(*_Xerror)
	if !ok {
		err = newXerror(cause)
	}
	err.trace(2, msg)
	return err
}

type Xerror interface {
	Error() string
	Cause() any
}

type _Xerror struct {
	cause       any
	stacktrace []uintptr      // first stack trace
	msgtrace   []_TraceItem   // all messages traced
}


func newXerror(cause any) *_Xerror {
	return &_Xerror{cause:cause}
}

func (err *_Xerror) Error() string {
	return fmt.Sprintf("%#v", err)
}

// Return the "cause" of this error.
// Cause could be used for error handling/switching,
// or for holding general error/debug information.
func (err *_Xerror) Cause() any {
	return err.cause
}

// Add tracing information with msg.
// Set n=0 unless wrapped with some function, then n > 0.
func (err *_Xerror) trace(n int, msg string) {
	if err.stacktrace == nil {
		var pcs = make([]uintptr, 32)
		n := runtime.Callers(n + 2, pcs)
		err.stacktrace = pcs[:n]
	}

	pc, _, _, _ := runtime.Caller(n + 1)
	err.msgtrace = append(err.msgtrace, _TraceItem{pc:pc, msg:msg})
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
					name := getFuncName(fun)
					file, line := fun.FileLine(pc)
					s.Write([]byte(fmt.Sprintf(" %3d  %s:%d:%s\n", i, file, line, name)))
				}
			}
			s.Write([]byte("--= /Xerror =--\n"))
		} else {
			s.Write([]byte(fmt.Sprintf("Xerror{%#v}", err.cause)))
		}
	}
}

type _TraceItem struct {
	pc  uintptr
	msg string
}

func (ti _TraceItem) String() string {
	fun := runtime.FuncForPC(ti.pc)
	name := getFuncName(fun)
	file, line := fun.FileLine(ti.pc)
	if len(ti.msg) == 0 {
		return fmt.Sprintf("%s:%d:%s", file, line, name)
	}
	return fmt.Sprintf("%s:%d:%s  %s", file, line, name, ti.msg)
}

func getFuncName(f *runtime.Func) string {
	name := f.Name()
	i := strings.LastIndexByte(name, '.')
	if i >= 0 {
		return name[i+1:]
	}
	return name
}

