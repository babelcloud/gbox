package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
	"strings"
)

// Scrcpy packet header size
const PacketHeaderSize = 12

// Packet flags
const (
	PacketFlagConfig   = uint64(1) << 63
	PacketFlagKeyFrame = uint64(1) << 62
	PacketPTSMask      = PacketFlagKeyFrame - 1
)

// Codec IDs
const (
	CodecIDH264     = uint32(0x68323634) // "h264" in ASCII
	CodecIDH265     = uint32(0x68323635) // "h265" in ASCII
	CodecIDAV1      = uint32(0x00617631) // "av1" in ASCII
	CodecIDOPUS     = uint32(0x6f707573) // "opus" in ASCII
	CodecIDAAC      = uint32(0x00616163) // "aac" in ASCII
	CodecIDFLAC     = uint32(0x666c6163) // "flac" in ASCII
	CodecIDRAW      = uint32(0x00726177) // "raw" in ASCII
	CodecIDDisabled = uint32(0x80000000) // Audio/Video disabled
)

// Video packet structure
type VideoPacket struct {
	PTS        uint64
	PacketSize uint32
	Data       []byte
	IsKeyFrame bool
	IsConfig   bool
}

// Audio packet structure
type AudioPacket struct {
	PTS        uint64
	PacketSize uint32
	Data       []byte
}

// Device metadata
type DeviceMeta struct {
	DeviceName string
	Width      uint32
	Height     uint32
}

// Control message types
const (
	ControlMsgTypeInjectKeycode      = 0
	ControlMsgTypeInjectText         = 1
	ControlMsgTypeInjectTouchEvent   = 2
	ControlMsgTypeInjectScrollEvent  = 3
	ControlMsgTypeBackOrScreenOn     = 4
	ControlMsgTypeExpandNotification = 5
	ControlMsgTypeExpandSettings     = 6
	ControlMsgTypeCollapsePanels     = 7
	ControlMsgTypeGetClipboard       = 8
	ControlMsgTypeSetClipboard       = 9
	ControlMsgTypeSetDisplayPower    = 10
	ControlMsgTypeRotateDevice       = 11
	ControlMsgTypeUhidCreate         = 12
	ControlMsgTypeUhidInput          = 13
	ControlMsgTypeUhidDestroy        = 14
	ControlMsgTypeOpenHardKeyboard   = 15
	ControlMsgTypeStartApp           = 16
	ControlMsgTypeResetVideo         = 17
)

// Control message structure
type ControlMessage struct {
	Type     uint8
	Sequence uint64
	Data     []byte
}

// ScrcpyTouchEvent represents internal touch event for scrcpy protocol
type ScrcpyTouchEvent struct {
	Action    uint8
	PointerID uint64
	Position  Position
	Pressure  float32
	Buttons   uint32
}

// ScrcpyKeyEvent represents internal key event for scrcpy protocol
type ScrcpyKeyEvent struct {
	Action    uint8
	Keycode   uint32
	Repeat    uint32
	MetaState uint32
}

// Position structure
type Position struct {
	X float32
	Y float32
}

// Read video packet from stream
func ReadVideoPacket(reader io.Reader) (*VideoPacket, error) {
	header := make([]byte, PacketHeaderSize)
	n, err := io.ReadFull(reader, header)
	if err != nil {
		if n == 0 && err == io.EOF {
			return nil, io.EOF
		}
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	ptsFlags := binary.BigEndian.Uint64(header[0:8])
	packetSize := binary.BigEndian.Uint32(header[8:12])

	if packetSize == 0 {
		return nil, fmt.Errorf("invalid packet size: 0")
	}
	
	// Sanity check packet size
	if packetSize > 10*1024*1024 { // 10MB max
		return nil, fmt.Errorf("packet size too large: %d", packetSize)
	}

	data := make([]byte, packetSize)
	if _, err := io.ReadFull(reader, data); err != nil {
		return nil, fmt.Errorf("failed to read packet data: %w", err)
	}

	return &VideoPacket{
		PTS:        ptsFlags & PacketPTSMask,
		PacketSize: packetSize,
		Data:       data,
		IsKeyFrame: (ptsFlags & PacketFlagKeyFrame) != 0,
		IsConfig:   (ptsFlags & PacketFlagConfig) != 0,
	}, nil
}

// Read audio packet from stream
func ReadAudioPacket(reader io.Reader) (*AudioPacket, error) {
	header := make([]byte, PacketHeaderSize)
	n, err := io.ReadFull(reader, header)
	if err != nil {
		if n == 0 && err == io.EOF {
			return nil, io.EOF
		}
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	ptsFlags := binary.BigEndian.Uint64(header[0:8])
	packetSize := binary.BigEndian.Uint32(header[8:12])

	if packetSize == 0 {
		return nil, fmt.Errorf("invalid packet size: 0")
	}
	
	// Sanity check packet size
	if packetSize > 1*1024*1024 { // 1MB max for audio
		return nil, fmt.Errorf("packet size too large: %d", packetSize)
	}

	data := make([]byte, packetSize)
	if _, err := io.ReadFull(reader, data); err != nil {
		return nil, fmt.Errorf("failed to read packet data: %w", err)
	}

	return &AudioPacket{
		PTS:        ptsFlags & PacketPTSMask,
		PacketSize: packetSize,
		Data:       data,
	}, nil
}

// Read device metadata (following ACTUAL scrcpy protocol - only device name!)
func ReadDeviceMeta(reader io.Reader) (*DeviceMeta, error) {
	// According to REAL scrcpy source, device_read_info only reads device name (64 bytes)
	const deviceNameFieldLength = 64
	nameBytes := make([]byte, deviceNameFieldLength)
	if _, err := io.ReadFull(reader, nameBytes); err != nil {
		return nil, err
	}

	// Remove null bytes and get device name
	deviceName := strings.TrimRight(string(nameBytes), "\x00")

	return &DeviceMeta{
		DeviceName: deviceName,
		Width:      0, // Will be determined from video stream
		Height:     0, // Will be determined from video stream
	}, nil
}

// Serialize control message
func SerializeControlMessage(msg *ControlMessage) []byte {
	// Scrcpy control message format:
	// - type (1 byte)
	// - payload (varies by type)
	buf := make([]byte, 0, 1024)
	buf = append(buf, msg.Type)
	buf = append(buf, msg.Data...)
	return buf
}

// SerializeTouchEvent converts API TouchEvent to scrcpy format
func SerializeTouchEvent(event *TouchEvent, screenWidth, screenHeight uint16) []byte {
	buf := make([]byte, 0, 32)
	
	// Convert action string to byte
	var action uint8
	switch event.Action {
	case "down":
		action = 0
	case "up":
		action = 1
	case "move":
		action = 2
	default:
		action = 2
	}
	buf = append(buf, action)

	// Pointer ID (8 bytes)
	ptrBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(ptrBytes, uint64(event.PointerID))
	buf = append(buf, ptrBytes...)

	// Position structure:
	// - x (4 bytes)
	// - y (4 bytes)  
	// - screenWidth (2 bytes)
	// - screenHeight (2 bytes)
	posBytes := make([]byte, 12)
	binary.BigEndian.PutUint32(posBytes[0:4], uint32(event.X * float64(screenWidth)))
	binary.BigEndian.PutUint32(posBytes[4:8], uint32(event.Y * float64(screenHeight)))
	// Screen dimensions - use actual device screen size
	binary.BigEndian.PutUint16(posBytes[8:10], screenWidth)
	binary.BigEndian.PutUint16(posBytes[10:12], screenHeight)
	buf = append(buf, posBytes...)

	// Pressure (16-bit, 0xFFFF = 1.0)
	pressureBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(pressureBytes, uint16(event.Pressure*0xFFFF))
	buf = append(buf, pressureBytes...)

	// Action button (32-bit) - which button triggered the action
	actionButtonBytes := make([]byte, 4)
	if action == 0 || action == 1 { // DOWN or UP
		binary.BigEndian.PutUint32(actionButtonBytes, 1) // Primary button
	} else {
		binary.BigEndian.PutUint32(actionButtonBytes, 0) // No button for MOVE
	}
	buf = append(buf, actionButtonBytes...)

	// Buttons (32-bit) - current button state
	buttonBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(buttonBytes, 1) // Primary button
	buf = append(buf, buttonBytes...)

	return buf
}

// SerializeKeyEvent converts API KeyEvent to scrcpy format
func SerializeKeyEvent(event *KeyEvent) []byte {
	buf := make([]byte, 0, 16)
	
	// Convert action string to byte
	var action uint8
	switch event.Action {
	case "down":
		action = 0
	case "up":
		action = 1
	default:
		action = 1
	}
	buf = append(buf, action)

	// Keycode
	keyBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(keyBytes, uint32(event.Keycode))
	buf = append(buf, keyBytes...)

	// Repeat
	repeatBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(repeatBytes, uint32(event.Repeat))
	buf = append(buf, repeatBytes...)

	// Meta state
	metaBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(metaBytes, uint32(event.MetaState))
	buf = append(buf, metaBytes...)

	return buf
}