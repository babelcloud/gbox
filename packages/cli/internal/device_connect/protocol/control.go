package protocol

import (
	"encoding/binary"
	"fmt"
)

// Control message type aliases for compatibility
const (
	ControlMsgTypeInjectTouch  = ControlMsgTypeInjectTouchEvent
	ControlMsgTypeInjectScroll = ControlMsgTypeInjectScrollEvent

	// Touch action constants
	TouchActionDown = 0
	TouchActionUp   = 1
	TouchActionMove = 2
)

// KeyEvent represents a keyboard event
type KeyEvent struct {
	Action    string
	Keycode   int
	MetaState int
	Repeat    int
}

// TouchEvent represents a touch/mouse event
type TouchEvent struct {
	Action    string
	X         float64
	Y         float64
	Pressure  float64
	PointerID int
}

// ScrollEvent represents a scroll event
type ScrollEvent struct {
	X       float64
	Y       float64
	HScroll float64
	VScroll float64
}

// EncodeKeyEvent encodes a key event for scrcpy protocol (like scrcpy-proxy)
func EncodeKeyEvent(event KeyEvent) []byte {
	buf := make([]byte, 0, 16)

	// Action (1 byte)
	var actionCode byte
	if event.Action == "up" {
		actionCode = 1
	} else {
		actionCode = 0
	}
	buf = append(buf, actionCode)

	// Keycode (4 bytes)
	keyBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(keyBytes, uint32(event.Keycode))
	buf = append(buf, keyBytes...)

	// Repeat (4 bytes)
	repeatBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(repeatBytes, uint32(event.Repeat))
	buf = append(buf, repeatBytes...)

	// Meta state (4 bytes)
	metaBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(metaBytes, uint32(event.MetaState))
	buf = append(buf, metaBytes...)

	return buf
}

// EncodeTextEvent encodes a text event for scrcpy protocol
// Based on scrcpy official implementation in control_msg.c
// Note: This function returns only the message data (without message type)
func EncodeTextEvent(text string) []byte {
	textBytes := []byte(text)
	textLen := len(textBytes)

	// Message format: [length][text]
	buf := make([]byte, 4+textLen)

	// Text length (4 bytes, big endian)
	binary.BigEndian.PutUint32(buf[0:4], uint32(textLen))

	// Text content
	copy(buf[4:], textBytes)

	return buf
}

// EncodeTouchEvent encodes a touch event for scrcpy protocol (exactly like scrcpy-proxy)
func EncodeTouchEvent(event TouchEvent, screenWidth, screenHeight int) []byte {
	buf := make([]byte, 0, 32)

	// Action (1 byte)
	var actionCode byte
	switch event.Action {
	case "down":
		actionCode = 0 // ACTION_DOWN
	case "up":
		actionCode = 1 // ACTION_UP
	case "move":
		actionCode = 2 // ACTION_MOVE
	}
	buf = append(buf, actionCode)

	// Pointer ID (8 bytes) - always 0 like scrcpy-proxy
	ptrBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(ptrBytes, 0)
	buf = append(buf, ptrBytes...)

	// Position structure:
	// - x (4 bytes) - convert normalized coordinates to screen pixels
	// - y (4 bytes) - convert normalized coordinates to screen pixels
	// - screenWidth (2 bytes)
	// - screenHeight (2 bytes)
	posBytes := make([]byte, 12)
	screenX := uint32(event.X * float64(screenWidth))
	screenY := uint32(event.Y * float64(screenHeight))
	binary.BigEndian.PutUint32(posBytes[0:4], screenX)
	binary.BigEndian.PutUint32(posBytes[4:8], screenY)
	// Screen dimensions - use actual device screen size
	binary.BigEndian.PutUint16(posBytes[8:10], uint16(screenWidth))
	binary.BigEndian.PutUint16(posBytes[10:12], uint16(screenHeight))
	buf = append(buf, posBytes...)

	// Pressure (16-bit, 0xFFFF = 1.0) - always 1.0 like scrcpy-proxy
	pressureBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(pressureBytes, 0xFFFF) // 1.0 pressure
	buf = append(buf, pressureBytes...)

	// Action button (32-bit) - always 1 like scrcpy-proxy
	actionButtonBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(actionButtonBytes, 1) // Primary button
	buf = append(buf, actionButtonBytes...)

	// Buttons (32-bit) - always 1 like scrcpy-proxy
	buttonBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(buttonBytes, 1) // Primary button pressed
	buf = append(buf, buttonBytes...)

	return buf
}

// EncodeScrollEvent encodes a scroll event for scrcpy protocol
// Based on scrcpy official implementation in control_msg.c
// Note: This function returns only the message data (without message type)
func EncodeScrollEvent(event ScrollEvent, screenWidth, screenHeight int) []byte {
	buf := make([]byte, 20)

	// Position (exactly like scrcpy's write_position function)
	// Following the exact layout from app/src/control_msg.c:write_position()
	screenX := uint32(event.X * float64(screenWidth))
	screenY := uint32(event.Y * float64(screenHeight))

	// write_position writes to buf[0], which contains:
	// buf[0:4] = x coordinate (4 bytes, big endian)
	// buf[4:8] = y coordinate (4 bytes, big endian)
	// buf[8:10] = screen width (2 bytes, big endian)
	// buf[10:12] = screen height (2 bytes, big endian)
	binary.BigEndian.PutUint32(buf[0:4], screenX)
	binary.BigEndian.PutUint32(buf[4:8], screenY)
	binary.BigEndian.PutUint16(buf[8:10], uint16(screenWidth))
	binary.BigEndian.PutUint16(buf[10:12], uint16(screenHeight))

	// Scroll amounts - following scrcpy's normalization
	// Accept values in the range [-16, 16], normalize to [-1, 1]
	hScrollNorm := event.HScroll / 16.0
	if hScrollNorm > 1.0 {
		hScrollNorm = 1.0
	} else if hScrollNorm < -1.0 {
		hScrollNorm = -1.0
	}

	vScrollNorm := event.VScroll / 16.0
	if vScrollNorm > 1.0 {
		vScrollNorm = 1.0
	} else if vScrollNorm < -1.0 {
		vScrollNorm = -1.0
	}

	// Convert to 16-bit fixed point (exactly like scrcpy's sc_float_to_i16fp)
	// scrcpy uses: int32_t i = f * 0x1p15f; // 2^15
	// Then clamps to [0x7fff, -0x8000] range
	hScrollInt32 := int32(hScrollNorm * 32768) // 2^15
	vScrollInt32 := int32(vScrollNorm * 32768) // 2^15

	// Clamp to scrcpy's range: [0x7fff, -0x8000]
	if hScrollInt32 >= 0x7fff {
		hScrollInt32 = 0x7fff
	}
	if vScrollInt32 >= 0x7fff {
		vScrollInt32 = 0x7fff
	}

	// Convert to int16 (this handles the two's complement conversion automatically)
	hScroll := int16(hScrollInt32)
	vScroll := int16(vScrollInt32)

	// Convert to uint16 exactly like scrcpy does: (uint16_t) hscroll
	// This preserves the two's complement representation
	hScrollUint16 := uint16(hScroll)
	vScrollUint16 := uint16(vScroll)

	binary.BigEndian.PutUint16(buf[12:14], hScrollUint16)
	binary.BigEndian.PutUint16(buf[14:16], vScrollUint16)

	// Buttons (none)
	binary.BigEndian.PutUint32(buf[16:20], 0)

	// Debug logging
	fmt.Printf("EncodeScrollEvent: input=(%.2f, %.2f, %.2f, %.2f), screen=%dx%d\n",
		event.X, event.Y, event.HScroll, event.VScroll, screenWidth, screenHeight)
	fmt.Printf("EncodeScrollEvent: calculated position=(%d, %d), screen_size=(%d, %d)\n",
		screenX, screenY, screenWidth, screenHeight)
	fmt.Printf("EncodeScrollEvent: normalized=(%.2f, %.2f), int32=(%d, %d), int16=(%d, %d)\n",
		hScrollNorm, vScrollNorm, hScrollInt32, vScrollInt32, hScroll, vScroll)
	fmt.Printf("EncodeScrollEvent: uint16_values=(%d, %d), encoded buffer: %v\n",
		hScrollUint16, vScrollUint16, buf)

	// Detailed buffer analysis
	fmt.Printf("EncodeScrollEvent: buffer breakdown:\n")
	fmt.Printf("  [0:4] = %v (x=%d)\n", buf[0:4], binary.BigEndian.Uint32(buf[0:4]))
	fmt.Printf("  [4:8] = %v (y=%d)\n", buf[4:8], binary.BigEndian.Uint32(buf[4:8]))
	fmt.Printf("  [8:10] = %v (width=%d)\n", buf[8:10], binary.BigEndian.Uint16(buf[8:10]))
	fmt.Printf("  [10:12] = %v (height=%d)\n", buf[10:12], binary.BigEndian.Uint16(buf[10:12]))
	fmt.Printf("  [12:14] = %v (hScroll=%d)\n", buf[12:14], binary.BigEndian.Uint16(buf[12:14]))
	fmt.Printf("  [14:16] = %v (vScroll=%d)\n", buf[14:16], binary.BigEndian.Uint16(buf[14:16]))
	fmt.Printf("  [16:20] = %v (buttons=%d)\n", buf[16:20], binary.BigEndian.Uint32(buf[16:20]))

	// Compare with scrcpy test case format
	fmt.Printf("EncodeScrollEvent: scrcpy test comparison - expecting position=(%d, %d), screen=(%d, %d)\n",
		screenX, screenY, screenWidth, screenHeight)

	return buf
}
