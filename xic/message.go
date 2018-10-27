package xic

import (
	"fmt"
	"math"
	"bytes"

	"halftwo/mangos/vbs"
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

type _MessageHeader struct {
	Magic byte	// 'X'
	Version byte	// '!'
	Type byte	// 'Q', 'A', 'H', 'B', 'C'
	Flags byte	// 0x00 or 0x02
	BodySize int32	// in big endian byte order
}

type _Message interface {
	Type() MsgType
}

type _OutMessage interface {
	_Message
	Bytes() []byte
}

type _InMessage interface {
	_Message
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

var theHelloMessage = _HelloMessage{}
var theByeMessage = _ByeMessage{}


type _HelloMessage struct {
}

func (m _HelloMessage) Type() MsgType {
	return 'H'
}

var helloMessageBytes = [8]byte{'X','!','H'}

func (m _HelloMessage) Bytes() []byte {
	return helloMessageBytes[:]
}


type _ByeMessage struct {
}

func (m _ByeMessage) Type() MsgType {
	return 'B'
}

var byeMessageBytes = [8]byte{'X','!','B'}

func (m _ByeMessage) Bytes() []byte {
	return byeMessageBytes[:]
}

type _OutCheck struct {
	buf []byte
}
var _ _OutMessage = (*_OutCheck)(nil)

func newOutCheck(cmd string, args interface{}) *_OutCheck {
	b := &bytes.Buffer{}
	enc := vbs.NewEncoder(b)
	b.Write(commonHeaderBytes[:])
	enc.Encode(cmd)
	err := enc.Encode(args)
	if err != nil {
		panic("vbs.Encoder error")
	}
	c := &_OutCheck{buf:b.Bytes()}
	fillHeader(c.buf, 'C')
	return c
}

func (c *_OutCheck) Type() MsgType {
	return 'C'
}

func (c *_OutCheck) Bytes() []byte {
	return c.buf
}


type _OutQuest struct {
	txid int64
	reserved int
	start int
	buf []byte
}
var _ _OutMessage = (*_OutQuest)(nil)

func newOutQuest(txid int64, service, method string, ctx Context, args interface{}) *_OutQuest {
	q := &_OutQuest{txid:txid, start:-1}
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

func (q *_OutQuest) Type() MsgType {
	return 'Q'
}

func (q *_OutQuest) Bytes() []byte {
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

func (q *_OutQuest) SetTxid(txid int64) {
	if q.txid != 0 {
		q.txid = txid
		q.start = -1
	}
}

type _OutAnswer struct {
	txid int64
	reserved int
	start int
	buf []byte
}
var _ _OutMessage = (*_OutAnswer)(nil)

func newOutAnswer(status int, args interface{}) *_OutAnswer {
	a := &_OutAnswer{txid:-1, start:-1}
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

func newOutAnswerNormal(args interface{}) *_OutAnswer {
	return newOutAnswer(0, args)
}

func newOutAnswerExceptional(args interface{}) *_OutAnswer {
	return newOutAnswer(-1, args)
}

func (a *_OutAnswer) Type() MsgType {
	return 'A'
}

func (a *_OutAnswer) Bytes() []byte {
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

func (a *_OutAnswer) SetTxid(txid int64) {
	if a.txid != 0 {
		a.txid = txid
		a.start = -1
	}
}

type _InMsg struct {
	argsOff int
	buf []byte
}

func (m *_InMsg) DecodeArgs(args interface{}) error {
	dec := vbs.NewDecoderBytes(m.buf[m.argsOff:])
	err := dec.Decode(args)
	if err != nil {
		return err
	} else if dec.More() {
		return fmt.Errorf("Surplus bytes left after decoding arguments")
	}
	return nil
}

type _InCheck struct {
	cmd string
	_InMsg
}

func newInCheck(buf []byte) *_InCheck {
	c := &_InCheck{}
	dec := vbs.NewDecoderBytes(buf)
	dec.Decode(&c.cmd)
	c.argsOff = dec.Size()
	c.buf = buf
	return c
}

func (q *_InCheck) Type() MsgType {
	return 'C'
}

type _InQuest struct {
	txid int64
	service string
	method string
	ctx Context
	_InMsg
}

func newInQuest(buf []byte) *_InQuest {
	q := &_InQuest{}
	dec := vbs.NewDecoderBytes(buf)
	dec.Decode(&q.txid)
	dec.Decode(&q.service)
	dec.Decode(&q.method)
	dec.Decode(&q.ctx)
	q.argsOff = dec.Size()
	q.buf = buf
	return q
}

func (q *_InQuest) Type() MsgType {
	return 'Q'
}

type _InAnswer struct {
	txid int64
	status int
	_InMsg
}

func newInAnswer(buf []byte) *_InAnswer {
	a := &_InAnswer{}
	dec := vbs.NewDecoderBytes(buf)
	dec.Decode(&a.txid)
	dec.Decode(&a.status)
	a.argsOff = dec.Size()
	a.buf = buf
	return a
}

func (a *_InAnswer) Type() MsgType {
	return 'A'
}

func DecodeMessage(header _MessageHeader, buf []byte) (_Message, error) {
	if header.Magic != 'X' || header.Version != '!' {
		return nil, fmt.Errorf("Unknown message Magic(%d) and Version(%d)", header.Magic, header.Version)
	}
	if header.Type == 'H' || header.Type == 'B' {
		if header.Flags != 0 || header.BodySize != 0 {
			return nil, fmt.Errorf("Invalid Hello or Bye message")
		}
	}

	var msg _Message
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

