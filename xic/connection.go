package xic

import (
	"bytes"
	"container/list"
	"errors"
	"fmt"
	"math/big"
	"net"
	"reflect"
	"runtime"
	"strings"
	"time"
	"sync"
	"sync/atomic"
	"os"
	"io"

	"halftwo/mangos/dlog"
	"halftwo/mangos/srp6a"
)

type _Current struct {
	_InQuest
	con *_Connection
}

func newCurrent(con *_Connection, q *_InQuest) *_Current {
	return &_Current{_InQuest: *q, con: con}
}

func (cur *_Current) Txid() int64	{ return cur.txid }
func (cur *_Current) Service() string	{ return cur.service }
func (cur *_Current) Method() string	{ return cur.method }
func (cur *_Current) Ctx() Context	{ return cur.ctx }
func (cur *_Current) Con() Connection	{ return cur.con }

type _ConState int32
const (
	con_INIT	_ConState = iota
	con_CONNECT
	con_HANDSHAKE
	con_ACTIVE
	con_CLOSING		// graceful closing is in process
	con_BYE
	con_CLOSED
	con_ERROR
)

type _Connection struct {
	c		net.Conn
	state		_ConState
	engine          *_Engine
	incoming        bool
	adapter         atomic.Value // Adapter
	serviceHint     string
	cipher          *_Cipher
	timeout         time.Duration
	closeTimeout	time.Duration
	connectTimeout	time.Duration
	maxQ		int32
	numQ		atomic.Int32
	endpoint        *EndpointInfo
	lastTxid        int64
	pending         map[int64]*_Result
	mq              OutMsgQueue
	mutex           sync.Mutex
	cond		sync.Cond
	err             error
}

type OutMsgQueue struct {
	lst *list.List
	num int
}

func (q *OutMsgQueue) Num() int {
	return q.num
}

func (q *OutMsgQueue) PushBack(msg _OutMessage) {
	q.lst.PushBack(msg)
	q.num++
}

func (q *OutMsgQueue) PopFront() _OutMessage {
	e := q.lst.Front()
	if e == nil {
		return nil
	}
	q.lst.Remove(e)
	q.num--
	return e.Value.(_OutMessage)
}

func _newConnection(engine *_Engine, incoming bool) *_Connection {
	con := &_Connection{engine: engine, incoming: incoming}
	con.mq = OutMsgQueue{lst:list.New()}
	con.cond.L = &con.mutex
	con.pending = make(map[int64]*_Result)
	return con
}

func (con *_Connection) _set_timeouts(ei *EndpointInfo) {
	if ei != nil {
		con.timeout = timeout2duration(ei.timeout)
		con.closeTimeout = timeout2duration(ei.closeTimeout)
		con.connectTimeout = timeout2duration(ei.connectTimeout)

		if con.closeTimeout == 0 {
			con.closeTimeout = con.timeout
		}
		if con.connectTimeout == 0 {
			con.connectTimeout = con.timeout
		}
	}
}

func newOutgoingConnection(engine *_Engine, serviceHint string, ei *EndpointInfo) *_Connection {
	con := _newConnection(engine, false)
	con.endpoint = ei

	con._set_timeouts(ei)
	go con.client_run()
	return con
}

func newIncomingConnection(listener *_Listener, c net.Conn) *_Connection {
	adapter := listener.adapter
	con := _newConnection(adapter.engine, true)
	con.c = c
	con.endpoint = listener.endpoint
	con.adapter.Store(adapter)
	adapter.engine.incomingConnection(con)

	con._set_timeouts(con.endpoint)
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

func (con *_Connection) IsLive() bool { return con.state <= con_ACTIVE }
func (con *_Connection) Incoming() bool { return con.incoming }
func (con *_Connection) Timeout() uint32 { return uint32(con.timeout / time.Millisecond) }
func (con *_Connection) Endpoint() string { return con.endpoint.String() }

func (con *_Connection) Close(force bool) {
	if force {
		con.closeForcefully()
	} else {
		con.closeGracefully()
	}
}

func (con *_Connection) set_state(state _ConState) {
	con.mutex.Lock()
	if con.state < state {
		con.state = state
		con.cond.Broadcast()
	}
	con.mutex.Unlock()
}

func (con *_Connection) set_error(err error) {
	con.mutex.Lock()
	if con.state < con_CLOSED {
		con.state = con_ERROR
		con.err = err
		con.cond.Broadcast()
	}
	con.mutex.Unlock()
}

func (con *_Connection) closeGracefully() {
	con.set_state(con_CLOSING)
}

func (con *_Connection) closeForcefully() {
	err := NewException(ConnectionClosedException, "")
	con.set_error(err)	// TODO
	con.close_and_reply(false)
}

func (con *_Connection) close_and_reply(retryable bool) {
	con.mutex.Lock()
	netcon := con.c
	con.c = nil

	pending := con.pending
	con.pending = nil

	err := con.err
	if err == nil {
		con.state = con_CLOSED
		con.cond.Broadcast()
	}
	con.mutex.Unlock()

	if netcon != nil {
		netcon.Close()
	}

	// TODO: retryable

	for _, res := range pending {
		// TODO
		res.err = err
		res.broadcast()
	}
}

func (con *_Connection) CreateProxy(service string) (Proxy, error) {
	if strings.IndexByte(service, '@') >= 0 {
		return nil, errors.New("Service name can't contain '@'")
	}
	con.mutex.Lock()
	if con.pending == nil {
		con.pending = make(map[int64]*_Result)
	}
	con.mutex.Unlock()
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
	con.adapter.Store(adapter)
}

func (con *_Connection) _generate_txid() int64 {
	con.lastTxid++
	if con.lastTxid < 0 {
		con.lastTxid = 1
	}
	return con.lastTxid
}

func (con *_Connection) invoke(prx *_Proxy, q *_OutQuest, res *_Result) {
	ok := false
	con.mutex.Lock()
	if con.state <= con_ACTIVE {
		ok = true
		if q.txid != 0 {
			txid := con._generate_txid()
			res.txid = txid
			q.txid = txid
			con.pending[txid] = res
		}
	}
	con.mutex.Unlock()

	if ok {
		con.sendMessage(q)
	} else if res != nil {
		res.err = errors.New("Connection closing or closed")
	}
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
	Reason string `vbs:"reason"`
}
type _AuthArgs struct {
	Method string `vbs:"method"`
}
type _S1Args struct {
	I string `vbs:"I"`
}
type _S2Args struct {
	Hash string `vbs:"hash"`
	N    []byte `vbs:"N"`
	Gen  []byte `vbs:"g"`
	Salt []byte `vbs:"s"`
	B    []byte `vbs:"B"`
}
type _S3Args struct {
	A  []byte `vbs:"A"`
	M1 []byte `vbs:"M1"`
}
type _S4Args struct {
	M2     []byte `vbs:"M2"`
	Cipher string `vbs:"CIPHER"`
	Mode   int    `vbs:"MODE"`
}

func (con *_Connection) check_send(cmd string, args any) bool {
	msg := newOutCheck(cmd, args)
	if err := con.send_msg(msg); err != nil {
		con.set_error(err)
		return false
	}
	return true
}

func expect_check_msg(msg _Message, cmd string, args any) error {
	check, ok := msg.(*_InCheck)
	if !ok || check.cmd != cmd {
		return NewExceptionf(ProtocolException, "Unexpected cmd of CheckMessage %s", check.cmd)
	}
	return check.DecodeArgs(args)
}

func (con *_Connection) check_expect(cmd string, args any) bool {
	msg := con.must_read_msg()
	if msg == nil {
		return false
	}
	if err := expect_check_msg(msg, cmd, args); err != nil {
		con.set_error(err)
		return false
	}
	return true
}

const (
	ck_AUTHENTICATE = "AUTHENTICATE"
	ck_FORBIDDEN    = "FORBIDDEN"
	ck_SRP6a1       = "SRP6a1"
	ck_SRP6a2       = "SRP6a2"
	ck_SRP6a3       = "SRP6a3"
	ck_SRP6a4       = "SRP6a4"
)

func (con *_Connection) server_handshake() bool {
	var err error
	con.set_state(con_HANDSHAKE)
	if con.engine.shadowBox != nil {
		var auth _AuthArgs
		auth.Method = "SRP6a"
		if !con.check_send(ck_AUTHENTICATE, &auth) {
			goto done
		}

		var s1 _S1Args
		if !con.check_expect(ck_SRP6a1, &s1) {
			goto done
		}

		v := con.engine.shadowBox.GetVerifier(s1.I)
		if v == nil {
			err = errors.New("No such identity")
			goto done
		}

		var srp6svr *srp6a.Srp6aServer
		var s2 _S2Args
		srp6svr, err = con.engine.shadowBox.CreateSrp6aServer(v.ParamId, v.HashId)
		if err != nil {
			goto done
		}
		srp6svr.SetV(v.Verifier)
		s2.Hash = srp6svr.HashName()
		s2.N = srp6svr.N()
		s2.Gen = srp6svr.G()
		s2.Salt = v.Salt
		s2.B = srp6svr.GenerateB()
		if !con.check_send(ck_SRP6a2, &s2) {
			goto done
		}

		var s3 _S3Args
		con.check_expect(ck_SRP6a3, &s3)
		srp6svr.SetA(s3.A)
		M1 := srp6svr.ComputeM1()
		if !bytes.Equal(M1, s3.M1) {
			err = errors.New("srp6a M1 not equal")
			goto done
		}

		cihper_suite := con.engine.cipher
		var s4 _S4Args
		s4.M2 = srp6svr.ComputeM2()
		s4.Cipher = cihper_suite.String()
		s4.Mode = 1
		if !con.check_send(ck_SRP6a4, &s4) {
			goto done
		}

		key := srp6svr.ComputeK()
		if con.cipher, err = newXicCipher(cihper_suite, key, true); err != nil {
			goto done
		}
	}

	err = con.send_msg(theHelloMessage)
done:
	if err != nil {
		con.set_error(err)
		return false
	}
	return true
}

func (con *_Connection) client_handshake() bool {
	var err error
	con.set_state(con_HANDSHAKE)
	msg := con.must_read_msg()
	if msg != nil && msg.Type() == CheckMsgType {
		var auth _AuthArgs
		if err = expect_check_msg(msg, ck_AUTHENTICATE, &auth); err != nil {
			goto done
		}

		if auth.Method != "SRP6a" {
			err = errors.New("Unknown auth method")
			goto done
		}

		if con.engine.secretBox == nil {
			err = errors.New("No SecretBox supplied")
			goto done
		}

		id, pass := con.engine.secretBox.FindEndpoint(con.serviceHint, con.endpoint)
		if id == "" || pass == "" {
			err = errors.New("No matched secret found")
			goto done
		}

		srp6cl := srp6a.NewClientEmpty()
		srp6cl.SetIdentity(id, pass)

		var s1 _S1Args
		s1.I = id
		if !con.check_send(ck_SRP6a1, &s1) {
			goto done
		}

		var s2 _S2Args
		con.check_expect(ck_SRP6a2, &s2)
		g := new(big.Int).SetBytes(s2.Gen)
		srp6cl.SetHash(s2.Hash)
		srp6cl.SetParameter(int(g.Int64()), s2.N, len(s2.N)*8)
		srp6cl.SetSalt(s2.Salt)
		srp6cl.SetB(s2.B)

		var s3 _S3Args
		s3.A = srp6cl.GenerateA()
		s3.M1 = srp6cl.ComputeM1()
		if !con.check_send(ck_SRP6a3, &s3) {
			goto done
		}

		var s4 _S4Args
		if !con.check_expect(ck_SRP6a4, &s4) {
			goto done
		}

		M2 := srp6cl.ComputeM2()
		if !bytes.Equal(M2, s4.M2) {
			err = errors.New("srp6a M2 not equal")
			goto done
		}

		key := srp6cl.ComputeK()
		suite := String2CipherSuite(s4.Cipher)
		if con.cipher, err = newXicCipher(suite, key, false); err != nil {
			goto done
		}

		msg = con.must_read_msg()
	}

	if msg != nil && msg.Type() != HelloMsgType {
		err = errors.New("Unexpected message received, expect Hello message")
	}
done:
	if err != nil {
		con.set_error(err)
		return false
	}
	return true
}

func (con *_Connection) server_run() {
	if !con.server_handshake() {
		con.close_and_reply(true)
		return
	}

	con.process_loop()
}

func timeout2duration(timeout uint32) time.Duration {
	return time.Millisecond * time.Duration(timeout)
}

func timeout2deadline(timeout uint32) time.Time {
	if timeout > 0 {
		return time.Now().Add(time.Millisecond * time.Duration(timeout))
	}
	return time.Time{}
}

func (con *_Connection) client_run() {
	con.set_state(con_CONNECT)
	ei := con.endpoint
	netc, err := net.DialTimeout(ei.proto, ei.Address(), con.connectTimeout)
	if err != nil {
		con.set_error(err)
		con.close_and_reply(true)
		return
	}

	con.c = netc
	if !con.client_handshake() {
		con.close_and_reply(true)
		return
	}

	con.process_loop()
}

func err2OutAnswer(quest *_InQuest, err error) *_OutAnswer {
	outErr := NewArguments()
	outErr.Set("raiser", fmt.Sprintf("%s*%s @", quest.method, quest.service))
	ex, ok := err.(Exception)
	if ok {
		outErr.Set("exname", ex.Exname())
		outErr.Set("code", ex.Code())
		outErr.Set("tag", ex.Tag())
		outErr.Set("message", ex.Message())
		detail := map[string]any{"file": ex.File(), "line": ex.Line()}
		outErr.Set("detail", detail)
	} else {
		outErr.Set("message", err.Error())
	}
	answer := newOutAnswerExceptional(quest.txid, outErr)
	return answer
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
					answer = newOutAnswerNormal(quest.txid, out.Interface())
				}
			}
		} else {
			var inArgs Arguments
			var outArgs Arguments
			err = cur.DecodeArgs(&inArgs)
			if err == nil {
				err = si.Servant.Xic(cur, inArgs, &outArgs)
			}
			if err == nil {
				answer = newOutAnswerNormal(quest.txid, outArgs)
			}
		}
	}

	if quest.txid != 0 {
		if oneway && err == nil {
			err = fmt.Errorf("Oneway method invoked as twoway")
		}

		if err != nil {
			answer = err2OutAnswer(quest, err)
		} else if answer == nil {
			panic("Can't reach here")
		}

		con.sendMessage(answer)
	} else if err != nil {
		dlog.Log("XIC.WARN", "%s", err.Error())
	}
	con.engine.numQ.Add(-1)
}

func (con *_Connection) handleAnswer(answer *_InAnswer) {
	con.mutex.Lock()
	res, ok := con.pending[answer.txid]
	if ok {
		delete(con.pending, answer.txid)
	}
	if con.byebye_ok() {
		con.cond.Broadcast()
	}
	con.mutex.Unlock()

	if !ok {
		dlog.Log("XIC.WARN", "Unknown answer with txid=%d", answer.txid)
		return
	}

	if answer.status != 0 {
		args := NewArguments()
		res.err = answer.DecodeArgs(args)
		if res.err == nil {
			res.err = &_Exception{name: args.GetString("exname"),
				code: int(args.GetInt("code")),
				tag:  args.GetString("tag"),
				msg:  args.GetString("mess age")}
		}
	} else if res.out != nil {
		res.err = answer.DecodeArgs(res.out)
	}

	res.broadcast()
}

func checkHeader(header _MessageHeader) error {
	if header.Magic != 'X' || header.Version != '!' {
		return fmt.Errorf("Unknown message Magic(%d) and Version(%d)", header.Magic, header.Version)
	}

	switch header.Type {
	case QuestMsgType, AnswerMsgType, CheckMsgType:
		if (header.Flags &^ FLAG_MASK) != 0 {
			return errors.New("Unknown message Flags")
		} else if header.BodySize > MaxMessageSize {
			if (header.Flags & FLAG_CIPHER) == 0 || header.BodySize - CipherMacSize > MaxMessageSize {
				return errors.New("Message size too large")
			}
		}
	case HelloMsgType, ByeMsgType:
		if header.Flags != 0 || header.BodySize != 0 {
			return fmt.Errorf("Invalid Hello or Bye message")
		}
	default:
		return fmt.Errorf("Unknown message Type(%d)", header.Type)
	}

	return nil
}

func ZZZ(x ...any) {
	_, file, line, _ := runtime.Caller(1)
	fmt.Println("XXX", file, line, x)
}

func (con *_Connection) _msgDeadline() time.Time {
	if con.timeout > 0 {
		return time.Now().Add(con.timeout)
	}
	return time.Time{}
}

const _DEFAULT_CLOSE_TIMEOUT = time.Second * 900
func (con *_Connection) _closeDeadline() time.Time {
	now := time.Now()
	if con.closeTimeout > 0 {
		return now.Add(con.closeTimeout)
	}
	return now.Add(_DEFAULT_CLOSE_TIMEOUT)
}

const _DEFAULT_CONNECT_TIMEOUT = time.Second * 60
func (con *_Connection) _connectDeadline() time.Time {
	now := time.Now()
	if con.connectTimeout > 0 {
		return now.Add(con.connectTimeout)
	}
	return now.Add(_DEFAULT_CONNECT_TIMEOUT)
}

func (con *_Connection) _deadline() time.Time {
	con.mutex.Lock()
	state := con.state
	con.mutex.Unlock()

	if state == con_ACTIVE {
		return con._msgDeadline()
	} else if state < con_ACTIVE {
		return con._connectDeadline()
	}
	return con._closeDeadline()
}

func (con *_Connection) _read_header(buf []byte, mustget bool) error {
again:
	con.c.SetReadDeadline(con._deadline())
	n, err := io.ReadFull(con.c, buf)
	if err != nil {
		if !mustget && n == 0 && errors.Is(err, os.ErrDeadlineExceeded) {
			goto again
		}
	}
	return err
}

func (con *_Connection) recv_msg(must bool) (msg _Message) {
	var err error
	var bodybuf []byte
	var headbuf [MsgHeaderSize]byte
	if err = con._read_header(headbuf[:], must); err != nil {
		if err == io.EOF {
			con.mutex.Lock()
			if con.state == con_BYE {
				err = nil
			}
			con.mutex.Unlock()

			if err == nil {
				con.close_and_reply(true)
				return
			}
		}
		con.set_error(err)
		return
	}

	header := buf2header(headbuf[:])
	if err = checkHeader(header); err != nil {
		dlog.Log("XIC.WARN", "Invalid xic header %v", header)
		goto done
	}

	if header.BodySize > 0 {
		bodybuf = make([]byte, header.BodySize)
		if _, err = io.ReadFull(con.c, bodybuf); err != nil {
			goto done
		}
	}

	if (header.Flags & FLAG_CIPHER) != 0 {
		cipher := con.cipher
		if cipher == nil {
			err = errors.New("FLAG_CIPHER set but no cipher negotiated")
			goto done
		}
		if header.BodySize <= CipherMacSize {
			err = errors.New("Invalid message BodySize")
			goto done
		}
		header.BodySize -= CipherMacSize
		cipher.InputStart(headbuf[:])
		cipher.InputUpdate(bodybuf, bodybuf[:header.BodySize])
		if !cipher.InputFinish(bodybuf[header.BodySize:]) {
			err = errors.New("Failed to decrypt msg body")
			goto done
		}
		bodybuf = bodybuf[:header.BodySize]
	}

	msg, err = DecodeMessage(header, bodybuf)
done:
	if err != nil {
		con.set_error(err)
	}
	return
}

func (con *_Connection) must_read_msg() _Message {
	return con.recv_msg(true)
}

func (con *_Connection) try_read_msg() _Message {
	return con.recv_msg(false)
}

func (con *_Connection) send_msg(msg _OutMessage) error {
	var mac [CipherMacSize]byte
	encrypted := false
	cipher := con.cipher

	buf := msg.Bytes()
	msgType := msg.Type()
	if cipher != nil && (msgType == QuestMsgType || msgType == AnswerMsgType) {
		encrypted = true
		hdr := buf2header(buf[:MsgHeaderSize])
		hdr.Flags = FLAG_CIPHER
		hdr.BodySize += CipherMacSize
		hdr.FillBuffer(buf[:MsgHeaderSize])

		cipher.OutputStart(buf[:MsgHeaderSize])
		cipher.OutputUpdate(buf[MsgHeaderSize:], buf[MsgHeaderSize:])
		cipher.OutputFinish(mac[:])
	}

	con.c.SetWriteDeadline(con._deadline())
	_, err := con.c.Write(buf)
	if err != nil {
		return err
	}

	if encrypted {
		_, err = con.c.Write(mac[:])
	}
	return err
}

func (con *_Connection) sendMessage(msg _OutMessage) {
	// msg.Type() == AnswerMsgType || msg.Type() == QuestMsgType
	con.mutex.Lock()
	con.mq.PushBack(msg)
	con.cond.Broadcast()
	con.mutex.Unlock()
}

func (con *_Connection) byebye_ok() bool {
	return con.state == con_CLOSING && con.numQ.Load() == 0 && len(con.pending) == 0
}

func (con *_Connection) send_loop() {
	var err error
	for {
		con.mutex.Lock()
		for con.mq.Num() == 0 && con.state <= con_CLOSING && !con.byebye_ok() {
			con.cond.Wait()
		}
		state := con.state
		byebye := con.byebye_ok()
		con.mutex.Unlock()

		if state > con_CLOSING {
			goto done
		} else if byebye {
			err = con.send_msg(theByeMessage)
			if err == nil {
				con.set_state(con_BYE)
			}
			goto done
		}

		for {
			con.mutex.Lock()
			msg := con.mq.PopFront()
			con.mutex.Unlock()

			if msg == nil {
				break
			}

			if msg.Type() == AnswerMsgType {
				con.numQ.Add(-1)
			}

			err = con.send_msg(msg)
			if err != nil {
				goto done
			}
		}
	}
done:
	if err != nil {
		con.set_error(err)
	}
}

func (con *_Connection) check_doable(quest *_InQuest) bool {
	var err error
	doit := false
	engine := con.engine

	con.mutex.Lock()
	if con.state == con_ACTIVE {
		if con.maxQ > 0 && con.numQ.Load() >= con.maxQ {
			err = NewException(ConnectionOverloadException, "")
		} else if engine.numQ.Load() >= engine.maxQ {
			err = NewException(ConnectionOverloadException, "")
		} else {
			doit = true
			engine.numQ.Add(1)
			con.numQ.Add(1)
		}
	}
	con.mutex.Unlock()

	if err != nil {
		answer := err2OutAnswer(quest, err)
		con.sendMessage(answer)
	}
	return doit
}

func (con *_Connection) process_loop() {
	con.set_state(con_ACTIVE)
	go con.send_loop()

	var err error
	for {
		msg := con.try_read_msg()
		if msg == nil {
			break
		}

		switch msg.Type() {
		case QuestMsgType:
			quest := msg.(*_InQuest)
			if con.check_doable(quest) {
				adp := con.adapter.Load()
				adapter := adp.(Adapter)
				go con.handleQuest(adapter, quest)
			}

		case AnswerMsgType:
			answer := msg.(*_InAnswer)
			con.handleAnswer(answer)

		case ByeMsgType:
			ZZZ("Bye")
			con.mutex.Lock()
			state := con.state
			if state < con_CLOSED && con.numQ.Load() > 0 {
				err = fmt.Errorf("Unexpected xic bye message")
			}
			con.mutex.Unlock()
			goto done

		default:
			err = fmt.Errorf("Unexpected xic message type(%#x) received", msg.Type())
			goto done
		}
	}
done:
	if err != nil {
		con.set_error(err)
	}
	con.close_and_reply(true)
}

var ByeMessageException = errors.New("ByeMessageException")

