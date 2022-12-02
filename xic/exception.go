package xic

import (
	"fmt"
	"runtime"

	"halftwo/mangos/xerr"
)

const (
	ProtocolException ExNameType	= "ProtocolException"
	ConnectionClosedException	= "ConnectionClosedException"
	QuestNotServedException         = "QuestNotServedException"
)

const (
	UnknownException ExNameType	= "UnknownException"
	ServiceNotFoundException	= "ServiceNotFoundException"
	MethodNotFoundException		= "MethodNotFoundException"
	AdapterAbsentException		= "AdapterAbsentException"
	ConnectionOverloadException	= "ConnectionOverloadException"
	EngineOverloadException		= "EngineOverloadException"
	AuthFailedException		= "AuthFailedException"
	InvalidParameterException	= "InvalidParameterException"
)

type _Exception struct {
	name   ExNameType
	code   int
	msg    string
	locus  string
	remote bool
}

type _RemoteEx struct {
	_Exception
}


func newRemoteExCode(name ExNameType, code int, msg string, con *_Connection) *_RemoteEx {
	locus := fmt.Sprintf("remote:%s", con.remoteAddr())
	ex := &_RemoteEx{_Exception{name:name, code:code, msg:msg, locus:locus, remote:true}}
	return ex
}

func _new_ex(name ExNameType, code int, msg string) *_Exception {
	_, file, line, _ := runtime.Caller(2)
	file = xerr.TrimFileName(file, 3)
	locus := fmt.Sprintf("%s:%d", file, line)
	ex := &_Exception{name:name, code:code, msg:msg, locus:locus}
	return ex
}

func newException(name ExNameType) *_Exception {
	return _new_ex(name, 0, "")
}

func newEx(name ExNameType, msg string) *_Exception {
	return _new_ex(name, 0, msg)
}

func newExCode(name ExNameType, code int, msg string) *_Exception {
	return _new_ex(name, code, msg)
}

func newExf(name ExNameType, format string, a ...any) *_Exception {
	msg := fmt.Sprintf(format, a...)
	return _new_ex(name, 0, msg)
}

func newExCodef(name ExNameType, code int, format string, a ...any) *_Exception {
	msg := fmt.Sprintf(format, a...)
	return _new_ex(name, code, msg)
}


func (ex *_Exception) Name() ExNameType { return ex.name }
func (ex *_Exception) Code() int { return ex.code }
func (ex *_Exception) Message() string { return ex.msg }
func (ex *_Exception) Locus() string { return ex.locus }
func (ex *_Exception) IsRemote() bool { return ex.remote }

func (ex *_Exception) Error() string { return fmt.Sprintf("%v", ex) }

func (ex *_Exception) Format(s fmt.State, verb rune) {
	switch verb {
	case 'p':
		s.Write([]byte(fmt.Sprintf("%p", &ex)))
	default:
		name := ex.name
		if name == "" {
			name = UnknownException
		}
		s.Write([]byte("xic."))
		s.Write([]byte(name))
		if ex.code != 0 {
			fmt.Fprintf(s, "(%d)", ex.code)
		}

		if s.Flag('#') {
			s.Write([]byte(" "))
			s.Write([]byte(ex.locus))
		}

		if ex.msg != "" {
			s.Write([]byte(": "))
			s.Write([]byte(ex.msg))
		}
	}
}

