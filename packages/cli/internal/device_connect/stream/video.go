package stream

import (
	"encoding/binary"
	"io"
	"log"
	"time"
	
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
)

// VideoHandler handles video stream processing
type VideoHandler struct {
	track        *webrtc.TrackLocalStaticSample
	width        int
	height       int
	lastKeyframe time.Time
}

// NewVideoHandler creates a new video stream handler
func NewVideoHandler(track *webrtc.TrackLocalStaticSample) *VideoHandler {
	return &VideoHandler{
		track:        track,
		lastKeyframe: time.Now(),
	}
}

// HandleStream processes video stream from device
func (vh *VideoHandler) HandleStream(reader io.Reader) error {
	// Read video metadata
	metaBuf := make([]byte, 12) // codecId(4) + width(4) + height(4)
	if _, err := io.ReadFull(reader, metaBuf); err != nil {
		return err
	}
	
	codecID := binary.BigEndian.Uint32(metaBuf[0:4])
	vh.width = int(binary.BigEndian.Uint32(metaBuf[4:8]))
	vh.height = int(binary.BigEndian.Uint32(metaBuf[8:12]))
	
	log.Printf("Video stream started - Codec: %d, Resolution: %dx%d", codecID, vh.width, vh.height)
	
	// Start streaming video packets
	return vh.streamPackets(reader)
}

// streamPackets reads and processes video packets
func (vh *VideoHandler) streamPackets(reader io.Reader) error {
	const maxPacketSize = 1024 * 1024 // 1MB max packet size
	sequenceNumber := uint16(0)
	
	for {
		// Read packet header (8 bytes: PTS high + PTS low)
		header := make([]byte, 8)
		if _, err := io.ReadFull(reader, header); err != nil {
			if err == io.EOF {
				log.Println("Video stream ended")
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
		if packetSize > maxPacketSize {
			log.Printf("Warning: Large video packet: %d bytes", packetSize)
			continue
		}
		
		// Read packet data
		packetData := make([]byte, packetSize)
		if _, err := io.ReadFull(reader, packetData); err != nil {
			return err
		}
		
		// Check for keyframe (H.264 NAL unit type)
		if len(packetData) > 4 {
			nalType := packetData[4] & 0x1F
			if nalType == 5 || nalType == 7 || nalType == 8 { // IDR, SPS, PPS
				vh.lastKeyframe = time.Now()
			}
		}
		
		// Fragment and send via RTP
		if err := vh.sendVideoPacket(packetData, pts, &sequenceNumber); err != nil {
			log.Printf("Error sending video packet: %v", err)
		}
	}
}

// sendVideoPacket sends video data as RTP packets
func (vh *VideoHandler) sendVideoPacket(data []byte, pts uint64, seqNum *uint16) error {
	if vh.track == nil {
		return nil
	}
	
	const maxRTPPayloadSize = 1200 // Leave room for RTP headers
	
	// Fragment large NAL units
	if len(data) <= maxRTPPayloadSize {
		// Single NAL unit - write directly as sample
		*seqNum++
		
		sample := media.Sample{
			Data:     data,
			Duration: time.Millisecond * 33, // ~30 fps
		}
		return vh.track.WriteSample(sample)
	}
	
	// Fragment into FU-A packets (H.264 fragmentation)
	nalHeader := data[0]
	data = data[1:] // Skip NAL header
	
	for len(data) > 0 {
		payloadSize := len(data)
		if payloadSize > maxRTPPayloadSize-2 { // -2 for FU header
			payloadSize = maxRTPPayloadSize - 2
		}
		
		// Build FU-A header
		fuHeader := make([]byte, 2)
		fuHeader[0] = (nalHeader & 0xE0) | 28 // FU-A type
		fuHeader[1] = nalHeader & 0x1F        // Original NAL type
		
		if len(data) == payloadSize {
			fuHeader[1] |= 0x40 // End bit
		}
		if len(data) == len(data) { // First fragment (when data is still complete)
			fuHeader[1] |= 0x80 // Start bit
		}
		
		payload := append(fuHeader, data[:payloadSize]...)
		marker := len(data) == payloadSize // Last fragment
		
		// No longer need RTP packet for WriteSample
		*seqNum++
		_ = marker // Mark as used
		
		// Collect all fragments and write complete NAL unit
		if marker {
			sample := media.Sample{
				Data:     payload,
				Duration: time.Millisecond * 33,
			}
			if err := vh.track.WriteSample(sample); err != nil {
				return err
			}
		}
		
		data = data[payloadSize:]
	}
	
	return nil
}

// GetDimensions returns the video dimensions
func (vh *VideoHandler) GetDimensions() (int, int) {
	return vh.width, vh.height
}

// RequestKeyframe requests a keyframe from the encoder
func (vh *VideoHandler) RequestKeyframe() {
	// This would send a request to the device for a keyframe
	log.Println("Keyframe requested")
}