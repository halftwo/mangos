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
	_RECORD_TYPE_RAW	= 0
	_RECORD_VERSION		= 6

	_RECORD_HEAD_SIZE	= 18
	_RECORD_BIG_ENDIAN	= 0x08
)

const DLOG_RECORD_SIZE		= 3984	// should be less than 3990

// big endian byte order
type _RecordHeadV6 struct {
	size uint16	// include the size itself and trailing '\0' 
	ttev byte	// truncated:1, type:3, bigendian:1, version:3
	locusEnd uint8
	pid  uint32
	msec int64
	port uint16
}

type _RecordMan struct {
	pid uint32
	ttev byte
	identityEnd uint8
	tagEnd uint8
	locusEnd uint8
	off int
	size uint16
	buf [DLOG_RECORD_SIZE]byte
}

var recPool = sync.Pool{
	New: func() any {
		pid := uint32(os.Getpid())
		r := new(_RecordMan)
		r.pid = pid
		r.ttev = (_RECORD_TYPE_RAW << 4) | _RECORD_BIG_ENDIAN | (_RECORD_VERSION)
		r.buf[4] = byte(pid >> 24)
		r.buf[5] = byte(pid >> 16)
		r.buf[6] = byte(pid >> 8)
		r.buf[7] = byte(pid)
		return r
	},
}

func (rec *_RecordMan) Reset() {
	rec.off = _RECORD_HEAD_SIZE
	rec.locusEnd = 0
	rec.size = 0
}

// Max length of identity, tag and locus strings
const (
	IDENTITY_MAX	= 63
	TAG_MAX		= 63
	LOCUS_MAX       = 127
)

func (rec *_RecordMan) SetIdentityTagLocus(identity string, tag Tag, locus string) {
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

func (rec *_RecordMan) writeByteNoCheck(b byte) {
	rec.buf[rec.off] = b
	rec.off++
}

func (rec *_RecordMan) putMaxNoCheck(s string, max int) {
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

func (rec *_RecordMan) Write(buf []byte) (int, error) {
	if rec.off < len(rec.buf) {
		copy(rec.buf[rec.off:], buf)
	}
	k := len(buf)
	rec.off += k
	return k, nil
}

func (rec *_RecordMan) Bytes() []byte {
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

	ttev := rec.ttev
	if size < len(rec.buf) {
		size++
	} else {
		// truncated
		ttev |= 0x80
		size = len(rec.buf)
	}
	rec.size = uint16(size)
	rec.buf[size - 1] = 0	// trailing '\0'

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
	return rec.buf[_RECORD_HEAD_SIZE:rec.size]
}

func (rec *_RecordMan) Truncated() bool {
	return rec.off >= len(rec.buf)
}

var theLogger = NewLogger("")

type Logger struct {
	option uint32
	identity string
	con atomic.Value	// net.Conn
	lastFailTime time.Time
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

func getLocus(skip int) (locus string) {
	_, file, line, ok := runtime.Caller(skip + 1)
	if ok {
		k := strings.LastIndexByte(file, '/')
		if k > 0 {
			k = strings.LastIndexByte(file[:k], '/')
		}
		file := file[k+1:]
		locus = file + ":" + strconv.Itoa(line)
	}
	return
}

// Log send a dlog to dlogd. 
// identity is from the logger's default.
// locus is from runtime.Caller()
func (lg *Logger) Log(tag Tag, format string, a ...any) {
	var locus string
	if lg == theLogger {
		locus = getLocus(2)
	} else {
		locus = getLocus(1)
	}
	lg.Allog(lg.identity, tag, locus, format, a...)
}

// NB: the rec content is destroyed in this function
func (lg *Logger) printAlt(rec *_RecordMan) {
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
	rec := recPool.Get().(*_RecordMan)
	defer recPool.Put(rec)

	rec.Reset()
	rec.SetIdentityTagLocus(identity, tag, locus)
	fmt.Fprintf(rec, format, a...)
	lg.dolog(rec)
}

func (lg *Logger) dolog(rec *_RecordMan) {
	var do_alt bool
	if lg.option & OPT_NONET == 0 {
		var err error
		con, ok := lg.con.Load().(net.Conn)
		if !ok {
			con = lg.dial()
			if con == nil {
				if lg.option & OPT_ALTERR != 0 {
					do_alt = true
					goto net_done
				}
			}
		}

		buf := rec.Bytes()
		_, err = con.Write(buf)
		if err != nil {
			lg.shut(con)
			if lg.option & OPT_ALTERR != 0 {
				do_alt = true
			}
		}
	}

	if lg.option & OPT_ALTOUT != 0 {
		do_alt = true
	}

net_done:
	if do_alt {
		lg.printAlt(rec)
	}
}

func (lg *Logger) dial() net.Conn {
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

func (lg *Logger) shut(con net.Conn) {
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

