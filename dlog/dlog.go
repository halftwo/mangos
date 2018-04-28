package dlog

import (
	"net"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"unsafe"
	"runtime"
	"strconv"
	"strings"
)

const RECORD_TYPE_RAW	= 0
const RECORD_VERSION	= 2

type timeval struct {
	sec int32
	usec int32
}

type head struct {
        size uint16	// include the size itself and trailing '\0' 
	ttv byte	// truncated:1, type:3, version:4
	locusEnd uint8
	port uint16
	pid  int16
        time timeval
}

type record struct {
	off int
	head head
	buf [4000]byte
}

var recPool = sync.Pool{
	New: func() interface{} {
		r := new(record)
		r.head.ttv = (RECORD_TYPE_RAW << 4) | (RECORD_VERSION)
		r.head.pid = int16(os.Getpid())
		return r
	},
}

func (rec *record) Reset() {
	rec.off = int(unsafe.Sizeof(rec.head))
}

const (
	IDENTITY_MAX	= 63
	TAG_MAX		= 63
	LOCUS_MAX       = 127
)

func (rec *record) SetIdentityTagLocus(identity, tag, locus string) {

	rec.putMax(identity, IDENTITY_MAX)
	rec.WriteByte(' ')
	rec.putMax(tag, TAG_MAX)
	rec.WriteByte(' ')
	rec.putMax(locus, LOCUS_MAX)

	rec.head.locusEnd = uint8(rec.off) - uint8(unsafe.Sizeof(rec.head))
	rec.WriteByte(' ')
}

func (rec *record) putMax(s string, max int) {
	if rec.off < len(rec.buf) {
		k := len(s)
		buf := []byte(s)
		if k <= max {
			if k > 0 {
				rec.off += copy(rec.buf[rec.off:], buf)
			} else {
				rec.buf[rec.off] = '-'
				rec.off++
			}
		} else {
			rec.off += copy(rec.buf[rec.off:], buf[:max])
		}
	}
}

func (rec *record) Write(buf []byte) (int, error) {
	if rec.off < len(rec.buf) {
		k := copy(rec.buf[rec.off:], buf)
		rec.off += k
	}
	return len(buf), nil
}

func (rec *record) WriteByte(b byte) error {
	if rec.off < len(rec.buf) {
		rec.buf[rec.off] = b
		rec.off++
	}
	return nil
}

func (rec *record) Bytes() []byte {
	if rec.off < len(rec.buf) {
		rec.buf[rec.off] = 0
		rec.off++
		rec.head.ttv &= 0x7F
	} else { // truncated
		rec.buf[len(rec.buf)-1] = 0
		rec.off = len(rec.buf)
		rec.head.ttv |= 0x80
	}
	rec.head.size = uint16(rec.off)

	h := &rec.head
	// TODO test ByteOrder
	rec.buf[0] = byte(h.size)
	rec.buf[1] = byte(h.size >> 8)
	rec.buf[2] = h.ttv
	rec.buf[3] = h.locusEnd
	rec.buf[6] = byte(h.pid)
	rec.buf[7] = byte(h.pid >> 8)
	return rec.buf[:rec.off]
}

func (rec *record) Truncated() bool {
	return rec.off >= len(rec.buf)
}

var theLogger = newdlogger()

type dlogger struct {
	identity string
	option int
	con atomic.Value // net.Conn
}

func newdlogger() *dlogger {
	id := os.Args[0]
	i := strings.LastIndexByte(id, os.PathSeparator)
	if i >= 0 {
		id = id[i+1:]
	}
	return &dlogger{identity:id}
}

func (lg *dlogger) SetIdentity(identity string) {
	lg.identity = identity
}

func (lg *dlogger) SetOption(option int) {
	lg.option = option
}

func (lg *dlogger) Log(tag string, format string, a ...interface{}) {
	var locus string

	_, file, line, ok := runtime.Caller(0)
	if ok {
		i := strings.LastIndexByte(file, os.PathSeparator)
		if i >= 0 {
			file = file[i+1:]
		}
		locus = file + ":" + strconv.Itoa(line)
	}

	lg.XLog(lg.identity, tag, locus, format, a...)
}

func (lg *dlogger) XLog(identity string, tag string, locus string, format string, a ...interface{}) {
	rec := recPool.Get().(*record)
	rec.Reset()
	rec.SetIdentityTagLocus(identity, tag, locus)
	fmt.Fprintf(rec, format, a...)

	var con net.Conn
	c := lg.con.Load()
	if c != nil {
		con = c.(net.Conn)
	} else {
		con = dialUdp()
		if con == nil {
			// TODO
			return
		}
		lg.con.Store(con)
	}

	buf := rec.Bytes()
	_, err := con.Write(buf)
	if err != nil {
		// TODO
	}
	recPool.Put(rec)
}

func dialUdp() net.Conn {
	con, err := net.Dial("udp", "[::1]:6109")
	if err != nil {
		return nil
	}
	return con
}


func SetIdentity(identity string) {
	theLogger.SetIdentity(identity)
}

func SetOption(option int) {
	theLogger.SetOption(option)
}

func Log(tag string, format string, a ...interface{}) {
	theLogger.Log(tag, format, a...)
}

func XLog(identity string, tag string, locus string, format string, a ...interface{}) {
	theLogger.XLog(identity, tag, locus, format, a...)
}

