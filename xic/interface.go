package xic

import (
	"os"
	"reflect"
)

/*
   Server EntreeFunc procedure:
  	Engine.CreateAdapter
	Adapter.AddServant
	...
	Adapter.Activate

   Client EntreeFunc procedure:
	Engine.StringToProxy
	Proxy.Invoke
	...
	Engine.Shutdown // or call this in some other place
*/
type EntreeFunc func(engine Engine, args []string) error

/*
   Normal SignalHandler procedure:
	sig := <-sigChan
	Engine.Shutdown
	Engine.WaitForShutdown
	os.Exit
*/
type SignalHandler func(sigChan <-chan os.Signal)


func Run(entree EntreeFunc) error {
	engine, err := Start(entree)
	if err != nil {
		return err
	}
	engine.WaitForShutdown()
	return nil
}

func Start(entree EntreeFunc) (Engine, error) {
	return start_setting_signal(entree, nil, nil)
}

func StartSetting(entree EntreeFunc, setting Setting) (Engine, error) {
	return start_setting_signal(entree, setting, nil)
}

func StartSettingSignal(entree EntreeFunc, setting Setting, sigFun SignalHandler) (Engine, error) {
	return start_setting_signal(entree, setting, sigFun)
}

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
	Id() string		// universal unique
	Setting() Setting

	Throb(func()string)	// dlog every minute

	// get the endpoints from setting
	CreateAdapter(name string) (Adapter, error)
	CreateAdapterEndpoints(name string, endpoints string) (Adapter, error)

	// slack adapter has no endpoints
	CreateSlackAdapter() (Adapter, error)

	StringToProxy(proxy string) (Proxy, error)

	MaxQ() int32
	SetMaxQ(max int32)

	SetSecretBox(secret *SecretBox)
	SetShadowBox(secret *ShadowBox)

	SignalChannel() chan<- os.Signal

	Shutdown()
	WaitForShutdown()
}

type MethodInfo struct {
	Name    string
	Method  reflect.Method
	InType  reflect.Type
	OutType reflect.Type
	Oneway  bool
}

type ServantInfo struct {
	Service string
	Servant Servant
	Methods map[string]*MethodInfo
}

type Adapter interface {
	Engine() Engine
	Name() string
	Endpoints() string

	Activate() error
	Deactivate() error

	AddServant(service string, servant Servant) (Proxy, error)
	MustAddServant(service string, servant Servant) Proxy
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
	   Argument in must be a (pointer to) map[string]any or a (pointer to) struct
	   Argument out must be a (pointer to) map[string]any or a pointer to struct

	   Twoway method:
		   Xic_abc(cur Current, in *ArgsIn, out *ArgsOut) error
	   Oneway method:
		   Xic_abc(cur Current, in *ArgsIn) error
	*/
	Xic(cur Current, in Arguments, out Arguments) error
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
	Endpoints() string

	Context() Context
	SetContext(ctx Context)

	LoadBalance() LoadBalance

	TimedProxy(timeout, closeTimeout, connectTimeout int) Proxy

	// in must be (pointer to) struct or map[string]any
	// out must be pointer to struct or map[string]any
	// If out is nil, the answer is discarded
	Invoke(method string, in, out any) error
	InvokeCtx(ctx Context, method string, in, out any) error

	InvokeAsync(method string, in, out any) Result
	InvokeCtxAsync(ctx Context, method string, in, out any) Result

	InvokeOneway(method string, in any) error
	InvokeCtxOneway(ctx Context, method string, in any) error
}

type Connection interface {
	Id() string		// universal unique

	// In the form "tcp/local/remote".
	// For example, "tcp/192.168.0.1+1234/192.168.0.99+54321"
	String() string

	Close(force bool)
	CreateFixedProxy(service string) (Proxy, error)

	Adapter() Adapter

	Incoming() bool
	Timeout() uint32
	Endpoint() string
}


type ExNameType string

type Exception interface {
	error
	Name() ExNameType
	Code() int
	Message() string
	Locus() string
	IsRemote() bool
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

