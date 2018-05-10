package xic

import (
	"fmt"
	"runtime"
	"strings"
)

const (
	ProtocolException = "ProtocolException"
	ConnectionClosedException = "ConnectionClosedException"
	ServiceNotFoundException = "ServiceNotFoundException"
	MethodNotFoundException = "MethodNotFoundException"
	// TODO
)

type stdException struct {
	name string
	code int
	tag string
	msg string
	file string
	line int
	what string
}

func newEx(name string, code int, tag string, msg string) *stdException {
	_, file, line, _ := runtime.Caller(2)
	ex := &stdException{name:name, file:file, line:line, code:code, tag:tag, msg:msg}
	return ex
}

func NewException(name string, msg string) *stdException {
	return newEx(name, 0, "", msg)
}

func NewExceptionCode(name string, code int, msg string) *stdException {
	return newEx(name, code, "", msg)
}

func NewExceptionCodeTag(name string, code int, tag string, msg string) *stdException {
	return newEx(name, code, tag, msg)
}

func NewExceptionf(name string, format string, a...interface{}) *stdException {
	msg := fmt.Sprintf(format, a...)
	return newEx(name, 0, "", msg)
}

func NewExceptionCodef(name string, code int, format string, a...interface{}) *stdException {
	msg := fmt.Sprintf(format, a...)
	return newEx(name, code, "", msg)
}

func NewExceptionCodeTagf(name string, code int, tag string, format string, a...interface{}) *stdException {
	msg := fmt.Sprintf(format, a...)
	return newEx(name, code, tag, msg)
}

func (ex *stdException) Exname() string {
	return ex.name
}

func (ex *stdException) File() string {
	return ex.file
}

func (ex *stdException) Line() int {
	return ex.line
}

func (ex *stdException) Code() int {
	return ex.code
}

func (ex *stdException) Tag() string {
	return ex.tag
}

func (ex *stdException) Message() string {
	return ex.msg
}

func (ex *stdException) Error() string {
	if ex.what == "" {
		w := &strings.Builder{}
		w.WriteString(ex.name)
		if ex.tag != "" {
			fmt.Fprintf(w, "(%d,%s)", ex.code, ex.tag)
		} else {
			fmt.Fprintf(w, "(%ds)", ex.code)
		}

		if ex.file != "" {
			fmt.Fprintf(w, " at %s:%d", ex.file, ex.line)
		}

		if ex.msg != "" {
			w.WriteString(" --- ")
			w.WriteString(ex.msg)
		}

		ex.what = w.String()
	}
	return ex.what
}

