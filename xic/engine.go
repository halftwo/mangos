package xic

import (
	"fmt"
	"sync"
	"os"
)

type _Engine struct {
	mutex sync.Mutex
	setting Setting
	name string
	id string

	shadowBox *ShadowBox
	secretBox *SecretBox

	adapterMap map[string]*_Adapter
	proxyMap map[string]*_Proxy
	outConMap map[string]*_Connection
	inConList []*_Connection
	shutdownChan chan os.Signal
}

func newEngine() *_Engine {
	return newEngineSettingName(NewSetting(), "")
}

func newEngineSetting(setting Setting) *_Engine {
	return newEngineSettingName(setting, "")
}

func newEngineSettingName(setting Setting, name string) *_Engine {
	// TODO
	id := GenerateRandomBase57Id(23)
	engine := &_Engine{
		setting: setting,
		name: name,
		id: id,
		shutdownChan: make(chan os.Signal, 1),
	}
	engine.adapterMap = make(map[string]*_Adapter)
	engine.proxyMap = make(map[string]*_Proxy)
	engine.outConMap = make(map[string]*_Connection)
	return engine
}

func (engine *_Engine) Setting() Setting {
	return engine.setting
}

func (engine *_Engine) Name() string {
	return engine.name
}

func (engine *_Engine) Id() string {
	return engine.id
}

func (engine *_Engine) CreateAdapter(name string) (Adapter, error) {
	return engine.CreateAdapterEndpoints(name, "")
}

func (engine *_Engine) CreateAdapterEndpoints(name string, endpoints string) (Adapter, error) {
	// TODO
	if name == "" {
		name = "xic"
	}
	if endpoints == "" && engine.setting != nil {
		endpoints = engine.setting.Get(name + ".Endpoints")
	}

	if endpoints == "" {
		return nil, fmt.Errorf("No endpoints for Adapter(%s)", name)
	}

	adapter, err := newAdapter(engine, name, endpoints)
	if err != nil {
		return nil, err
	}

	err = addAdapter(engine, adapter)
	if err != nil {
		return nil, err
	}
	return adapter, nil
}

func (engine *_Engine) CreateSlackAdapter() (Adapter, error) {
	adapter, err := newAdapter(engine, "", "")
	if err != nil {
		return nil, err
	}

	err = addAdapter(engine, adapter)
	if err != nil {
		return nil, err
	}
	return adapter, nil
}

func addAdapter(engine *_Engine, adapter *_Adapter) error {
	engine.mutex.Lock()
	defer engine.mutex.Unlock()
	_, ok := engine.adapterMap[adapter.Name()]
	if ok {
		return fmt.Errorf("Adapter(%s) already created", adapter.Name())
	}
	engine.adapterMap[adapter.Name()] = adapter
	return nil
}

func (engine *_Engine) StringToProxy(proxy string) (Proxy, error) {
	prx, ok := engine.proxyMap[proxy]
	if ok {
		return prx, nil
	}

	prx = newProxy(engine, proxy)
	engine.proxyMap[proxy] = prx
	return prx, nil
}

// implements os.Signal interface
type PseudoShutdownSignal struct{}
func (s PseudoShutdownSignal) String() string { return "shutdown" }
func (s PseudoShutdownSignal) Signal() {}


func (engine *_Engine) Shutdown() {
	engine.shutdownChan<- PseudoShutdownSignal{}
}

func (engine *_Engine) WaitForShutdown() {
	sig := <-engine.shutdownChan
	if _, ok := sig.(PseudoShutdownSignal); ok {
		fmt.Println("WaitForShutdown: method Shutdown called")
	} else {
		fmt.Println("WaitForShutdown: signal", sig.String(), "received")
	}

	// TODO
	inConList := engine.inConList
	engine.inConList = nil
	outConMap := engine.outConMap
	engine.outConMap = nil

	for _, c := range inConList {
		c.grace()
	}
	for _, c := range outConMap {
		c.grace()
	}
}

func (engine *_Engine) makeFixedProxy(service string, con *_Connection) (Proxy, error) {
	prx, ok := engine.proxyMap[service]
	if ok {
		if prx.Connection() == con {
			return prx, nil
		}
	}

	prx = newProxyConnection(engine, service, con)
	engine.proxyMap[service] = prx
	return prx, nil
}

func (engine *_Engine) makeConnection(serviceHint string, endpoint string) (*_Connection, error) {
	con, ok := engine.outConMap[endpoint]
	if ok {
		if con.IsLive() {
			return con, nil
		}
	}

	ei, err := parseEndpoint(endpoint)
	if err != nil {
		return nil, err
	}

	con = newOutgoingConnection(engine, serviceHint, ei)
	engine.outConMap[endpoint] = con
	return con, nil
}

func (engine *_Engine) incomingConnection(con *_Connection) {
	engine.inConList = append(engine.inConList, con)
}

