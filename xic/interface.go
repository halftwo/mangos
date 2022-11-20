package xic

import (
	"reflect"
)

type EntreeFunction func(engine Engine, args []string) error

type Setting interface {
	Set(name string, value string)
	Remove(name string)
	Insert(name, value string) bool

	Has(name string) bool
	Get(name string) string
	GetDefault(name string, dft string) string

	Int(name string) int64
	IntDefault(name string, dft int64) int64

	Bool(name string) bool
	BoolDefault(name string, dft bool) bool

	Float(name string) float64
	FloatDefault(name string, dft float64) float64

	Pathname(name string) string
	PathnameDefault(name string, dft string) string

	StringSlice(name string) []string

	LoadFile(filename string) error
}

type Engine interface {
	Setting() Setting
	Name() string
	Id() string

	CreateAdapter(name string) (Adapter, error)
	CreateAdapterEndpoints(name string, endpoints string) (Adapter, error)
	CreateSlackAdapter() (Adapter, error)

	StringToProxy(proxy string) (Proxy, error)

	MaxQ() int32
	SetMaxQ(max int32)

	SetSecretBox(secret *SecretBox)
	SetShadowBox(secret *ShadowBox)

	Shutdown()
	WaitForShutdown()
}

type MethodInfo struct {
	name    string
	method  reflect.Method
	oneway  bool
	inType  reflect.Type
	outType reflect.Type
}

type ServantInfo struct {
	Service string
	Servant Servant
	methods map[string]*MethodInfo
}

type Adapter interface {
	Engine() Engine
	Name() string
	Endpoints() string

	Activate() error
	Deactivate() error

	AddServant(service string, servant Servant) (Proxy, error)
	RemoveServant(service string)
	FindServant(service string) *ServantInfo

	DefaultServant() *ServantInfo
	SetDefaultServant(Servant) error
}

type Current interface {
	Txid() int64
	Service() string
	Method() string
	Ctx() Context
	Con() Connection
}

type Servant interface {
	/*
	   Twoway method
	   Xic_xyz(cur Current, in *ArgsIn, out *ArgsOut) error

	   Oneway method
	   Xic_xyz(cur Current, in *ArgsIn) error
	*/
	Xic(cur Current, in Arguments, out *Arguments) error
}

type LoadBalance int

const (
	LB_NORMAL LoadBalance = iota
	LB_RANDOM
	LB_HASH
)

type Proxy interface {
	Engine() Engine
	Service() string
	String() string

	Context() Context
	SetContext(ctx Context)

	Connection() Connection
	ResetConnection()

	LoadBalance() LoadBalance

	TimedProxy(timeout, closeTimeout, connectTimeout int) Proxy

	// in must be (pointer to) struct or map[string]any
	// out must be pointer to struct or map[string]any
	Invoke(method string, in, out any) error
	InvokeCtx(ctx Context, method string, in, out any) error

	InvokeAsync(method string, in, out any) Result
	InvokeCtxAsync(ctx Context, method string, in, out any) Result

	InvokeOneway(method string, in any) error
	InvokeCtxOneway(ctx Context, method string, in any) error
}

type Connection interface {
	String() string
	Close(force bool)
	CreateProxy(service string) (Proxy, error)

	Adapter() Adapter
	SetAdapter(adapter Adapter)

	Incoming() bool
	Timeout() uint32
	Endpoint() string
}

type Exception interface {
	error
	Exname() string
	Code() int
	Tag() string
	Message() string
	File() string
	Line() int
}

type Result interface {
	Txid() int64
	Service() string
	Method() string
	In() any

	Wait()		// wait until out or err is set
	Done() bool

	Out() any	// Don't call it before Wait() returns or Done() returns true
	Err() error	// Don't call it before Wait() returns or Done() returns true
}

