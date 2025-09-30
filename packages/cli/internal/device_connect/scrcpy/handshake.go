package scrcpy

import (
	"encoding/binary"
	"fmt"
	"io"
)

// readVideoHeader reads codec (4 bytes), width (4), height (4) from scrcpy stream.
func readVideoHeader(r io.Reader) (codec uint32, width uint32, height uint32, err error) {
	buf := make([]byte, 12)
	if _, err = io.ReadFull(r, buf); err != nil {
		return 0, 0, 0, fmt.Errorf("readVideoHeader: %w", err)
	}
	codec = binary.BigEndian.Uint32(buf[0:4])
	width = binary.BigEndian.Uint32(buf[4:8])
	height = binary.BigEndian.Uint32(buf[8:12])
	return
}
