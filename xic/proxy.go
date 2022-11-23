package xic

import (
        "fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
        "errors"
        "math/rand"
        "hash/crc32"
        "hash/crc64"

	"halftwo/mangos/xstr"
	"halftwo/mangos/carp"
	"halftwo/mangos/dlog"
)

type _Proxy struct {
	engine	*_Engine
	service string
	str     string
	lb      LoadBalance
	fixed   bool
	ctx     atomic.Value // Context
	cons    []*_Connection
	endpoints []string
	idx	int
        cseq    carp.Carp
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

var crc64Table = crc64.MakeTable(0x95AC9329AC4BC9B5)

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
	prx.cons = make([]*_Connection, len(prx.endpoints))
	prx.str = bd.String()

        if prx.lb == LB_HASH {
		members := make([]uint64, len(prx.endpoints))
		for i := 0; i < len(prx.endpoints); i++ {
			members[i] = crc64.Checksum([]byte(prx.endpoints[i]), crc64Table)
		}
                prx.cseq = carp.NewCarp(members, nil)
        }
	return prx
}

func newProxyWithConnection(engine *_Engine, service string, con *_Connection) *_Proxy {
	prx := &_Proxy{engine: engine, service: service, str: service}
	prx.ctx.Store(Context{})
	prx.fixed = true
	prx.cons = append(prx.cons, con)
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

func (prx *_Proxy) LoadBalance() LoadBalance {
	return prx.lb
}

func (prx *_Proxy) TimedProxy(timeout, closeTimeout, connectTimeout int) Proxy {
	// TODO
	panic("Not implemented")
	var prx2 Proxy
	return prx2
}

func (prx *_Proxy) pick_random() (con *_Connection, err error) {
        num := len(prx.cons)
        k := rand.Intn(num)
        con = prx.cons[k]
        // TODO: eleminate error connection
        if con == nil || !con.IsLive() {
                con, err = prx.engine.makeConnection(prx.service, prx.endpoints[k])
                if err != nil {
                        return
                }
                prx.cons[k] = con
        }
        return
}

func (prx *_Proxy) pick_hash(ctx Context) (con *_Connection, err error) {
        xichint := ctx.Get("XIC_HINT")
	if xichint == nil {
		dlog.Log("XIC.WARN", "XIC_HINT not specified in context")
		return prx.pick_normal()
	}

        var hint uint32
        switch v := xichint.(type) {
        case int64:
                hint = uint32(v >> 32) ^ uint32(v)
        case string:
                hint = crc32.ChecksumIEEE([]byte(v))
        case []byte:
                hint = crc32.ChecksumIEEE(v)
        case float32, float64:
                s := fmt.Sprintf("%.16G", v);
                hint = crc32.ChecksumIEEE([]byte(s))
	default:
		dlog.Log("XIC.WARN", "XIC_HINT invalid in context")
		return prx.pick_normal()
        }

	k := prx.cseq.Which(hint)
	con = prx.cons[k]
	if con == nil || !con.IsLive() {
                con, err = prx.engine.makeConnection(prx.service, prx.endpoints[k])
                if err != nil {
                        return
                }
                prx.cons[k] = con
		return
	}

	/*
        var seqs [5]int
	ss := prx.cseq.Sequence(hint, seqs[:])
	*/
	// TODO
	return nil, errors.New("Not Implemented")
}

func (prx *_Proxy) pick_normal() (*_Connection, error) {
	con := prx.cons[prx.idx]
	if con == nil || !con.IsLive() {
		if prx.fixed {
			return nil, errors.New("Broken connection of fixed proxy")
		}

		prx.idx++
		if prx.idx >= len(prx.endpoints) {
			prx.idx = 0
		}
		var err error
		con, err = prx.engine.makeConnection(prx.service, prx.endpoints[prx.idx])
		if err != nil {
			return con, err
		}
		prx.cons[prx.idx] = con
	}
	return con, nil
}

func (prx *_Proxy) pickConnection(ctx Context) (*_Connection, error) {
	if prx.lb == LB_NORMAL || len(prx.cons) == 1 {
		return prx.pick_normal()
	} else if (prx.lb == LB_RANDOM) {
		return prx.pick_random()
	}

	// LB_HASH
	return prx.pick_hash(ctx)
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
	res := prx.InvokeCtxAsync(ctx, method, in, out)
	res.Wait()
	return res.Err()
}

func (prx *_Proxy) InvokeOneway(method string, in any) error {
	return prx.InvokeCtxOneway(nil, method, in)
}

func (prx *_Proxy) InvokeCtxOneway(ctx Context, method string, in any) error {
	if ctx != nil {
		ctx.Extend(prx.Context())
	} else {
		ctx = prx.Context()
	}
	q := newOutQuest(0, prx.service, method, ctx, in)
	con, err := prx.pickConnection(ctx)
	if err != nil {
		return err
	}

	con.invoke(prx, q, nil)
	return nil
}

func (prx *_Proxy) InvokeAsync(method string, in, out any) Result {
	return prx.InvokeCtxAsync(nil, method, in, out)
}

func (prx *_Proxy) InvokeCtxAsync(ctx Context, method string, in, out any) Result {
	if ctx != nil {
		ctx.Extend(prx.Context())
	} else {
		ctx = prx.Context()
	}

	res := &_Result{txid: -1, service: prx.service, method: method, in: in, out: out}
	res.cond.L = &res.mtx

	con, err := prx.pickConnection(ctx)
	if err != nil {
		res.err = err
	} else {
		q := newOutQuest(-1, prx.service, method, ctx, in)
		con.invoke(prx, q, res)
	}

	if res.err != nil {
		res.broadcast()
	}
	return res
}

