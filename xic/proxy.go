package xic

import (
	"fmt"
	"sync/atomic"
	"strings"

	"halftwo/mangos/xstr"
)

type stdProxy struct {
	engine *stdEngine
	service string
	str string
	lb LoadBalance
	fixed bool
	ctx atomic.Value	// Context
	cons []*stdConnection
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

func newProxy(engine *stdEngine, proxy string) *stdProxy {
	sp := xstr.NewSplitter(proxy, "@")
	tk := xstr.NewTokenizerSpace(sp.Next())

	service := tk.Next()
	prx := &stdProxy{engine:engine, service:service}
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

func newProxyConnection(engine *stdEngine, service string, con *stdConnection) *stdProxy {
	prx := &stdProxy{engine:engine, service:service, str:service}
	prx.ctx.Store(Context{})
	if con != nil {
		prx.fixed = true
		prx.cons = append(prx.cons, con)
	}
	return prx
}

func (prx *stdProxy) Engine() Engine {
	return prx.engine
}

func (prx *stdProxy) Service() string {
	return prx.service
}

func (prx *stdProxy) String() string {
	return prx.str
}

func (prx *stdProxy) Context() Context {
	ctx := prx.ctx.Load().(Context)
	return ctx
}

func (prx *stdProxy) SetContext(ctx Context) {
	if ctx != nil {
		prx.ctx.Store(ctx)
	}
}

func (prx *stdProxy) Connection() Connection {
	// TODO
	var con Connection
	return con
}

func (prx *stdProxy) ResetConnection() {
	for _, c := range prx.cons {
		c.Close(false)
	}
	prx.cons = prx.cons[:0]
}

func (prx *stdProxy) LoadBalance() LoadBalance {
	return prx.lb
}

func (prx *stdProxy) TimedProxy(timeout, closeTimeout, connectTimeout int) Proxy {
	// TODO
	var prx2 Proxy
	return prx2
}


func (prx *stdProxy) pickConnection(q *outQuest) (*stdConnection, error) {
	// TODO

	if len(prx.cons) == 0 {
		var con *stdConnection
		for _, ep := range prx.endpoints {
			con = prx.engine.makeConnection(prx.service, ep)
			if con != nil {
				break
			}
		}
		if con == nil {
			return nil, fmt.Errorf("No connection")
		}
		prx.cons = append(prx.cons, con)
	}
	return prx.cons[0], nil
}

func (prx *stdProxy) Invoke(method string, in interface{}, out interface{}) error {
	return prx.InvokeCtx(prx.Context(), method, in, out)
}

func (prx *stdProxy) InvokeCtx(ctx Context, method string, in interface{}, out interface{}) error {
	ivk := prx.InvokeCtxAsync(ctx, method, in, out, nil)
	select {
	case <-ivk.Done:
	}
	return ivk.Err
}

func (prx *stdProxy) InvokeOneway(method string, in interface{}) error {
	return prx.InvokeCtxOneway(prx.Context(), method, in)
}

func (prx *stdProxy) InvokeCtxOneway(ctx Context, method string, in interface{}) error {
	q := newOutQuest(0, prx.service, method, ctx, in)
	con, err := prx.pickConnection(q)
	if err != nil {
		return err
	}

	con.invoke(prx, q, nil)
	return nil
}

func (prx *stdProxy) InvokeAsync(method string, in interface{}, out interface{}, done chan *Invoking) *Invoking {
	return prx.InvokeCtxAsync(prx.Context(), method, in, out, done)
}

func (prx *stdProxy) InvokeCtxAsync(ctx Context, method string, in interface{}, out interface{}, done chan *Invoking) *Invoking {
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

