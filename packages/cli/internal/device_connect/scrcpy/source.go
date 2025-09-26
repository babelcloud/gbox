package scrcpy

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/core"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/device"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/pipeline"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/protocol"
	"github.com/babelcloud/gbox/packages/cli/internal/util"
)

// Source implements the core.Source interface for scrcpy devices
type Source struct {
	mu            sync.RWMutex
	deviceSerial  string
	pipeline      *pipeline.Pipeline
	cancel        context.CancelFunc
	streamingMode string // Streaming mode (h264, webrtc, mse)

	// Connections
	audioConn   net.Conn
	controlConn net.Conn

	// Handshake info
	videoWidth  int
	videoHeight int
	spsPps      []byte
}

// NewSource creates a new scrcpy source
func NewSource(deviceSerial string) *Source {
	return NewSourceWithMode(deviceSerial, "webrtc") // Default mode
}

func NewSourceWithMode(deviceSerial string, streamingMode string) *Source {
	return &Source{
		deviceSerial:  deviceSerial,
		pipeline:      pipeline.NewPipeline(),
		streamingMode: streamingMode,
	}
}

// Start implements core.Source
func (s *Source) Start(ctx context.Context, deviceSerial string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel != nil {
		return fmt.Errorf("source already started")
	}

	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	// Start scrcpy reader in background
	go s.runReader(ctx)

	util.GetLogger().Info("Scrcpy source started", "device", deviceSerial)
	return nil
}

// Stop implements core.Source
func (s *Source) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}

	// Close all active connections to ensure clean state
	if s.audioConn != nil {
		s.audioConn.Close()
		s.audioConn = nil
	}
	if s.controlConn != nil {
		s.controlConn.Close()
		s.controlConn = nil
	}

	util.GetLogger().Info("Scrcpy source stopped", "device", s.deviceSerial)
	return nil
}

// SubscribeVideo implements core.Source
func (s *Source) SubscribeVideo(subscriberID string, bufferSize int) <-chan core.VideoSample {
	return s.pipeline.SubscribeVideo(subscriberID, bufferSize)
}

// UnsubscribeVideo implements core.Source
func (s *Source) UnsubscribeVideo(subscriberID string) {
	s.pipeline.UnsubscribeVideo(subscriberID)
}

// SubscribeAudio implements core.Source
func (s *Source) SubscribeAudio(subscriberID string, bufferSize int) <-chan core.AudioSample {
	return s.pipeline.SubscribeAudio(subscriberID, bufferSize)
}

// UnsubscribeAudio implements core.Source
func (s *Source) UnsubscribeAudio(subscriberID string) {
	s.pipeline.UnsubscribeAudio(subscriberID)
}

// SubscribeControl implements core.Source
func (s *Source) SubscribeControl(subscriberID string, bufferSize int) <-chan core.ControlMessage {
	// Control channel not implemented yet
	return make(chan core.ControlMessage, bufferSize)
}

// UnsubscribeControl implements core.Source
func (s *Source) UnsubscribeControl(subscriberID string) {
	// Control channel not implemented yet
}

// SendControl implements core.Source
func (s *Source) SendControl(msg core.ControlMessage) error {
	s.mu.RLock()
	conn := s.controlConn
	s.mu.RUnlock()

	if conn == nil {
		// Don't return error for control connection not ready during startup
		// This is expected during the initial connection phase
		util.GetLogger().Warn("Control connection not ready, ignoring control message",
			"device", s.deviceSerial, "msg_type", msg.Type)
		return nil
	}

	// Serialize control message
	data := protocol.SerializeControlMessage(&protocol.ControlMessage{
		Type: uint8(msg.Type),
		Data: msg.Data,
	})

	// Send to device
	if _, err := conn.Write(data); err != nil {
		util.GetLogger().Error("Failed to send control message to device", "device", s.deviceSerial, "error", err)
		return fmt.Errorf("failed to send control message: %w", err)
	}

	util.GetLogger().Debug("Control message sent successfully", "device", s.deviceSerial, "msg_type", msg.Type)
	return nil
}

// GetSpsPps implements core.Source
func (s *Source) GetSpsPps() []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.spsPps
}

// GetConnectionInfo implements core.Source
func (s *Source) GetConnectionInfo() (deviceSerial string, videoWidth, videoHeight int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.deviceSerial, s.videoWidth, s.videoHeight
}

// RequestKeyframe requests a keyframe from the device
func (s *Source) RequestKeyframe() {
	s.requestKeyframeAsync()
}

// GetPipeline returns the pipeline for backward compatibility
func (s *Source) GetPipeline() *pipeline.Pipeline {
	return s.pipeline
}

// runReader runs the scrcpy reader in a separate goroutine
func (s *Source) runReader(ctx context.Context) {
	logger := util.GetLogger()
	logger.Info("Scrcpy reader started", "device", s.deviceSerial)

	// Ensure we clean up the cancel function when this goroutine exits
	defer func() {
		s.mu.Lock()
		s.cancel = nil
		s.mu.Unlock()
		logger.Info("Scrcpy reader stopped", "device", s.deviceSerial)
	}()

	// Create scrcpy connection
	scrcpyConn, err := s.createScrcpyConnection()
	if err != nil {
		logger.Error("Failed to create scrcpy connection", "device", s.deviceSerial, "error", err)
		return
	}

	// Connect to scrcpy server
	conn, err := scrcpyConn.Connect()
	if err != nil {
		logger.Error("Failed to connect to scrcpy server", "device", s.deviceSerial, "error", err)
		return
	}
	defer conn.Close()

	logger.Info("Connected to scrcpy server", "device", s.deviceSerial)

	// Start listening for additional connections (audio/control)
	if scrcpyConn.Listener != nil {
		go s.handleStreamConnections(ctx, scrcpyConn.Listener)
	}

	// Start reading video stream from the first connection
	go s.handleVideoStream(ctx, conn)

	// Wait for context cancellation
	<-ctx.Done()
}

// createScrcpyConnection creates a scrcpy connection for the device
func (s *Source) createScrcpyConnection() (*device.ScrcpyConnection, error) {
	// Generate a unique session ID
	scid := uint32(10000 + time.Now().UnixNano()%55536)
	return device.NewScrcpyConnectionWithMode(s.deviceSerial, scid, s.streamingMode), nil
}

// handleStreamConnections handles additional scrcpy connections (audio/control)
func (s *Source) handleStreamConnections(ctx context.Context, listener net.Listener) {
	logger := util.GetLogger()

	connectionCount := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Set accept timeout to prevent indefinite blocking
		if tcpListener, ok := listener.(*net.TCPListener); ok {
			tcpListener.SetDeadline(time.Now().Add(1 * time.Second))
		}

		conn, err := listener.Accept()
		if err != nil {
			// Check if it's a timeout error
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue // Continue trying to accept
			}
			logger.Error("Failed to accept stream connection", "error", err)
			continue
		}

		connectionCount++
		logger.Info("New stream connection received", "device", s.deviceSerial, "connection_number", connectionCount)
		go s.handleStreamConnection(ctx, conn)
	}
}

// handleStreamConnection handles a single stream connection
func (s *Source) handleStreamConnection(ctx context.Context, conn net.Conn) {
	logger := util.GetLogger()

	// scrcpy uses connection order to identify stream types:
	// 1st connection: video (handled separately in runReader)
	// 2nd connection: audio
	// 3rd connection: control

	// Check if this is audio or control based on existing connections
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.audioConn == nil {
		// This is the audio stream (2nd connection)
		logger.Info("🎵 Audio stream connected", "device", s.deviceSerial)
		s.audioConn = conn
		go s.handleAudioStream(ctx, conn)
	} else if s.controlConn == nil {
		// This is the control stream (3rd connection)
		logger.Info("Control stream connected", "device", s.deviceSerial)
		s.controlConn = conn
		go s.handleControlStream(ctx, conn)
	} else {
		// Unexpected additional connection
		logger.Warn("Unexpected additional connection received", "device", s.deviceSerial)
		conn.Close()
	}
}

// handleVideoStream processes the video stream
func (s *Source) handleVideoStream(ctx context.Context, conn net.Conn) {
	logger := util.GetLogger()
	logger.Debug("Starting video stream processing", "device", s.deviceSerial)

	// Read video metadata and handshake information
	if err := s.readVideoMetadata(conn); err != nil {
		logger.Error("Failed to read video metadata", "device", s.deviceSerial, "error", err)
		return
	}

	// Start reading video packets
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Read video packet
		packet, err := protocol.ReadVideoPacket(conn)
		if err != nil {
			// Check if context was cancelled while reading
			select {
			case <-ctx.Done():
				logger.Debug("Video stream context cancelled during read", "device", s.deviceSerial)
				return
			default:
			}

			if err == io.EOF {
				logger.Info("Video stream ended", "device", s.deviceSerial)
			} else if strings.Contains(err.Error(), "use of closed network connection") {
				logger.Debug("Video connection closed", "device", s.deviceSerial)
			} else {
				logger.Error("Failed to read video packet", "device", s.deviceSerial, "error", err)
			}
			return
		}

		// Create video sample
		sample := core.VideoSample{
			Data:  packet.Data,
			IsKey: packet.IsKeyFrame,
			PTS:   int64(packet.PTS),
		}

		// Cache SPS/PPS if this is a config packet, and do NOT publish as video sample
		if packet.IsConfig {
			logger.Info("Config packet received - caching SPS/PPS", "device", s.deviceSerial, "size", len(packet.Data))
			s.mu.Lock()
			s.spsPps = append([]byte{}, packet.Data...)
			s.mu.Unlock()
			s.pipeline.CacheSpsPps(packet.Data)
			logger.Info("SPS/PPS cached successfully", "device", s.deviceSerial, "size", len(packet.Data))
			continue
		}

		// Log keyframes for monitoring
		if packet.IsKeyFrame {
			logger.Debug("Video keyframe received", "device", s.deviceSerial, "size", len(packet.Data))
		}

		// Publish to pipeline
		s.pipeline.PublishVideo(sample)
	}
}

// handleAudioStream processes the audio stream
func (s *Source) handleAudioStream(ctx context.Context, conn net.Conn) {
	logger := util.GetLogger()
	logger.Info("🎵 Starting audio stream processing", "device", s.deviceSerial)
	defer func() {
		conn.Close()
		logger.Info("🎵 Audio stream processing stopped", "device", s.deviceSerial)
	}()

	// Read audio metadata
	if err := s.readAudioMetadata(conn); err != nil {
		logger.Error("❌ Failed to read audio metadata", "device", s.deviceSerial, "error", err)
		return
	}
	logger.Info("✅ Audio metadata read successfully", "device", s.deviceSerial)

	// Start reading audio packets
	packetCount := 0
	lastPacketTime := time.Now()
	timeoutDuration := 30 * time.Second // 30 seconds timeout for no audio data

	logger.Info("🎵 Starting audio packet reading loop", "device", s.deviceSerial)

	for {
		select {
		case <-ctx.Done():
			logger.Debug("Audio stream context cancelled", "device", s.deviceSerial)
			return
		default:
		}

		// Check for timeout - if no audio packets received for 30 seconds, log warning
		if time.Since(lastPacketTime) > timeoutDuration && packetCount == 0 {
			logger.Warn("🎵 No audio packets received for 30 seconds - device may not have audio activity or audio permissions may be missing",
				"device", s.deviceSerial,
				"timeout", timeoutDuration,
				"packets_received", packetCount)
			// Continue waiting, don't return - audio might start later
		}

		// Read audio packet with timeout to prevent blocking on closed connections
		packet, err := protocol.ReadAudioPacket(conn)
		if err != nil {
			// Check if context was cancelled while reading
			select {
			case <-ctx.Done():
				logger.Debug("Audio stream context cancelled during read", "device", s.deviceSerial)
				return
			default:
			}

			if err == io.EOF {
				logger.Info("Audio stream ended", "device", s.deviceSerial, "total_packets", packetCount)
			} else if strings.Contains(err.Error(), "use of closed network connection") {
				logger.Debug("Audio connection closed", "device", s.deviceSerial)
			} else {
				logger.Error("Failed to read audio packet", "device", s.deviceSerial, "error", err, "packets_received", packetCount)
			}
			return
		}

		// Update packet count and timestamp
		packetCount++
		lastPacketTime = time.Now()

		// Skip audio config packets (they are not media samples)
		if packet.IsConfig {
			continue
		}

		// Create audio sample
		sample := core.AudioSample{
			Data: packet.Data,
			PTS:  int64(packet.PTS),
		}

		// Log first few audio packets and progress
		if len(sample.Data) > 0 {
			if packetCount <= 5 {
				logger.Info("🎵 Audio packet received", "device", s.deviceSerial, "packet", packetCount, "size", len(sample.Data), "pts", sample.PTS)
			} else if packetCount%100 == 0 {
				logger.Debug("🎵 Audio streaming progress", "device", s.deviceSerial, "packets", packetCount, "size", len(sample.Data))
			}
		} else {
			logger.Warn("🎵 Empty audio packet received", "device", s.deviceSerial, "packet", packetCount)
		}

		// Publish to pipeline
		s.pipeline.PublishAudio(sample)
	}
}

// handleControlStream processes the control stream
func (s *Source) handleControlStream(ctx context.Context, conn net.Conn) {
	logger := util.GetLogger()
	logger.Debug("Starting control stream processing", "device", s.deviceSerial)
	defer conn.Close()

	// Start drain to prevent blocking
	drainControl(conn)

	// Keep connection alive but don't process control messages for now
	<-ctx.Done()
	logger.Info("Control stream ended", "device", s.deviceSerial)
}

// readVideoMetadata reads video metadata from the connection
func (s *Source) readVideoMetadata(conn net.Conn) error {
	logger := util.GetLogger()

	// Read device name
	deviceName := make([]byte, 64)
	if _, err := io.ReadFull(conn, deviceName); err != nil {
		return fmt.Errorf("failed to read device name: %w", err)
	}
	// Clean device name by removing null characters and trimming
	cleanDeviceName := strings.TrimRight(string(deviceName), "\x00")
	logger.Info("Device name read", "device", s.deviceSerial, "name", cleanDeviceName)

	// Read video metadata (codecId, width, height)
	metaBuf := make([]byte, 12)
	if _, err := io.ReadFull(conn, metaBuf); err != nil {
		return fmt.Errorf("failed to read video metadata: %w", err)
	}

	codecID := binary.BigEndian.Uint32(metaBuf[0:4])
	width := int(binary.BigEndian.Uint32(metaBuf[4:8]))
	height := int(binary.BigEndian.Uint32(metaBuf[8:12]))

	s.mu.Lock()
	s.videoWidth = width
	s.videoHeight = height
	s.mu.Unlock()

	logger.Info("Video metadata read", "device", s.deviceSerial,
		"codec_id", codecID, "width", width, "height", height)

	return nil
}

// readAudioMetadata reads audio metadata from the connection
func (s *Source) readAudioMetadata(conn net.Conn) error {
	logger := util.GetLogger()

	// Read audio metadata (codecId)
	metaBuf := make([]byte, 4)
	if _, err := io.ReadFull(conn, metaBuf); err != nil {
		return fmt.Errorf("failed to read audio metadata: %w", err)
	}

	codecID := binary.BigEndian.Uint32(metaBuf)
	logger.Info("Audio metadata read", "device", s.deviceSerial, "codec_id", codecID)

	return nil
}
