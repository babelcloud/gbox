package webrtc

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/device"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/protocol"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/stream"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
)

// Bridge bridges WebRTC connection with Android device streams
type Bridge struct {
	DeviceSerial string
	WebRTCConn   *webrtc.PeerConnection
	DataChannel  *webrtc.DataChannel
	WSConnection interface{} // WebSocket connection for sending info

	// Tracks
	VideoTrack *webrtc.TrackLocalStaticSample
	AudioTrack *webrtc.TrackLocalStaticSample

	// Device connections
	scrcpyConn *device.ScrcpyConnection

	// Stream connections
	videoConn   net.Conn
	audioConn   net.Conn
	controlConn net.Conn

	// Control handler
	controlHandler *stream.ControlHandler

	// Video dimensions
	VideoWidth  int
	VideoHeight int

	// Control flow
	controlReady chan struct{}
	controlMutex sync.Mutex // Protect controlReady channel operations
	Context      context.Context
	cancel       context.CancelFunc

	// Synchronization
	mu     sync.Mutex
	closed bool
}

// NewBridge creates a new WebRTC bridge for a device
func NewBridge(deviceSerial string, adbPath string) (*Bridge, error) {
	log.Printf("NewBridge called for device: %s", deviceSerial)
	ctx, cancel := context.WithCancel(context.Background())

	// Create WebRTC peer connection
	pc, err := CreatePeerConnection()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create peer connection: %w", err)
	}

	// Create bridge
	bridge := &Bridge{
		DeviceSerial: deviceSerial,
		WebRTCConn:   pc,
		Context:      ctx,
		cancel:       cancel,
		controlReady: make(chan struct{}),
		VideoWidth:   720, // Default dimensions
		VideoHeight:  1280,
	}

	// Set up data channel receiver (frontend will create the data channel)
	pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		log.Printf("Received DataChannel: %s", dc.Label())
		if dc.Label() == "control" {
			bridge.DataChannel = dc
			log.Printf("Control DataChannel received and assigned")
			// Set up control handler when DataChannel is received
			if bridge.controlHandler != nil {
				log.Printf("Setting up control handler with received DataChannel")
				bridge.controlHandler.UpdateDataChannel(dc)
				bridge.controlHandler.HandleIncomingMessages()
			}
		}
	})

	// Set up data channel handlers
	bridge.setupDataChannelHandlers()

	// Pre-create control handler with nil connection and nil DataChannel (will be updated when DataChannel is received)
	log.Printf("Creating ControlHandler with nil DataChannel (will be updated when received)")
	bridge.controlHandler = stream.NewControlHandler(nil, nil, 1080, 1920)
	log.Printf("ControlHandler created successfully")
	// HandleIncomingMessages will be called when DataChannel is received

	// Pre-create video and audio tracks for WebRTC negotiation
	// Default to H264 video track
	videoTrack, err := AddVideoTrack(pc, "h264")
	if err != nil {
		pc.Close()
		cancel()
		return nil, fmt.Errorf("failed to add video track: %w", err)
	}
	bridge.VideoTrack = videoTrack

	// Add audio track
	audioTrack, err := AddAudioTrack(pc, "opus")
	if err != nil {
		pc.Close()
		cancel()
		return nil, fmt.Errorf("failed to add audio track: %w", err)
	}
	bridge.AudioTrack = audioTrack

	return bridge, nil
}

// Start starts the bridge connection to device
func (b *Bridge) Start() error {
	// Generate SCID for this connection - use a valid port range (27183-37183)
	// This gives us 10000 possible ports for concurrent connections
	scid := uint32(27183 + (time.Now().UnixNano() % 10000))

	// Create scrcpy connection
	b.scrcpyConn = device.NewScrcpyConnection(b.DeviceSerial, scid)

	// Connect to scrcpy server
	conn, err := b.scrcpyConn.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect to scrcpy: %w", err)
	}

	// Store the connection
	b.videoConn = conn

	// Start media streaming from the first connection
	go b.startMediaStreaming(conn)

	// Accept additional stream connections (audio, control)
	if b.scrcpyConn.Listener != nil {
		go b.acceptStreamConnections(b.scrcpyConn.Listener)
	}

	return nil
}

// acceptStreamConnections accepts incoming stream connections from device
func (b *Bridge) acceptStreamConnections(listener net.Listener) {
	connectionCount := 1 // Start from 1 since video connection is already handled

	for {
		select {
		case <-b.Context.Done():
			return
		default:
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-b.Context.Done():
					return
				default:
					log.Printf("Failed to accept stream connection: %v", err)
					continue
				}
			}

			connectionCount++
			log.Printf("Accepted stream connection #%d", connectionCount)

			go b.handleStreamConnection(conn)
		}
	}
}

// Codec IDs are imported from protocol package

// handleStreamConnection handles an incoming stream connection
func (b *Bridge) handleStreamConnection(conn net.Conn) {
	defer conn.Close()

	// Set timeout for codec ID reading
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	// Read codec ID
	codecIDBytes := make([]byte, 4)
	n, err := io.ReadFull(conn, codecIDBytes)
	if err != nil {
		if err == io.EOF && n == 0 {
			log.Println("Connection closed immediately, treating as control")
			b.handleControlStream(conn)
			return
		}
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			log.Println("No codec ID received, treating as control")
			b.handleControlStream(conn)
			return
		}
		log.Printf("Failed to read codec ID: %v (read %d bytes)", err, n)
		// If we read some bytes but not enough, it might be a partial codec ID
		if n > 0 {
			log.Printf("Partial codec ID data: %v", codecIDBytes[:n])
		}
		conn.Close()
		return
	}

	conn.SetReadDeadline(time.Time{})
	codecID := binary.BigEndian.Uint32(codecIDBytes)

	switch codecID {
	case protocol.CodecIDH264, protocol.CodecIDH265, protocol.CodecIDAV1:
		log.Printf("Identified video stream with codec 0x%08x", codecID)
		b.videoConn = conn
		b.handleVideoStream(conn, codecID)

	case protocol.CodecIDOPUS, protocol.CodecIDAAC, protocol.CodecIDFLAC, protocol.CodecIDRAW:
		log.Printf("Identified audio stream with codec 0x%08x", codecID)
		b.audioConn = conn
		b.handleAudioStream(conn, codecID)

	default:
		if codecID == 0 {
			log.Println("Stream explicitly disabled (codec ID = 0)")
		} else if codecID == 1 {
			log.Println("Stream configuration error (codec ID = 1)")
		} else {
			log.Printf("Unknown codec 0x%08x, treating as control stream", codecID)
			b.handleControlStream(conn)
		}
	}
}

// startMediaStreaming starts the main media streaming process
func (b *Bridge) startMediaStreaming(conn net.Conn) {
	log.Println("Starting media streaming...")

	if conn == nil {
		log.Println("Connection is nil!")
		return
	}

	// Read device metadata from the first connection
	log.Println("Reading device metadata from first connection...")

	meta, err := device.ReadDeviceMeta(conn)
	if err != nil {
		log.Printf("Failed to read device metadata: %v", err)
		meta = &device.DeviceMeta{
			DeviceName: "Unknown Device",
			Width:      1080,
			Height:     1920,
		}
	}

	log.Printf("Device: %s (%dx%d)", meta.DeviceName, meta.Width, meta.Height)

	// Start optimized streaming
	go b.handleVideoStreamOptimized(conn)
}

// handleVideoStreamOptimized processes the first video connection
func (b *Bridge) handleVideoStreamOptimized(conn net.Conn) {
	log.Println("Processing optimized video stream")

	// Read codec ID from video stream
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	codecIDBytes := make([]byte, 4)
	n, err := io.ReadFull(conn, codecIDBytes)
	if err != nil {
		log.Printf("Failed to read video codec ID: %v (read %d bytes)", err, n)
		return
	}
	conn.SetReadDeadline(time.Time{})

	codecID := binary.BigEndian.Uint32(codecIDBytes)
	var codecName string
	var isVideoCodec bool

	switch codecID {
	case protocol.CodecIDH264:
		codecName = "H264"
		isVideoCodec = true
	case protocol.CodecIDH265:
		codecName = "H265"
		isVideoCodec = true
	case protocol.CodecIDAV1:
		codecName = "AV1"
		isVideoCodec = true
	case 0x36340000: // Special case: "64" + nulls - treat as H264
		codecName = "H264 (fallback)"
		isVideoCodec = true
		codecID = protocol.CodecIDH264 // Use standard H264 ID
	default:
		codecName = fmt.Sprintf("UNKNOWN(0x%08x)", codecID)
		isVideoCodec = false
	}
	log.Printf("Video codec: %s", codecName)

	// Verify it's a video codec
	if isVideoCodec {
		b.handleVideoStream(conn, codecID)
	} else {
		log.Printf("Expected video codec but got: %s", codecName)
		conn.Close()
	}
}

// handleVideoStream processes video stream with codec
func (b *Bridge) handleVideoStream(conn net.Conn, codecID uint32) {
	log.Printf("Starting video stream handler with codec ID: 0x%08x", codecID)

	// Read video dimensions
	sizeData := make([]byte, 8)
	if _, err := io.ReadFull(conn, sizeData); err != nil {
		log.Printf("Failed to read video size: %v", err)
		return
	}

	width := binary.BigEndian.Uint32(sizeData[0:4])
	height := binary.BigEndian.Uint32(sizeData[4:8])
	b.VideoWidth = int(width)
	b.VideoHeight = int(height)
	log.Printf("Video stream dimensions: %dx%d", width, height)

	// Update control handler with actual screen dimensions
	if b.controlHandler != nil {
		b.controlHandler.UpdateScreenDimensions(int(width), int(height))
	}

	// Video track should already be created in NewBridge
	if b.VideoTrack == nil {
		log.Printf("Video track is nil! This should not happen.")
		return
	}

	// Request keyframe from scrcpy server
	b.requestKeyFrame()

	// Start streaming video packets with ultra-low latency optimization
	b.streamVideoOptimized(conn)
}

// handleAudioStream processes audio stream
func (b *Bridge) handleAudioStream(conn net.Conn, codecID uint32) {
	log.Printf("Starting audio stream handler with codec ID: 0x%08x", codecID)

	// Audio track should already be created in NewBridge
	if b.AudioTrack == nil {
		log.Printf("Audio track is nil! This should not happen.")
		return
	}

	// Start streaming audio packets
	b.streamAudio(conn)
}

// handleControlStream processes control stream
func (b *Bridge) handleControlStream(conn net.Conn) {
	log.Println("Control stream handler started")
	b.controlConn = conn

	// Update control handler with actual connection and screen dimensions
	if b.controlHandler != nil {
		b.controlHandler.UpdateConnection(conn)
		// Update screen dimensions if available
		if b.VideoWidth > 0 && b.VideoHeight > 0 {
			b.controlHandler.UpdateScreenDimensions(b.VideoWidth, b.VideoHeight)
		}
	} else {
		// Fallback: create new control handler if somehow not created
		screenWidth := b.VideoWidth
		screenHeight := b.VideoHeight
		if screenWidth == 0 || screenHeight == 0 {
			screenWidth = 1080
			screenHeight = 1920
		}
		b.controlHandler = stream.NewControlHandler(conn, b.DataChannel, screenWidth, screenHeight)
		b.controlHandler.HandleIncomingMessages()
	}

	// Signal that control is ready (only once)
	b.controlMutex.Lock()
	select {
	case <-b.controlReady:
		// Already closed, do nothing
	default:
		close(b.controlReady)
	}
	b.controlMutex.Unlock()

	defer func() {
		conn.Close()
		b.controlConn = nil
		b.controlHandler = nil
	}()

	// Keep connection alive - don't try to read from it as it's write-only
	// Just wait for context cancellation
	<-b.Context.Done()
	log.Println("Control connection closing due to context cancellation")
}

// setupDataChannelHandlers sets up data channel event handlers
func (b *Bridge) setupDataChannelHandlers() {
	// DataChannel will be set up when received via OnDataChannel
	// This function is kept for future use if needed
}

// DataChannel messages are now handled by ControlHandler
// This avoids duplication and ensures consistent message processing

// handleKeyEvent delegates to control handler
func (b *Bridge) handleKeyEvent(message map[string]interface{}) {
	// This is now handled by ControlHandler
	// Keep for backward compatibility with WebSocket handlers
}

// handleTouchEvent delegates to control handler
func (b *Bridge) handleTouchEvent(message map[string]interface{}) {
	// This is now handled by ControlHandler
	// Keep for backward compatibility with WebSocket handlers
}

// requestKeyFrame requests a keyframe from the video encoder
func (b *Bridge) requestKeyFrame() {
	if b.controlHandler != nil {
		b.controlHandler.SendKeyFrameRequest()
	} else {
		log.Printf("Control handler not available for keyframe request")
	}
}

// Close closes the bridge and all its connections
func (b *Bridge) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil
	}
	b.closed = true

	// Cancel context
	if b.cancel != nil {
		b.cancel()
	}

	// Close WebRTC connection
	if b.WebRTCConn != nil {
		b.WebRTCConn.Close()
	}

	// Close stream connections
	if b.videoConn != nil {
		b.videoConn.Close()
	}
	if b.audioConn != nil {
		b.audioConn.Close()
	}
	if b.controlConn != nil {
		b.controlConn.Close()
	}

	// Close scrcpy connection
	if b.scrcpyConn != nil {
		b.scrcpyConn.Close()
	}

	log.Printf("Closed WebRTC bridge for device %s", b.DeviceSerial)
	return nil
}

// createVideoTrack creates a WebRTC video track
func (b *Bridge) createVideoTrack(codecID uint32) error {
	if b.VideoTrack != nil {
		return nil
	}

	var mimeType string
	var rtpCodecCap webrtc.RTPCodecCapability

	// RTCP feedback for keyframe requests
	videoRTCPFeedback := []webrtc.RTCPFeedback{
		{Type: "ccm", Parameter: "fir"},
		{Type: "nack", Parameter: "pli"},
		{Type: "goog-remb", Parameter: ""},
	}

	switch codecID {
	case protocol.CodecIDH264:
		mimeType = webrtc.MimeTypeH264
		rtpCodecCap = webrtc.RTPCodecCapability{
			MimeType:     webrtc.MimeTypeH264,
			ClockRate:    90000,
			RTCPFeedback: videoRTCPFeedback,
		}
	case protocol.CodecIDH265:
		mimeType = webrtc.MimeTypeH265
		rtpCodecCap = webrtc.RTPCodecCapability{
			MimeType:     webrtc.MimeTypeH265,
			ClockRate:    90000,
			RTCPFeedback: videoRTCPFeedback,
		}
	case protocol.CodecIDAV1:
		mimeType = webrtc.MimeTypeAV1
		rtpCodecCap = webrtc.RTPCodecCapability{
			MimeType:     webrtc.MimeTypeAV1,
			ClockRate:    90000,
			RTCPFeedback: videoRTCPFeedback,
		}
	default:
		return fmt.Errorf("unsupported video codec: 0x%08x", codecID)
	}

	// Force specific H.264 profile for better compatibility
	if codecID == protocol.CodecIDH264 {
		// Use baseline profile for faster decoding
		rtpCodecCap.SDPFmtpLine = "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f"
	}

	videoTrack, err := webrtc.NewTrackLocalStaticSample(rtpCodecCap, "video", "scrcpy-video")
	if err != nil {
		return fmt.Errorf("failed to create video track: %w", err)
	}

	if _, err := b.WebRTCConn.AddTrack(videoTrack); err != nil {
		return fmt.Errorf("failed to add video track: %w", err)
	}

	b.VideoTrack = videoTrack
	log.Printf("Created video track with codec %s", mimeType)
	return nil
}

// createAudioTrack creates a WebRTC audio track
func (b *Bridge) createAudioTrack() error {
	if b.AudioTrack != nil {
		return nil
	}

	audioTrack, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{
			MimeType:  webrtc.MimeTypeOpus,
			ClockRate: 48000,
			Channels:  2,
		},
		"audio",
		"scrcpy-audio",
	)
	if err != nil {
		return fmt.Errorf("failed to create audio track: %w", err)
	}

	if _, err := b.WebRTCConn.AddTrack(audioTrack); err != nil {
		return fmt.Errorf("failed to add audio track: %w", err)
	}

	b.AudioTrack = audioTrack
	log.Println("Created audio track")
	return nil
}

// streamVideoOptimized streams video packets to WebRTC
func (b *Bridge) streamVideoOptimized(reader io.Reader) {
	var lastVideoTimestamp int64 = 0
	packetCount := 0
	var h264Sps []byte
	var h264Pps []byte
	startCode := []byte{0x00, 0x00, 0x00, 0x01}
	decoderReady := false
	firstFrameSent := false

	for {
		select {
		case <-b.Context.Done():
			return
		default:
			packet, err := device.ReadVideoPacket(reader)
			if err != nil {
				select {
				case <-b.Context.Done():
					return
				default:
					log.Printf("Failed to read video packet #%d: %v", packetCount, err)
					return
				}
			}

			packetCount++

			if b.VideoTrack == nil {
				log.Println("Video track not initialized")
				return
			}

			// Calculate duration between frames
			timestamp := int64(packet.PTS)
			var duration time.Duration
			if lastVideoTimestamp > 0 && timestamp > lastVideoTimestamp {
				duration = time.Duration(timestamp-lastVideoTimestamp) * time.Microsecond
				duration = min(duration, 33*time.Millisecond) // Cap at 30 FPS
			}
			lastVideoTimestamp = timestamp

			// Process config packets for SPS/PPS
			if packet.IsConfig && len(packet.Data) >= 8 {
				data := addStartCodeIfNeeded(packet.Data)
				spsPpsInfo := bytes.Split(data, startCode)

				if len(spsPpsInfo) >= 3 {
					if len(spsPpsInfo[1]) > 0 {
						// ALWAYS use 4-byte start code for Chrome compatibility
						h264Sps = append([]byte{0x00, 0x00, 0x00, 0x01}, spsPpsInfo[1]...)
						sample := media.Sample{
							Data:     h264Sps,
							Duration: 0, // Zero duration for immediate processing
						}
						if err := b.VideoTrack.WriteSample(sample); err != nil {
							log.Printf("Failed to write SPS: %v", err)
						}
					}
					if len(spsPpsInfo[2]) > 0 {
						// ALWAYS use 4-byte start code for Chrome compatibility
						h264Pps = append([]byte{0x00, 0x00, 0x00, 0x01}, spsPpsInfo[2]...)
						sample := media.Sample{
							Data:     h264Pps,
							Duration: 0, // Zero duration for immediate processing
						}
						if err := b.VideoTrack.WriteSample(sample); err != nil {
							log.Printf("Failed to write PPS: %v", err)
						}
					}
				}

				// Mark decoder ready after config packets
				if len(h264Sps) > 0 && len(h264Pps) > 0 {
					decoderReady = true
					log.Printf("SPS/PPS ready for decoding")
				}
				continue
			}

			// For keyframes, send SPS/PPS first (only if not already sent)
			if packet.IsKeyFrame && len(h264Sps) > 0 && len(h264Pps) > 0 {
				// Send SPS/PPS before keyframe for proper decoding
				sample := media.Sample{Data: h264Sps, Duration: 0}
				b.VideoTrack.WriteSample(sample)

				sample = media.Sample{Data: h264Pps, Duration: 0}
				b.VideoTrack.WriteSample(sample)

				decoderReady = true
			}

			// Decide whether to send frame
			shouldSendFrame := false

			if packet.IsKeyFrame {
				shouldSendFrame = true
				if !firstFrameSent {
					firstFrameSent = true
					decoderReady = true
					log.Printf("First keyframe received")
				}
			} else if decoderReady {
				shouldSendFrame = true
			}

			if shouldSendFrame {
				processedData := addStartCodeIfNeeded(packet.Data)
				sample := media.Sample{
					Data:     processedData,
					Duration: duration,
					// No timestamp for minimal latency
				}
				if err := b.VideoTrack.WriteSample(sample); err != nil {
					log.Printf("Failed to write video sample: %v", err)
					return
				}

				// Keyframes are now requested only when needed:
				// - On connection establishment (handled in setupDataChannel)
				// - On video size changes (handled by frontend)
				// - On manual user requests
				// Removed periodic keyframe requests to reduce log spam
			}
		}
	}
}

// streamAudio streams audio packets to WebRTC
func (b *Bridge) streamAudio(conn net.Conn) {
	packetCount := 0

	for {
		select {
		case <-b.Context.Done():
			return
		default:
			packet, err := device.ReadAudioPacket(conn)
			if err != nil {
				log.Printf("Failed to read audio packet #%d: %v", packetCount, err)
				return
			}

			packetCount++

			if b.AudioTrack == nil {
				log.Println("Audio track not initialized")
				return
			}

			// For Opus, use fixed 20ms frame duration
			sample := media.Sample{
				Data:     packet.Data,
				Duration: 20 * time.Millisecond,
			}
			if err := b.AudioTrack.WriteSample(sample); err != nil {
				log.Printf("Failed to write audio sample: %v", err)
				return
			}
		}
	}
}

// Helper function to add H.264 start codes
func addStartCodeIfNeeded(data []byte) []byte {
	if len(data) < 4 {
		return data
	}

	startCode3 := []byte{0x00, 0x00, 0x01}
	startCode4 := []byte{0x00, 0x00, 0x00, 0x01}

	if len(data) >= 4 && bytes.Equal(data[:4], startCode4) {
		return data
	}

	if len(data) >= 3 && bytes.Equal(data[:3], startCode3) {
		result := make([]byte, 0, len(data)+1)
		result = append(result, startCode4...)
		result = append(result, data[3:]...)
		return result
	}

	result := make([]byte, 0, len(data)+4)
	result = append(result, startCode4...)
	result = append(result, data...)
	return result
}

// Helper function for min
func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

// SendControlMessage delegates to control handler
func (b *Bridge) SendControlMessage(msg *device.ControlMessage) error {
	// This is now handled by ControlHandler
	// Keep for backward compatibility
	if b.controlHandler != nil {
		// ControlHandler handles the actual sending
		return nil
	}
	return fmt.Errorf("control handler not available")
}

// HandleTouchEvent handles touch events from WebSocket
func (b *Bridge) HandleTouchEvent(message map[string]interface{}) {
	if b.controlHandler != nil {
		// Extract touch event parameters and delegate to control handler
		action, _ := message["action"].(string)
		x, _ := message["x"].(float64)
		y, _ := message["y"].(float64)
		pressure, _ := message["pressure"].(float64)
		pointerId, _ := message["pointerId"].(float64)

		b.controlHandler.SendTouchEventToDevice(action, x, y, pressure, int(pointerId))
	} else {
		log.Printf("Control handler not available for touch event")
	}
}

// HandleKeyEvent handles key events from WebSocket
func (b *Bridge) HandleKeyEvent(message map[string]interface{}) {
	if b.controlHandler != nil {
		// Extract key event parameters and delegate to control handler
		action, _ := message["action"].(string)
		keycode, _ := message["keycode"].(float64)
		metaState, _ := message["metaState"].(float64)
		repeat, _ := message["repeat"].(float64)

		b.controlHandler.SendKeyEventToDevice(action, int(keycode), int(metaState), int(repeat))
	} else {
		log.Printf("Control handler not available for key event")
	}
}

// HandleScrollEvent handles scroll events from WebSocket
func (b *Bridge) HandleScrollEvent(message map[string]interface{}) {
	if b.controlHandler != nil {
		// Extract scroll event parameters and delegate to control handler
		x, _ := message["x"].(float64)
		y, _ := message["y"].(float64)
		hScroll, _ := message["hScroll"].(float64)
		vScroll, _ := message["vScroll"].(float64)

		b.controlHandler.SendScrollEventToDevice(x, y, hScroll, vScroll)
	} else {
		log.Printf("Control handler not available for scroll event")
	}
}

// handleScrollEvent delegates to control handler
func (b *Bridge) handleScrollEvent(message map[string]interface{}) {
	// This is now handled by ControlHandler
	// Keep for backward compatibility with WebSocket handlers
}
