package device

import (
	"encoding/binary"
	"fmt"
	"io"
	"strings"
)

// DeviceMeta contains device metadata
type DeviceMeta struct {
	DeviceName string
	Width      uint32
	Height     uint32
}

// VideoPacket represents a video packet
type VideoPacket struct {
	PTS        uint64
	Data       []byte
	IsConfig   bool
	IsKeyFrame bool
}

// AudioPacket represents an audio packet
type AudioPacket struct {
	PTS  uint64
	Data []byte
}

// ControlMessage represents a control message
type ControlMessage struct {
	Type     uint8
	Sequence uint32
	Data     []byte
}

// ReadDeviceMeta reads device metadata from connection
func ReadDeviceMeta(conn io.Reader) (*DeviceMeta, error) {
	// According to scrcpy protocol, device metadata only contains device name (64 bytes)
	const deviceNameFieldLength = 64
	nameBytes := make([]byte, deviceNameFieldLength)
	if _, err := io.ReadFull(conn, nameBytes); err != nil {
		return nil, fmt.Errorf("failed to read device name: %w", err)
	}

	// Remove null bytes and get device name
	deviceName := strings.TrimRight(string(nameBytes), "\x00")

	return &DeviceMeta{
		DeviceName: deviceName,
		Width:      0, // Will be determined from video stream
		Height:     0, // Will be determined from video stream
	}, nil
}

// ReadVideoPacket reads a video packet from the stream
func ReadVideoPacket(reader io.Reader) (*VideoPacket, error) {
	// Read packet header (8 bytes: PTS)
	header := make([]byte, 8)
	if _, err := io.ReadFull(reader, header); err != nil {
		if err == io.EOF {
			return nil, err
		}
		return nil, fmt.Errorf("failed to read packet header: %w", err)
	}

	pts := binary.BigEndian.Uint64(header)

	// Read packet size (4 bytes)
	sizeBuf := make([]byte, 4)
	if _, err := io.ReadFull(reader, sizeBuf); err != nil {
		return nil, fmt.Errorf("failed to read packet size: %w", err)
	}

	packetSize := binary.BigEndian.Uint32(sizeBuf)

	// Sanity check
	if packetSize > 10*1024*1024 { // 10MB max
		return nil, fmt.Errorf("packet size too large: %d", packetSize)
	}

	// Read packet data
	packetData := make([]byte, packetSize)
	if _, err := io.ReadFull(reader, packetData); err != nil {
		return nil, fmt.Errorf("failed to read packet data: %w", err)
	}

	// Detect config and keyframe packets
	isConfig := false
	isKeyFrame := false

	if len(packetData) > 4 {
		// Check NAL unit type for H.264
		nalType := packetData[4] & 0x1F
		if nalType == 7 || nalType == 8 { // SPS or PPS
			isConfig = true
		} else if nalType == 5 { // IDR frame
			isKeyFrame = true
		}

		// Also check if it starts with 0x00000001 followed by 0x67 (SPS) or 0x68 (PPS)
		if len(packetData) > 5 &&
			packetData[0] == 0 && packetData[1] == 0 &&
			packetData[2] == 0 && packetData[3] == 1 {
			if packetData[4] == 0x67 || packetData[4] == 0x68 {
				isConfig = true
			} else if packetData[4] == 0x65 {
				isKeyFrame = true
			}
		}
	}

	return &VideoPacket{
		PTS:        pts,
		Data:       packetData,
		IsConfig:   isConfig,
		IsKeyFrame: isKeyFrame,
	}, nil
}

// ReadAudioPacket reads an audio packet from the stream
func ReadAudioPacket(reader io.Reader) (*AudioPacket, error) {
	// Read packet header (8 bytes: PTS)
	header := make([]byte, 8)
	if _, err := io.ReadFull(reader, header); err != nil {
		if err == io.EOF {
			return nil, err
		}
		return nil, fmt.Errorf("failed to read packet header: %w", err)
	}

	pts := binary.BigEndian.Uint64(header)

	// Read packet size (4 bytes)
	sizeBuf := make([]byte, 4)
	if _, err := io.ReadFull(reader, sizeBuf); err != nil {
		return nil, fmt.Errorf("failed to read packet size: %w", err)
	}

	packetSize := binary.BigEndian.Uint32(sizeBuf)

	// Sanity check
	if packetSize > 1024*1024 { // 1MB max for audio
		return nil, fmt.Errorf("audio packet size too large: %d", packetSize)
	}

	// Read packet data
	packetData := make([]byte, packetSize)
	if _, err := io.ReadFull(reader, packetData); err != nil {
		return nil, fmt.Errorf("failed to read packet data: %w", err)
	}

	return &AudioPacket{
		PTS:  pts,
		Data: packetData,
	}, nil
}
