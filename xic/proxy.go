package xic

import (
	"sync/atomic"
	"strings"

	"halftwo/mangos/xstr"
)

type _Proxy struct {
	engine *_Engine
	service string
	str string
	lb LoadBalance
	fixed bool
	ctx atomic.Value	// Context
	cons []*_Connection
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
	prx := &_Proxy{engine:engine, service:service}
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
	prx := &_Proxy{engine:engine, service:service, str:service}
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

func (prx *_Proxy) Invoke(method string, in interface{}, out interface{}) error {
	return prx.InvokeCtx(prx.Context(), method, in, out)
}

func (prx *_Proxy) InvokeCtx(ctx Context, method string, in interface{}, out interface{}) error {
	ivk := prx.InvokeCtxAsync(ctx, method, in, out, nil)
	select {
	case <-ivk.Done:
	}
	return ivk.Err
}

func (prx *_Proxy) InvokeOneway(method string, in interface{}) error {
	return prx.InvokeCtxOneway(prx.Context(), method, in)
}

func (prx *_Proxy) InvokeCtxOneway(ctx Context, method string, in interface{}) error {
	q := newOutQuest(0, prx.service, method, ctx, in)
	con, err := prx.pickConnection(q)
	if err != nil {
		return err
	}

	con.invoke(prx, q, nil)
	return nil
}

func (prx *_Proxy) InvokeAsync(method string, in interface{}, out interface{}, done chan *Invoking) *Invoking {
	return prx.InvokeCtxAsync(prx.Context(), method, in, out, done)
}

func (prx *_Proxy) InvokeCtxAsync(ctx Context, method string, in interface{}, out interface{}, done chan *Invoking) *Invoking {
	if done == nil {
		done = make(chan *Invoking, 1)
	} else if cap(done) == 0 {
		panic("cap(done) == 0")
	}

	ivk := &Invoking{Txid:-1, In:in, Out:out, Done:done}
	q := newOutQuest(-1, prx.service, method, ctx, in)
	con, err := prx.pickConnection(q)
	if err != nil {
		ivk.Err = err
		ivk.Done <- ivk
	} else {
		con.invoke(prx, q, ivk)
	}
	return ivk
}

