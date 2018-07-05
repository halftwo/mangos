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
	bw := NewBytesWriterAppend(buf)
	bw.Reset()
	return bw
}

// Write implements io.Writer interface.
func (bw BytesWriter) Write(data []byte) (int, error) {
	if len(data) > 0 {
		*bw.buf = append(*bw.buf, data...)
	}
	return len(data), nil
}

// Reset set the buffer pos to the first byte.
func (bw BytesWriter) Reset() {
	*bw.buf = (*bw.buf)[:0]
}

// Bytes returns the underlying buffer.
func (bw BytesWriter) Bytes() []byte {
	return *bw.buf
}


type BytesCounter uint64

// Write add the len(data) to the counter
func (bc *BytesCounter) Write(data []byte) (int, error) {
	*bc += BytesCounter(len(data))
	return len(data), nil
}

