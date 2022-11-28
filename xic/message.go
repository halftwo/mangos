package xic

import (
	"bytes"
	"fmt"
	"math"
	"encoding/binary"

	"halftwo/mangos/vbs"
)

type MsgType byte

const (
	QuestMsgType MsgType = 'Q'
	AnswerMsgType        = 'A'
	CheckMsgType	     = 'C'
	HelloMsgType         = 'H'
	ByeMsgType           = 'B'
)

const MsgHeaderSize = 8

const (
	FLAG_CIPHER = 0x02
	FLAG_MASK = FLAG_CIPHER
)

type _MessageHeader struct {
	Magic    byte		// 'X'
	Version  byte		// '!'
	Type     MsgType        // 'Q', 'A', 'H', 'B', 'C'
	Flags    byte           // 0x00 or 0x02
	BodySize int32          // in big endian byte order
}

type _Message interface {
	Type() MsgType
}

type _OutMessage interface {
	_Message
	Bytes() []byte
}


func buf2header(buf []byte) (hdr _MessageHeader) {
	hdr.Magic = buf[0]
	hdr.Version = buf[1]
	hdr.Type = MsgType(buf[2])
	hdr.Flags = buf[3]
	hdr.BodySize = int32(binary.BigEndian.Uint32(buf[4:8]))
	return
}

func (hdr *_MessageHeader) FillBuffer(buf []byte) {
	buf[0] = hdr.Magic
	buf[1] = hdr.Version
	buf[2] = byte(hdr.Type)
	buf[3] = hdr.Flags
	binary.BigEndian.PutUint32(buf[4:8], uint32(hdr.BodySize))
}

var commonHeaderBytes = [8]byte{'X', '!'}

func fillHeader(packet []byte, t MsgType) {
	size := len(packet) - MsgHeaderSize
	if size < 0 {
		panic("msg body size < 0")
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
	return HelloMsgType
}

var helloMessageBytes = [8]byte{'X', '!', 'H'}

func (m _HelloMessage) Bytes() []byte {
	return helloMessageBytes[:]
}

type _ByeMessage struct {
}

func (m _ByeMessage) Type() MsgType {
	return ByeMsgType
}

var byeMessageBytes = [8]byte{'X', '!', 'B'}

func (m _ByeMessage) Bytes() []byte {
	return byeMessageBytes[:]
}

type _OutCheck struct {
	buf []byte
}

var _ _OutMessage = (*_OutCheck)(nil)

func newOutCheck(cmd string, args any) *_OutCheck {
	b := &bytes.Buffer{}
	enc := vbs.NewEncoder(b)
	b.Write(commonHeaderBytes[:])
	enc.Encode(cmd)
	err := enc.Encode(args)
	if err != nil {
		panic("vbs.Encoder error")
	}
	c := &_OutCheck{buf: b.Bytes()}
	fillHeader(c.buf, CheckMsgType)
	return c
}

func (c *_OutCheck) Type() MsgType {
	return CheckMsgType
}

func (c *_OutCheck) Bytes() []byte {
	return c.buf
}

type _OutQuest struct {
	txid     int64
	reserved int
	start    int
	buf      []byte
}

var _ _OutMessage = (*_OutQuest)(nil)

func newOutQuest(txid int64, service, method string, ctx Context, args any) *_OutQuest {
	q := &_OutQuest{txid: txid, start: -1}
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
	return QuestMsgType
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
		q.start -= MsgHeaderSize
		fillHeader(q.buf[q.start:], QuestMsgType)
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
	txid     int64
	reserved int
	start    int
	buf      []byte
}

var _ _OutMessage = (*_OutAnswer)(nil)

func newOutAnswer(normal bool, txid int64, args any) *_OutAnswer {
	a := &_OutAnswer{txid:txid, start: -1}
	b := &bytes.Buffer{}
	enc := vbs.NewEncoder(b)
	b.Write(commonHeaderBytes[:])
	enc.Encode(math.MaxInt64)
	a.reserved = b.Len()

        if normal {
                enc.Encode(0)
        } else {
                enc.Encode(-1)
        }
	err := enc.Encode(args)
	if err != nil {
		panic("vbs.Encoder error")
	}
	a.buf = b.Bytes()
	return a
}

func newOutAnswerNormal(txid int64, args any) *_OutAnswer {
	return newOutAnswer(true, txid, args)
}

func newOutAnswerExceptional(txid int64, args any) *_OutAnswer {
	return newOutAnswer(false, txid, args)
}

func (a *_OutAnswer) Type() MsgType {
	return AnswerMsgType
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
		a.start -= MsgHeaderSize
		fillHeader(a.buf[a.start:], AnswerMsgType)
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
	buf     []byte
}

func (m *_InMsg) DecodeArgs(args any) error {
	dec := vbs.NewDecoderBytes(m.buf[m.argsOff:], true)
	err := dec.Decode(args)
	if err != nil {
		return err
	} else if dec.More() {
		return fmt.Errorf("Surplus bytes left after decoding arguments")
	}
	return nil
}

type _InCheck struct {
	_InMsg
	cmd string
}

func newInCheck(buf []byte) *_InCheck {
	c := &_InCheck{}
	dec := vbs.NewDecoderBytes(buf, true)
	dec.Decode(&c.cmd)
	c.argsOff = dec.Size()
	c.buf = buf
	return c
}

func (q *_InCheck) Type() MsgType {
	return CheckMsgType
}

type _InQuest struct {
	_InMsg
	txid    int64
	service string
	method  string
	ctx     Context
}

func newInQuest(buf []byte) *_InQuest {
	q := &_InQuest{}
	dec := vbs.NewDecoderBytes(buf, true)
	dec.Decode(&q.txid)
	dec.Decode(&q.service)
	dec.Decode(&q.method)
	dec.Decode(&q.ctx)
	q.argsOff = dec.Size()
	q.buf = buf
	return q
}

func (q *_InQuest) Type() MsgType {
	return QuestMsgType
}

type _InAnswer struct {
	txid   int64
	status int
	_InMsg
}

func newInAnswer(buf []byte) *_InAnswer {
	a := &_InAnswer{}
	dec := vbs.NewDecoderBytes(buf, true)
	dec.Decode(&a.txid)
	dec.Decode(&a.status)
	a.argsOff = dec.Size()
	a.buf = buf
	return a
}

func (a *_InAnswer) Type() MsgType {
	return AnswerMsgType
}

func DecodeMessage(header _MessageHeader, buf []byte) (_Message, error) {
	if header.Magic != 'X' || header.Version != '!' {
		return nil, fmt.Errorf("Unknown message Magic(%d) and Version(%d)", header.Magic, header.Version)
	}
	if header.Type == HelloMsgType || header.Type == ByeMsgType {
		if header.Flags != 0 || header.BodySize != 0 {
			return nil, fmt.Errorf("Invalid Hello or Bye message")
		}
	}

	var msg _Message
	switch header.Type {
	case QuestMsgType:
		msg = newInQuest(buf)
	case AnswerMsgType:
		msg = newInAnswer(buf)
	case CheckMsgType:
		msg = newInCheck(buf)
	case HelloMsgType:
		msg = theHelloMessage
	case ByeMsgType:
		msg = theByeMessage
	default:
		return nil, fmt.Errorf("Unknown message Type(%d)", header.Type)
	}
	return msg, nil
}
