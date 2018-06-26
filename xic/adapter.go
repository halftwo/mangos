package xic

import (
	"sync"
	"sync/atomic"
	"net"
	"fmt"
	"strings"

	"halftwo/mangos/crock32"
)

type adapterState int32

const (
	adapter_INIT adapterState = iota
	adapter_ACTIVE
	adapter_FINISHED
)

type stdAdapter struct {
	engine *stdEngine
	name string
	endpoints string
	state adapterState

	listeners []*stdListener
	srvMap sync.Map
	dftService atomic.Value
}

type stdListener struct {
	adapter *stdAdapter
	listener net.Listener
}

func newListener(adapter *stdAdapter, ei *EndpointInfo) (*stdListener, error) {
	listener, err := net.Listen(ei.Proto(), ei.Address())
	if err != nil {
		return nil, err
	}
	l := &stdListener{adapter:adapter, listener:listener}
	return l, nil
}

func (l *stdListener) activate() {
	go func() {
		for {
			c, err := l.listener.Accept()
			if err != nil {
				break
			}

			// TODO
			con := newIncomingConnection(l.adapter, c)
			con.start()
		}
	}()
}

func (l *stdListener) deactivate() {
	l.listener.Close()
}

func newAdapter(engine *stdEngine, name string, endpoints string) (*stdAdapter, error) {
	if name == "" {
		uuid := GenerateRandomUuidBytes()
		buf := make([]byte, 1 + crock32.EncodeLen(len(uuid)))
		buf[0] = '_'
		crock32.EncodeLower(buf[1:], uuid)
		name = string(buf)
	}

	adapter := &stdAdapter{engine:engine, name:name}

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

func (adp *stdAdapter) Engine() Engine {
	return adp.engine
}

func (adp *stdAdapter) Name() string {
	return adp.name
}

func (adp *stdAdapter) Endpoints() string {
	return adp.endpoints
}

func (adp *stdAdapter) Activate() error {
	if !atomic.CompareAndSwapInt32((*int32)(&adp.state), int32(adapter_INIT), int32(adapter_ACTIVE)) {
		return fmt.Errorf("Adapter(%s) already activated", adp.name)
	}

	for _, l := range adp.listeners {
		l.activate()
	}
	return nil
}

func (adp *stdAdapter) Deactivate() error {
	if !atomic.CompareAndSwapInt32((*int32)(&adp.state), int32(adapter_ACTIVE), int32(adapter_FINISHED)) {
		return fmt.Errorf("Adapter(%s) not in active state", adp.name)
	}

	for _, l := range adp.listeners {
		l.deactivate()
	}
	return nil
}

func (adp *stdAdapter) AddServant(service string, servant Servant) (Proxy, error) {
	si, err := getServantInfo(service, servant)
	if err != nil {
		return nil, err
	}
	adp.srvMap.Store(service, si)

	proxy := service
	if len(adp.endpoints) > 0 {
		proxy += adp.endpoints
	}
	prx, _ := adp.engine.StringToProxy(proxy)
	return prx, nil
}

func (adp *stdAdapter) RemoveServant(service string) {
	adp.srvMap.Delete(service)
}

func (adp *stdAdapter) FindServant(service string) *ServantInfo {
	srv, ok := adp.srvMap.Load(service)
	if ok {
		return srv.(*ServantInfo)
	}
	return nil
}

func (adp *stdAdapter) DefaultServant() *ServantInfo {
	srv := adp.dftService.Load()
	if srv != nil {
		return srv.(*ServantInfo)
	}
	return nil
}

func (adp *stdAdapter) SetDefaultServant(servant Servant) error {
	si, err := getServantInfo("", servant)
	if err != nil {
		return err
	}
	adp.dftService.Store(si)
	return nil
}

