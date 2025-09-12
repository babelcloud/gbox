package stream

import (
	"encoding/binary"
	"io"
	"log"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
)

// AudioHandler handles audio stream processing
type AudioHandler struct {
	track      *webrtc.TrackLocalStaticSample
	sampleRate uint32
	channels   uint16
}

// NewAudioHandler creates a new audio stream handler
func NewAudioHandler(track *webrtc.TrackLocalStaticSample) *AudioHandler {
	return &AudioHandler{
		track:      track,
		sampleRate: 48000, // Default Opus sample rate
		channels:   2,     // Stereo
	}
}

// HandleStream processes audio stream from device
func (vh *AudioHandler) HandleStream(reader io.Reader) error {
	// Read audio metadata
	metaBuf := make([]byte, 4) // codecId
	if _, err := io.ReadFull(reader, metaBuf); err != nil {
		return err
	}
	
	codecID := binary.BigEndian.Uint32(metaBuf)
	log.Printf("Audio stream started - Codec: %d", codecID)
	
	// Start streaming audio packets
	return vh.streamPackets(reader)
}

// streamPackets reads and processes audio packets
func (vh *AudioHandler) streamPackets(reader io.Reader) error {
	for {
		// Read packet header (8 bytes: PTS)
		header := make([]byte, 8)
		if _, err := io.ReadFull(reader, header); err != nil {
			if err == io.EOF {
				log.Println("Audio stream ended")
				return nil
			}
			return err
		}
		
		pts := binary.BigEndian.Uint64(header)
		
		// Read packet size (4 bytes)
		sizeBuf := make([]byte, 4)
		if _, err := io.ReadFull(reader, sizeBuf); err != nil {
			return err
		}
		
		packetSize := binary.BigEndian.Uint32(sizeBuf)
		if packetSize > 1024*1024 { // 1MB max
			log.Printf("Warning: Large audio packet: %d bytes", packetSize)
			continue
		}
		
		// Read packet data
		packetData := make([]byte, packetSize)
		if _, err := io.ReadFull(reader, packetData); err != nil {
			return err
		}
		
		// Send audio packet
		if err := vh.sendAudioPacket(packetData, pts); err != nil {
			log.Printf("Error sending audio packet: %v", err)
		}
	}
}

// sendAudioPacket sends audio data via WebRTC
func (vh *AudioHandler) sendAudioPacket(data []byte, pts uint64) error {
	if vh.track == nil {
		return nil
	}
	
	// Calculate duration based on Opus frame size (20ms frames typical)
	duration := 20 * time.Millisecond
	
	sample := media.Sample{
		Data:     data,
		Duration: duration,
	}
	
	return vh.track.WriteSample(sample)
}