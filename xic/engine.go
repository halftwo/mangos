package xic

import (
	"os"
	"math"
	"time"
	"errors"
	"sync"
	"sync/atomic"

	"halftwo/mangos/dlog"
	"halftwo/mangos/xerr"
)

const ENGINE_VERSION = "Go.221209.22120919"

const DEFAULT_ENGINE_MAXQ = 10000

const SLACK_ADAPTER_NAME = "-SLACK-"

const (
	eng_ACTIVE = iota
	eng_SHUTTING
	eng_SHUTTED
)

type _Engine struct {
	id string
	setting Setting

	maxQ int32
	numQ atomic.Int32

	cipher _CipherSuite
	shadowBox *ShadowBox
	secretBox *SecretBox

	keeper *ServantInfo
	slackAdapter *_Adapter
	adapterMap map[string]*_Adapter
	proxyMap map[string]*_Proxy
	outConMap map[string]*_Connection
	inConList []*_Connection

	sigChan chan os.Signal

	startTS string
	doneChan chan struct{}
	once sync.Once		// for throb_routine
	throbFunc atomic.Value	// func()string
	ticker *time.Ticker

	state int
	mutex sync.Mutex
	cond sync.Cond
}

var ErrEngineShutted = errors.New("Engine is shutting or shutted")

func newEngineSetting(setting Setting) *_Engine {
	engine := &_Engine{
		setting: setting,
		id: GenerateRandomBase57Id(23),
		maxQ: DEFAULT_ENGINE_MAXQ,
		sigChan: make(chan os.Signal, 1),
		doneChan: make(chan struct{}),
		startTS: dlog.TimeString(time.Now()),
	}
	engine.cond.L = &engine.mutex

	var err error
	keeper, err := getServantInfo("\x00", &_KeeperServant{engine:engine})
	if err != nil {
		panic("failed to getServantInfo for the _KeeperServant")
	}
	engine.keeper = keeper
	engine.adapterMap = make(map[string]*_Adapter)
	engine.proxyMap = make(map[string]*_Proxy)
	engine.outConMap = make(map[string]*_Connection)

	shadow := setting.Pathname("xic.passport.shadow")
	if shadow != "" {
		engine.shadowBox, err = NewShadowBoxFromFile(shadow)
		if err != nil {
			dlog.Allog(dlog.Id(), "XIC.WARN", "", "Failed to open shadow file %#v", shadow)
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
			dlog.Allog(dlog.Id(), "XIC.WARN", "", "Failed to open secret file %#v", secret)
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

func (engine *_Engine) Throb(fn func()string) {
	if fn != nil {
		engine.throbFunc.Store(fn)
	}
	engine.once.Do(func() {
		dlog.Allog(dlog.Id(), "DEBUT", ENGINE_VERSION, "id=%s start=%s", engine.id, engine.startTS)
		go engine.throb_routine()
	})
}

func sleepDuration(now time.Time) time.Duration {
	// wake up at every minute and 0.4 second, i.e.
	// at 00:00.4, 01:00.4, 02:00.4, 03:00.4, ...
	// in case the clock is not acurate enough
	_, _, sec := now.Clock()
	return time.Second * time.Duration(60 - sec) + 400_000_000 - time.Duration(now.Nanosecond())
}

func (engine *_Engine) throb_routine() {
	now := time.Now()
	engine.ticker = time.NewTicker(sleepDuration(now))
	for {
		select {
		case <-engine.doneChan:
			goto done
		case t := <-engine.ticker.C:
			engine.ticker.Reset(sleepDuration(t))
			fn := engine.throbFunc.Load()
			if fn == nil {
				dlog.Allog(dlog.Id(), "THROB", ENGINE_VERSION, "id=%s start=%s", engine.id, engine.startTS)
				continue
			}

			s := fn.(func()string)()
			if s != "" {
				dlog.Allog(dlog.Id(), "THROB", ENGINE_VERSION, "id=%s start=%s %s", engine.id, engine.startTS, s)
			}
		}
	}
done:
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
			return nil, xerr.Errorf("%#v not given", itemkey)
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
	engine.mutex.Lock()
	defer engine.mutex.Unlock()

	if engine.slackAdapter != nil {
		return nil, xerr.Errorf("SlackAdapter already created")
	}

	adapter, err := newAdapter(engine, SLACK_ADAPTER_NAME, "")
	if err != nil {
		return nil, err
	}
	engine.slackAdapter = adapter
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
		return xerr.Errorf("Adapter(%s) already created", adapter.Name())
	}
	engine.adapterMap[adapter.Name()] = adapter
	return nil
}

func (engine *_Engine) findAdapter(name string) *_Adapter {
	engine.mutex.Lock()
	defer engine.mutex.Unlock()
	return engine.adapterMap[name]
}

func (engine *_Engine) getAllAdapters(aps map[string]string) {
	engine.mutex.Lock()
	defer engine.mutex.Unlock()

	for k, v := range engine.adapterMap {
		aps[k] = v.endpoints
	}
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
		dlog.Allog(dlog.Id(), "XIC.INFO", "", "Signal (%s) received, shutting down.", sig.String())
	} else {
		dlog.Allog(dlog.Id(), "XIC.INFO", "", "Signal channel closed, shutting down.")
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

	close(engine.doneChan)

	engine.mutex.Lock()
	engine.state = eng_SHUTTED
	engine.cond.Broadcast()
	engine.mutex.Unlock()
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

