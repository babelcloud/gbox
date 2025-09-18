package mse

import (
	"net"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/core"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/transport/control"
)

// ControlHandler wraps the shared control handler for MSE transport
type ControlHandler struct {
	*control.Handler
}

// NewControlHandler creates a new MSE control handler
func NewControlHandler(conn net.Conn, screenWidth, screenHeight int) *ControlHandler {
	// Create shared control handler with connection (no DataChannel for MSE)
	sharedHandler := control.NewHandler(conn, nil, screenWidth, screenHeight)
	
	return &ControlHandler{
		Handler: sharedHandler,
	}
}

// SetSource sets the scrcpy source for sending control messages
func (h *ControlHandler) SetSource(source core.Source) {
	h.Handler.SetSource(source)
}

// UpdateConnection updates the control connection
func (h *ControlHandler) UpdateConnection(conn net.Conn) {
	h.Handler.UpdateConnection(conn)
}

// UpdateScreenDimensions updates the screen dimensions
func (h *ControlHandler) UpdateScreenDimensions(width, height int) {
	h.Handler.UpdateScreenDimensions(width, height)
}

// HandleIncomingMessages handles incoming control messages
func (h *ControlHandler) HandleIncomingMessages() {
	// For MSE, we don't have WebRTC DataChannel, so we handle messages differently
	// This could be implemented to read from the connection directly
	// For now, we'll use the shared handler's logic
	h.Handler.HandleIncomingMessages()
}
