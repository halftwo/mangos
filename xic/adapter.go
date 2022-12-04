package xic

import (
	"sync"
	"sync/atomic"
	"net"
	"strings"

	"halftwo/mangos/xerr"
)

type _AdapterState int32

const (
	adapter_INIT _AdapterState = iota
	adapter_ACTIVE
	adapter_FINISHED
)

type _Adapter struct {
	engine *_Engine
	name string
	endpoints string
	state _AdapterState	// atomic

	listeners []*_Listener
	srvMap sync.Map
	dftService atomic.Value
}

type _Listener struct {
	listener net.Listener
	adapter *_Adapter
	endpoint *EndpointInfo
}

func newListener(adapter *_Adapter, ei *EndpointInfo) (*_Listener, error) {
	listener, err := net.Listen(ei.Proto(), ei.Address())
	if err != nil {
		return nil, xerr.Trace(err)
	}
	l := &_Listener{listener:listener, adapter:adapter, endpoint:ei}
	return l, nil
}

func (l *_Listener) activate() {
	go func() {
		for {
			c, err := l.listener.Accept()
			if err != nil {
				break
			}

			// TODO
			newIncomingConnection(l, c)
		}
	}()
}

func (l *_Listener) deactivate() {
	l.listener.Close()
}

func newAdapter(engine *_Engine, name string, endpoints string) (*_Adapter, error) {
	if name == "" {
		panic("Adapter must have a name")
	}

	adapter := &_Adapter{engine:engine, name:name}

	eps := []string{}
	for _, endpoint := range strings.Split(endpoints, "@") {
		endpoint = strings.TrimSpace(endpoint)
		if endpoint == "" {
			continue
		}
		ei, err := parseEndpoint(endpoint)
		if err != nil {
			return nil, err
		}
		// TODO
		l, err := newListener(adapter, ei)
		if err != nil {
			return nil, err
		}
		adapter.listeners = append(adapter.listeners, l)
		eps = append(eps, ei.String())
	}

	endpoints = strings.Join(eps, " ")
	state := adapter_INIT
	if endpoints == "" {
		state = adapter_ACTIVE
	}
	adapter.endpoints = endpoints
	adapter.state = state
	return adapter, nil
}

func (adp *_Adapter) Engine() Engine {
	return adp.engine
}

func (adp *_Adapter) Name() string {
	return adp.name
}

func (adp *_Adapter) Endpoints() string {
	return adp.endpoints
}

func (adp *_Adapter) Activate() error {
	if !atomic.CompareAndSwapInt32((*int32)(&adp.state), int32(adapter_INIT), int32(adapter_ACTIVE)) {
		return xerr.Errorf("Adapter(%s) already activated", adp.name)
	}

	for _, l := range adp.listeners {
		l.activate()
	}
	return nil
}

func (adp *_Adapter) Deactivate() error {
	if !atomic.CompareAndSwapInt32((*int32)(&adp.state), int32(adapter_ACTIVE), int32(adapter_FINISHED)) {
		return xerr.Errorf("Adapter(%s) not in active state", adp.name)
	}

	for _, l := range adp.listeners {
		l.deactivate()
	}
	return nil
}

func (adp *_Adapter) AddServant(service string, servant Servant) (Proxy, error) {
	si, err := getServantInfo(service, servant)
	if err != nil {
		return nil, err
	}
	if _, loaded := adp.srvMap.LoadOrStore(service, si); loaded {
		return nil, xerr.Errorf("Service \"%s\" already added", service);
	}

	proxy := service
	if len(adp.endpoints) > 0 {
		proxy += adp.endpoints
	}
	prx, _ := adp.engine.StringToProxy(proxy)
	return prx, nil
}

func (adp *_Adapter) MustAddServant(service string, servant Servant) Proxy {
	prx, err := adp.AddServant(service, servant)
	if err != nil {
		panic(err)
	}
	return prx
}

func (adp *_Adapter) RemoveServant(service string) {
	adp.srvMap.Delete(service)
}

func (adp *_Adapter) FindServant(service string) *ServantInfo {
	srv, ok := adp.srvMap.Load(service)
	if ok {
		return srv.(*ServantInfo)
	}
	return nil
}

func (adp *_Adapter) DefaultServant() *ServantInfo {
	srv := adp.dftService.Load()
	if srv != nil {
		return srv.(*ServantInfo)
	}
	return nil
}

func (adp *_Adapter) SetDefaultServant(servant Servant) error {
	si, err := getServantInfo("", servant)
	if err != nil {
		return err
	}
	adp.dftService.Store(si)
	return nil
}

