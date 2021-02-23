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
	state _ConState
	engine *_Engine
	adapter atomic.Value	// Adapter
	serviceHint string
	shadowBox *ShadowBox
	_srp6a interface{}
	cipher *_Cipher
	incoming bool
	timeout int
	concurrent int
	endpoint string
	lastTxid int64
	pending map[int64]*Invoking
	mutex sync.Mutex
}

func newOutgoingConnection(engine *_Engine, serviceHint string, endpoint string) *_Connection {
	ei, err := parseEndpoint(endpoint)
	if err != nil {
		return nil
	}

	c, err := net.Dial(ei.proto, ei.Address())
	if err != nil {
		return nil
	}
	con := &_Connection{engine:engine, c:c, incoming:false, serviceHint:serviceHint, pending:make(map[int64]*Invoking)}
	return con
}

func newIncomingConnection(adapter *_Adapter, c net.Conn) *_Connection {
	engine := adapter.engine
	con := &_Connection{engine:engine, c:c, incoming:true}
	con.adapter.Store(adapter)
	engine.incomingConnection(con)
	return con
}

func (con *_Connection) String() string {
	laddr := con.c.LocalAddr()
	raddr := con.c.RemoteAddr()
	{
		l, ok := laddr.(*net.TCPAddr)
		if ok {
			r := raddr.(*net.TCPAddr)
			return fmt.Sprintf("tcp/%s+%d/%s+%d", l.IP.String(), l.Port, r.IP.String(), r.Port)
		}
	}
	{
		l, ok := laddr.(*net.UDPAddr)
		if ok {
			r := raddr.(*net.UDPAddr)
			return fmt.Sprintf("udp/%s+%d/%s+%d", l.IP.String(), l.Port, r.IP.String(), r.Port)
		}
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
	return con.endpoint
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

func (con *_Connection) sendMessage(msg _OutMessage) error {
	buf := msg.Bytes()
	_, err := con.c.Write(buf)
	return err
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
	if vk.Txid != 0 {
		con.mutex.Lock()
		txid := con.generateTxid()
		vk.Txid = txid
		con.pending[txid] = vk
		q.SetTxid(txid)
		con.mutex.Unlock()
	}
	con.sendMessage(q)
	return nil
}

func (con *_Connection) shut() {
	con.c.Close()
	// TODO
}

func (con *_Connection) grace() {
	// TODO
	con.sendMessage(theByeMessage)
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

func (con *_Connection) handleCheck(check *_InCheck) {
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

	var msg _OutMessage
	if con.incoming {	// server
		switch check.cmd {
		case "SRP6a1":
			var s1 _S1Args;
			check.DecodeArgs(&s1)

			v := con.shadowBox.GetVerifier(s1.I)
			if v == nil {
				// TODO
			}

			var s2 _S2Args;
			server, err := con.shadowBox.CreateSrp6aServer(v.ParamId, v.HashId)
			if err != nil {
				// TODO
			}
			con._srp6a = server
			server.SetV(v.Verifier)
			s2.Hash = server.HashName()
			s2.N = server.N()
			s2.Gen = server.G()
			s2.Salt = v.Salt
			s2.B = server.GenerateB()
			msg = newOutCheck("SRP6a2", &s2)

		case "SRP6a3":
			var s3 _S3Args;
			check.DecodeArgs(&s3)

			server := con._srp6a.(*srp6a.Srp6aServer)
			server.SetA(s3.A)
			M1 := server.ComputeM1()
			if !bytes.Equal(M1, s3.M1) {
				// TODO
			}

			var s4 _S4Args;
			s4.M2 = server.ComputeM2()
			s4.Cipher = "AES128-EAX"	// TEST
			s4.Mode = 1			// TEST
			msg = newOutCheck("SRP6a4", &s4)
		}
	} else {	// client
		switch check.cmd {
		case "FORBIDDEN":
			var args _ForbiddenArgs
			check.DecodeArgs(&args)

		case "AUTHENTICATE":
			var args _AuthArgs
			check.DecodeArgs(&args)
			if args.Method != "SRP6a" {
				// TODO
			}

			secretBox := (*SecretBox)(nil) // TEST
			id, pass := secretBox.Find(con.serviceHint, con.endpoint)
			if id == "" || pass == "" {
				// TODO
			}

			client := srp6a.NewClientEmpty()
			con._srp6a = client
			client.SetIdentity(id, pass)
			var s1 _S1Args;
			s1.I = id
			msg = newOutCheck("SRP6a1", &s1)

		case "SRP6a2":
			var s2 _S2Args;
			check.DecodeArgs(&s2)

			Gen := new(big.Int).SetBytes(s2.Gen)
			client := con._srp6a.(*srp6a.Srp6aClient)
			client.SetHash(s2.Hash)
			client.SetParameter(int(Gen.Int64()), s2.N, len(s2.N) * 8)
			client.SetSalt(s2.Salt)
			client.SetB(s2.B)

			var s3 _S3Args;
			s3.A = client.GenerateA()
			s3.M1 = client.ComputeM1()
			msg = newOutCheck("SRP6a1", &s3)

		case "SRP6a4":
			var s4 _S4Args;
			check.DecodeArgs(&s4)

			client := con._srp6a.(*srp6a.Srp6aClient)
			M2 := client.ComputeM2()
			if !bytes.Equal(M2, s4.M2) {
				//TODO
			}
			// TODO
		}
	}

	if msg != nil {
		// TODO
	}
}

func (con *_Connection) handleQuest(adapter Adapter, quest *_InQuest) {
	var err error
	txid := quest.txid
	si := adapter.FindServant(quest.service)
	if si == nil {
		si := adapter.DefaultServant()
		if si == nil {
			err = NewExceptionf(ServiceNotFoundException, "%s", quest.service)
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
	if txid != 0 {
		if oneway {
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

		answer.SetTxid(txid)
		con.sendMessage(answer)
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
		if header.Flags != 0 && header.Flags != 0x01 {
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

func (con *_Connection) start() {
	go con.run()
}

func (con *_Connection) run() {
	// TODO
	var wrong error
	if con.incoming {
		wrong = con.sendMessage(theHelloMessage)
		if wrong != nil {
			return
		}

		// TODO: check

		con.state = con_ACTIVE
	} else {
		con.state = con_WAITING_HELLO
	}

loop:
	for {
		var header _MessageHeader
		if wrong = binary.Read(con.c, binary.BigEndian, &header); wrong != nil {
			break
		}

		if wrong = checkHeader(header); wrong != nil {
			break
		}

		buf := make([]byte, header.BodySize)
		n, err := con.c.Read(buf)
		if err != nil {
			wrong = err
			break
		} else if n != len(buf) {
			wrong = fmt.Errorf("Received less data (%d) than specified in the header (%d)", n, len(buf))
			break
		}

		msg, err := DecodeMessage(header, buf)
		if err != nil {
			wrong = err
			break
		}

		switch msg.Type() {
		case 'Q':
			state := _ConState(atomic.LoadInt32((*int32)(&con.state)))
			if state < con_ACTIVE {
				wrong = errors.New("Unexpected Quest message received")
				break loop
			} else if state > con_ACTIVE {
				// ignored
				continue loop
			}

			adp := con.adapter.Load()
			if adp == nil {
				wrong = errors.New("No Adapter set for the connection")
				break loop
			}

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

		case 'C':
			state := _ConState(atomic.LoadInt32((*int32)(&con.state)))
			if state != con_WAITING_HELLO {
				wrong = errors.New("Unexpected Check message received")
			}
			check := msg.(*_InCheck)
			con.handleCheck(check)
			// TODO

		case 'H':
			if !atomic.CompareAndSwapInt32((*int32)(&con.state), int32(con_WAITING_HELLO), int32(con_ACTIVE)) {
				wrong = errors.New("Unexpected Hello message received")
				break loop
			}
		case 'B':
			if !atomic.CompareAndSwapInt32((*int32)(&con.state), int32(con_ACTIVE), int32(con_CLOSED)) {
				wrong = errors.New("Unexpected Bye message received")
			}
			break loop
		}
	}

	if wrong != nil {
		fmt.Println("ERROR:", wrong)
		con.shut()
	} else {
		con.grace()
	}
}

