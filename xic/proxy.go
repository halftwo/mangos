package xic

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"halftwo/mangos/xstr"
)

type _Proxy struct {
	engine    *_Engine
	service   string
	str       string
	lb        LoadBalance
	fixed     bool
	ctx       atomic.Value // Context
	cons      []*_Connection
	endpoints []string
}

func (lb LoadBalance) String() string {
	switch lb {
	case LB_HASH:
		return "-lb:hash"
	case LB_RANDOM:
		return "-lb:random"
	case LB_NORMAL:
		return "-lb:normal"
	}
	return ""
}

func newProxy(engine *_Engine, proxy string) *_Proxy {
	sp := xstr.NewSplitter(proxy, "@")
	tk := xstr.NewTokenizerSpace(sp.Next())

	service := tk.Next()
	prx := &_Proxy{engine: engine, service: service}
	prx.ctx.Store(Context{})

	for tk.HasMore() {
		s := tk.Next()
		switch {
		case s == "-lb:hash":
			prx.lb = LB_HASH
		case s == "-lb:random":
			prx.lb = LB_RANDOM
		case s == "-lb:normal":
			prx.lb = LB_NORMAL
		}
	}

	for sp.HasMore() {
		endpoint := sp.Next()
		ep, err := parseEndpoint(endpoint)
		if err != nil {
			continue
		}
		prx.endpoints = append(prx.endpoints, ep.String())
	}

	bd := &strings.Builder{}
	bd.WriteString(service)
	if prx.lb != LB_NORMAL {
		bd.WriteByte(' ')
		bd.WriteString(prx.lb.String())
	}

	if len(prx.endpoints) > 0 {
		for _, ep := range prx.endpoints {
			bd.WriteByte(' ')
			bd.WriteString(ep)
		}
	}
	prx.str = bd.String()
	return prx
}

func newProxyConnection(engine *_Engine, service string, con *_Connection) *_Proxy {
	prx := &_Proxy{engine: engine, service: service, str: service}
	prx.ctx.Store(Context{})
	if con != nil {
		prx.fixed = true
		prx.cons = append(prx.cons, con)
	}
	return prx
}

func (prx *_Proxy) Engine() Engine {
	return prx.engine
}

func (prx *_Proxy) Service() string {
	return prx.service
}

func (prx *_Proxy) String() string {
	return prx.str
}

func (prx *_Proxy) Context() Context {
	ctx := prx.ctx.Load().(Context)
	return ctx
}

func (prx *_Proxy) SetContext(ctx Context) {
	if ctx != nil {
		prx.ctx.Store(ctx)
	}
}

func (prx *_Proxy) Connection() Connection {
	// TODO
	var con Connection
	return con
}

func (prx *_Proxy) ResetConnection() {
	for _, c := range prx.cons {
		c.Close(false)
	}
	prx.cons = prx.cons[:0]
}

func (prx *_Proxy) LoadBalance() LoadBalance {
	return prx.lb
}

func (prx *_Proxy) TimedProxy(timeout, closeTimeout, connectTimeout int) Proxy {
	// TODO
	var prx2 Proxy
	return prx2
}

func (prx *_Proxy) pickConnection(q *_OutQuest) (*_Connection, error) {
	// TODO

	if len(prx.cons) == 0 {
		var con *_Connection
		var err error
		for _, ep := range prx.endpoints {
			con, err = prx.engine.makeConnection(prx.service, ep)
			if con != nil {
				break
			}
		}
		if con == nil {
			return nil, err
		}
		prx.cons = append(prx.cons, con)
	}
	return prx.cons[0], nil
}

type _Result struct {
	txid     int64
	service  string
	method   string
	in       any
	out      any
	deadline time.Time
	err      error
	done     atomic.Bool
	mtx      sync.Mutex
	cond     sync.Cond
}

func (r *_Result) Txid() int64     { return r.txid }
func (r *_Result) Service() string { return r.service }
func (r *_Result) Method() string  { return r.method }
func (r *_Result) In() any         { return r.in }
func (r *_Result) Out() any        { return r.out }
func (r *_Result) Err() error      { return r.err }
func (r *_Result) Done() bool      { return r.done.Load() }

func (r *_Result) Wait() {
	r.cond.L.Lock()
	for !r.done.Load() {
		r.cond.Wait()
	}
	r.cond.L.Unlock()
}

func (r *_Result) broadcast() {
	r.cond.L.Lock()
	r.done.Store(true)
	r.cond.Broadcast()
	r.cond.L.Unlock()
}

func (prx *_Proxy) Invoke(method string, in, out any) error {
	return prx.InvokeCtx(prx.Context(), method, in, out)
}

func (prx *_Proxy) InvokeCtx(ctx Context, method string, in, out any) error {
	result := prx.InvokeCtxAsync(ctx, method, in, out)
	return result.Err()
}

func (prx *_Proxy) InvokeOneway(method string, in any) error {
	return prx.InvokeCtxOneway(prx.Context(), method, in)
}

func (prx *_Proxy) InvokeCtxOneway(ctx Context, method string, in any) error {
	q := newOutQuest(0, prx.service, method, ctx, in)
	con, err := prx.pickConnection(q)
	if err != nil {
		return err
	}

	con.invoke(prx, q, nil)
	return nil
}

func (prx *_Proxy) InvokeAsync(method string, in, out any) Result {
	return prx.InvokeCtxAsync(prx.Context(), method, in, out)
}

func (prx *_Proxy) InvokeCtxAsync(ctx Context, method string, in, out any) Result {
	res := &_Result{txid: -1, service: prx.service, method: method, in: in, out: out}
	res.cond.L = &res.mtx
	q := newOutQuest(-1, prx.service, method, ctx, in)
	con, err := prx.pickConnection(q)
	if err != nil {
		res.err = err
		res.broadcast()
	} else {
		con.invoke(prx, q, res)
	}
	return res
}
