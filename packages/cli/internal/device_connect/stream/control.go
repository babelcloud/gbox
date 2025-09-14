package stream

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/device"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/protocol"
	"github.com/babelcloud/gbox/packages/cli/internal/util"
	"github.com/pion/webrtc/v4"
)

// ControlHandler handles control stream and messages
type ControlHandler struct {
	conn         net.Conn
	dataChannel  *webrtc.DataChannel
	screenWidth  int
	screenHeight int
}

// NewControlHandler creates a new control stream handler
func NewControlHandler(conn net.Conn, dataChannel *webrtc.DataChannel, screenWidth, screenHeight int) *ControlHandler {
	logger := util.GetLogger()
	logger.Debug("Creating control handler",
		"conn_available", conn != nil,
		"datachannel_available", dataChannel != nil,
		"screen_width", screenWidth,
		"screen_height", screenHeight)
	return &ControlHandler{
		conn:         conn,
		dataChannel:  dataChannel,
		screenWidth:  screenWidth,
		screenHeight: screenHeight,
	}
}

// HandleIncomingMessages handles control messages from WebRTC
func (ch *ControlHandler) HandleIncomingMessages() {
	logger := util.GetLogger()
	logger.Debug("HandleIncomingMessages called")
	
	if ch.dataChannel == nil {
		logger.Error("DataChannel is nil, cannot set up message handling")
		return
	}

	logger.Debug("Setting up DataChannel message handling", 
		"state", ch.dataChannel.ReadyState())

	ch.dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
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
		case float64:
			// Convert numeric type to string for clipboard messages
			switch int(v) {
			case 8:
				msgType = "clipboard_get"
			case 9:
				msgType = "clipboard_set"
			default:
				logger := util.GetLogger()
				logger.Error("Unknown numeric control message type", "type", int(v))
				return
			}
		default:
			logger := util.GetLogger()
			logger.Error("Control message missing or invalid type field", "message", message)
			return
		}

		// Log ping messages at debug level
		if msgType == "ping" {
			logger := util.GetLogger()
			logger.Debug("Ping message received", 
				"data_length", len(msg.Data),
				"data", string(msg.Data))
		} else {
			// For non-ping messages, log appropriately
			logger := util.GetLogger()
			logger.Debug("DataChannel message received", 
				"data_length", len(msg.Data),
				"data", string(msg.Data))
			
			// Log touch/key events at debug level, others at info level
			if msgType == "touch" || msgType == "key" {
				logger.Debug("Received control message", "type", msgType)
			} else {
				logger.Info("Received control message", "type", msgType)
			}
		}

		switch msgType {
		case "ping":
			ch.handlePingMessage(message)
		case "key":
			ch.handleKeyEvent(message)
		case "touch":
			ch.handleTouchEvent(message)
		case "scroll":
			ch.handleScrollEvent(message)
		case "reset_video":
			ch.handleResetVideo(message)
		case "clipboard_get":
			ch.handleClipboardGet(message)
		case "clipboard_set":
			ch.handleClipboardSet(message)
		default:
			logger := util.GetLogger()
			logger.Warn("Unknown control message type", "type", msgType)
		}
	})

	logger.Debug("OnMessage handler set successfully for DataChannel")
}

// handlePingMessage handles ping/pong messages for connection health
func (ch *ControlHandler) handlePingMessage(message map[string]interface{}) {
	if id, hasId := message["id"].(string); hasId {
		pongResponse := map[string]interface{}{
			"type":      "pong",
			"id":        id,
			"timestamp": time.Now().UnixNano() / int64(time.Millisecond),
		}

		if pongData, err := json.Marshal(pongResponse); err == nil {
			if ch.dataChannel != nil && ch.dataChannel.ReadyState() == webrtc.DataChannelStateOpen {
				if err := ch.dataChannel.Send(pongData); err != nil {
					logger := util.GetLogger()
					logger.Error("Failed to send pong response", "error", err)
				} else {
					logger := util.GetLogger()
					logger.Debug("Pong response sent", "ping_id", id)
				}
			}
		}
	}
}

// handleKeyEvent processes keyboard events
func (ch *ControlHandler) handleKeyEvent(message map[string]interface{}) {
	action, _ := message["action"].(string)
	keycode, _ := message["keycode"].(float64)
	metaState, _ := message["metaState"].(float64)
	repeat, _ := message["repeat"].(float64)

	logger := util.GetLogger()
	logger.Debug("Key event", "action", action, "keycode", int(keycode), "meta_state", int(metaState))

	// Send to device via control connection
	if ch.conn != nil {
		ch.SendKeyEventToDevice(action, int(keycode), int(metaState), int(repeat))
	}
}

// handleTouchEvent processes touch/mouse events
func (ch *ControlHandler) handleTouchEvent(message map[string]interface{}) {
	action, _ := message["action"].(string)
	x, _ := message["x"].(float64)
	y, _ := message["y"].(float64)
	pressure, _ := message["pressure"].(float64)
	pointerId, _ := message["pointerId"].(float64)

	logger := util.GetLogger()
	logger.Debug("Touch event", "action", action, "x", x, "y", y, "pressure", pressure)

	// Send to device via control connection
	if ch.conn != nil {
		logger := util.GetLogger()
		logger.Debug("Sending touch event to device", "action", action, "x", x, "y", y)
		ch.SendTouchEventToDevice(action, x, y, pressure, int(pointerId))
	} else {
		logger := util.GetLogger()
		logger.Debug("Control connection is nil, cannot send touch event")
	}
}

// handleScrollEvent processes scroll events
func (ch *ControlHandler) handleScrollEvent(message map[string]interface{}) {
	x, _ := message["x"].(float64)
	y, _ := message["y"].(float64)
	hScroll, _ := message["hScroll"].(float64)
	vScroll, _ := message["vScroll"].(float64)

	logger := util.GetLogger()
	logger.Debug("Scroll event", "x", x, "y", y, "hScroll", hScroll, "vScroll", vScroll)

	// Send to device via control connection
	if ch.conn != nil {
		logger := util.GetLogger()
		logger.Debug("Sending scroll event to device", "x", x, "y", y, "hScroll", hScroll, "vScroll", vScroll)
		ch.SendScrollEventToDevice(x, y, hScroll, vScroll)
	} else {
		logger := util.GetLogger()
		logger.Debug("Control connection is nil, cannot send scroll event - this is expected during initial connection setup")
		// This is expected during initial connection setup, the connection will be updated later
		// We could queue the event here if needed, but for now just log it
	}
}

// handleResetVideo handles video reset requests (keyframe)
func (ch *ControlHandler) handleResetVideo(message map[string]interface{}) {
	logger := util.GetLogger()
	logger.Info("Reset video requested (keyframe)")
	// This would trigger a keyframe request
}

// handleClipboardGet handles clipboard get requests
func (ch *ControlHandler) handleClipboardGet(message map[string]interface{}) {
	logger := util.GetLogger()
	logger.Info("Clipboard get requested")
	// TODO: Implement clipboard get functionality
	// This would get clipboard content from Android device and send it back
}

// handleClipboardSet handles clipboard set requests
func (ch *ControlHandler) handleClipboardSet(message map[string]interface{}) {
	logger := util.GetLogger()
	logger.Info("Clipboard set requested")

	// Check if this is a JSON format message (new format) or binary format (old format)
	if textInterface, ok := message["text"]; ok {
		// JSON format: {"type": "clipboard_set", "text": "你好", "paste": true}
		text, ok := textInterface.(string)
		if !ok {
			logger := util.GetLogger()
			logger.Error("Clipboard set message text field is not a string")
			return
		}

		paste := false
		if pasteInterface, ok := message["paste"]; ok {
			if pasteBool, ok := pasteInterface.(bool); ok {
				paste = pasteBool
			}
		}

		logger := util.GetLogger()
		logger.Debug("Clipboard set (JSON format)", "text", text, "paste", paste)

		// Send clipboard data to Android device using scrcpy protocol
		ch.sendClipboardToDevice(text, paste)
		return
	}

	// Binary format: extract data from message
	dataInterface, ok := message["data"]
	if !ok {
		logger := util.GetLogger()
		logger.Error("Clipboard set message missing both text and data fields")
		return
	}

	// Convert data to byte array - handle both array and map formats
	var data []byte

	// Try array format first (new format)
	if dataArray, ok := dataInterface.([]interface{}); ok {
		for _, val := range dataArray {
			if byteVal, ok := val.(float64); ok {
				data = append(data, byte(byteVal))
			}
		}
	} else if dataMap, ok := dataInterface.(map[string]interface{}); ok {
		// Fallback to map format (old format)
		for i := 0; i < len(dataMap); i++ {
			if val, exists := dataMap[fmt.Sprintf("%d", i)]; exists {
				if byteVal, ok := val.(float64); ok {
					data = append(data, byte(byteVal))
				}
			}
		}
	} else {
		logger := util.GetLogger()
		logger.Error("Clipboard set message data is not in expected format (array or map)")
		return
	}

	if len(data) < 13 {
		logger := util.GetLogger()
		logger.Error("Clipboard set message data too short", "bytes", len(data))
		return
	}

	// Parse clipboard data according to scrcpy protocol
	// [Sequence (8 bytes)][Paste flag (1 byte)][Text length (4 bytes, big endian)][Text data]
	// Note: Type is handled separately, not in data
	sequence := int64(data[0])<<56 | int64(data[1])<<48 | int64(data[2])<<40 | int64(data[3])<<32 |
		int64(data[4])<<24 | int64(data[5])<<16 | int64(data[6])<<8 | int64(data[7])
	pasteFlag := data[8]
	textLength := int(data[9])<<24 | int(data[10])<<16 | int(data[11])<<8 | int(data[12])

	if len(data) < 13+textLength {
		logger := util.GetLogger()
		logger.Error("Clipboard set message data incomplete", "expected", 13+textLength, "got", len(data))
		return
	}

	text := string(data[13 : 13+textLength])
	logger.Debug("Clipboard set (binary format)", "sequence", sequence, "paste", pasteFlag, "text", text)

	// Send clipboard data to Android device using scrcpy protocol
	ch.sendClipboardToDevice(text, pasteFlag == 1)
}

// sendClipboardToDevice sends clipboard data to Android device
func (ch *ControlHandler) sendClipboardToDevice(text string, paste bool) {
	if ch.conn == nil {
		logger := util.GetLogger()
		logger.Error("No connection available for clipboard operation")
		return
	}

	// Create clipboard control message according to scrcpy protocol
	// Format: [Sequence (8 bytes)][Paste flag (1 byte)][Text length (4 bytes)][Text data]
	// Note: Type is handled by ControlMessage.Type field, not in buffer
	textBytes := []byte(text)
	textLength := len(textBytes)
	buffer := make([]byte, 8+1+4+textLength)
	offset := 0

	// Sequence (8 bytes, big endian) - use 0 for now
	buffer[offset] = 0
	buffer[offset+1] = 0
	buffer[offset+2] = 0
	buffer[offset+3] = 0
	buffer[offset+4] = 0
	buffer[offset+5] = 0
	buffer[offset+6] = 0
	buffer[offset+7] = 0
	offset += 8

	// Paste flag (1 byte) - 0 for just set, 1 for set and paste
	if paste {
		buffer[offset] = 1
	} else {
		buffer[offset] = 0
	}
	offset++

	// Text length (4 bytes, big endian) - use actual text length
	buffer[offset] = byte(textLength >> 24)
	buffer[offset+1] = byte(textLength >> 16)
	buffer[offset+2] = byte(textLength >> 8)
	buffer[offset+3] = byte(textLength)
	offset += 4

	// Text data
	copy(buffer[offset:], textBytes)

	// Debug: verify buffer size matches expected size
	expectedSize := 8 + 1 + 4 + textLength
	if len(buffer) != expectedSize {
		logger := util.GetLogger()
		logger.Error("Buffer size mismatch", "expected", expectedSize, "actual", len(buffer))
	}

	// Create control message
	controlMsg := &device.ControlMessage{
		Type:     protocol.ControlMsgTypeSetClipboard,
		Sequence: 0,
		Data:     buffer,
	}

	// Debug: log the buffer content
	logger := util.GetLogger()
	logger.Debug("Clipboard buffer", "length", len(buffer))
	if len(buffer) >= 20 {
		logger.Debug("Clipboard buffer details", "first_20_bytes", buffer[:20], "last_20_bytes", buffer[len(buffer)-20:])
	} else {
		logger.Debug("Clipboard buffer content", "buffer", buffer)
	}

	// Send to device
	ch.sendControlMessage(controlMsg)
	logger.Info("Clipboard data sent to device", "text", text, "paste", paste)
}

// SendKeyEventToDevice sends key event to Android device using protocol package
func (ch *ControlHandler) SendKeyEventToDevice(action string, keycode, metaState, repeat int) {
	if ch.conn == nil {
		return
	}

	keyEvent := &protocol.KeyEvent{
		Action:    action,
		Keycode:   keycode,
		Repeat:    repeat,
		MetaState: metaState,
	}

	controlMsg := &device.ControlMessage{
		Type:     protocol.ControlMsgTypeInjectKeycode,
		Sequence: 0,
		Data:     protocol.EncodeKeyEvent(*keyEvent),
	}

	ch.sendControlMessage(controlMsg)
}

// SendTouchEventToDevice sends touch event to Android device using protocol package
func (ch *ControlHandler) SendTouchEventToDevice(action string, x, y, pressure float64, pointerId int) {
	if ch.conn == nil {
		return
	}

	// Check if touch point is within screen bounds
	if x < 0 || x > float64(ch.screenWidth) || y < 0 || y > float64(ch.screenHeight) {
		logger := util.GetLogger()
		logger.Debug("Touch event outside screen bounds, ignoring", 
			"x", x, "y", y, "screen_width", ch.screenWidth, "screen_height", ch.screenHeight)
		return
	}

	touchEvent := &protocol.TouchEvent{
		Action:    action,
		X:         x,
		Y:         y,
		PointerID: pointerId,
		Pressure:  pressure,
	}

	controlMsg := &device.ControlMessage{
		Type:     protocol.ControlMsgTypeInjectTouchEvent,
		Sequence: 0,
		Data:     protocol.EncodeTouchEvent(*touchEvent, ch.screenWidth, ch.screenHeight),
	}

	ch.sendControlMessage(controlMsg)
}

// SendScrollEventToDevice sends scroll event to Android device using protocol package
func (ch *ControlHandler) SendScrollEventToDevice(x, y, hScroll, vScroll float64) {
	if ch.conn == nil {
		logger := util.GetLogger()
		logger.Debug("SendScrollEventToDevice: control connection is nil")
		return
	}

	scrollEvent := &protocol.ScrollEvent{
		X:       x,
		Y:       y,
		HScroll: hScroll,
		VScroll: vScroll,
	}

	logger := util.GetLogger()
	logger.Debug("SendScrollEventToDevice: creating scroll event", "screen_width", ch.screenWidth, "screen_height", ch.screenHeight)
	logger.Debug("SendScrollEventToDevice: scroll event data", "x", x, "y", y, "hScroll", hScroll, "vScroll", vScroll)

	controlMsg := &device.ControlMessage{
		Type:     protocol.ControlMsgTypeInjectScrollEvent,
		Sequence: 0,
		Data:     protocol.EncodeScrollEvent(*scrollEvent, ch.screenWidth, ch.screenHeight),
	}

	logger.Debug("SendScrollEventToDevice: encoded data", "length", len(controlMsg.Data))
	ch.sendControlMessage(controlMsg)
}

// sendControlMessage sends a control message to the device
func (ch *ControlHandler) sendControlMessage(msg *device.ControlMessage) {
	if ch.conn == nil {
		logger := util.GetLogger()
		logger.Error("Control connection is nil, cannot send message", "type", msg.Type)
		return
	}

	// Serialize control message according to scrcpy protocol
	// Format: [message_type][data]
	buf := make([]byte, 1+len(msg.Data))
	buf[0] = byte(msg.Type)
	copy(buf[1:], msg.Data)

	logger := util.GetLogger()
	logger.Debug("Sending control message to device", "type", msg.Type, "data_len", len(msg.Data))

	// Debug: log the actual data being sent to scrcpy server for clipboard messages
	if msg.Type == protocol.ControlMsgTypeSetClipboard {
		logger := util.GetLogger()
		logger.Debug("Clipboard message details", "total_length", len(buf))
		if len(buf) >= 20 {
			logger.Debug("Clipboard message data", "first_20_bytes", buf[:20], "last_20_bytes", buf[len(buf)-20:])
		} else {
			logger.Debug("Clipboard message all data", "data", buf)
		}
	}

	if _, err := ch.conn.Write(buf); err != nil {
		logger := util.GetLogger()
		logger.Error("Failed to send control message", "error", err)
		// Mark connection as invalid to prevent further attempts
		ch.conn = nil
	} else {
		logger := util.GetLogger()
		logger.Debug("Control message sent successfully")
	}
}

// SendKeyFrameRequest sends a keyframe request to the device
func (ch *ControlHandler) SendKeyFrameRequest() {
	keyframeRequest := &device.ControlMessage{
		Type:     protocol.ControlMsgTypeResetVideo,
		Sequence: 0,
		Data:     []byte{},
	}
	ch.sendControlMessage(keyframeRequest)
}

// UpdateScreenDimensions updates the screen dimensions for coordinate conversion
func (ch *ControlHandler) UpdateScreenDimensions(width, height int) {
	ch.screenWidth = width
	ch.screenHeight = height
	if util.IsVerbose() {
		logger := util.GetLogger()
		logger.Debug("Updated screen dimensions", "width", width, "height", height)
	}
}

// UpdateConnection updates the control connection
func (ch *ControlHandler) UpdateConnection(conn net.Conn) {
	ch.conn = conn
	if util.IsVerbose() {
		logger := util.GetLogger()
		logger.Debug("Updated control connection")
	}
}

// UpdateDataChannel updates the DataChannel
func (ch *ControlHandler) UpdateDataChannel(dataChannel *webrtc.DataChannel) {
	ch.dataChannel = dataChannel
	if util.IsVerbose() {
		logger := util.GetLogger()
		logger.Debug("Updated DataChannel")
	}
}

// HandleOutgoingMessages handles messages from device to WebRTC
func (ch *ControlHandler) HandleOutgoingMessages() {
	if ch.conn == nil {
		return
	}

	// Read clipboard or other events from device
	buffer := make([]byte, 4096)
	for {
		n, err := ch.conn.Read(buffer)
		if err != nil {
			if err != io.EOF {
				logger := util.GetLogger()
				logger.Error("Control stream read error", "error", err)
			}
			break
		}

		if n > 0 {
			// Process control response from device
			logger := util.GetLogger()
			logger.Debug("Received control response", "bytes", n)
		}
	}
}
