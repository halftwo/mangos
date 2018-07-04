package xstr

type BytesAppender struct {
	buf *[]byte
}

func NewBytesAppender(buf *[]byte) BytesAppender {
	if buf == nil {
		buf = new([]byte)
	}
	return BytesAppender{buf}
}

func (ba BytesAppender) Write(data []byte) (int, error) {
	if len(data) > 0 {
		*ba.buf = append(*ba.buf, data...)
	}
	return len(data), nil
}

func (ba BytesAppender) Bytes() []byte {
	return *ba.buf
}

