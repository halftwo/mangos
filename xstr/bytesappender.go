package xstr

type BytesAppender struct {
	buf *[]byte
}

// NewBytesAppender will reset the buffer pos to the first byte
func NewBytesAppender(buf *[]byte) BytesAppender {
	if buf == nil {
		buf = new([]byte)
	}
	*buf = (*buf)[:0]
	return BytesAppender{buf}
}

// Write implements io.Writer interface
func (ba BytesAppender) Write(data []byte) (int, error) {
	if len(data) > 0 {
		*ba.buf = append(*ba.buf, data...)
	}
	return len(data), nil
}

// Reset set the buffer pos to the first byte
func (ba BytesAppender) Reset() {
	*ba.buf = (*ba.buf)[:0]
}

// Bytes returns the underlying buffer
func (ba BytesAppender) Bytes() []byte {
	return *ba.buf
}

