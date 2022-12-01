package xstr

type BytesWriter struct {
	BufPtr *[]byte
}

// buf pointer can't be nil
func NewBytesWriter(buf *[]byte) BytesWriter {
	return BytesWriter{buf}
}

// Write implements io.Writer interface.
func (bw BytesWriter) Write(data []byte) (int, error) {
	if len(data) > 0 {
		*bw.BufPtr = append(*bw.BufPtr, data...)
	}
	return len(data), nil
}

// Reset set the buffer pos to the first byte.
func (bw BytesWriter) Reset() {
	*bw.BufPtr = (*bw.BufPtr)[:0]
}

// Bytes returns the underlying buffer.
func (bw BytesWriter) Bytes() []byte {
	return *bw.BufPtr
}


type BytesCounter uint64

// Write add the len(data) to the counter
func (bc *BytesCounter) Write(data []byte) (int, error) {
	*bc += BytesCounter(len(data))
	return len(data), nil
}

