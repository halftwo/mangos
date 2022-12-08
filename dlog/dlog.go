// Package dlog implements dlog client.
// The dlog servers in C++ can be found at 
// https://github.com/halftwo/knotty/dlog
package dlog

import (
	"io"
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

type Tag string

const (
	ALERT	Tag	= "ALERT"
	ERROR		= "ERROR"
	WARN		= "WARN"
	NOTICE		= "NOTICE"
	INFO		= "INFO"
	DEBUG		= "DEBUG"
)

const (
	_RECORD_HEAD_SIZE	= 18

	_RECORD_VERSION		= 6
	_RECORD_BIG_ENDIAN	= 0x08

	_RECORD_TRUNCATED	= 0x80
)

const DLOG_RECORD_SIZE		= 3984	// should be less than 3990

// big endian byte order
type _RecordHead struct {
	size uint16	// include the size itself and trailing '\0' 
	ttev byte	// truncated:1, type:3, bigendian:1, version:3
	locusEnd uint8
	pid  uint32
	msec int64	// dlogd server will set this field
	port uint16	// dlogd server will set this field
}

type _Record struct {
	off int
	size uint16
	ttev byte
	locusEnd uint8
	tagEnd uint8
	identityEnd uint8
	buf [DLOG_RECORD_SIZE]byte
}

var thePid = uint32(os.Getpid())

var recPool = sync.Pool{
	New: func() any {
		r := new(_Record)
		r.buf[4] = byte(thePid >> 24)
		r.buf[5] = byte(thePid >> 16)
		r.buf[6] = byte(thePid >> 8)
		r.buf[7] = byte(thePid)
		return r
	},
}

func (rec *_Record) Reset() {
	rec.off = _RECORD_HEAD_SIZE
	rec.size = 0
	rec.ttev = _RECORD_BIG_ENDIAN | _RECORD_VERSION
}

// Max length of identity, tag and locus strings
const (
	IDENTITY_MAX	= 63
	TAG_MAX		= 63
	LOCUS_MAX       = 127
)

func (rec *_Record) SetIdentityTagLocus(identity string, tag Tag, locus string) {
	rec.putMaxNoCheck(identity, IDENTITY_MAX)
	rec.identityEnd = uint8(rec.off - _RECORD_HEAD_SIZE)
	rec.writeByteNoCheck(' ')
	rec.putMaxNoCheck(string(tag), TAG_MAX)
	rec.tagEnd = uint8(rec.off - _RECORD_HEAD_SIZE)
	rec.writeByteNoCheck(' ')
	rec.putMaxNoCheck(locus, LOCUS_MAX)
	rec.locusEnd = uint8(rec.off - _RECORD_HEAD_SIZE)
	rec.writeByteNoCheck(' ')
}

func (rec *_Record) writeByteNoCheck(b byte) {
	rec.buf[rec.off] = b
	rec.off++
}

func (rec *_Record) putMaxNoCheck(s string, max int) {
	k := len(s)
	if k > max {
		k = max
	}

	if k > 0 {
		copy(rec.buf[rec.off:], s[:k])
		rec.off += k
	} else {
		rec.buf[rec.off] = '-'
		rec.off++
	}
}

func (rec *_Record) Write(buf []byte) (int, error) {
	if rec.off < len(rec.buf) {
		copy(rec.buf[rec.off:], buf)
	}
	k := len(buf)
	rec.off += k
	return k, nil
}

func (rec *_Record) Bytes() []byte {
	if rec.size > 0 {
		return rec.buf[:rec.size]
	}

	size := rec.off
	if size <= len(rec.buf) {
		// trim trailing '\r' and '\n'
		for ; size > _RECORD_HEAD_SIZE; size-- {
			c := rec.buf[size - 1]
			if c != '\r' && c != '\n' {
				break
			}
		}
	}

	if size < len(rec.buf) {
		size++
	} else {
		rec.ttev |= _RECORD_TRUNCATED
		size = len(rec.buf)
	}
	rec.size = uint16(size)
	rec.buf[size - 1] = 0	// trailing '\0'

	rec.buf[0] = byte(size >> 8)
	rec.buf[1] = byte(size)
	rec.buf[2] = rec.ttev
	rec.buf[3] = rec.locusEnd
	return rec.buf[:size]
}

func (rec *_Record) BodyBytes() []byte {
	if rec.size == 0 {
		rec.Bytes()
	}
	return rec.buf[_RECORD_HEAD_SIZE:rec.size]
}

func (rec *_Record) Truncated() bool {
	return rec.off >= len(rec.buf)
}

var theLogger = NewLogger("")


type Logger struct {
	option uint32
	connected atomic.Uint32
	con atomic.Value	// net.Conn
	lastFailTime time.Time
	identity string
	altOut io.Writer
	mutex sync.Mutex
}

func NewLogger(id string) *Logger {
	if id == "" {
		id = os.Args[0]
		i := strings.LastIndexByte(id, os.PathSeparator)
		id = id[i+1:]
	}

	lg := &Logger{identity:id, altOut:os.Stderr}
	return lg
}

// Options for dlog
const (
	OPT_ALTOUT	= 0x01	// Always print to alternative output in addition to write to dlogd.
	OPT_ALTERR	= 0x02	// If failed to connect dlogd, print to the alternative output.
	OPT_TCP		= 0x04	// Use TCP instead of UDP to connect dlogd server.
	OPT_NONET	= 0x08	// DON'T write to dlogd server.
)

func (lg *Logger) SetOption(option int) {
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

func (lg *Logger) Option() int {
	return int(atomic.LoadUint32(&lg.option))
}

func (lg *Logger) Id() string {
	lg.mutex.Lock()
	id := lg.identity
	lg.mutex.Unlock()
	return id
}

func (lg *Logger) SetAltWriter(w io.Writer) {
	if w == nil {
		w = os.Stderr
	}
	lg.mutex.Lock()
	lg.altOut = w
	lg.mutex.Unlock()
}

// Log send a dlog to dlogd. 
// identity is from the logger's default.
// locus is from runtime.Caller()
func (lg *Logger) Log(tag Tag, format string, a ...any) {
	var locus string
	if lg == theLogger {
		locus = GetFileLine(2)
	} else {
		locus = GetFileLine(1)
	}
	lg.Allog(lg.identity, tag, locus, format, a...)
}

// NB: the rec content is destroyed in this function
func (lg *Logger) printAlt(rec *_Record) {
	var ts[18] byte
	k := timeNoZone(ts[:], time.Now())
	n := (_RECORD_HEAD_SIZE + int(rec.identityEnd)) - k
	buf := rec.Bytes()[n:]
	copy(buf, ts[:k])
	buf[len(buf)-1] = '\n'	// replace the trailing '\0' with '\n'

	lg.mutex.Lock()
	lg.altOut.Write(buf)
	lg.mutex.Unlock()
}

// Allog send a dlog to dlogd. 
// identity and locus are also specified in the arguments.
func (lg *Logger) Allog(identity string, tag Tag, locus string, format string, a ...any) {
	rec := recPool.Get().(*_Record)
	defer recPool.Put(rec)

	rec.Reset()
	rec.SetIdentityTagLocus(identity, tag, locus)
	fmt.Fprintf(rec, format, a...)
	lg.dolog(rec)
}

func (lg *Logger) dolog(rec *_Record) {
	fail := false
	if lg.option & OPT_NONET == 0 {
		var err error
		con := lg.getConn()
		if con == nil {
			fail = true
			goto net_done
		}

		buf := rec.Bytes()
		_, err = con.Write(buf)
		if err != nil {
			fail = true
			lg.shut(con)
		}
	}

net_done:
	if (lg.option & OPT_ALTOUT != 0) || (fail && lg.option & OPT_ALTERR != 0) {
		lg.printAlt(rec)
	}
}

func (lg *Logger) getConn() net.Conn {
	if lg.connected.Load() == 1 {
		return lg.con.Load().(net.Conn)
	}

	return lg.dial()
}

func (lg *Logger) dial() net.Conn {
	lg.mutex.Lock()
	defer lg.mutex.Unlock()

	if lg.connected.Load() == 1 {
		return lg.con.Load().(net.Conn)
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
	lg.connected.Store(1)
	return con
}

func (lg *Logger) shut(con net.Conn) {
	doit := false
	if lg.connected.Load() == 1 {
		lg.mutex.Lock()
		if lg.connected.Load() == 1 {
			if lg.con.Load() == con {
				lg.connected.Store(0)
				doit = true
			}
		}
		lg.mutex.Unlock()
	}
	if doit {
		con.Close()
	}
}

func SetOption(option int) {
	theLogger.SetOption(option)
}

func Option() int {
	return theLogger.Option()
}

func Id() string {
	return theLogger.Id()
}

func SetAltWriter(w io.Writer) {
	theLogger.SetAltWriter(w)
}

func Log(tag Tag, format string, a ...any) {
	theLogger.Log(tag, format, a...)
}

func Allog(identity string, tag Tag, locus string, format string, a ...any) {
	theLogger.Allog(identity, tag, locus, format, a...)
}


func GetFileLine(stackSkip int) string {
	_, file, line, ok := runtime.Caller(stackSkip + 1)
	if !ok {
		return ""
	}
	k := strings.LastIndexByte(file, '/')
	if k > 0 {
		k = strings.LastIndexByte(file[:k], '/')
	}
	return file[k+1:] + ":" + strconv.Itoa(line)
}

func LogPanic() {
	if x := recover(); x != nil {
		Log("PANIC", "%#v", x)
		os.Exit(1)
	}
}

