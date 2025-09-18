package control

import (
	"net"

	"github.com/pion/webrtc/v4"
)

// HandlerInterface defines the interface for control handlers
// This allows different transport types to implement their own control handling
type HandlerInterface interface {
	// HandleIncomingMessages starts handling incoming control messages
	HandleIncomingMessages()

	// UpdateDataChannel updates the WebRTC data channel (for WebRTC transport)
	UpdateDataChannel(dataChannel *webrtc.DataChannel)

	// UpdateConnection updates the control connection (for non-WebRTC transports)
	UpdateConnection(conn net.Conn)

	// UpdateScreenDimensions updates the screen dimensions
	UpdateScreenDimensions(width, height int)

	// SetSource sets the scrcpy source for sending control messages
	SetSource(source interface{}) // Using interface{} to avoid circular imports

	// SendKeyFrameRequest sends a keyframe request
	SendKeyFrameRequest()

	// SendKeyEventToDevice sends a key event to the device
	SendKeyEventToDevice(action string, keycode, metaState, repeat int)

	// SendTouchEventToDevice sends a touch event to the device
	SendTouchEventToDevice(action string, x, y, pressure float64, pointerId int)

	// SendScrollEventToDevice sends a scroll event to the device
	SendScrollEventToDevice(x, y, hScroll, vScroll float64)
}
