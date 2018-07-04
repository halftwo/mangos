package xstr

type BytesWriter struct {
	buf *[]byte
}

// NewBytesWriterAppend create a BytesWriter object.
// New data will append to the underlying buffer.
func NewBytesWriterAppend(buf *[]byte) BytesWriter {
	if buf == nil {
		buf = new([]byte)
	}
	return BytesWriter{buf}
}

// NewBytesWriter will reset the buffer pos to the first byte.
// New data will write from the begining of the underlying buffer.
func NewBytesWriter(buf *[]byte) BytesWriter {
	ba := NewBytesWriter(buf)
	ba.Reset()
	return ba
}

// Write implements io.Writer interface.
func (ba BytesWriter) Write(data []byte) (int, error) {
	if len(data) > 0 {
		*ba.buf = append(*ba.buf, data...)
	}
	return len(data), nil
}

// Reset set the buffer pos to the first byte.
func (ba BytesWriter) Reset() {
	*ba.buf = (*ba.buf)[:0]
}

// Bytes returns the underlying buffer.
func (ba BytesWriter) Bytes() []byte {
	return *ba.buf
}

