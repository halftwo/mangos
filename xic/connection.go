package xic

import (
	"fmt"
	"net"
	"strings"
	"errors"
	"sync"
	"reflect"
	"runtime"
	"sync/atomic"
	"encoding/binary"

	"mangos/dlog"
)

type stdCurrent struct {
	inQuest
	con *stdConnection
	args Arguments
}

func newCurrent(con *stdConnection, q *inQuest) *stdCurrent {
	return &stdCurrent{inQuest:*q, con:con}
}

func (cur *stdCurrent) Txid() int64 {
	return cur.txid
}

func (cur *stdCurrent) Service() string {
	return cur.service
}

func (cur *stdCurrent) Method() string {
	return cur.method
}

func (cur *stdCurrent) Ctx() Context {
	return cur.ctx
}

func (cur *stdCurrent) Args() Arguments {
	if cur.args == nil {
		cur.args = NewArguments()
		cur.DecodeArgs(cur.args)
	}
	return cur.args
}

func (cur *stdCurrent) Con() Connection {
	return cur.con
}


type conState int32
const (
	con_INIT conState = iota
	con_WAITING_HELLO	// client waiting for server hello message
	con_ACTIVE
	con_CLOSE		// Close is called
	con_CLOSING		// graceful closing is in process
	con_CLOSED
	con_ERROR
)

type stdConnection struct {
	c net.Conn
	state conState
	engine *stdEngine
	adapter atomic.Value	// Adapter
	serviceHint string
	incoming bool
	timeout int
	concurrent int
	endpoint string
	lastTxid int64
	pending map[int64]*Invoking
	mutex sync.Mutex
}

func newOutgoingConnection(engine *stdEngine, serviceHint string, endpoint string) *stdConnection {
	ei, err := parseEndpoint(endpoint)
	if err != nil {
		return nil
	}

	c, err := net.Dial(ei.proto, ei.Address())
	if err != nil {
		return nil
	}
	con := &stdConnection{engine:engine, c:c, incoming:false, serviceHint:serviceHint, pending:make(map[int64]*Invoking)}
	return con
}

func newIncomingConnection(adapter *stdAdapter, c net.Conn) *stdConnection {
	engine := adapter.engine
	con := &stdConnection{engine:engine, c:c, incoming:true}
	con.adapter.Store(adapter)
	engine.incomingConnection(con)
	return con
}

func (con *stdConnection) String() string {
	laddr := con.c.LocalAddr()
	return fmt.Sprintf("%s/%s/%s", laddr.Network(), laddr.String(), con.c.RemoteAddr().String())
}

func (con *stdConnection) IsLive() bool {
	state := conState(atomic.LoadInt32((*int32)(&con.state)))
	return state < con_CLOSE
}

func (con *stdConnection) Incoming() bool {
	return con.incoming
}

func (con *stdConnection) Timeout() int {
	return con.timeout
}

func (con *stdConnection) Endpoint() string {
	return con.endpoint
}

func (con *stdConnection) Close(force bool) {
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

func (con *stdConnection) CreateProxy(service string) (Proxy, error) {
	if strings.IndexByte(service, '@') >= 0 {
		return nil, errors.New("Service name can't contain '@'")
	}
	if con.pending == nil {
		con.pending = make(map[int64]*Invoking)
	}
	prx, err := con.engine.makeFixedProxy(service, con)
	return prx, err
}


func (con *stdConnection) Adapter() Adapter {
	a := con.adapter.Load()
	if a != nil {
		return a.(Adapter)
	}
	return nil
}

func (con *stdConnection) SetAdapter(adapter Adapter) {
	con.mutex.Lock()
	con.adapter.Store(adapter)
	con.mutex.Unlock()
}

func (con *stdConnection) sendMessage(msg xicOutMessage) error {
	buf := msg.Bytes()
	_, err := con.c.Write(buf)
	return err
}

func (con *stdConnection) generateTxid() int64 {
	con.lastTxid++
	if con.lastTxid < 0 {
		con.lastTxid = 1
	}
	txid := con.lastTxid
	return txid
}

func (con *stdConnection) invoke(prx *stdProxy, q *outQuest, vk *Invoking) error {
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

func (con *stdConnection) shut() {
	con.c.Close()
	// TODO
}

func (con *stdConnection) grace() {
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

func (con *stdConnection) handleQuest(adapter Adapter, quest *inQuest) {
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
	var answer *outAnswer
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

func (con *stdConnection) handleAnswer(answer *inAnswer) {
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
			ivk.Err = &stdException{name:args.GetString("exname"),
					code:int(args.GetInt("code")),
					tag:args.GetString("tag"),
					msg:args.GetString("message")}
		}
	} else {
		ivk.Err = answer.DecodeArgs(ivk.Out)
	}

	ivk.Done <- ivk
}

func checkHeader(header xicMessageHeader) error {
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

func (con *stdConnection) start() {
	go con.run()
}

func (con *stdConnection) run() {
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
		var header xicMessageHeader
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
			state := conState(atomic.LoadInt32((*int32)(&con.state)))
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
			quest := msg.(*inQuest)
			if con.concurrent > 1 {
				go con.handleQuest(adapter, quest)
			} else {
				con.handleQuest(adapter, quest)
			}

			// TODO
		case 'A':
			answer := msg.(*inAnswer)
			con.handleAnswer(answer)
		case 'C':
			state := conState(atomic.LoadInt32((*int32)(&con.state)))
			if state != con_WAITING_HELLO {
				wrong = errors.New("Unexpected Check message received")
			}
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

