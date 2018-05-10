package xic

import (
	"fmt"
	"math"
	"bytes"

	"mangos/vbs"
)

type MsgType byte

const (
	CheckMessage MsgType	= 'C'
	HelloMessage		= 'H'
	ByeMessage		= 'B'
	QuestMessage		= 'Q'
	AnswerMessage		= 'A'
)

const headerSize = 8

type xicMessageHeader struct {
	Magic byte	// 'X'
	Version byte	// '!'
	Type byte	// 'Q', 'A', 'H', 'B', 'C'
	Flags byte	// 0x00 or 0x01
	BodySize int32	// in big endian byte order
}

type xicMessage interface {
	Type() MsgType
}

type xicOutMessage interface {
	xicMessage
	Bytes() []byte
}

type xicInMessage interface {
	xicMessage
	Args() Arguments
}

var commonHeaderBytes = [8]byte{'X','!'}

func fillHeader(packet []byte, t MsgType) {
	size := len(packet) - 8
	if size < 0 {
		panic("Can't reach here")
	} else if size > MaxMessageSize {
		// TODO
	}
	packet[0] = 'X'
	packet[1] = '!'
	packet[2] = byte(t)
	packet[3] = 0
	packet[4] = byte(size >> 24)
	packet[5] = byte(size >> 16)
	packet[6] = byte(size >> 8)
	packet[7] = byte(size)
}

var theHelloMessage = stdHelloMessage{}
var theByeMessage = stdByeMessage{}


type stdHelloMessage struct {
}

func (m stdHelloMessage) Type() MsgType {
	return 'H'
}

var helloMessageBytes = [8]byte{'X','!','H'}

func (m stdHelloMessage) Bytes() []byte {
	return helloMessageBytes[:]
}


type stdByeMessage struct {
}

func (m stdByeMessage) Type() MsgType {
	return 'B'
}

var byeMessageBytes = [8]byte{'X','!','B'}

func (m stdByeMessage) Bytes() []byte {
	return byeMessageBytes[:]
}

type outCheck struct {
	buf []byte
}

func newOutCheck(cmd string, args interface{}) *outCheck {
	b := &bytes.Buffer{}
	enc := vbs.NewEncoder(b)
	b.Write(commonHeaderBytes[:])
	enc.Encode(cmd)
	err := enc.Encode(args)
	if err != nil {
		panic("vbs.Encoder error")
	}
	c := &outCheck{buf:b.Bytes()}
	fillHeader(c.buf, 'C')
	return c
}

func (c *outCheck) Type() MsgType {
	return 'C'
}

func (c *outCheck) Bytes() []byte {
	return c.buf
}


type outQuest struct {
	txid int64
	reserved int
	start int
	buf []byte
}

func newOutQuest(txid int64, service, method string, ctx Context, args interface{}) *outQuest {
	q := &outQuest{txid:txid, start:-1}
	b := &bytes.Buffer{}
	enc := vbs.NewEncoder(b)
	b.Write(commonHeaderBytes[:])
	enc.Encode(math.MaxInt64)
	q.reserved = b.Len()

	enc.Encode(service)
	enc.Encode(method)
	enc.Encode(ctx)
	err := enc.Encode(args)
	if err != nil {
		panic("vbs.Encoder error")
	}
	q.buf = b.Bytes()
	return q
}

func (q *outQuest) Type() MsgType {
	return 'Q'
}

func (q *outQuest) Bytes() []byte {
	if q.start < 0 {
		if q.txid < 0 {
			panic("txid not set yet")
		}
		n := 0
		bp := vbs.BytesPacker{}
		bp.PackInteger(&n, q.txid)
		q.start = q.reserved - n
		copy(q.buf[q.start:], bp[:n])
		q.start -= headerSize
		fillHeader(q.buf[q.start:], 'Q')
	}
	return q.buf[q.start:]
}

func (q *outQuest) SetTxid(txid int64) {
	if q.txid != 0 {
		q.txid = txid
		q.start = -1
	}
}

type outAnswer struct {
	txid int64
	reserved int
	start int
	buf []byte
}

func newOutAnswer(status int, args interface{}) *outAnswer {
	a := &outAnswer{txid:-1, start:-1}
	b := &bytes.Buffer{}
	enc := vbs.NewEncoder(b)
	b.Write(commonHeaderBytes[:])
	enc.Encode(math.MaxInt64)
	a.reserved = b.Len()

	enc.Encode(status)
	err := enc.Encode(args)
	if err != nil {
		panic("vbs.Encoder error")
	}
	a.buf = b.Bytes()
	return a
}

func newOutAnswerNormal(args interface{}) *outAnswer {
	return newOutAnswer(0, args)
}

func newOutAnswerExceptional(args interface{}) *outAnswer {
	return newOutAnswer(-1, args)
}

func (a *outAnswer) Type() MsgType {
	return 'A'
}

func (a *outAnswer) Bytes() []byte {
	if a.start < 0 {
		if a.txid < 0 {
			panic("txid not set yet")
		}
		n := 0
		bp := vbs.BytesPacker{}
		bp.PackInteger(&n, a.txid)
		a.start = a.reserved - n
		copy(a.buf[a.start:], bp[:n])
		a.start -= headerSize
		fillHeader(a.buf[a.start:], 'A')
	}
	return a.buf[a.start:]
}

func (a *outAnswer) SetTxid(txid int64) {
	if a.txid != 0 {
		a.txid = txid
		a.start = -1
	}
}

type inMsg struct {
	argsOff int
	buf []byte
}

func (m *inMsg) DecodeArgs(args interface{}) error {
	dec := vbs.NewDecoderBytes(m.buf[m.argsOff:])
	err := dec.Decode(args)
	if err != nil {
		return err
	} else if dec.More() {
		return fmt.Errorf("Surplus bytes left after decoding arguments")
	}
	return nil
}

type inCheck struct {
	cmd string
	inMsg
}

func newInCheck(buf []byte) *inCheck {
	c := &inCheck{}
	dec := vbs.NewDecoderBytes(buf)
	dec.Decode(&c.cmd)
	c.argsOff = dec.Consumed()
	c.buf = buf
	return c
}

func (q *inCheck) Type() MsgType {
	return 'C'
}

type inQuest struct {
	txid int64
	service string
	method string
	ctx Context
	inMsg
}

func newInQuest(buf []byte) *inQuest {
	q := &inQuest{}
	dec := vbs.NewDecoderBytes(buf)
	dec.Decode(&q.txid)
	dec.Decode(&q.service)
	dec.Decode(&q.method)
	dec.Decode(&q.ctx)
	q.argsOff = dec.Consumed()
	q.buf = buf
	return q
}

func (q *inQuest) Type() MsgType {
	return 'Q'
}

type inAnswer struct {
	txid int64
	status int
	inMsg
}

func newInAnswer(buf []byte) *inAnswer {
	a := &inAnswer{}
	dec := vbs.NewDecoderBytes(buf)
	dec.Decode(&a.txid)
	dec.Decode(&a.status)
	a.argsOff = dec.Consumed()
	a.buf = buf
	return a
}

func (a *inAnswer) Type() MsgType {
	return 'A'
}

func DecodeMessage(header xicMessageHeader, buf []byte) (xicMessage, error) {
	if header.Magic != 'X' || header.Version != '!' {
		return nil, fmt.Errorf("Unknown message Magic(%d) and Version(%d)", header.Magic, header.Version)
	}
	if header.Type == 'H' || header.Type == 'B' {
		if header.Flags != 0 || header.BodySize != 0 {
			return nil, fmt.Errorf("Invalid Hello or Bye message")
		}
	}

	var msg xicMessage
	switch header.Type {
	case 'H':
		msg = theHelloMessage
	case 'B':
		msg = theByeMessage
	case 'C':
		msg = newInCheck(buf)
	case 'Q':
		msg = newInQuest(buf)
	case 'A':
		msg = newInAnswer(buf)
	default:
		return nil, fmt.Errorf("Unknown message Type(%d)", header.Type)
	}
	return msg, nil
}

