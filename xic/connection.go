package xic

import (
	"bytes"
	"container/list"
	"encoding/binary"
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

const (
	con_INIT	int32 = iota
	con_CONNECT
	con_HANDSHAKE
	con_ACTIVE
	con_CLOSING		// graceful closing is in process
	con_CLOSED
	con_ERROR
)

type _Connection struct {
	c		net.Conn
	state		atomic.Int32
	engine          *_Engine
	incoming        bool
	adapter         atomic.Value // Adapter
	serviceHint     string
	cipher          *_Cipher
	timeout         uint32
	closeTimeout	uint32
	connectTimeout	uint32
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

func newOutgoingConnection(engine *_Engine, serviceHint string, ei *EndpointInfo) *_Connection {
	con := _newConnection(engine, false)
	con.endpoint = ei
	if ei != nil {
		con.timeout = ei.timeout
		con.closeTimeout = ei.closeTimeout
		con.connectTimeout = ei.connectTimeout

		if con.closeTimeout == 0 {
			con.closeTimeout = ei.timeout
		}
		if con.connectTimeout == 0 {
			con.connectTimeout = ei.timeout
		}
	}
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

func (con *_Connection) IsLive() bool { return con.state.Load() <= con_ACTIVE }
func (con *_Connection) Incoming() bool { return con.incoming }
func (con *_Connection) Timeout() uint32 { return con.timeout }
func (con *_Connection) Endpoint() string { return con.endpoint.String() }

func (con *_Connection) Close(force bool) {
	if force {
		con.closeForcefully()
	} else {
		con.closeGracefully()
	}
}

func (con *_Connection) closeGracefully() {
	con.mutex.Lock()
	con.state.Store(con_CLOSING)
	con.cond.Broadcast()
	con.mutex.Unlock()
}

func (con *_Connection) closeForcefully() {
	con.err = NewException(ConnectionClosedException, "")
	con.close_and_reply(false)
}

func (con *_Connection) close_and_reply(retryable bool) {
	con.c.Close()

	con.mutex.Lock()
	pending := con.pending
	con.pending = nil
	err := con.err
	con.state.Store(con_CLOSED)
	con.cond.Broadcast()
	con.mutex.Unlock()

	if (retryable) {
		// TODO
	} else {
		for _, res := range pending {
			res.err = err
			res.broadcast()
		}
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
	if con.state.Load() > con_ACTIVE {
		res.err = errors.New("Connection closing or closed")
		return
	}

	if res != nil && res.txid != 0 {
		con.mutex.Lock()
		txid := con._generate_txid()
		res.txid = txid
		con.pending[txid] = res
		q.SetTxid(txid)
		con.mutex.Unlock()
	}
	con.sendMessage(q)
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
	con.send_msg(msg)
	// TODO
	return con.err == nil
}

func (con *_Connection) check_expect(cmd string, args any) bool {
	msg := con.read_msg()
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
	ck_FORBIDDEN    = "FORBIDDEN"
	ck_SRP6a1       = "SRP6a1"
	ck_SRP6a2       = "SRP6a2"
	ck_SRP6a3       = "SRP6a3"
	ck_SRP6a4       = "SRP6a4"
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
		s4.Cipher = "AES128-EAX" // TEST
		s4.Mode = 1
	}

	if con.err == nil {
		con.send_msg(theHelloMessage)
	}
}

func (con *_Connection) client_handshake() {
	msg := con.read_msg()

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
		srp6cl.SetParameter(int(g.Int64()), s2.N, len(s2.N)*8)
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

		msg = con.read_msg()
	}

	if msg != nil && msg.Type() != HelloMsgType {
		con.err = errors.New("Unexpected message received, expect Hello message")
		return
	}
}

func (con *_Connection) server_run() {
	con.state.Store(con_HANDSHAKE)
	con.server_handshake()

	if con.err != nil {
		var forbidden _ForbiddenArgs
		forbidden.Reason = con.err.Error()
		con.check_send(ck_FORBIDDEN, &forbidden)
		con.close_and_reply(true)
		return
	}

	con.state.Store(con_ACTIVE)
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
	var err error

	con.state.Store(con_CONNECT)
	ei := con.endpoint
	con.c, err = net.DialTimeout(ei.proto, ei.Address(), timeout2duration(con.connectTimeout))
	if err != nil {
		con.state.Store(con_ERROR)
		con.close_and_reply(true)
		return
	}

	con.state.Store(con_HANDSHAKE)
	con.client_handshake()

	if con.err != nil {
		con.state.Store(con_ERROR)
		con.close_and_reply(true)
		return
	}

	con.state.Store(con_ACTIVE)
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
	} else {
		res.err = answer.DecodeArgs(res.out)
	}

	res.broadcast()
}

func checkHeader(header _MessageHeader) error {
	if header.Magic != 'X' || header.Version != '!' {
		return fmt.Errorf("Unknown message Magic(%d) and Version(%d)", header.Magic, header.Version)
	}

	switch MsgType(header.Type) {
	case QuestMsgType, AnswerMsgType, CheckMsgType:
		if (header.Flags &^ 0x01) != 0 {
			return errors.New("Unknown message Flags")
		} else if int(header.BodySize) > MaxMessageSize {
			return errors.New("Message size too large")
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

func (con *_Connection) _read_header(header *_MessageHeader) error {
again:
	state := con.state.Load()
	timeout := con.timeout
	if state > con_ACTIVE && con.closeTimeout > 0 {
		timeout = con.closeTimeout
	}
	con.c.SetReadDeadline(timeout2deadline(timeout))
	var buf [HeaderSize]byte
	n, err := io.ReadFull(con.c, buf[:])
	if err != nil {
		if n == 0 && errors.Is(err, os.ErrDeadlineExceeded) {
			goto again
		}
		return err
	}

	return binary.Read(bytes.NewReader(buf[:]), binary.BigEndian, header)
}

func (con *_Connection) read_msg() _Message {
	var header _MessageHeader
	if err := con._read_header(&header); err != nil {
		con.err = err
		return nil
	}

	if err := checkHeader(header); err != nil {
		con.err = err
		dlog.Log("XIC.WARN", "Invalid xic header %v", header)
		return nil
	}

	var buf []byte
	if header.BodySize > 0 {
		buf = make([]byte, header.BodySize)
		_, err := io.ReadFull(con.c, buf)
		if err != nil {
			con.err = err
			return nil
		}
	}

	msg, err := DecodeMessage(header, buf)
	con.err = err
	return msg
}

func (con *_Connection) send_msg(msg _OutMessage) error {
	buf := msg.Bytes()
	if con.timeout != 0 {
		con.c.SetWriteDeadline(timeout2deadline(con.timeout))
	}
	_, err := con.c.Write(buf)
	return err
}

func (con *_Connection) sendMessage(msg _OutMessage) {
	// msg.Type() == AnswerMsgType || msg.Type() == QuestMsgType
	con.mutex.Lock()
	con.mq.PushBack(msg)
	con.cond.Broadcast()
	con.mutex.Unlock()
}

func (con *_Connection) send_loop() {
	for {
		for {
			con.mutex.Lock()
			msg := con.mq.PopFront()
			con.mutex.Unlock()

			if msg == nil {
				break
			}

			doit := false
			switch msg.Type() {
			case QuestMsgType:
				if con.state.Load() == con_ACTIVE {
					doit = true
				}
			case AnswerMsgType:
				doit = true
				con.numQ.Add(-1)
			default:
				panic("Can't reach here")
			}

			if doit {
				if err := con.send_msg(msg); err != nil {
					con.err = err
					break
				}
			}
		}

		con.mutex.Lock()
		state := con.state.Load()
		silent := (con.numQ.Load() == 0 && len(con.pending) == 0)
		con.mutex.Unlock()

		if (state > con_ACTIVE) {
			if (state == con_CLOSING && silent) {
				con.send_msg(theByeMessage)
			}
			return
		}

		con.mutex.Lock()
		for con.mq.Num() == 0 && con.state.Load() == con_ACTIVE {
			con.cond.Wait()
		}
		con.mutex.Unlock()
	}
}

func (con *_Connection) check_overload(quest *_InQuest) bool {
	if con.maxQ > 0 && con.numQ.Load() >= con.maxQ {
		answer := err2OutAnswer(quest, NewException(ConnectionOverloadException, ""))
		con.sendMessage(answer)
		return true
	}

	engine := con.engine
	if engine.numQ.Load() >= engine.maxQ {
		answer := err2OutAnswer(quest, NewException(ConnectionOverloadException, ""))
		con.sendMessage(answer)
		return true
	}

	engine.numQ.Add(1)
	con.numQ.Add(1)
	return false
}

func (con *_Connection) process_loop() {
	go con.send_loop()

	for {
		msg := con.read_msg()
		if msg == nil {
			break
		}

		switch msg.Type() {
		case QuestMsgType:
			state := con.state.Load()
			if state == con_ACTIVE {
				quest := msg.(*_InQuest)
				if con.check_overload(quest) {
					break
				}

				adp := con.adapter.Load()
				adapter := adp.(Adapter)
				go con.handleQuest(adapter, quest)
			} else if state == con_CLOSING {
				// do nothing
				// the Quest is discarded
			} else {
				con.err = errors.New("Unexpected Quest message received")
			}
			// TODO

		case AnswerMsgType:
			answer := msg.(*_InAnswer)
			con.handleAnswer(answer)

		case ByeMsgType:
			ZZZ("Bye")
			// TODO: some checks?
			goto done

		case CheckMsgType:
			con.err = errors.New("Unexpected Check message received")
		case HelloMsgType:
			con.err = errors.New("Unexpected Hello message received")
		}

		if con.err != nil {
			break
		}
	}
done:
	con.err = ByeMessageException
	con.close_and_reply(true)
}

var ByeMessageException = errors.New("ByeMessageException")

