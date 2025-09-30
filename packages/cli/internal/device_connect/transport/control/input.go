package control

import (
	"time"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/protocol"
	"github.com/babelcloud/gbox/packages/cli/internal/util"
	"github.com/pion/webrtc/v4"
)

// handlePingMessage handles ping messages to keep connection alive
func (h *Handler) handlePingMessage(message map[string]interface{}) {
	logger := util.GetLogger()
	logger.Debug("Received ping message")

	// Send pong response
	if h.dataChannel != nil && h.dataChannel.ReadyState() == webrtc.DataChannelStateOpen {
		if err := h.dataChannel.SendText("pong"); err != nil {
			logger.Error("Failed to send pong", "error", err)
		}
	}
}

// handleKeyEvent handles keyboard events
func (h *Handler) handleKeyEvent(message map[string]interface{}) {
	action, ok := message["action"].(string)
	if !ok {
		util.GetLogger().Error("Invalid key action")
		return
	}

	keycode, ok := message["keycode"].(float64)
	if !ok {
		util.GetLogger().Error("Invalid keycode")
		return
	}

	metaState, ok := message["metaState"].(float64)
	if !ok {
		metaState = 0
	}

	repeat, ok := message["repeat"].(float64)
	if !ok {
		repeat = 0
	}

	h.SendKeyEventToDevice(action, int(keycode), int(metaState), int(repeat))
}

// handleTouchEvent handles touch events (mouse events)
func (h *Handler) handleTouchEvent(message map[string]interface{}) {
	// Handle both string and numeric action values
	var actionStr string
	if action, ok := message["action"].(string); ok {
		// Frontend sends string action
		actionStr = action
	} else if action, ok := message["action"].(float64); ok {
		// Legacy numeric action support
		switch int(action) {
		case 0:
			actionStr = "down"
		case 1:
			actionStr = "up"
		case 2:
			actionStr = "move"
		default:
			actionStr = "move"
		}
	} else {
		util.GetLogger().Error("Invalid touch action")
		return
	}

	x, ok := message["x"].(float64)
	if !ok {
		util.GetLogger().Error("Invalid touch x coordinate")
		return
	}

	y, ok := message["y"].(float64)
	if !ok {
		util.GetLogger().Error("Invalid touch y coordinate")
		return
	}

	pressure, ok := message["pressure"].(float64)
	if !ok {
		pressure = 1.0
	}

	pointerId, ok := message["pointerId"].(float64)
	if !ok {
		pointerId = 0
	}

	h.SendTouchEventToDevice(actionStr, x, y, pressure, int(pointerId))
}

// handleScrollEvent handles scroll events
func (h *Handler) handleScrollEvent(message map[string]interface{}) {
	x, ok := message["x"].(float64)
	if !ok {
		util.GetLogger().Error("Invalid scroll x coordinate")
		return
	}

	y, ok := message["y"].(float64)
	if !ok {
		util.GetLogger().Error("Invalid scroll y coordinate")
		return
	}

	hScroll, ok := message["hScroll"].(float64)
	if !ok {
		hScroll = 0
	}

	vScroll, ok := message["vScroll"].(float64)
	if !ok {
		vScroll = 0
	}

	h.SendScrollEventToDevice(x, y, hScroll, vScroll)
}

// handleInjectText handles text injection
func (h *Handler) handleInjectText(message map[string]interface{}) {
	text, ok := message["text"].(string)
	if !ok {
		util.GetLogger().Error("Invalid text for injection")
		return
	}

	// Send text input events
	for _, char := range text {
		// Convert character to keycode (simplified)
		keycode := int(char)
		if keycode > 127 {
			keycode = 0 // Unknown character
		}

		// Send key down
		h.SendKeyEventToDevice("down", keycode, 0, 0)
		time.Sleep(10 * time.Millisecond)
		// Send key up
		h.SendKeyEventToDevice("up", keycode, 0, 0)
		time.Sleep(10 * time.Millisecond)
	}
}

// handleResetVideo handles video reset requests
func (h *Handler) handleResetVideo(message map[string]interface{}) {
	util.GetLogger().Info("Video reset requested")
	// Request a keyframe
	h.SendKeyFrameRequest()
}

// SendKeyEventToDevice sends a key event to the device
func (h *Handler) SendKeyEventToDevice(action string, keycode, metaState, repeat int) {
	logger := util.GetLogger()
	logger.Debug("Sending key event", "action", action, "keycode", keycode, "metaState", metaState, "repeat", repeat)

	// Create key event
	keyEvent := protocol.KeyEvent{
		Action:    action,
		Keycode:   keycode,
		MetaState: metaState,
		Repeat:    repeat,
	}

	// Encode key event
	data := protocol.EncodeKeyEvent(keyEvent)

	// Create control message
	msg := &protocol.ControlMessage{
		Type: protocol.ControlMsgTypeInjectKeycode,
		Data: data,
	}

	h.sendControlMessage(msg)
}

// SendTouchEventToDevice sends a touch event to the device
func (h *Handler) SendTouchEventToDevice(action string, x, y, pressure float64, pointerId int) {
	logger := util.GetLogger()
	logger.Debug("Sending touch event", "action", action, "x", x, "y", y, "pressure", pressure, "pointerId", pointerId, "screenWidth", h.screenWidth, "screenHeight", h.screenHeight)

	// Create touch event
	touchEvent := protocol.TouchEvent{
		Action:    action,
		X:         x,
		Y:         y,
		Pressure:  pressure,
		PointerID: pointerId,
	}

	// Encode touch event
	data := protocol.EncodeTouchEvent(touchEvent, h.screenWidth, h.screenHeight)

	// Create control message
	msg := &protocol.ControlMessage{
		Type: protocol.ControlMsgTypeInjectTouchEvent,
		Data: data,
	}

	h.sendControlMessage(msg)
}

// SendScrollEventToDevice sends a scroll event to the device
func (h *Handler) SendScrollEventToDevice(x, y, hScroll, vScroll float64) {
	logger := util.GetLogger()
	logger.Debug("Sending scroll event", "x", x, "y", y, "hScroll", hScroll, "vScroll", vScroll)

	// Create scroll event
	scrollEvent := protocol.ScrollEvent{
		X:       x,
		Y:       y,
		HScroll: hScroll,
		VScroll: vScroll,
	}

	// Encode scroll event
	data := protocol.EncodeScrollEvent(scrollEvent, h.screenWidth, h.screenHeight)

	// Create control message
	msg := &protocol.ControlMessage{
		Type: protocol.ControlMsgTypeInjectScrollEvent,
		Data: data,
	}

	h.sendControlMessage(msg)
}

// SendKeyFrameRequest sends a keyframe request to the device
func (h *Handler) SendKeyFrameRequest() {
	logger := util.GetLogger()
	logger.Debug("Sending keyframe request")

	// Create control message for video reset (which requests keyframe)
	msg := &protocol.ControlMessage{
		Type: protocol.ControlMsgTypeResetVideo,
		Data: []byte{}, // Empty data for reset video
	}

	h.sendControlMessage(msg)
}
