package stream

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/device"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/protocol"
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
	log.Printf("NewControlHandler: creating with conn=%v, dataChannel=%v, screen=%dx%d",
		conn != nil, dataChannel != nil, screenWidth, screenHeight)
	return &ControlHandler{
		conn:         conn,
		dataChannel:  dataChannel,
		screenWidth:  screenWidth,
		screenHeight: screenHeight,
	}
}

// HandleIncomingMessages handles control messages from WebRTC
func (ch *ControlHandler) HandleIncomingMessages() {
	log.Printf("HandleIncomingMessages called")
	if ch.dataChannel == nil {
		log.Printf("DataChannel is nil, cannot set up message handling")
		return
	}

	log.Printf("Setting up DataChannel message handling, DataChannel state: %s", ch.dataChannel.ReadyState())
	log.Printf("About to set OnMessage handler for DataChannel")

	ch.dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		log.Printf("DataChannel message received, data length: %d", len(msg.Data))
		log.Printf("DataChannel message data: %s", string(msg.Data))

		// Parse control message
		var message map[string]interface{}
		if err := json.Unmarshal(msg.Data, &message); err != nil {
			log.Printf("Failed to parse control message: %v", err)
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
				log.Printf("Unknown numeric control message type: %d", int(v))
				return
			}
		default:
			log.Printf("Control message missing or invalid type field: %v", message)
			return
		}

		log.Printf("Received control message: type=%s", msgType)

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
			log.Printf("Unknown control message type: %s", msgType)
		}
	})

	log.Printf("OnMessage handler set successfully for DataChannel")
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
					log.Printf("Failed to send pong response: %v", err)
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

	log.Printf("Key event: action=%s, keycode=%d, meta=%d", action, int(keycode), int(metaState))

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

	log.Printf("Touch event: action=%s, pos=(%.2f, %.2f), pressure=%.2f", action, x, y, pressure)

	// Send to device via control connection
	if ch.conn != nil {
		log.Printf("Sending touch event to device: action=%s, x=%.2f, y=%.2f", action, x, y)
		ch.SendTouchEventToDevice(action, x, y, pressure, int(pointerId))
	} else {
		log.Printf("Control connection is nil, cannot send touch event")
	}
}

// handleScrollEvent processes scroll events
func (ch *ControlHandler) handleScrollEvent(message map[string]interface{}) {
	x, _ := message["x"].(float64)
	y, _ := message["y"].(float64)
	hScroll, _ := message["hScroll"].(float64)
	vScroll, _ := message["vScroll"].(float64)

	log.Printf("Scroll event: pos=(%.2f, %.2f), scroll=(%.2f, %.2f)", x, y, hScroll, vScroll)

	// Send to device via control connection
	if ch.conn != nil {
		log.Printf("Sending scroll event to device: x=%.2f, y=%.2f, hScroll=%.2f, vScroll=%.2f", x, y, hScroll, vScroll)
		ch.SendScrollEventToDevice(x, y, hScroll, vScroll)
	} else {
		log.Printf("Control connection is nil, cannot send scroll event - this is expected during initial connection setup")
		// This is expected during initial connection setup, the connection will be updated later
		// We could queue the event here if needed, but for now just log it
	}
}

// handleResetVideo handles video reset requests (keyframe)
func (ch *ControlHandler) handleResetVideo(message map[string]interface{}) {
	log.Println("Reset video requested (keyframe)")
	// This would trigger a keyframe request
}

// handleClipboardGet handles clipboard get requests
func (ch *ControlHandler) handleClipboardGet(message map[string]interface{}) {
	log.Println("Clipboard get requested")
	// TODO: Implement clipboard get functionality
	// This would get clipboard content from Android device and send it back
}

// handleClipboardSet handles clipboard set requests
func (ch *ControlHandler) handleClipboardSet(message map[string]interface{}) {
	log.Println("Clipboard set requested")

	// Check if this is a JSON format message (new format) or binary format (old format)
	if textInterface, ok := message["text"]; ok {
		// JSON format: {"type": "clipboard_set", "text": "你好", "paste": true}
		text, ok := textInterface.(string)
		if !ok {
			log.Printf("Clipboard set message text field is not a string")
			return
		}

		paste := false
		if pasteInterface, ok := message["paste"]; ok {
			if pasteBool, ok := pasteInterface.(bool); ok {
				paste = pasteBool
			}
		}

		log.Printf("Clipboard set (JSON format): text='%s', paste=%v", text, paste)

		// Send clipboard data to Android device using scrcpy protocol
		ch.sendClipboardToDevice(text, paste)
		return
	}

	// Binary format: extract data from message
	dataInterface, ok := message["data"]
	if !ok {
		log.Printf("Clipboard set message missing both text and data fields")
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
		log.Printf("Clipboard set message data is not in expected format (array or map)")
		return
	}

	if len(data) < 13 {
		log.Printf("Clipboard set message data too short: %d bytes", len(data))
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
		log.Printf("Clipboard set message data incomplete: expected %d bytes, got %d", 13+textLength, len(data))
		return
	}

	text := string(data[13 : 13+textLength])
	log.Printf("Clipboard set (binary format): sequence=%d, paste=%d, text='%s'", sequence, pasteFlag, text)

	// Send clipboard data to Android device using scrcpy protocol
	ch.sendClipboardToDevice(text, pasteFlag == 1)
}

// sendClipboardToDevice sends clipboard data to Android device
func (ch *ControlHandler) sendClipboardToDevice(text string, paste bool) {
	if ch.conn == nil {
		log.Printf("No connection available for clipboard operation")
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
		log.Printf("ERROR: Buffer size mismatch! Expected: %d, Actual: %d", expectedSize, len(buffer))
	}

	// Create control message
	controlMsg := &device.ControlMessage{
		Type:     protocol.ControlMsgTypeSetClipboard,
		Sequence: 0,
		Data:     buffer,
	}

	// Debug: log the buffer content
	log.Printf("Clipboard buffer length: %d", len(buffer))
	if len(buffer) >= 20 {
		log.Printf("Clipboard buffer first 20 bytes: %v", buffer[:20])
		log.Printf("Clipboard buffer last 20 bytes: %v", buffer[len(buffer)-20:])
	} else {
		log.Printf("Clipboard buffer content: %v", buffer)
	}

	// Send to device
	ch.sendControlMessage(controlMsg)
	log.Printf("Clipboard data sent to device: text='%s', paste=%v", text, paste)
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
		log.Printf("SendScrollEventToDevice: control connection is nil")
		return
	}

	scrollEvent := &protocol.ScrollEvent{
		X:       x,
		Y:       y,
		HScroll: hScroll,
		VScroll: vScroll,
	}

	log.Printf("SendScrollEventToDevice: creating scroll event with screen=%dx%d", ch.screenWidth, ch.screenHeight)
	log.Printf("SendScrollEventToDevice: scroll event data: x=%.2f, y=%.2f, hScroll=%.2f, vScroll=%.2f", x, y, hScroll, vScroll)

	controlMsg := &device.ControlMessage{
		Type:     protocol.ControlMsgTypeInjectScrollEvent,
		Sequence: 0,
		Data:     protocol.EncodeScrollEvent(*scrollEvent, ch.screenWidth, ch.screenHeight),
	}

	log.Printf("SendScrollEventToDevice: encoded data length=%d", len(controlMsg.Data))
	ch.sendControlMessage(controlMsg)
}

// sendControlMessage sends a control message to the device
func (ch *ControlHandler) sendControlMessage(msg *device.ControlMessage) {
	if ch.conn == nil {
		log.Printf("Control connection is nil, cannot send message type %d", msg.Type)
		return
	}

	// Serialize control message according to scrcpy protocol
	// Format: [message_type][data]
	buf := make([]byte, 1+len(msg.Data))
	buf[0] = byte(msg.Type)
	copy(buf[1:], msg.Data)

	log.Printf("Sending control message to device: type=%d, data_len=%d", msg.Type, len(msg.Data))

	// Debug: log the actual data being sent to scrcpy server for clipboard messages
	if msg.Type == protocol.ControlMsgTypeSetClipboard {
		log.Printf("Clipboard message - Total length: %d", len(buf))
		if len(buf) >= 20 {
			log.Printf("First 20 bytes: %v", buf[:20])
			log.Printf("Last 20 bytes: %v", buf[len(buf)-20:])
		} else {
			log.Printf("All data: %v", buf)
		}
	}

	if _, err := ch.conn.Write(buf); err != nil {
		log.Printf("Failed to send control message: %v", err)
		// Mark connection as invalid to prevent further attempts
		ch.conn = nil
	} else {
		log.Printf("Control message sent successfully")
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
	log.Printf("Updated screen dimensions: %dx%d", width, height)
}

// UpdateConnection updates the control connection
func (ch *ControlHandler) UpdateConnection(conn net.Conn) {
	ch.conn = conn
	log.Printf("Updated control connection")
}

// UpdateDataChannel updates the DataChannel
func (ch *ControlHandler) UpdateDataChannel(dataChannel *webrtc.DataChannel) {
	ch.dataChannel = dataChannel
	log.Printf("Updated DataChannel")
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
				log.Printf("Control stream read error: %v", err)
			}
			break
		}

		if n > 0 {
			// Process control response from device
			log.Printf("Received control response: %d bytes", n)
		}
	}
}
