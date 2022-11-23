package xic

import (
	"fmt"
	"sync"
	"os"
	"math"
	"time"
	"errors"
	"sync/atomic"

	"halftwo/mangos/dlog"
)

const DEFAULT_MAX_QUEST_NUMBER = 10000

const (
	eng_ACTIVE = iota
	eng_SHUTTING
	eng_SHUTTED
)

type _Engine struct {
	setting Setting
	name string
	id string

	maxQ int32
	numQ atomic.Int32

	cipher _CipherSuite
	shadowBox *ShadowBox
	secretBox *SecretBox

	adapterMap map[string]*_Adapter
	proxyMap map[string]*_Proxy
	outConMap map[string]*_Connection
	inConList []*_Connection

	sigChan chan os.Signal

	once sync.Once
	state int
	mutex sync.Mutex
	cond sync.Cond
}

var ErrEngineShutted = errors.New("Engine is shutting or shutted")

func newEngineSetting(setting Setting) *_Engine {
	return newEngineSettingName(setting, "")
}

func newEngineSettingName(setting Setting, name string) *_Engine {
	id := GenerateRandomBase57Id(23)
	engine := &_Engine{
		setting: setting,
		name: name,
		id: id,
		maxQ: DEFAULT_MAX_QUEST_NUMBER,
		sigChan: make(chan os.Signal, 1),
	}
	engine.cond.L = &engine.mutex

	var err error
	engine.adapterMap = make(map[string]*_Adapter)
	engine.proxyMap = make(map[string]*_Proxy)
	engine.outConMap = make(map[string]*_Connection)

	shadow := setting.Pathname("xic.passport.shadow")
	if shadow != "" {
		engine.shadowBox, err = NewShadowBoxFromFile(shadow)
		if err != nil {
			dlog.Log("XIC.WARN", "Failed to open shadow file %s", shadow)
		}
	}

	engine.cipher = String2CipherSuite(setting.Get("xic.cipher"))
	if engine.cipher == CIPHER_UNKNOWN {
		engine.cipher = AES128_EAX
	}

	secret := setting.Pathname("xic.passport.secret")
	if secret != "" {
		engine.secretBox, err = NewSecretBoxFromFile(secret)
		if err != nil {
			dlog.Log("XIC.WARN", "Failed to open secret file %s", secret)
		}
	}

	go engine.wait_for_shutting_routine()
	return engine
}

func (engine *_Engine) Shutted() bool {
	engine.mutex.Lock()
	shutted := (engine.state == eng_SHUTTED)
	engine.mutex.Unlock()
	return shutted
}

func (engine *_Engine) SignalChannel() chan<- os.Signal {
	return engine.sigChan
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

func (engine *_Engine) MaxQ() int32 {
	return engine.maxQ
}

func (engine *_Engine) SetMaxQ(max int32) {
	if max <= 0 {
		engine.maxQ = math.MaxInt32
	} else {
		engine.maxQ = max
	}
}

func (engine *_Engine) SetSecretBox(sb *SecretBox) {
	engine.secretBox = sb
}

func (engine *_Engine) SetShadowBox(sb *ShadowBox) {
	engine.shadowBox = sb
}

func (engine *_Engine) CreateAdapter(name string) (Adapter, error) {
	return engine.CreateAdapterEndpoints(name, "")
}

func (engine *_Engine) CreateAdapterEndpoints(name string, endpoints string) (Adapter, error) {
	if name == "" {
		name = "xic"
	}
	if endpoints == "" {
		itemkey := name + ".Endpoints"
		endpoints = engine.setting.Get(itemkey)
		if endpoints == "" {
			return nil, fmt.Errorf("%s not set", itemkey)
		}
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
	if engine.state != eng_ACTIVE {
		return ErrEngineShutted
	}

	_, ok := engine.adapterMap[adapter.Name()]
	if ok {
		return fmt.Errorf("Adapter(%s) already created", adapter.Name())
	}
	engine.adapterMap[adapter.Name()] = adapter
	return nil
}

func (engine *_Engine) StringToProxy(proxy string) (Proxy, error) {
	engine.mutex.Lock()
	defer engine.mutex.Unlock()
	if engine.state != eng_ACTIVE {
		return nil, ErrEngineShutted
	}

	prx, ok := engine.proxyMap[proxy]
	if ok {
		return prx, nil
	}

	prx = newProxy(engine, proxy)
	engine.proxyMap[proxy] = prx
	return prx, nil
}

func (engine *_Engine) Shutdown() {
	engine.mutex.Lock()
	if engine.state < eng_SHUTTING {
		engine.state = eng_SHUTTING
		engine.cond.Broadcast()
	}
	engine.mutex.Unlock()
}

func (engine *_Engine) WaitForShutdown() {
	engine.mutex.Lock()
	for engine.state < eng_SHUTTED {
		engine.cond.Wait()
	}
	engine.mutex.Unlock()
}

func (engine *_Engine) sig_handler_routine(sigChan <-chan os.Signal) {
	sig, ok := <-sigChan
	if ok {
		fmt.Fprintln(os.Stderr, "XIC: signal", sig.String(), "received, shutting down.")
	} else {
		fmt.Fprintln(os.Stderr, "XIC: channel of signal closed, shutting down.")
	}
	engine.Shutdown()
	engine.WaitForShutdown()
	os.Exit(0)
}

func (engine *_Engine) wait_for_shutting_routine() {
	engine.mutex.Lock()
	for engine.state < eng_SHUTTING {
		engine.cond.Wait()
	}
	adapterMap := engine.adapterMap
	engine.adapterMap = nil

	inConList := engine.inConList
	engine.inConList = nil

	outConMap := engine.outConMap
	engine.outConMap = nil

	engine.proxyMap = nil
	engine.mutex.Unlock()

	for _, a := range adapterMap {
		a.Deactivate()
	}
	for _, c := range inConList {
		c.closeGracefully()
	}
	for _, c := range outConMap {
		c.closeGracefully()
	}

	// TODO: wait for connections closed
	time.Sleep(time.Millisecond)	// XXX

	engine.mutex.Lock()
	engine.state = eng_SHUTTED
	engine.cond.Broadcast()
	engine.mutex.Unlock()
}

func (engine *_Engine) makeFixedProxy(service string, con *_Connection) (Proxy, error) {
	engine.mutex.Lock()
	defer engine.mutex.Unlock()
	if engine.state != eng_ACTIVE {
		return nil, ErrEngineShutted
	}

	prx, ok := engine.proxyMap[service]
	if ok {
		if prx.fixed && prx.cons[0] == con {
			return prx, nil
		}
	}

	prx = newProxyWithConnection(engine, service, con)
	engine.proxyMap[service] = prx
	return prx, nil
}

func (engine *_Engine) makeConnection(serviceHint string, endpoint string) (*_Connection, error) {
	engine.mutex.Lock()
	defer engine.mutex.Unlock()
	if engine.state != eng_ACTIVE {
		return nil, ErrEngineShutted
	}

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
	engine.mutex.Lock()
	defer engine.mutex.Unlock()
	engine.inConList = append(engine.inConList, con)
}

