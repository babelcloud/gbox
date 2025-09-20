package control

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/core"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/protocol"
	"github.com/babelcloud/gbox/packages/cli/internal/util"
	"github.com/pion/webrtc/v4"
)

// Handler handles control stream and messages
type Handler struct {
	conn         net.Conn
	dataChannel  *webrtc.DataChannel
	screenWidth  int
	screenHeight int
	source       core.Source // Reference to scrcpy source for sending control messages
}

// NewHandler creates a new control stream handler
func NewHandler(conn net.Conn, dataChannel *webrtc.DataChannel, screenWidth, screenHeight int) *Handler {
	logger := util.GetLogger()
	logger.Debug("Creating control handler",
		"conn_available", conn != nil,
		"datachannel_available", dataChannel != nil,
		"screen_width", screenWidth,
		"screen_height", screenHeight)
	return &Handler{
		conn:         conn,
		dataChannel:  dataChannel,
		screenWidth:  screenWidth,
		screenHeight: screenHeight,
		source:       nil, // Will be set later
	}
}

// HandleIncomingMessages handles control messages from WebRTC
func (h *Handler) HandleIncomingMessages() {
	logger := util.GetLogger()
	logger.Debug("HandleIncomingMessages called")

	if h.dataChannel == nil {
		logger.Error("DataChannel is nil, cannot set up message handling")
		return
	}

	logger.Debug("Setting up DataChannel message handling",
		"state", h.dataChannel.ReadyState())

	h.dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		// Parse control message first to determine if it's a ping
		var message map[string]interface{}
		if err := json.Unmarshal(msg.Data, &message); err != nil {
			logger := util.GetLogger()
			logger.Error("Failed to parse control message", "error", err)
			return
		}

		// Handle both string and numeric type fields
		var msgType string
		switch v := message["type"].(type) {
		case string:
			msgType = v
		case float64: // JSON numbers are float64
			msgType = fmt.Sprintf("%d", int(v))
		default:
			logger.Error("Unknown message type format", "type", v)
			return
		}

		logger.Debug("Received control message", "type", msgType)

		switch msgType {
		case "ping":
			// Respond to ping to keep connection alive
			h.handlePingMessage(message)
		case "touch":
			// Handle touch events (mouse events from frontend)
			h.handleTouchEvent(message)
		case "mousemove", "mousedown", "mouseup":
			// Legacy mouse event support
			h.handleTouchEvent(message)
		case "scroll":
			h.handleScrollEvent(message)
		case "key":
			// Handle key events
			h.handleKeyEvent(message)
		case "keydown", "keyup":
			// Legacy key event support
			h.handleKeyEvent(message)
		case "inject_text":
			h.handleInjectText(message)
		case "clipboard_set", "set_clipboard":
			h.handleClipboardSet(message)
		case "clipboard_get", "get_clipboard":
			h.handleClipboardGet(message)
		case "request_keyframe":
			h.SendKeyFrameRequest()
		case "reset_video":
			h.handleResetVideo(message)
		case "power_on":
			h.SendKeyEventToDevice("down", 26, 0, 0) // Power key
		case "power_off":
			h.SendKeyEventToDevice("down", 26, 0, 0) // Power key
		case "rotate_device":
			h.SendKeyEventToDevice("down", 82, 0, 0) // Menu key
		case "expand_notification_panel":
			h.SendKeyEventToDevice("down", 82, 0, 0) // Menu key
		case "expand_settings_panel":
			h.SendKeyEventToDevice("down", 82, 0, 0) // Menu key
		case "collapse_panels":
			h.SendKeyEventToDevice("down", 4, 0, 0) // Back key
		case "back_or_screen_on":
			h.SendKeyEventToDevice("down", 4, 0, 0) // Back key
		case "home":
			h.SendKeyEventToDevice("down", 3, 0, 0) // Home key
		case "app_switch":
			h.SendKeyEventToDevice("down", 187, 0, 0) // App switch key
		case "menu":
			h.SendKeyEventToDevice("down", 82, 0, 0) // Menu key
		case "volume_up":
			h.SendKeyEventToDevice("down", 24, 0, 0) // Volume up key
		case "volume_down":
			h.SendKeyEventToDevice("down", 25, 0, 0) // Volume down key
		default:
			logger.Warn("Unknown control message type", "type", msgType)
		}
	})
}

// UpdateDataChannel updates the DataChannel for the control handler
func (h *Handler) UpdateDataChannel(dataChannel *webrtc.DataChannel) {
	h.dataChannel = dataChannel
}

// UpdateConnection updates the control connection
func (h *Handler) UpdateConnection(conn net.Conn) {
	h.conn = conn
}

// UpdateScreenDimensions updates the screen dimensions
func (h *Handler) UpdateScreenDimensions(width, height int) {
	logger := util.GetLogger()
	logger.Info("Updating screen dimensions", "width", width, "height", height)
	h.screenWidth = width
	h.screenHeight = height
}

// SetSource sets the scrcpy source for sending control messages
func (h *Handler) SetSource(source core.Source) {
	h.source = source
}

// sendControlMessage sends a control message to the device
func (h *Handler) sendControlMessage(msg *protocol.ControlMessage) {
	logger := util.GetLogger()

	// Try to send via scrcpy source first (preferred for WebRTC)
	if h.source != nil {
		coreMsg := core.ControlMessage{
			Type: int32(msg.Type),
			Data: msg.Data,
		}
		if err := h.source.SendControl(coreMsg); err != nil {
			logger.Error("Failed to send control message via source", "error", err)
			return
		}
		logger.Debug("Control message sent via source")
		return
	}

	// Fallback to direct connection (legacy)
	if h.conn == nil {
		logger.Error("Control connection is nil, cannot send message")
		return
	}

	data := protocol.SerializeControlMessage(msg)
	if _, err := h.conn.Write(data); err != nil {
		logger.Error("Failed to send control message", "error", err)
	}
}
