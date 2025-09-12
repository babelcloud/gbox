package webrtc

import (
	"encoding/hex"
	"io"
	"log"
)

// DebugReader wraps an io.Reader to log what's being read
type DebugReader struct {
	reader io.Reader
	name   string
}

func NewDebugReader(reader io.Reader, name string) *DebugReader {
	return &DebugReader{reader: reader, name: name}
}

func (d *DebugReader) Read(p []byte) (n int, err error) {
	n, err = d.reader.Read(p)
	if n > 0 {
		log.Printf("[%s] Read %d bytes: %s", d.name, n, hex.EncodeToString(p[:n]))
	}
	return n, err
}