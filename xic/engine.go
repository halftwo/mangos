package xic

import (
	"fmt"
	"sync"
)

type stdEngine struct {
	mutex sync.Mutex
	setting Setting
	name string
	uuid string

	adapterMap map[string]*stdAdapter
	proxyMap map[string]*stdProxy
	outConMap map[string]*stdConnection
	inConList []*stdConnection
	done chan struct{}
}

func newEngine() *stdEngine {
	return newEngineSettingName(NewSetting(), "")
}

func newEngineSetting(setting Setting) *stdEngine {
	return newEngineSettingName(setting, "")
}

func newEngineSettingName(setting Setting, name string) *stdEngine {
	// TODO
	uuid := GenerateRandomUuid()
	done := make(chan struct{}, 1)
	engine := &stdEngine{setting:setting, name:name, uuid:uuid, done:done}
	engine.adapterMap = make(map[string]*stdAdapter)
	engine.proxyMap = make(map[string]*stdProxy)
	engine.outConMap = make(map[string]*stdConnection)
	return engine
}

func (engine *stdEngine) Setting() Setting {
	return engine.setting
}

func (engine *stdEngine) Name() string {
	return engine.name
}

func (engine *stdEngine) Uuid() string {
	return engine.uuid
}

func (engine *stdEngine) CreateAdapter(name string) (Adapter, error) {
	return engine.CreateAdapterEndpoints(name, "")
}

func (engine *stdEngine) CreateAdapterEndpoints(name string, endpoints string) (Adapter, error) {
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

func (engine *stdEngine) CreateSlackAdapter() (Adapter, error) {
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

func addAdapter(engine *stdEngine, adapter *stdAdapter) error {
	engine.mutex.Lock()
	defer engine.mutex.Unlock()
	_, ok := engine.adapterMap[adapter.Name()]
	if ok {
		return fmt.Errorf("Adapter(%s) already created", adapter.Name())
	}
	engine.adapterMap[adapter.Name()] = adapter
	return nil
}

func (engine *stdEngine) StringToProxy(proxy string) (Proxy, error) {
	prx, ok := engine.proxyMap[proxy]
	if ok {
		return prx, nil
	}

	prx = newProxy(engine, proxy)
	engine.proxyMap[proxy] = prx
	return prx, nil
}

func (engine *stdEngine) Shutdown() {
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
	close(engine.done)
}

func (engine *stdEngine) WaitForShutdown() {
	// TODO

	select {
	case <-engine.done:
	}
}

func (engine *stdEngine) makeFixedProxy(service string, con *stdConnection) (Proxy, error) {
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

func (engine *stdEngine) makeConnection(serviceHint string, endpoint string) *stdConnection {
	con, ok := engine.outConMap[endpoint]
	if ok {
		if con.IsLive() {
			return con
		}
	}

	con = newOutgoingConnection(engine, serviceHint, endpoint)
	if con != nil {
		engine.outConMap[endpoint] = con
		con.start()
	}
	return con
}

func (engine *stdEngine) incomingConnection(con *stdConnection) {
	engine.inConList = append(engine.inConList, con)
}

