package scrcpy

import (
	"io"
	"net"
	"time"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/protocol"
	"github.com/babelcloud/gbox/packages/cli/internal/util"
)

// drainControl starts a non-blocking drain on the control connection to avoid backpressure.
func drainControl(conn net.Conn) {
	if conn == nil {
		return
	}
	go func(c net.Conn) { io.Copy(io.Discard, c) }(conn)
}

// requestKeyframeAsync sends a keyframe reset request when controlConn is available.
// It is safe to call multiple times; if controlConn is nil it will wait briefly
// and retry once to avoid blocking the caller.
func (s *Source) requestKeyframeAsync() {
	go func() {
		logger := util.GetLogger()
		// small grace period if control is about to be set
		deadline := time.Now().Add(1500 * time.Millisecond)
		for time.Now().Before(deadline) {
			s.mu.Lock()
			conn := s.controlConn
			s.mu.Unlock()
			if conn != nil {
				// Serialize control message: [type][payload]
				buf := []byte{byte(protocol.ControlMsgTypeResetVideo)}
				if _, err := conn.Write(buf); err != nil {
					logger.Warn("Failed to send keyframe request", "error", err)
				} else {
					logger.Debug("Keyframe request sent")
				}
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		logger.Debug("Control not ready; skip keyframe request")
	}()
}
