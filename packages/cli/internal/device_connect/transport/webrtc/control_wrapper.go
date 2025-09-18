package webrtc

import (
	"net"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/core"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/transport/control"
	"github.com/pion/webrtc/v4"
)

// ControlHandlerWrapper wraps the shared control handler for WebRTC transport
type ControlHandlerWrapper struct {
	*control.Handler
}

// NewControlHandlerWrapper creates a new WebRTC control handler wrapper
func NewControlHandlerWrapper(dataChannel *webrtc.DataChannel, screenWidth, screenHeight int) *ControlHandlerWrapper {
	// Create shared control handler with DataChannel (no connection for WebRTC)
	sharedHandler := control.NewHandler(nil, dataChannel, screenWidth, screenHeight)
	
	return &ControlHandlerWrapper{
		Handler: sharedHandler,
	}
}

// SetSource sets the scrcpy source for sending control messages
func (h *ControlHandlerWrapper) SetSource(source core.Source) {
	h.Handler.SetSource(source)
}

// UpdateDataChannel updates the WebRTC data channel
func (h *ControlHandlerWrapper) UpdateDataChannel(dataChannel *webrtc.DataChannel) {
	h.Handler.UpdateDataChannel(dataChannel)
}

// UpdateConnection updates the control connection (not used for WebRTC)
func (h *ControlHandlerWrapper) UpdateConnection(conn net.Conn) {
	h.Handler.UpdateConnection(conn)
}

// UpdateScreenDimensions updates the screen dimensions
func (h *ControlHandlerWrapper) UpdateScreenDimensions(width, height int) {
	h.Handler.UpdateScreenDimensions(width, height)
}

// HandleIncomingMessages handles incoming control messages
func (h *ControlHandlerWrapper) HandleIncomingMessages() {
	h.Handler.HandleIncomingMessages()
}
