package xstr

import (
	"io"
	"fmt"
)

type ChunkWriter struct {
	w io.Writer
	chunk []byte
	pos int
	err error
}

// NewChunkWriter create a ChunkWriter object.
func NewChunkWriter(w io.Writer, chunkSize int) *ChunkWriter {
	chunk := make([]byte, chunkSize)
	return &ChunkWriter{w:w, chunk:chunk}
}

// Write implements io.Writer interface.
func (cw *ChunkWriter) Write(data []byte) (int, error) {
	if cw.err != nil {
		return 0, cw.err
	}

	chunkSize := len(cw.chunk)
	dataSize := len(data)
	if cw.pos > 0 {
		n := copy(cw.chunk[cw.pos:], data)
		cw.pos += n
		if cw.pos == chunkSize {
			cw.doWrite(cw.chunk)
			if cw.err != nil {
				return 0, cw.err
			}
			cw.pos = 0
		}
		if n == dataSize {
			return n, nil
		}
		data = data[n:]
	}

	for ; len(data) >= chunkSize; data = data[chunkSize:] {
		cw.doWrite(data[:chunkSize])
		if cw.err != nil {
			return 0, cw.err
		}
	}

	if len(data) > 0 {
		k := copy(cw.chunk[cw.pos:], data)
		cw.pos += k
	}

	return dataSize, cw.err
}

// WriteByte implements io.ByteWriter interface.
func (cw *ChunkWriter) WriteByte(c byte) error {
	buf := [1]byte{c}
	_, err := cw.Write(buf[:])
	return err
}

// Flush() writes the data in chunk buffer (if any) to the underlying writer.
func (cw *ChunkWriter) Flush() error {
	if cw.pos > 0 {
		cw.doWrite(cw.chunk[:cw.pos])
		if cw.err == nil {
			cw.pos = 0
		}
	}
	return cw.err
}

func (cw *ChunkWriter) doWrite(data []byte) {
	if cw.err == nil {
		n, err := cw.w.Write(data)
		if err != nil {
			cw.err = err
		} else if n != len(data) {
			cw.err = fmt.Errorf("Write less than expected data")
		}
	}
}

