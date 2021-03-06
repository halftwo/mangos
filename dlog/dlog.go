// Package dlog implements dlog client.
// The dlog servers in C++ can be found at 
// https://github.com/halftwo/knotty/dlog
package dlog

import (
	"net"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const record_TYPE_RAW	= 0
const record_VERSION	= 4

const record_HEAD_SIZE	= 16
const record_BIG_ENDIAN = 0x08

// little endian byte order
type _RecordHead struct {
	size uint16	// include the size itself and trailing '\0' 
	ttev byte	// truncated:1, type:3, bigendian:1, version:3
	locusEnd uint8
	port uint16
	pid  uint16
	usec int64
}

type _RecordMan struct {
	pid uint16
	ttev byte
	locusEnd uint8
	size uint16
	off int
	buf [4000]byte
}

var recPool = sync.Pool{
	New: func() interface{} {
		pid := uint16(os.Getpid())
		r := new(_RecordMan)
		r.pid = pid
		r.ttev = (record_TYPE_RAW << 4) | record_BIG_ENDIAN | (record_VERSION)
		r.buf[6] = byte(pid >> 8)
		r.buf[7] = byte(pid)
		return r
	},
}

func (rec *_RecordMan) Reset() {
	rec.off = record_HEAD_SIZE
	rec.locusEnd = 0
	rec.size = 0
}

// Max length of identity, tag and locus strings
const (
	IDENTITY_MAX	= 63
	TAG_MAX		= 63
	LOCUS_MAX       = 127
)

func (rec *_RecordMan) SetIdentityTagLocus(identity, tag, locus string) {
	rec.putMax(identity, IDENTITY_MAX)
	rec.WriteByte(' ')
	rec.putMax(tag, TAG_MAX)
	rec.WriteByte(' ')
	rec.putMax(locus, LOCUS_MAX)
	rec.locusEnd = uint8(rec.off - record_HEAD_SIZE)
	rec.WriteByte(' ')
}

func (rec *_RecordMan) putMax(s string, max int) {
	k := len(s)
	if k > max {
		k = max
	}

	if rec.off < len(rec.buf) {
		if k > 0 {
			buf := []byte(s)
			copy(rec.buf[rec.off:], buf[:k])
		} else {
			rec.buf[rec.off] = '-'
			k = 1
		}
	}
	rec.off += k
}

func (rec *_RecordMan) Write(buf []byte) (int, error) {
	if rec.off < len(rec.buf) {
		copy(rec.buf[rec.off:], buf)
	}
	k := len(buf)
	rec.off += k
	return k, nil
}

func (rec *_RecordMan) WriteByte(b byte) error {
	if rec.off < len(rec.buf) {
		rec.buf[rec.off] = b
	}
	rec.off++
	return nil
}

func (rec *_RecordMan) Bytes() []byte {
	if rec.size > 0 {
		return rec.buf[:rec.size]
	}

	size := rec.off
	if size <= len(rec.buf) {
		// trim trailing '\r' and '\n'
		for ; size > record_HEAD_SIZE; size-- {
			c := rec.buf[size - 1]
			if c != '\r' && c != '\n' {
				break
			}
		}
	}

	ttev := rec.ttev
	if size < len(rec.buf) {
		size++
	} else {
		// truncated
		ttev |= 0x80
		size = len(rec.buf)
	}
	rec.size = uint16(size)
	rec.buf[size - 1] = 0

	rec.buf[0] = byte(size >> 8)
	rec.buf[1] = byte(size)
	rec.buf[2] = ttev
	rec.buf[3] = rec.locusEnd
	return rec.buf[:size]
}

func (rec *_RecordMan) BodyBytes() []byte {
	if rec.size == 0 {
		rec.Bytes()
	}
	return rec.buf[record_HEAD_SIZE:rec.size]
}

func (rec *_RecordMan) Truncated() bool {
	return rec.off >= len(rec.buf)
}

var theLogger = NewDlogger("")

type Dlogger struct {
	option uint32
	identity string
	con atomic.Value	// net.Conn
	lastFailTime time.Time
	mutex sync.Mutex
}

func NewDlogger(identity string) *Dlogger {
	var id = identity
	if identity == "" {
		id = os.Args[0]
		var i = strings.LastIndexByte(id, os.PathSeparator)
		if i >= 0 {
			id = id[i+1:]
		}
	}

	var lg = &Dlogger{identity:id}
	return lg
}

// Options for dlog
const (
	OPT_STDERR	= 0x01	// Always print to stderr in addition to write to dlogd.
	OPT_PERROR	= 0x02	// If failed to connect dlogd, print to stderr.
	OPT_TCP		= 0x04	// Use TCP instead of UDP to connect dlogd server.
)

func (lg *Dlogger) SetOption(option int) {
	opt := uint32(option)
	old := atomic.SwapUint32(&lg.option, opt)
	old_tcp := old & OPT_TCP
	new_tcp := opt & OPT_TCP
	if (old_tcp ^ new_tcp) != 0 {
		c := lg.con.Load()
		if c != nil {
			lg.shut(c.(net.Conn))
		}
	}
}

func getLocus(skip int) (locus string) {
	_, file, line, ok := runtime.Caller(skip + 1)
	if ok {
		i := strings.LastIndexByte(file, os.PathSeparator)
		if i >= 0 {
			file = file[i+1:]
		}
		locus = file + ":" + strconv.Itoa(line)
	}
	return
}

// Log send a dlog to dlogd. 
// identity is from the logger's default.
// locus is from runtime.Caller()
func (lg *Dlogger) Log(tag string, format string, a ...interface{}) {
	var locus string
	if lg == theLogger {
		locus = getLocus(2)
	} else {
		locus = getLocus(1)
	}
	lg.XLog(lg.identity, tag, locus, format, a...)
}

func (lg *Dlogger) printStderr(rec *_RecordMan) {
	now := time.Now()
	ts := now.Format("060102-150405")
	fmt.Fprintf(os.Stderr, "%s :: %d+%d %s\n", ts, rec.pid, 0, rec.BodyBytes())
}

// XLog send a dlog to dlogd. 
// identity and locus are also specified in the arguments.
func (lg *Dlogger) XLog(identity string, tag string, locus string, format string, a ...interface{}) {
	rec := recPool.Get().(*_RecordMan)
	defer recPool.Put(rec)

	rec.Reset()
	rec.SetIdentityTagLocus(identity, tag, locus)
	fmt.Fprintf(rec, format, a...)
	buf := rec.Bytes()

	perror_done := false
	if lg.option & OPT_STDERR != 0 {
		perror_done = true
		lg.printStderr(rec)
	}

	var con net.Conn
	c := lg.con.Load()
	if c != nil {
		con = c.(net.Conn)
	} else {
		con = lg.dial()
		if con == nil {
			if (lg.option & OPT_PERROR != 0) && !perror_done {
				perror_done = true
				lg.printStderr(rec)
			}
			return
		}
	}

	_, err := con.Write(buf)
	if err != nil {
		lg.shut(con)
		if (lg.option & OPT_PERROR != 0) && !perror_done {
			perror_done = true
			lg.printStderr(rec)
		}
	}
}

func (lg *Dlogger) dial() net.Conn {
	lg.mutex.Lock()
	defer lg.mutex.Unlock()

	c := lg.con.Load()
	if c != nil {
		return c.(net.Conn)
	}

	if time.Since(lg.lastFailTime) < time.Second {
		return nil
	}

	var con net.Conn
	var err error
	if lg.option & OPT_TCP != 0 {
		con, err = net.Dial("tcp", "127.0.0.1:6109")
	} else {
		con, err = net.Dial("udp", "127.0.0.1:6109")
	}

	if err != nil {
		lg.lastFailTime = time.Now()
		return nil
	}
	lg.con.Store(con)
	return con
}

func (lg *Dlogger) shut(con net.Conn) {
	con.Close()

	lg.mutex.Lock()
	defer lg.mutex.Unlock()

	c := lg.con.Load()
	if c == con {
		lg.con.Store(nil)
	}
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

