package xic

import (
	"fmt"
	"net"
	"bytes"
	"strings"
	"errors"
	"sync"
	"reflect"
	"runtime"
	"sync/atomic"
	"encoding/binary"
	"math/big"
	"container/list"

	"halftwo/mangos/dlog"
	"halftwo/mangos/srp6a"
)


type _Current struct {
	_InQuest
	con *_Connection
	args Arguments
}

func newCurrent(con *_Connection, q *_InQuest) *_Current {
	return &_Current{_InQuest:*q, con:con}
}

func (cur *_Current) Txid() int64 {
	return cur.txid
}

func (cur *_Current) Service() string {
	return cur.service
}

func (cur *_Current) Method() string {
	return cur.method
}

func (cur *_Current) Ctx() Context {
	return cur.ctx
}

func (cur *_Current) Args() Arguments {
	if cur.args == nil {
		cur.args = NewArguments()
		cur.DecodeArgs(cur.args)
	}
	return cur.args
}

func (cur *_Current) Con() Connection {
	return cur.con
}


type _ConState int32
const (
	con_INIT _ConState = iota
	con_WAITING_HELLO	// client waiting for server hello message
	con_ACTIVE
	con_CLOSE		// Close is called
	con_CLOSING		// graceful closing is in process
	con_CLOSED
	con_ERROR
)

type _Connection struct {
	c net.Conn
	state _ConState		// atomic
	engine *_Engine
	incoming bool
	adapter atomic.Value	// Adapter
	serviceHint string
	cipher *_Cipher
	timeout int
	concurrent int
	endpoint *EndpointInfo
	lastTxid int64
	pending map[int64]*Invoking
	mq *list.List
	mutex sync.Mutex
	err error
}

func _newConnection(engine *_Engine, incoming bool) *_Connection {
	con := &_Connection{engine:engine, incoming:incoming}
	con.pending = make(map[int64]*Invoking)
	con.mq = list.New()
	return con
}

func newOutgoingConnection(engine *_Engine, serviceHint string, ei *EndpointInfo) *_Connection {
	con := _newConnection(engine, false)
	con.endpoint = ei
	go con.client_run()
	return con
}

func newIncomingConnection(adapter *_Adapter, c net.Conn) *_Connection {
	con := _newConnection(adapter.engine, true)
	con.c = c
	con.adapter.Store(adapter)
	adapter.engine.incomingConnection(con)
	go con.server_run()
	return con
}

func (con *_Connection) String() string {
	laddr := con.c.LocalAddr()
	raddr := con.c.RemoteAddr()
	switch l := laddr.(type) {
	case *net.TCPAddr:
		r := raddr.(*net.TCPAddr)
		return fmt.Sprintf("tcp/%s+%d/%s+%d", l.IP.String(), l.Port, r.IP.String(), r.Port)
	case *net.UDPAddr:
		r := raddr.(*net.UDPAddr)
		return fmt.Sprintf("udp/%s+%d/%s+%d", l.IP.String(), l.Port, r.IP.String(), r.Port)
	}
	return fmt.Sprintf("%s/%s/%s", laddr.Network(), laddr.String(), raddr.String())
}

func (con *_Connection) IsLive() bool {
	state := _ConState(atomic.LoadInt32((*int32)(&con.state)))
	return state < con_CLOSE
}

func (con *_Connection) Incoming() bool {
	return con.incoming
}

func (con *_Connection) Timeout() int {
	return con.timeout
}

func (con *_Connection) Endpoint() string {
	return con.endpoint.String()
}

func (con *_Connection) Close(force bool) {
	// TODO
	if force {
		con.shut()
	}

	pending := con.pending
	con.pending = nil
	for _, ivk := range pending {
		ex := NewException(ConnectionClosedException, "")
		ivk.Err = ex
		ivk.Done <- ivk
	}
}

func (con *_Connection) CreateProxy(service string) (Proxy, error) {
	if strings.IndexByte(service, '@') >= 0 {
		return nil, errors.New("Service name can't contain '@'")
	}
	if con.pending == nil {
		con.pending = make(map[int64]*Invoking)
	}
	prx, err := con.engine.makeFixedProxy(service, con)
	return prx, err
}


func (con *_Connection) Adapter() Adapter {
	a := con.adapter.Load()
	if a != nil {
		return a.(Adapter)
	}
	return nil
}

func (con *_Connection) SetAdapter(adapter Adapter) {
	con.mutex.Lock()
	con.adapter.Store(adapter)
	con.mutex.Unlock()
}

func (con *_Connection) generateTxid() int64 {
	con.lastTxid++
	if con.lastTxid < 0 {
		con.lastTxid = 1
	}
	txid := con.lastTxid
	return txid
}

func (con *_Connection) invoke(prx *_Proxy, q *_OutQuest, vk *Invoking) error {
	if vk != nil && vk.Txid != 0 {
		con.mutex.Lock()
		txid := con.generateTxid()
		vk.Txid = txid
		con.pending[txid] = vk
		q.SetTxid(txid)
		con.mutex.Unlock()
	}
	con.sendMsg(q)
	return nil
}

func (con *_Connection) shut() {
	con.c.Close()
	// TODO
}

func (con *_Connection) grace() {
	// TODO
	con.sendMsg(theByeMessage)
}

func makePointerValue(t reflect.Type) reflect.Value {
	var p reflect.Value
	if t.Kind() == reflect.Ptr {
		p = reflect.New(t.Elem())
	} else {
		p = reflect.New(t)
	}

	elem := p.Elem()
	if elem.Kind() == reflect.Map {
		elem.Set(reflect.MakeMap(elem.Type()))
	}
	return p
}


type _ForbiddenArgs struct {
	Reason string	`vbs:"reason"`
}
type _AuthArgs struct {
	Method string	`vbs:"method"`
}
type _S1Args struct {
	I string	`vbs:"I"`
}
type _S2Args struct {
	Hash string	`vbs:"hash"`
	N []byte	`vbs:"N"`
	Gen []byte	`vbs:"g"`
	Salt []byte	`vbs:"s"`
	B []byte	`vbs:"B"`
}
type _S3Args struct {
	A []byte	`vbs:"A"`
	M1 []byte	`vbs:"M1"`
}
type _S4Args struct {
	M2 []byte	`vbs:"M2"`
	Cipher string	`vbs:"CIPHER"`
	Mode int	`vbs:"MODE"`
}

func (con *_Connection) check_send(cmd string, args any) bool {
	msg := newOutCheck(cmd, args)
	con.sendMsg(msg)
	// TODO
	return con.err == nil
}

func (con *_Connection) check_expect(cmd string, args any) bool {
	msg := con.readMsg()
	check, ok := msg.(*_InCheck)
	if !ok || check.cmd != cmd {
		// TODO
		return false
	}
	check.DecodeArgs(args)
	return con.err == nil
}

const (
	ck_AUTHENTICATE = "AUTHENTICATE"
	ck_FORBIDDEN = "FORBIDDEN"
	ck_SRP6a1 = "SRP6a1"
	ck_SRP6a2 = "SRP6a2"
	ck_SRP6a3 = "SRP6a3"
	ck_SRP6a4 = "SRP6a4"
)

func (con *_Connection) server_handshake() {
	if con.engine.shadowBox != nil {
		// TODO
		var auth _AuthArgs
		auth.Method = "SRP6a"
		if !con.check_send(ck_AUTHENTICATE, &auth) {
			return
		}

		var s1 _S1Args
		if !con.check_expect(ck_SRP6a1, &s1) {
			return
		}

		v := con.engine.shadowBox.GetVerifier(s1.I)
		if v == nil {
			con.err = errors.New("No such identity")
			return
		}

		var s2 _S2Args
		srp6svr, err := con.engine.shadowBox.CreateSrp6aServer(v.ParamId, v.HashId)
		if err != nil {
			con.err = err
			return
		}
		srp6svr.SetV(v.Verifier)
		s2.Hash = srp6svr.HashName()
		s2.N = srp6svr.N()
		s2.Gen = srp6svr.G()
		s2.Salt = v.Salt
		s2.B = srp6svr.GenerateB()
		if !con.check_send(ck_SRP6a2, &s2) {
			return
		}

		var s3 _S3Args
		con.check_expect(ck_SRP6a3, &s3)
		srp6svr.SetA(s3.A)
		M1 := srp6svr.ComputeM1()
		if !bytes.Equal(M1, s3.M1) {
			con.err = errors.New("srp6a M1 not equal")
			return
		}

		var s4 _S4Args
		s4.M2 = srp6svr.ComputeM2()
		s4.Cipher = "AES128-EAX"	// TEST
		s4.Mode = 1			// TEST
		if !con.check_send(ck_SRP6a4, &s4) {
			return
		}
	}

	err := con.sendMsg(theHelloMessage)
	if err != nil {
		// TODO
	}
	con.state = con_ACTIVE
}

func (con *_Connection) client_handshake() {
	msg := con.readMsg()

	if msg != nil && msg.Type() == 'C' {
		var auth _AuthArgs
		if !con.check_expect(ck_AUTHENTICATE, &auth) {
			return
		}

		if auth.Method != "SRP6a" {
			con.err = errors.New("Unknown auth method")
			return
		}

		if con.engine.secretBox == nil {
			con.err = errors.New("No SecretBox supplied")
			return
		}

		id, pass := con.engine.secretBox.FindEndpoint(con.serviceHint, con.endpoint)
		if id == "" || pass == "" {
			con.err = errors.New("No matched secret found")
			return
		}

		srp6cl := srp6a.NewClientEmpty()
		srp6cl.SetIdentity(id, pass)

		var s1 _S1Args
		s1.I = id
		if !con.check_send(ck_SRP6a1, &s1) {
			return
		}

		var s2 _S2Args
		con.check_expect(ck_SRP6a2, &s2)
		g := new(big.Int).SetBytes(s2.Gen)
		srp6cl.SetHash(s2.Hash)
		srp6cl.SetParameter(int(g.Int64()), s2.N, len(s2.N) * 8)
		srp6cl.SetSalt(s2.Salt)
		srp6cl.SetB(s2.B)

		var s3 _S3Args
		s3.A = srp6cl.GenerateA()
		s3.M1 = srp6cl.ComputeM1()
		if !con.check_send(ck_SRP6a3, &s3) {
			return
		}

		var s4 _S4Args
		if !con.check_expect(ck_SRP6a4, &s4) {
			return
		}

		M2 := srp6cl.ComputeM2()
		if !bytes.Equal(M2, s4.M2) {
			con.err = errors.New("srp6a M2 not equal")
			return
		}

		msg = con.readMsg()
	}

	if msg != nil && msg.Type() != 'H' {
		con.err = errors.New("Unexpected message received, expect Hello message")
		return
	}

	con.state = con_ACTIVE
}

func (con *_Connection) server_run() {
	con.server_handshake()
	if con.err != nil {
		var forbidden _ForbiddenArgs
		forbidden.Reason = con.err.Error()
		con.check_send(ck_FORBIDDEN, &forbidden)
		// TODO
		return
	}
	con.process_loop()
}

func (con *_Connection) client_run() {
	var err error
	ei := con.endpoint
	con.c, err = net.Dial(ei.proto, ei.Address())
	if err != nil {
		con.state = con_ERROR
		// TODO
		return
	}

	con.client_handshake()
	if con.err != nil {
		// TODO
		return
	}
	con.process_loop()
}

func (con *_Connection) handleQuest(adapter Adapter, quest *_InQuest) {
	var err error
	var si *ServantInfo
	if adapter == nil {
		err = NewException(AdapterAbsentException, "")
	} else {
		si = adapter.FindServant(quest.service)
		if si == nil {
			si := adapter.DefaultServant()
			if si == nil {
				err = NewExceptionf(ServiceNotFoundException, "%s", quest.service)
			}
		}
	}

	oneway := false
	var answer *_OutAnswer
	cur := newCurrent(con, quest)
	if err == nil {
		mi, ok := si.methods[quest.method]
		if ok {
			in := makePointerValue(mi.inType)
			err = cur.DecodeArgs(in.Interface())
			if mi.inType.Kind() != reflect.Ptr {
				in = in.Elem()
			}

			fun := mi.method.Func
			oneway = mi.oneway
			if oneway {
				fun.Call([]reflect.Value{reflect.ValueOf(si.Servant), reflect.ValueOf(cur), in})
			} else {
				out := makePointerValue(mi.outType)
				rts := fun.Call([]reflect.Value{reflect.ValueOf(si.Servant), reflect.ValueOf(cur), in, out})
				if !rts[0].IsNil() {
					err = rts[0].Interface().(error)
				} else {
					answer = newOutAnswerNormal(out.Interface())
				}
			}
		} else {
			outArgs := NewArguments()
			err = si.Servant.Xic(cur, cur.Args(), &outArgs)
			if err == nil {
				answer = newOutAnswerNormal(outArgs)
			}
		}
	}

	ZZZ(err)
	if quest.txid != 0 {
		if oneway && err == nil {
			err = fmt.Errorf("Oneway method invoked as twoway")
		}

		if err != nil {
			outErr := NewArguments()
			outErr.Set("raiser", fmt.Sprintf("%s*%s @", cur.Method(), cur.Service()))
			ex, ok := err.(Exception)
			if ok {
				outErr.Set("exname", ex.Exname())
				outErr.Set("code", ex.Code())
				outErr.Set("tag", ex.Tag())
				outErr.Set("message", ex.Message())
				detail := map[string]interface{}{"file":ex.File(), "line":ex.Line()}
				outErr.Set("detail", detail)
			} else {
				outErr.Set("message", err.Error())
			}
			answer = newOutAnswerExceptional(outErr)
		} else if answer == nil {
			panic("Can't reach here")
		}

		answer.SetTxid(quest.txid)
		con.sendMsg(answer)
	} else if err != nil {
		// TODO: log the error
	}
}

func (con *_Connection) handleAnswer(answer *_InAnswer) {
	ivk, ok := con.pending[answer.txid]
	if !ok {
		dlog.Log("WARNING", "Unknown answer with txid=%d", answer.txid)
		return
	}
	delete(con.pending, answer.txid)

	if answer.status != 0 {
		args := NewArguments()
		ivk.Err = answer.DecodeArgs(args)
		if ivk.Err == nil {
			ivk.Err = &_Exception{name:args.GetString("exname"),
					code:int(args.GetInt("code")),
					tag:args.GetString("tag"),
					msg:args.GetString("message")}
		}
	} else {
		ivk.Err = answer.DecodeArgs(ivk.Out)
	}

	ivk.Done <- ivk
}

func checkHeader(header _MessageHeader) error {
	if header.Magic != 'X' || header.Version != '!' {
		return fmt.Errorf("Unknown message Magic(%d) and Version(%d)", header.Magic, header.Version)
	}

	switch header.Type {
	case 'Q', 'A', 'C':
		if (header.Flags &^ 0x01) != 0 {
			return errors.New("Unknown message Flags")
		} else if int(header.BodySize) > MaxMessageSize {
			return errors.New("Message size too large")
		}
	case 'H', 'B':
		if header.Flags != 0 || header.BodySize != 0 {
			return fmt.Errorf("Invalid Hello or Bye message")
		}
	default:
		return fmt.Errorf("Unknown message Type(%d)", header.Type)
	}

	return nil
}

func ZZZ(x ...interface{}) {
	_, file, line, _ := runtime.Caller(1)
	fmt.Println("XXX", file, line, x)
}

func (con *_Connection) readMsg() _Message {
	var header _MessageHeader
	if con.err = binary.Read(con.c, binary.BigEndian, &header); con.err != nil {
		return nil
	}

	if con.err = checkHeader(header); con.err != nil {
		return nil
	}

	buf := make([]byte, header.BodySize)
	n, err := con.c.Read(buf)
	if err != nil {
		con.err = err
		return nil
	} else if n != len(buf) {
		con.err = fmt.Errorf("Received less data (%d) than specified in the header (%d)", n, len(buf))
		return nil
	}

	msg, err := DecodeMessage(header, buf)
	con.err = err
	return msg
}

func (con *_Connection) sendMsg(msg _OutMessage) error {
	buf := msg.Bytes()
	_, err := con.c.Write(buf)
	return err
}


func (con *_Connection) send_loop() {
	for {
		// TODO
	}
}

func (con *_Connection) process_loop() {
	go con.send_loop()

	for {
		msg := con.readMsg()
		if msg == nil {
			break
		}

		switch msg.Type() {
		case 'Q':
			adp := con.adapter.Load()
			adapter := adp.(Adapter)
			quest := msg.(*_InQuest)
			if con.concurrent > 1 {
				go con.handleQuest(adapter, quest)
			} else {
				con.handleQuest(adapter, quest)
			}
			// TODO

		case 'A':
			answer := msg.(*_InAnswer)
			con.handleAnswer(answer)

		case 'B':
			// TODO
			break

		case 'C':
			con.err = errors.New("Unexpected Check message received")
			break
		case 'H':
			con.err = errors.New("Unexpected Hello message received")
			break
		}
	}

	if con.err != nil {
		con.shut()
	} else {
		con.grace()
	}
}

