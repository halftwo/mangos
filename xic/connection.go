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

const DEFAULT_CONNECTION_MAXQ = 1000

type _Connection struct {
	c		net.Conn
	state		_ConState
	incoming        bool
	closed		bool
	id		string
	engine          *_Engine
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
	_str		string
}

type OutMsgQueue struct {
	lst *list.List
	num int
}

func (q *OutMsgQueue) Clear() {
	q.lst.Init()
	q.num = 0
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
	con := &_Connection{
		id: GenerateRandomBase57Id(23),
		engine: engine,
		incoming: incoming,
		maxQ: DEFAULT_CONNECTION_MAXQ,
	}
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
	con.adapter.Store(engine.slackAdapter)

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

func netc2str(c net.Conn) string {
	laddr := c.LocalAddr()
	raddr := c.RemoteAddr()
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

func (con *_Connection) remoteAddr() string {
	return con.c.RemoteAddr().String()
}

func (con *_Connection) String() string {
	con.mutex.Lock()
	if con._str == "" {
		con._str = netc2str(con.c)
	}
	str := con._str
	con.mutex.Unlock()
	return str
}

func (con *_Connection) IsLive() bool { return con.state <= con_ACTIVE }
func (con *_Connection) Id() string { return con.id }
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
	err := newException(ConnectionClosedException)
	con.set_error(err)	// TODO
	con.close_and_reply(false)
}

func (con *_Connection) close_and_reply(retryable bool) {
	con.mutex.Lock()
	if con.closed {
		con.mutex.Unlock()
		return
	}
	con.closed = true
	con.mq.Clear()

	pending := con.pending
	con.pending = nil

	err := con.err
	if err == nil && con.state < con_CLOSED {
		con.state = con_CLOSED
		con.cond.Broadcast()
	}
	con.mutex.Unlock()

	if con.c != nil {
		con.c.Close()
	}

	if len(pending) > 0 {
		if err == nil {
			// TODO: retryable
			err = newException(QuestNotServedException)
		}

		for _, res := range pending {
			// TODO
			res.err = err
			res.broadcast()
		}
	}
}

func (con *_Connection) CreateFixedProxy(service string) (Proxy, error) {
	if strings.IndexByte(service, '@') >= 0 {
		return nil, newEx(InvalidParameterException, "Service name can't contain '@'")
	}
	con.mutex.Lock()
	if con.pending == nil {
		con.pending = make(map[int64]*_Result)
	}
	con.mutex.Unlock()

	prx := newProxyWithConnection(con.engine, service, con)
	return prx, nil
}

func (con *_Connection) Adapter() Adapter {
	a := con.adapter.Load()
	if a != nil {
		return a.(Adapter)
	}
	return nil
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
		res.err = newException(ConnectionClosedException)
	}
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
		return newExf(ProtocolException, "Unexpected cmd of CheckMessage %s", check.cmd)
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
			err = newEx(AuthFailedException, "No such identity")
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
			err = newEx(AuthFailedException, "srp6a M1 not equal")
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
			err = newEx(AuthFailedException, "Unknown auth method")
			goto done
		}

		if con.engine.secretBox == nil {
			err = newEx(AuthFailedException, "No SecretBox supplied")
			goto done
		}

		id, pass := con.engine.secretBox.FindEndpoint(con.serviceHint, con.endpoint)
		if id == "" || pass == "" {
			err = newEx(AuthFailedException, "No matched secret found")
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
			err = newEx(AuthFailedException, "srp6a M2 not equal")
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
		err = newEx(ProtocolException, "Unexpected message received, expect Hello message")
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
		outErr.Set("exname", ex.Name())
		outErr.Set("code", ex.Code())
		outErr.Set("message", ex.Message())
		outErr.Set("locus", ex.Locus())
	} else {
		outErr.Set("message", err.Error())
	}
	answer := newOutAnswerExceptional(quest.txid, outErr)
	return answer
}

func makePointerValue(t reflect.Type) reflect.Value {
	var p reflect.Value
	if t.Kind() == reflect.Pointer {
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

func (con *_Connection) handleQuest(adapter Adapter, quest *_InQuest) {
	var err error
	var answer *_OutAnswer
	var si *ServantInfo

	cli_oneway := quest.txid == 0
	srv_oneway := false

	if quest.service == "\x00" {
		si = con.engine.keeper
	} else {
		if adapter == nil {
			err = newException(AdapterAbsentException)
			goto wrong
		} else {
			si = adapter.FindServant(quest.service)
			if si == nil {
				si := adapter.DefaultServant()
				if si == nil {
					err = newExf(ServiceNotFoundException, "service=%#v", quest.service)
					goto wrong
				}
			}
		}
	}

	if mi, ok := si.Methods[quest.method]; ok {
		srv_oneway = mi.Oneway
		in := makePointerValue(mi.InType)
		err = quest.DecodeArgs(in.Interface())
		if err != nil {
			goto wrong
		}

		if mi.InType.Kind() != reflect.Pointer {
			in = in.Elem()
		}

		cur := newCurrent(con, quest)
		if srv_oneway {
			mi.Method.Func.Call([]reflect.Value{reflect.ValueOf(si.Servant), reflect.ValueOf(cur), in})
		} else {
			out := makePointerValue(mi.OutType)
			if mi.OutType.Kind() != reflect.Pointer {
				out = out.Elem()
			}
			rts := mi.Method.Func.Call([]reflect.Value{reflect.ValueOf(si.Servant), reflect.ValueOf(cur), in, out})
			if !rts[0].IsNil() {
				err = rts[0].Interface().(error)
				goto wrong
			}
			if cli_oneway {
				goto wrong
			}
			answer = newOutAnswerNormal(quest.txid, out.Interface())
		}
	} else {
		outArgs := Arguments{}
		if len(quest.method) > 0 && quest.method[0] == 0x00 {
			if quest.method == "\x00methods" {
				outArgs.Set("methods", getServantMethods(si))
			} else {
				err = newExf(MethodNotFoundException, "method=%#v", quest.method)
			}
		} else {
			inArgs := Arguments{}
			err = quest.DecodeArgs(inArgs)
			if err != nil {
				goto wrong
			}

			cur := newCurrent(con, quest)
			err = si.Servant.Xic(cur, inArgs, outArgs)
		}

		if err != nil {
			goto wrong
		}
		answer = newOutAnswerNormal(quest.txid, outArgs)
	}

wrong:
	if err != nil {
		dlog.Log("XIC.EXCEPT", "%s::%s return error --- %s", quest.service, quest.method, err.Error())
	}

	if cli_oneway {
		con.numQ.Add(-1)
		if !srv_oneway {
			dlog.Log("XIC.WARN", "%s::%s --- Twoway method invoked as oneway, con=%v", quest.service, quest.method, con)
		}
	} else {
		if srv_oneway {
			dlog.Log("XIC.WARN", "%s::%s --- Oneway method invoked as twoway, con=%v", quest.service, quest.method, con)
			if err == nil {
				answer = newOutAnswerNormal(quest.txid, struct{}{})
			}
		}

		if err != nil {
			answer = err2OutAnswer(quest, err)
		} else if answer == nil {
			panic("Can't reach here")
		}

		con.sendMessage(answer)
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
			res.err = newRemoteExCode(ExNameType(args.GetString("exname")), int(args.GetInt("code")), args.GetString("message"), con)
		}
	} else if res.out != nil {
		res.err = answer.DecodeArgs(res.out)
	}

	res.broadcast()
}

func checkHeader(header _MessageHeader) error {
	if header.Magic != 'X' || header.Version != '!' {
		return newExf(ProtocolException, "Unknown message Magic(%d) and Version(%d)", header.Magic, header.Version)
	}

	switch header.Type {
	case QuestMsgType, AnswerMsgType, CheckMsgType:
		if (header.Flags &^ FLAG_MASK) != 0 {
			return newEx(ProtocolException, "Unknown message Flags")
		} else if int(header.BodySize) > MaxMessageSize {
			if (header.Flags & FLAG_CIPHER) == 0 || int(header.BodySize) - CipherMacSize > MaxMessageSize {
				return newExf(ProtocolException, "Message size too large, should less than %d", MaxMessageSize)
			}
		}
	case HelloMsgType, ByeMsgType:
		if header.Flags != 0 || header.BodySize != 0 {
			return newEx(ProtocolException, "Invalid Hello or Bye message")
		}
	default:
		return newExf(ProtocolException, "Unknown message Type(%d)", header.Type)
	}

	return nil
}

func ZZZ(x ...any) {
	_, file, line, _ := runtime.Caller(1)
	fmt.Println("ZZZ", file, line, x)
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
			err = newEx(ProtocolException, "FLAG_CIPHER set but no cipher negotiated")
			goto done
		}
		if header.BodySize <= CipherMacSize {
			err = newEx(ProtocolException, "Invalid message BodySize")
			goto done
		}
		header.BodySize -= CipherMacSize
		cipher.InputStart(headbuf[:])
		cipher.InputUpdate(bodybuf, bodybuf[:header.BodySize])
		if !cipher.InputFinish(bodybuf[header.BodySize:]) {
			err = newEx(ProtocolException, "Failed to decrypt message")
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
	if con.state < con_BYE {
		con.mq.PushBack(msg)
		con.cond.Broadcast()
	}
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

			err = con.send_msg(msg)
			if err != nil {
				goto done
			}
			if msg.Type() == AnswerMsgType {
				con.numQ.Add(-1)
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
			err = newException(ConnectionOverloadException)
		} else if engine.numQ.Load() >= engine.maxQ {
			err = newException(ConnectionOverloadException)
		} else {
			doit = true
		}
		engine.numQ.Add(1)
		con.numQ.Add(1)
	}
	con.mutex.Unlock()

	if doit {
		if len(quest.service) == 0 {
			err = newEx(ServiceNotFoundException, "service=\"\"")
		} else if len(quest.method) == 0 {
			err = newEx(MethodNotFoundException, "method=\"\"")
		}
	}

	if err != nil {
		doit = false
		dlog.Log("XIC.WARN", "%s", err.Error())

		if quest.txid == 0 {
			con.numQ.Add(-1)
		} else {
			answer := err2OutAnswer(quest, err)
			con.sendMessage(answer)
		}
		engine.numQ.Add(-1)
		return false
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
			con.mutex.Lock()
			state := con.state
			if state < con_CLOSED && con.numQ.Load() > 0 {
				err = newEx(ProtocolException, "Unexpected xic bye message")
			}
			con.mutex.Unlock()
			goto done

		default:
			err = newExf(ProtocolException, "Unexpected xic message type(%#x) received", msg.Type())
			goto done
		}
	}
done:
	if err != nil {
		con.set_error(err)
	}
	con.close_and_reply(true)
}


