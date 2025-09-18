package handlers

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/scrcpy"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/transport/h264"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/transport/mse"
	"github.com/gorilla/websocket"
)

var controlUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now
	},
}

// StreamingHandlers contains handlers for streaming routes
type StreamingHandlers struct {
	// We'll pass necessary dependencies when needed
	serverService  ServerService   // Access to bridge manager through interface
	webrtcHandlers *WebRTCHandlers // WebRTC signaling handler
	pathPrefix     string          // URL path prefix for responses
}

// NewStreamingHandlers creates a new streaming handlers instance
func NewStreamingHandlers() *StreamingHandlers {
	return &StreamingHandlers{}
}

// SetServerService sets the server service dependency
func (h *StreamingHandlers) SetServerService(service ServerService) {
	h.serverService = service
	h.webrtcHandlers = NewWebRTCHandlers(service)
}

// SetPathPrefix sets the URL path prefix for responses
func (h *StreamingHandlers) SetPathPrefix(prefix string) {
	h.pathPrefix = prefix
}

// buildURL constructs a full URL with the correct prefix
func (h *StreamingHandlers) buildURL(path string) string {
	if h.pathPrefix != "" {
		return h.pathPrefix + path
	}
	return path
}

// HandleVideoStream handles H.264 and MSE video streaming
func (h *StreamingHandlers) HandleVideoStream(w http.ResponseWriter, r *http.Request) {
	// Extract device serial from path
	path := strings.TrimPrefix(r.URL.Path, "/stream/video/")
	parts := strings.Split(path, "?")
	deviceSerial := parts[0]

	if deviceSerial == "" {
		http.Error(w, "Device serial required", http.StatusBadRequest)
		return
	}

	// Parse mode and format parameters
	mode := r.URL.Query().Get("mode")
	format := r.URL.Query().Get("format")

	switch mode {
	case "h264":
		// Check format parameter for AVC vs Annex-B
		if format == "avc" {
			// AVC format H.264 streaming (for WebCodecs)
			handler := h264.NewAVCHTTPHandler(deviceSerial)
			handler.ServeHTTP(w, r)
		} else {
			// Direct H.264 streaming (Annex-B format)
			handler := h264.NewHTTPHandler(deviceSerial)
			handler.ServeHTTP(w, r)
		}

	case "mse":
		// MSE fMP4 streaming
		transport := mse.NewTransport(deviceSerial)
		transport.ServeHTTP(w, r)

	default:
		http.Error(w, "Invalid mode. Supported: h264, mse", http.StatusBadRequest)
	}
}

// HandleVideoWebSocket handles WebSocket video streaming (consolidated endpoint)
func (h *StreamingHandlers) HandleVideoWebSocket(w http.ResponseWriter, r *http.Request) {
	// Extract device serial from path /stream/video/ws/{device}
	path := strings.TrimPrefix(r.URL.Path, "/stream/video/ws/")
	parts := strings.Split(path, "?")
	deviceSerial := parts[0]

	if deviceSerial == "" {
		http.Error(w, "Device serial required", http.StatusBadRequest)
		return
	}

	if !isValidDeviceSerial(deviceSerial) {
		http.Error(w, "Invalid device serial", http.StatusBadRequest)
		return
	}

	// Use H.264 WebSocket handler
	handler := h264.NewWSHandler(deviceSerial)
	handler.ServeWebSocket(w, r)
}

// HandleAudioStream handles audio streaming endpoints
func (h *StreamingHandlers) HandleAudioStream(w http.ResponseWriter, r *http.Request) {
	// Extract device serial from path
	path := strings.TrimPrefix(r.URL.Path, "/stream/audio/")
	parts := strings.Split(path, "?")
	deviceSerial := parts[0]

	if deviceSerial == "" {
		http.Error(w, "Device serial required", http.StatusBadRequest)
		return
	}

	if !isValidDeviceSerial(deviceSerial) {
		http.Error(w, "Invalid device serial", http.StatusBadRequest)
		return
	}

	// Parse codec parameter
	codec := r.URL.Query().Get("codec")
	if codec == "" {
		codec = "aac" // Default to AAC
	}

	switch codec {
	case "aac":
		// AAC codec (default - placeholder)
		h.handleOpusAudioHTTP(w, r, deviceSerial)

	case "opus":
		// Opus codec
		h.handleOpusAudioHTTP(w, r, deviceSerial)

	default:
		http.Error(w, "Invalid codec. Supported: aac, opus", http.StatusBadRequest)
	}
}

// HandleStreamInfo provides information about available streams
func (h *StreamingHandlers) HandleStreamInfo(w http.ResponseWriter, r *http.Request) {
	deviceSerial := r.URL.Query().Get("device")
	if deviceSerial == "" {
		http.Error(w, "Device serial required", http.StatusBadRequest)
		return
	}

	// TODO: We need to access bridge manager here - will fix this in next step
	// For now, return basic info
	response := map[string]interface{}{
		"device":         deviceSerial,
		"supportedModes": []string{"h264", "mse", "webrtc"},
		"supportedFormats": map[string][]string{
			"h264": {"annexb", "avc"},
			"mse":  {"fmp4"},
		},
		"endpoints": map[string]string{
			"video":          h.buildURL(fmt.Sprintf("/stream/video/%s", deviceSerial)),
			"video_h264":     h.buildURL(fmt.Sprintf("/stream/video/%s?mode=h264", deviceSerial)),
			"video_h264_avc": h.buildURL(fmt.Sprintf("/stream/video/%s?mode=h264&format=avc", deviceSerial)),
			"video_mse":      h.buildURL(fmt.Sprintf("/stream/video/%s?mode=mse", deviceSerial)),
			"video_ws":       h.buildURL(fmt.Sprintf("/stream/video/ws/%s", deviceSerial)),
			"audio":          h.buildURL(fmt.Sprintf("/stream/audio/%s", deviceSerial)),
			"audio_aac":      h.buildURL(fmt.Sprintf("/stream/audio/%s?codec=aac", deviceSerial)),
			"audio_opus":     h.buildURL(fmt.Sprintf("/stream/audio/%s?codec=opus", deviceSerial)),
			"control":        h.buildURL(fmt.Sprintf("/stream/control/%s", deviceSerial)),
			"webrtc":         "/webrtc/signaling", // WebRTC uses signaling endpoint
		},
	}

	w.Header().Set("Content-Type", "application/json")
	RespondJSON(w, http.StatusOK, response)
}

// HandleStreamConnect handles stream connection requests
func (h *StreamingHandlers) HandleStreamConnect(w http.ResponseWriter, r *http.Request) {
	// Extract device serial from path like /api/stream/{device}/connect
	path := strings.TrimPrefix(r.URL.Path, "/api/stream/")
	parts := strings.Split(path, "/")

	if len(parts) < 2 {
		http.Error(w, "Invalid stream URL format", http.StatusBadRequest)
		return
	}

	deviceSerial := parts[0]
	action := parts[1] // "connect" or "disconnect"

	if !isValidDeviceSerial(deviceSerial) {
		http.Error(w, "Invalid device serial", http.StatusBadRequest)
		return
	}

	switch action {
	case "connect":
		h.handleStreamConnectDevice(w, r, deviceSerial)
	case "disconnect":
		h.handleStreamDisconnectDevice(w, r, deviceSerial)
	default:
		http.Error(w, fmt.Sprintf("Invalid action: %s", action), http.StatusBadRequest)
	}
}

// handleStreamConnectDevice handles device connection for streaming
func (h *StreamingHandlers) handleStreamConnectDevice(w http.ResponseWriter, r *http.Request, deviceSerial string) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// TODO: Need to access bridge manager - will implement properly later
	response := map[string]interface{}{
		"success":      true,
		"message":      "Stream connection not yet fully implemented in new architecture",
		"device":       deviceSerial,
		"streamActive": false,
	}
	w.Header().Set("Content-Type", "application/json")
	RespondJSON(w, http.StatusOK, response)
}

// handleStreamDisconnectDevice handles device disconnection for streaming
func (h *StreamingHandlers) handleStreamDisconnectDevice(w http.ResponseWriter, r *http.Request, deviceSerial string) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// TODO: Need to access bridge manager - will implement properly later
	response := map[string]interface{}{
		"success":      true,
		"message":      "Stream disconnection not yet fully implemented in new architecture",
		"device":       deviceSerial,
		"streamActive": false,
	}
	w.Header().Set("Content-Type", "application/json")
	RespondJSON(w, http.StatusOK, response)
}

// HandleControlWebSocket handles WebSocket connections for device control
func (h *StreamingHandlers) HandleControlWebSocket(w http.ResponseWriter, r *http.Request) {
	// Extract device serial from path like /stream/control/{device}
	path := strings.TrimPrefix(r.URL.Path, "/stream/control/")
	parts := strings.Split(path, "?")
	deviceSerial := parts[0]

	if deviceSerial == "" {
		http.Error(w, "Device serial required", http.StatusBadRequest)
		return
	}

	if !isValidDeviceSerial(deviceSerial) {
		http.Error(w, "Invalid device serial", http.StatusBadRequest)
		return
	}

	conn, err := controlUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade control WebSocket: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("Control WebSocket connection established for device: %s", deviceSerial)

	// Handle WebSocket messages
	for {
		var msg map[string]interface{}
		if err := conn.ReadJSON(&msg); err != nil {
			// Check for normal close conditions
			if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
				log.Printf("Control WebSocket closed normally for device: %s", deviceSerial)
			} else if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Control WebSocket read error: %v", err)
			}
			break
		}

		msgType, ok := msg["type"].(string)
		if !ok {
			continue
		}

		log.Printf("Control message received: type=%s, device=%s", msgType, deviceSerial)

		switch msgType {
		// WebRTC signaling messages - delegate to WebRTC handler
		case "ping", "offer", "answer", "ice-candidate":
			if h.webrtcHandlers != nil {
				h.delegateToWebRTCHandler(conn, msg, msgType, deviceSerial)
			} else {
				log.Printf("WebRTC handlers not initialized")
			}

		// Device control messages
		case "touch":
			h.handleTouchMessage(conn, msg, deviceSerial)

		case "key":
			h.handleKeyMessage(conn, msg, deviceSerial)

		case "scroll":
			h.handleScrollMessage(conn, msg, deviceSerial)

		case "clipboard_set":
			h.handleClipboardMessage(conn, msg, deviceSerial)

		case "reset_video":
			h.handleResetVideoMessage(conn, msg, deviceSerial)

		default:
			log.Printf("Unknown control message type: %s", msgType)
		}
	}
}

// delegateToWebRTCHandler forwards WebRTC signaling messages to the specialized handler
func (h *StreamingHandlers) delegateToWebRTCHandler(conn *websocket.Conn, msg map[string]interface{}, msgType, deviceSerial string) {
	switch msgType {
	case "ping":
		h.webrtcHandlers.handlePing(conn, msg)
	case "offer":
		h.webrtcHandlers.handleOffer(conn, msg, deviceSerial)
	case "answer":
		h.webrtcHandlers.handleAnswer(conn, msg, deviceSerial)
	case "ice-candidate":
		h.webrtcHandlers.handleIceCandidate(conn, msg, deviceSerial)
	}
}

// Helper function to parse device serial from various URL formats
func extractDeviceSerial(path string) string {
	// Remove common prefixes
	path = strings.TrimPrefix(path, "/stream/video/")
	path = strings.TrimPrefix(path, "/stream/ws/")
	path = strings.TrimPrefix(path, "/api/v1/devices/")

	// Split by / and ? to get just the device serial
	parts := strings.FieldsFunc(path, func(c rune) bool {
		return c == '/' || c == '?'
	})

	if len(parts) > 0 {
		return parts[0]
	}

	return ""
}

// Helper function to validate device serial format
func isValidDeviceSerial(serial string) bool {
	if serial == "" {
		return false
	}

	// Basic validation - should be alphanumeric with possible special chars
	if len(serial) < 1 || len(serial) > 64 {
		return false
	}

	// Allow alphanumeric, dots, dashes, underscores
	for _, c := range serial {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '.' || c == '-' || c == '_') {
			return false
		}
	}

	return true
}

// Helper function to parse quality parameter
func parseQuality(qualityStr string) int {
	if qualityStr == "" {
		return 80 // Default quality
	}

	quality, err := strconv.Atoi(qualityStr)
	if err != nil || quality < 1 || quality > 100 {
		return 80 // Default on invalid input
	}

	return quality
}

// Control message handlers

func (h *StreamingHandlers) handleTouchMessage(conn *websocket.Conn, msg map[string]interface{}, deviceSerial string) {
	action, _ := msg["action"].(string)
	x, _ := msg["x"].(float64)
	y, _ := msg["y"].(float64)
	pressure, _ := msg["pressure"].(float64)
	pointerId, _ := msg["pointerId"].(float64)

	log.Printf("Touch event: device=%s, action=%s, x=%.3f, y=%.3f, pressure=%.2f, pointerId=%.0f",
		deviceSerial, action, x, y, pressure, pointerId)

	// Forward touch event to bridge manager
	if h.serverService != nil {
		bridge, exists := h.serverService.GetBridge(deviceSerial)
		if exists && bridge != nil {
			bridge.HandleTouchEvent(msg)
		} else {
			log.Printf("Bridge not found for device %s", deviceSerial)
		}
	}
}

func (h *StreamingHandlers) handleKeyMessage(conn *websocket.Conn, msg map[string]interface{}, deviceSerial string) {
	action, _ := msg["action"].(string)
	keycode, _ := msg["keycode"].(float64)
	metaState, _ := msg["metaState"].(float64)

	log.Printf("Key event: device=%s, action=%s, keycode=%.0f, metaState=%.0f",
		deviceSerial, action, keycode, metaState)

	// Forward key event to bridge manager
	if h.serverService != nil {
		bridge, exists := h.serverService.GetBridge(deviceSerial)
		if exists && bridge != nil {
			bridge.HandleKeyEvent(msg)
		} else {
			log.Printf("Bridge not found for device %s", deviceSerial)
		}
	}
}

func (h *StreamingHandlers) handleScrollMessage(conn *websocket.Conn, msg map[string]interface{}, deviceSerial string) {
	x, _ := msg["x"].(float64)
	y, _ := msg["y"].(float64)
	hScroll, _ := msg["hScroll"].(float64)
	vScroll, _ := msg["vScroll"].(float64)

	log.Printf("Scroll event: device=%s, x=%.3f, y=%.3f, hScroll=%.2f, vScroll=%.2f",
		deviceSerial, x, y, hScroll, vScroll)

	// Forward scroll event to bridge manager
	if h.serverService != nil {
		bridge, exists := h.serverService.GetBridge(deviceSerial)
		if exists && bridge != nil {
			bridge.HandleScrollEvent(msg)
		} else {
			log.Printf("Bridge not found for device %s", deviceSerial)
		}
	}
}

func (h *StreamingHandlers) handleClipboardMessage(conn *websocket.Conn, msg map[string]interface{}, deviceSerial string) {
	text, _ := msg["text"].(string)
	paste, _ := msg["paste"].(bool)

	log.Printf("Clipboard event: device=%s, text_length=%d, paste=%t",
		deviceSerial, len(text), paste)

	// Clipboard handling - currently not implemented in bridge, so just log
	// TODO: Implement clipboard handling when bridge supports it
	log.Printf("Clipboard message received but bridge clipboard handling not yet implemented")
}

func (h *StreamingHandlers) handleResetVideoMessage(conn *websocket.Conn, msg map[string]interface{}, deviceSerial string) {
	log.Printf("Reset video event: device=%s", deviceSerial)

	// Video reset handling - currently not implemented in bridge, so just log
	// TODO: Implement video reset handling when bridge supports it
	log.Printf("Reset video message received but bridge video reset handling not yet implemented")
}

// handleOpusAudioStream handles Opus audio WebSocket connections
func (h *StreamingHandlers) handleOpusAudioStream(w http.ResponseWriter, r *http.Request, deviceSerial string) {
	logger := slog.With("device", deviceSerial)
	logger.Info("🎵 Starting Opus audio WebSocket stream", "url", r.URL.String())

	// Upgrade to WebSocket
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins for now
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("❌ Failed to upgrade to WebSocket", "error", err)
		return
	}
	defer conn.Close()

	logger.Info("✅ Opus audio WebSocket connection established")

	// Get audio stream from device source
	source := scrcpy.GetSource(deviceSerial)
	if source == nil {
		logger.Error("❌ Device source not found - is scrcpy running for this device?")
		conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "Device not connected"))
		return
	}

	logger.Info("✅ Found scrcpy source for audio streaming")

	// Subscribe to audio stream
	subscriberID := fmt.Sprintf("audio_ws_%p", conn)
	audioCh := source.SubscribeAudio(subscriberID, 100)
	defer source.UnsubscribeAudio(subscriberID)

	logger.Info("Subscribed to audio stream", "subscriberID", subscriberID)

	// Handle client messages in separate goroutine
	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				logger.Info("Audio WebSocket client disconnected", "error", err)
				return
			}
		}
	}()

	// Stream audio data to client
	audioFrameCount := 0
	logger.Info("🎵 Starting to stream audio data to WebSocket client")

	for audioSample := range audioCh {
		audioFrameCount++

		// Log first few frames to verify we're receiving audio data
		if audioFrameCount <= 5 || audioFrameCount%50 == 0 {
			logger.Info("🎵 Received audio sample", "frame", audioFrameCount, "size", len(audioSample.Data))
		}

		// Send Opus data wrapped in minimal OGG container for browser compatibility
		if len(audioSample.Data) > 0 {
			// Check if client requested OGG format
			format := r.URL.Query().Get("format")
			var dataToSend []byte

			if format == "ogg" {
				// Wrap Opus data in minimal OGG page
				dataToSend = h.wrapOpusInOgg(audioSample.Data, audioFrameCount)
			} else {
				// Send raw Opus data
				dataToSend = audioSample.Data
			}

			err := conn.WriteMessage(websocket.BinaryMessage, dataToSend)
			if err != nil {
				logger.Error("❌ Failed to send audio data to WebSocket", "error", err, "frame", audioFrameCount)
				break
			}

			// Log successful transmission for first few frames
			if audioFrameCount <= 5 {
				logger.Info("✅ Successfully sent audio data to WebSocket", "frame", audioFrameCount, "size", len(dataToSend), "format", format)
			}
		} else {
			logger.Warn("⚠️ Received empty audio sample", "frame", audioFrameCount)
		}
	}

	logger.Info("🎵 Audio stream ended", "totalFrames", audioFrameCount)
}

// wrapOpusInOgg wraps Opus audio data in a minimal OGG container for browser playback
func (h *StreamingHandlers) wrapOpusInOgg(opusData []byte, frameNumber int) []byte {
	// Send only OpusHead for first frame
	if frameNumber == 1 {
		return h.createOpusHead()
	}

	// Send only OpusTags for second frame
	if frameNumber == 2 {
		return h.createOpusTags()
	}

	// For frame 3 onwards, send audio data
	if frameNumber >= 3 {
		return h.createOpusAudioPage(opusData, frameNumber) // Use frame number directly for proper sequencing
	}

	// Should never reach here
	return []byte{}
}

// createOpusAudioPage creates an OGG page containing Opus audio data
func (h *StreamingHandlers) createOpusAudioPage(opusData []byte, pageSeq int) []byte {
	segmentLength := len(opusData)
	if segmentLength > 255 {
		segmentLength = 255 // OGG segment max length
		opusData = opusData[:255]
	}

	// Calculate granule position (cumulative sample position)
	// For 20ms frames at 48kHz: each frame = 960 samples
	// Page sequence starts at 3 for audio data (after OpusHead=0, OpusTags=1)
	granulePos := uint64(pageSeq-2) * 960 // pageSeq 3 -> granule 960, pageSeq 4 -> granule 1920, etc.

	oggHeader := []byte{
		0x4F, 0x67, 0x67, 0x53, // "OggS" magic signature
		0x00,                                                                                    // Version
		0x00,                                                                                    // Header type (0x00 = continuation)
		byte(granulePos), byte(granulePos >> 8), byte(granulePos >> 16), byte(granulePos >> 24), // Granule position (lower 4 bytes)
		byte(granulePos >> 32), byte(granulePos >> 40), byte(granulePos >> 48), byte(granulePos >> 56), // Granule position (upper 4 bytes)
		0x01, 0x00, 0x00, 0x00, // Serial number (4 bytes) - stream 1
		byte(pageSeq & 0xFF), byte((pageSeq >> 8) & 0xFF), byte((pageSeq >> 16) & 0xFF), byte((pageSeq >> 24) & 0xFF), // Page sequence number
		0x00, 0x00, 0x00, 0x00, // CRC checksum (4 bytes) - calculated below
		0x01,                // Number of page segments
		byte(segmentLength), // Segment table
	}

	// Combine header with data
	result := make([]byte, len(oggHeader)+len(opusData))
	copy(result, oggHeader)
	copy(result[len(oggHeader):], opusData)

	// Calculate and set CRC checksum
	crc := h.calculateOggCRC(result)
	result[22] = byte(crc & 0xFF)
	result[23] = byte((crc >> 8) & 0xFF)
	result[24] = byte((crc >> 16) & 0xFF)
	result[25] = byte((crc >> 24) & 0xFF)

	return result
}

// createOpusHead creates the OpusHead identification header
func (h *StreamingHandlers) createOpusHead() []byte {
	opusHead := []byte{
		0x4F, 0x70, 0x75, 0x73, 0x48, 0x65, 0x61, 0x64, // "OpusHead"
		0x01,       // Version
		0x02,       // Channel count (stereo)
		0x00, 0x00, // Pre-skip (0 samples) - little endian, let decoder handle
		0x80, 0xBB, 0x00, 0x00, // Sample rate (48000 Hz) - little endian
		0x00, 0x00, // Output gain (0 dB) - little endian
		0x00, // Channel mapping family (0 = RTP mapping)
	}

	// Wrap in OGG page
	oggHeader := []byte{
		0x4F, 0x67, 0x67, 0x53, // "OggS"
		0x00,                                           // Version
		0x02,                                           // Header type (beginning of stream)
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // Granule position
		0x01, 0x00, 0x00, 0x00, // Serial number
		0x00, 0x00, 0x00, 0x00, // Page sequence (0)
		0x00, 0x00, 0x00, 0x00, // CRC
		0x01,                // Segments
		byte(len(opusHead)), // Segment length
	}

	result := make([]byte, len(oggHeader)+len(opusHead))
	copy(result, oggHeader)
	copy(result[len(oggHeader):], opusHead)

	// Calculate and set CRC
	crc := h.calculateOggCRC(result)
	result[22] = byte(crc & 0xFF)
	result[23] = byte((crc >> 8) & 0xFF)
	result[24] = byte((crc >> 16) & 0xFF)
	result[25] = byte((crc >> 24) & 0xFF)

	return result
}

// createOpusTags creates the OpusTags comment header
func (h *StreamingHandlers) createOpusTags() []byte {
	vendor := "libopus"
	opusTags := make([]byte, 0)

	// OpusTags header
	opusTags = append(opusTags, []byte("OpusTags")...)

	// Vendor string length (little endian)
	vendorLen := len(vendor)
	opusTags = append(opusTags, byte(vendorLen), byte(vendorLen>>8), byte(vendorLen>>16), byte(vendorLen>>24))

	// Vendor string
	opusTags = append(opusTags, vendor...)

	// User comment list length (0 comments)
	opusTags = append(opusTags, 0x00, 0x00, 0x00, 0x00)

	// Wrap in OGG page
	oggHeader := []byte{
		0x4F, 0x67, 0x67, 0x53, // "OggS"
		0x00,                                           // Version
		0x00,                                           // Header type (continuation)
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // Granule position
		0x01, 0x00, 0x00, 0x00, // Serial number
		0x01, 0x00, 0x00, 0x00, // Page sequence (1)
		0x00, 0x00, 0x00, 0x00, // CRC
		0x01,                // Segments
		byte(len(opusTags)), // Segment length
	}

	result := make([]byte, len(oggHeader)+len(opusTags))
	copy(result, oggHeader)
	copy(result[len(oggHeader):], opusTags)

	// Calculate and set CRC
	crc := h.calculateOggCRC(result)
	result[22] = byte(crc & 0xFF)
	result[23] = byte((crc >> 8) & 0xFF)
	result[24] = byte((crc >> 16) & 0xFF)
	result[25] = byte((crc >> 24) & 0xFF)

	return result
}

// OGG CRC lookup table
var oggCRCTable = [256]uint32{
	0x00000000, 0x04c11db7, 0x09823b6e, 0x0d4326d9, 0x130476dc, 0x17c56b6b,
	0x1a864db2, 0x1e475005, 0x2608edb8, 0x22c9f00f, 0x2f8ad6d6, 0x2b4bcb61,
	0x350c9b64, 0x31cd86d3, 0x3c8ea00a, 0x384fbdbd, 0x4c11db70, 0x48d0c6c7,
	0x4593e01e, 0x4152fda9, 0x5f15adac, 0x5bd4b01b, 0x569796c2, 0x52568b75,
	0x6a1936c8, 0x6ed82b7f, 0x639b0da6, 0x675a1011, 0x791d4014, 0x7ddc5da3,
	0x709f7b7a, 0x745e66cd, 0x9823b6e0, 0x9ce2ab57, 0x91a18d8e, 0x95609039,
	0x8b27c03c, 0x8fe6dd8b, 0x82a5fb52, 0x8664e6e5, 0xbe2b5b58, 0xbaea46ef,
	0xb7a96036, 0xb3687d81, 0xad2f2d84, 0xa9ee3033, 0xa4ad16ea, 0xa06c0b5d,
	0xd4326d90, 0xd0f37027, 0xddb056fe, 0xd9714b49, 0xc7361b4c, 0xc3f706fb,
	0xceb42022, 0xca753d95, 0xf23a8028, 0xf6fb9d9f, 0xfbb8bb46, 0xff79a6f1,
	0xe13ef6f4, 0xe5ffeb43, 0xe8bccd9a, 0xec7dd02d, 0x34867077, 0x30476dc0,
	0x3d044b19, 0x39c556ae, 0x278206ab, 0x23431b1c, 0x2e003dc5, 0x2ac12072,
	0x128e9dcf, 0x164f8078, 0x1b0ca6a1, 0x1fcdbb16, 0x018aeb13, 0x054bf6a4,
	0x0808d07d, 0x0cc9cdca, 0x7897ab07, 0x7c56b6b0, 0x71159069, 0x75d48dde,
	0x6b93dddb, 0x6f52c06c, 0x6211e6b5, 0x66d0fb02, 0x5e9f46bf, 0x5a5e5b08,
	0x571d7dd1, 0x53dc6066, 0x4d9b3063, 0x495a2dd4, 0x44190b0d, 0x40d816ba,
	0xaca5c697, 0xa864db20, 0xa527fdf9, 0xa1e6e04e, 0xbfa1b04b, 0xbb60adfc,
	0xb6238b25, 0xb2e29692, 0x8aad2b2f, 0x8e6c3698, 0x832f1041, 0x87ee0df6,
	0x99a95df3, 0x9d684044, 0x902b669d, 0x94ea7b2a, 0xe0b41de7, 0xe4750050,
	0xe9362689, 0xedf73b3e, 0xf3b06b3b, 0xf771768c, 0xfa325055, 0xfef34de2,
	0xc6bcf05f, 0xc27dede8, 0xcf3ecb31, 0xcbffd686, 0xd5b88683, 0xd1799b34,
	0xdc3abded, 0xd8fba05a, 0x690ce0ee, 0x6dcdfd59, 0x608edb80, 0x644fc637,
	0x7a089632, 0x7ec98b85, 0x738aad5c, 0x774bb0eb, 0x4f040d56, 0x4bc510e1,
	0x46863638, 0x42472b8f, 0x5c007b8a, 0x58c1663d, 0x558240e4, 0x51435d53,
	0x251d3b9e, 0x21dc2629, 0x2c9f00f0, 0x285e1d47, 0x36194d42, 0x32d850f5,
	0x3f9b762c, 0x3b5a6b9b, 0x0315d626, 0x07d4cb91, 0x0a97ed48, 0x0e56f0ff,
	0x1011a0fa, 0x14d0bd4d, 0x19939b94, 0x1d528623, 0xf12f560e, 0xf5ee4bb9,
	0xf8ad6d60, 0xfc6c70d7, 0xe22b20d2, 0xe6ea3d65, 0xeba91bbc, 0xef68060b,
	0xd727bbb6, 0xd3e6a601, 0xdea580d8, 0xda649d6f, 0xc423cd6a, 0xc0e2d0dd,
	0xcda1f604, 0xc960ebb3, 0xbd3e8d7e, 0xb9ff90c9, 0xb4bcb610, 0xb07daba7,
	0xae3afba2, 0xaafbe615, 0xa7b8c0cc, 0xa379dd7b, 0x9b3660c6, 0x9ff77d71,
	0x92b45ba8, 0x9675461f, 0x8832161a, 0x8cf30bad, 0x81b02d74, 0x857130c3,
	0x5d8a9099, 0x594b8d2e, 0x5408abf7, 0x50c9b640, 0x4e8ee645, 0x4a4ffbf2,
	0x470cdd2b, 0x43cdc09c, 0x7b827d21, 0x7f436096, 0x7200464f, 0x76c15bf8,
	0x68860bfd, 0x6c47164a, 0x61043093, 0x65c52d24, 0x119b4be9, 0x155a565e,
	0x18197087, 0x1cd86d30, 0x029f3d35, 0x065e2082, 0x0b1d065b, 0x0fdc1bec,
	0x3793a651, 0x3352bbe6, 0x3e119d3f, 0x3ad08088, 0x2497d08d, 0x2056cd3a,
	0x2d15ebe3, 0x29d4f654, 0xc5a92679, 0xc1683bce, 0xcc2b1d17, 0xc8ea00a0,
	0xd6ad50a5, 0xd26c4d12, 0xdf2f6bcb, 0xdbee767c, 0xe3a1cbc1, 0xe760d676,
	0xea23f0af, 0xeee2ed18, 0xf0a5bd1d, 0xf464a0aa, 0xf9278673, 0xfde69bc4,
	0x89b8fd09, 0x8d79e0be, 0x803ac667, 0x84fbdbd0, 0x9abc8bd5, 0x9e7d9662,
	0x933eb0bb, 0x97ffad0c, 0xafb010b1, 0xab710d06, 0xa6322bdf, 0xa2f33668,
	0xbcb4666d, 0xb8757bda, 0xb5365d03, 0xb1f740b4,
}

// calculateOggCRC calculates proper OGG CRC-32 checksum
func (h *StreamingHandlers) calculateOggCRC(data []byte) uint32 {
	crc := uint32(0)

	// Set CRC field to 0 before calculation
	if len(data) >= 26 {
		data[22] = 0
		data[23] = 0
		data[24] = 0
		data[25] = 0
	}

	for _, b := range data {
		crc = (crc << 8) ^ oggCRCTable[((crc>>24)^uint32(b))&0xFF]
	}

	return crc
}

// createTestOGG creates a minimal test OGG file with silence to validate format
func (h *StreamingHandlers) createTestOGG() []byte {
	// Create OpusHead page
	opusHead := h.createOpusHead()

	// Create OpusTags page
	opusTags := h.createOpusTags()

	// Create a few silence frames (Opus silence frame is just TOC byte + minimal data)
	silenceFrame := []byte{0xFC, 0x00} // TOC byte + minimal silence data

	// Create audio pages with silence
	audioPage1 := h.createOpusAudioPage(silenceFrame, 2)
	audioPage2 := h.createOpusAudioPage(silenceFrame, 3)
	audioPage3 := h.createOpusAudioPage(silenceFrame, 4)

	// Combine all pages
	totalLen := len(opusHead) + len(opusTags) + len(audioPage1) + len(audioPage2) + len(audioPage3)
	result := make([]byte, totalLen)

	offset := 0
	copy(result[offset:], opusHead)
	offset += len(opusHead)

	copy(result[offset:], opusTags)
	offset += len(opusTags)

	copy(result[offset:], audioPage1)
	offset += len(audioPage1)

	copy(result[offset:], audioPage2)
	offset += len(audioPage2)

	copy(result[offset:], audioPage3)

	return result
}

// handleOpusAudioHTTP handles HTTP-based Opus audio streaming for direct browser testing
func (h *StreamingHandlers) handleOpusAudioHTTP(w http.ResponseWriter, r *http.Request, deviceSerial string) {
	logger := slog.With("device", deviceSerial)
	logger.Info("🎵 Starting Opus audio HTTP stream", "url", r.URL.String())

	// Set headers for audio streaming
	format := r.URL.Query().Get("format")
	saveFile := r.URL.Query().Get("save") == "true"
	debug := r.URL.Query().Get("debug") == "true"
	test := r.URL.Query().Get("test") == "true"

	if test {
		// Send a minimal test OGG file to validate our format
		w.Header().Set("Content-Type", "audio/ogg; codecs=opus")
		logger.Info("🧪 Serving test OGG file")
		testData := h.createTestOGG()
		w.Write(testData)
		return
	} else if debug {
		w.Header().Set("Content-Type", "text/plain")
		logger.Info("🔍 Serving debug mode - will show hex dump of first 10 frames")
	} else if format == "ogg" {
		if saveFile {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Disposition", "attachment; filename=\"audio.ogg\"")
			logger.Info("🎵 Serving OGG/Opus as download")
		} else {
			w.Header().Set("Content-Type", "audio/ogg; codecs=opus")
			logger.Info("🎵 Serving OGG/Opus format")
		}
	} else {
		if saveFile {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Disposition", "attachment; filename=\"audio.opus\"")
			logger.Info("🎵 Serving raw Opus as download")
		} else {
			w.Header().Set("Content-Type", "audio/opus")
			logger.Info("🎵 Serving raw Opus format")
		}
	}

	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Get audio stream from device source
	source := scrcpy.GetSource(deviceSerial)
	if source == nil {
		logger.Error("❌ Device source not found - is scrcpy running for this device?")
		http.Error(w, "Device not connected", http.StatusServiceUnavailable)
		return
	}

	logger.Info("✅ Found scrcpy source for HTTP audio streaming")

	// Subscribe to audio stream
	subscriberID := fmt.Sprintf("audio_http_%p", w)
	audioCh := source.SubscribeAudio(subscriberID, 100)
	defer source.UnsubscribeAudio(subscriberID)

	logger.Info("🎵 Subscribed to audio stream", "subscriberID", subscriberID)

	// Stream audio data to client
	audioFrameCount := 0
	logger.Info("🎵 Starting to stream audio data to HTTP client")

	for {
		select {
		case <-r.Context().Done():
			logger.Info("🎵 HTTP audio stream context cancelled")
			return

		case audioSample, ok := <-audioCh:
			if !ok {
				logger.Info("🎵 HTTP audio channel closed")
				return
			}

			audioFrameCount++

			// Log first few frames to verify we're receiving audio data
			if audioFrameCount <= 5 || audioFrameCount%50 == 0 {
				logger.Info("🎵 Received audio sample for HTTP", "frame", audioFrameCount, "size", len(audioSample.Data))
			}

			// Debug: Log raw data for first few frames to understand format
			if audioFrameCount <= 3 && len(audioSample.Data) > 0 {
				hexData := ""
				dataLen := len(audioSample.Data)
				if dataLen > 16 {
					dataLen = 16
				}
				for i, b := range audioSample.Data[:dataLen] {
					if i > 0 {
						hexData += " "
					}
					hexData += fmt.Sprintf("%02x", b)
				}
				logger.Info("🔍 Raw audio data", "frame", audioFrameCount, "hex", hexData)
			}

			// Send audio data
			if len(audioSample.Data) > 0 {
				var dataToSend []byte

				if debug {
					// Debug mode: output hex dump of first 10 frames
					if audioFrameCount <= 10 {
						debugText := fmt.Sprintf("Frame %d (size %d bytes):\n", audioFrameCount, len(audioSample.Data))

						// Add hex dump
						dataLen := len(audioSample.Data)
						if dataLen > 64 {
							dataLen = 64 // Limit to first 64 bytes
						}

						for i := 0; i < dataLen; i += 16 {
							end := i + 16
							if end > dataLen {
								end = dataLen
							}

							// Hex part
							hexPart := ""
							for j := i; j < end; j++ {
								hexPart += fmt.Sprintf("%02x ", audioSample.Data[j])
							}
							// Pad hex part to 48 characters (16 * 3)
							for len(hexPart) < 48 {
								hexPart += " "
							}

							// ASCII part
							asciiPart := ""
							for j := i; j < end; j++ {
								if audioSample.Data[j] >= 32 && audioSample.Data[j] <= 126 {
									asciiPart += string(audioSample.Data[j])
								} else {
									asciiPart += "."
								}
							}

							debugText += fmt.Sprintf("%04x: %s |%s|\n", i, hexPart, asciiPart)
						}
						debugText += "\n"

						if _, err := w.Write([]byte(debugText)); err != nil {
							logger.Error("❌ Failed to write debug data", "error", err)
							return
						}
					}

					// Stop after 10 frames in debug mode
					if audioFrameCount >= 10 {
						return
					}
				} else if format == "ogg" {
					// Wrap Opus data in minimal OGG page
					dataToSend = h.wrapOpusInOgg(audioSample.Data, audioFrameCount)

					// Debug: Log first few OGG pages
					if audioFrameCount <= 3 {
						logger.Info("🔧 OGG page created", "frame", audioFrameCount, "size", len(dataToSend))
						if len(dataToSend) >= 4 {
							header := string(dataToSend[0:4])
							logger.Info("🔧 OGG page header", "frame", audioFrameCount, "header", header)
						}
					}
				} else {
					// Send raw Opus data
					dataToSend = audioSample.Data
				}

				if !debug {
					if _, err := w.Write(dataToSend); err != nil {
						logger.Error("❌ Failed to write audio data to HTTP response", "error", err, "frame", audioFrameCount)
						return
					}

					// Flush data immediately for low latency
					if f, ok := w.(http.Flusher); ok {
						f.Flush()
					}

					// Log successful transmission for first few frames
					if audioFrameCount <= 5 {
						logger.Info("✅ Successfully sent audio data to HTTP client", "frame", audioFrameCount, "size", len(dataToSend), "format", format)
					}
				}
			} else {
				logger.Warn("⚠️ Received empty audio sample for HTTP", "frame", audioFrameCount)
			}
		}
	}
}

// handleRTPOpusHTTP handles RTP-wrapped Opus audio streaming for FFmpeg compatibility
func (h *StreamingHandlers) handleRTPOpusHTTP(w http.ResponseWriter, r *http.Request, deviceSerial string) {
	logger := slog.With("device", deviceSerial)
	logger.Info("🎵 Starting RTP/Opus audio HTTP stream", "url", r.URL.String())

	// Set headers for RTP streaming
	w.Header().Set("Content-Type", "application/rtp")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Get audio stream from device source
	source := scrcpy.GetSource(deviceSerial)
	if source == nil {
		logger.Error("❌ Device source not found - is scrcpy running for this device?")
		http.Error(w, "Device not connected", http.StatusServiceUnavailable)
		return
	}

	logger.Info("✅ Found scrcpy source for RTP/Opus streaming")

	// Subscribe to audio stream
	subscriberID := fmt.Sprintf("audio_rtp_%p", w)
	audioCh := source.SubscribeAudio(subscriberID, 100)
	defer source.UnsubscribeAudio(subscriberID)

	logger.Info("🎵 Subscribed to RTP/Opus stream", "subscriberID", subscriberID)

	// Stream RTP-wrapped audio data to client
	audioFrameCount := 0
	sequenceNumber := uint16(1)
	timestamp := uint32(0)
	ssrc := uint32(0x12345678) // Random SSRC

	logger.Info("🎵 Starting to stream RTP/Opus data to HTTP client")

	for {
		select {
		case <-r.Context().Done():
			logger.Info("🎵 RTP/Opus stream context cancelled")
			return

		case audioSample, ok := <-audioCh:
			if !ok {
				logger.Info("🎵 RTP/Opus channel closed")
				return
			}

			audioFrameCount++

			// Log first few frames to verify we're receiving audio data
			if audioFrameCount <= 5 || audioFrameCount%50 == 0 {
				logger.Info("🎵 Received audio sample for RTP", "frame", audioFrameCount, "size", len(audioSample.Data))
			}

			// Wrap Opus data in RTP packet
			if len(audioSample.Data) > 0 {
				rtpPacket := h.createRTPPacket(audioSample.Data, sequenceNumber, timestamp, ssrc)

				if _, err := w.Write(rtpPacket); err != nil {
					logger.Error("❌ Failed to write RTP data to HTTP response", "error", err, "frame", audioFrameCount)
					return
				}

				// Flush data immediately for low latency
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}

				// Update RTP sequence and timestamp
				sequenceNumber++
				timestamp += 960 // 20ms at 48kHz = 960 samples

				// Log successful transmission for first few frames
				if audioFrameCount <= 5 {
					logger.Info("✅ Successfully sent RTP/Opus data to HTTP client", "frame", audioFrameCount, "size", len(rtpPacket))
				}
			} else {
				logger.Warn("⚠️ Received empty audio sample for RTP", "frame", audioFrameCount)
			}
		}
	}
}

// createRTPPacket creates an RTP packet for Opus audio data
func (h *StreamingHandlers) createRTPPacket(opusData []byte, seqNum uint16, timestamp uint32, ssrc uint32) []byte {
	// RTP header: 12 bytes
	// V(2) P(1) X(1) CC(4) = 0x80 (version 2, no padding, no extension, no CSRC)
	// M(1) PT(7) = 0x60 (marker=0, payload type 96 for dynamic Opus)
	// Sequence number (2 bytes)
	// Timestamp (4 bytes)
	// SSRC (4 bytes)

	rtpHeader := make([]byte, 12)
	rtpHeader[0] = 0x80 // V=2, P=0, X=0, CC=0
	rtpHeader[1] = 96   // M=0, PT=96 (dynamic payload type for Opus)

	// Sequence number (big endian)
	rtpHeader[2] = byte(seqNum >> 8)
	rtpHeader[3] = byte(seqNum & 0xFF)

	// Timestamp (big endian)
	rtpHeader[4] = byte(timestamp >> 24)
	rtpHeader[5] = byte(timestamp >> 16)
	rtpHeader[6] = byte(timestamp >> 8)
	rtpHeader[7] = byte(timestamp & 0xFF)

	// SSRC (big endian)
	rtpHeader[8] = byte(ssrc >> 24)
	rtpHeader[9] = byte(ssrc >> 16)
	rtpHeader[10] = byte(ssrc >> 8)
	rtpHeader[11] = byte(ssrc & 0xFF)

	// Combine header with Opus payload
	rtpPacket := make([]byte, len(rtpHeader)+len(opusData))
	copy(rtpPacket, rtpHeader)
	copy(rtpPacket[len(rtpHeader):], opusData)

	return rtpPacket
}

// handleSDPFile generates an SDP file for RTP/Opus streaming
func (h *StreamingHandlers) handleSDPFile(w http.ResponseWriter, r *http.Request, deviceSerial string) {
	logger := slog.With("device", deviceSerial)
	logger.Info("📄 Serving SDP file for RTP/Opus stream", "device", deviceSerial)

	// Build the RTP stream URL
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}

	host := r.Host
	if host == "" {
		host = "localhost:29888" // fallback
	}

	rtpURL := fmt.Sprintf("%s://%s/api/stream/audio/%s?codec=rtp", scheme, host, deviceSerial)

	// Generate SDP content
	sdpContent := fmt.Sprintf(`v=0
o=- 0 0 IN IP4 127.0.0.1
s=Scrcpy Audio Stream
c=IN IP4 127.0.0.1
t=0 0
m=audio 0 RTP/AVP 96
a=rtpmap:96 opus/48000/2
a=fmtp:96 sprop-stereo=1
a=tool:scrcpy-gbox
a=source-filter: incl IN IP4 127.0.0.1 %s
`, rtpURL)

	// Set content type and headers
	w.Header().Set("Content-Type", "application/sdp")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s_audio.sdp\"", deviceSerial))
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Write SDP content
	if _, err := w.Write([]byte(sdpContent)); err != nil {
		logger.Error("Failed to write SDP content", "error", err)
		return
	}

	logger.Info("✅ SDP file served successfully", "device", deviceSerial)
}

// handleWebMOpusHTTP handles WebM-wrapped Opus audio streaming for FFmpeg compatibility
func (h *StreamingHandlers) handleWebMOpusHTTP(w http.ResponseWriter, r *http.Request, deviceSerial string) {
	logger := slog.With("device", deviceSerial)
	logger.Info("🎵 Starting WebM/Opus audio HTTP stream", "url", r.URL.String())

	// Set headers for WebM streaming
	w.Header().Set("Content-Type", "audio/webm; codecs=opus")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Get audio stream from device source
	source := scrcpy.GetSource(deviceSerial)
	if source == nil {
		logger.Error("❌ Device source not found - is scrcpy running for this device?")
		http.Error(w, "Device not connected", http.StatusServiceUnavailable)
		return
	}

	logger.Info("✅ Found scrcpy source for WebM/Opus streaming")

	// Subscribe to audio stream
	subscriberID := fmt.Sprintf("audio_webm_%p", w)
	audioCh := source.SubscribeAudio(subscriberID, 100)
	defer source.UnsubscribeAudio(subscriberID)

	logger.Info("🎵 Subscribed to WebM/Opus stream", "subscriberID", subscriberID)

	// Write WebM header
	webmHeader := h.createWebMHeader()
	if _, err := w.Write(webmHeader); err != nil {
		logger.Error("❌ Failed to write WebM header", "error", err)
		return
	}

	logger.Info("✅ WebM header sent", "size", len(webmHeader))

	// Stream WebM-wrapped audio data to client
	audioFrameCount := 0
	timestamp := uint64(0)

	logger.Info("🎵 Starting to stream WebM/Opus data to HTTP client")

	for {
		select {
		case <-r.Context().Done():
			logger.Info("🎵 WebM/Opus stream context cancelled")
			return

		case audioSample, ok := <-audioCh:
			if !ok {
				logger.Info("🎵 WebM/Opus channel closed")
				return
			}

			audioFrameCount++

			// Log first few frames to verify we're receiving audio data
			if audioFrameCount <= 5 || audioFrameCount%50 == 0 {
				logger.Info("🎵 Received audio sample for WebM", "frame", audioFrameCount, "size", len(audioSample.Data))
			}

			// Wrap Opus data in WebM SimpleBlock
			if len(audioSample.Data) > 0 {
				webmBlock := h.createWebMSimpleBlock(audioSample.Data, timestamp, audioFrameCount == 1)

				if _, err := w.Write(webmBlock); err != nil {
					logger.Error("❌ Failed to write WebM data to HTTP response", "error", err, "frame", audioFrameCount)
					return
				}

				// Flush data immediately for low latency
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}

				// Update timestamp (20ms per frame = 960 samples at 48kHz)
				timestamp += 20 // milliseconds

				// Log successful transmission for first few frames
				if audioFrameCount <= 5 {
					logger.Info("✅ Successfully sent WebM/Opus data to HTTP client", "frame", audioFrameCount, "size", len(webmBlock))
				}
			} else {
				logger.Warn("⚠️ Received empty audio sample for WebM", "frame", audioFrameCount)
			}
		}
	}
}

// createWebMHeader creates a minimal WebM header for Opus audio
func (h *StreamingHandlers) createWebMHeader() []byte {
	// This is a simplified WebM header for audio-only Opus stream
	// EBML Header + Segment + Info + Tracks
	header := []byte{
		// EBML Header
		0x1A, 0x45, 0xDF, 0xA3, // EBML ID
		0x9F,                   // Size (unknown/live stream)
		0x42, 0x86, 0x81, 0x01, // EBMLVersion = 1
		0x42, 0xF7, 0x81, 0x01, // EBMLReadVersion = 1
		0x42, 0xF2, 0x81, 0x04, // EBMLMaxIDLength = 4
		0x42, 0xF3, 0x81, 0x08, // EBMLMaxSizeLength = 8
		0x42, 0x82, 0x88, 0x77, 0x65, 0x62, 0x6D, 0x00, 0x00, 0x00, 0x00, // DocType = "webm"
		0x42, 0x87, 0x81, 0x04, // DocTypeVersion = 4
		0x42, 0x85, 0x81, 0x02, // DocTypeReadVersion = 2

		// Segment
		0x18, 0x53, 0x80, 0x67, // Segment ID
		0x01, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, // Size (unknown/live stream)

		// Info
		0x15, 0x49, 0xA9, 0x66, // Info ID
		0x8A,                                     // Size = 10
		0x2A, 0xD7, 0xB1, 0x83, 0x0F, 0x42, 0x40, // TimestampScale = 1000000 (1ms)

		// Tracks
		0x16, 0x54, 0xAE, 0x6B, // Tracks ID
		0x90, // Size = 16
		// TrackEntry
		0xAE,             // TrackEntry ID
		0x8D,             // Size = 13
		0xD7, 0x81, 0x01, // TrackNumber = 1
		0x73, 0xC5, 0x81, 0x01, // TrackUID = 1
		0x83, 0x81, 0x02, // TrackType = 2 (audio)
		0x86, 0x85, 0x6F, 0x70, 0x75, 0x73, 0x00, // CodecID = "A_OPUS"
	}

	return header
}

// createWebMSimpleBlock creates a WebM SimpleBlock for Opus audio data
func (h *StreamingHandlers) createWebMSimpleBlock(opusData []byte, timestamp uint64, keyframe bool) []byte {
	// SimpleBlock: Element ID + Size + Track Number + Timestamp + Flags + Data

	// Calculate size (track number + timestamp + flags + data)
	dataSize := 1 + 2 + 1 + len(opusData) // track(1) + timestamp(2) + flags(1) + data

	block := make([]byte, 0, 4+8+dataSize) // ID(4) + size(up to 8) + data

	// SimpleBlock Element ID
	block = append(block, 0xA3)

	// Size (variable length encoding)
	if dataSize < 127 {
		block = append(block, 0x80|byte(dataSize))
	} else {
		block = append(block, 0x40, byte(dataSize>>8), byte(dataSize&0xFF))
	}

	// Track number (1 = audio track)
	block = append(block, 0x81) // Track 1

	// Timestamp (relative to cluster, 16-bit signed)
	block = append(block, byte(timestamp>>8), byte(timestamp&0xFF))

	// Flags (keyframe flag)
	flags := byte(0x00)
	if keyframe {
		flags |= 0x80 // Keyframe flag
	}
	block = append(block, flags)

	// Opus data
	block = append(block, opusData...)

	return block
}

// handleOpusStreamHTTP handles properly formatted Opus audio streaming
func (h *StreamingHandlers) handleOpusStreamHTTP(w http.ResponseWriter, r *http.Request, deviceSerial string) {
	logger := slog.With("device", deviceSerial)
	logger.Info("🎵 Starting Opus stream HTTP", "url", r.URL.String())

	// Set headers for Opus streaming
	w.Header().Set("Content-Type", "audio/opus")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Get audio stream from device source
	source := scrcpy.GetSource(deviceSerial)
	if source == nil {
		logger.Error("❌ Device source not found - is scrcpy running for this device?")
		http.Error(w, "Device not connected", http.StatusServiceUnavailable)
		return
	}

	logger.Info("✅ Found scrcpy source for Opus streaming")

	// Subscribe to audio stream
	subscriberID := fmt.Sprintf("audio_opus_stream_%p", w)
	audioCh := source.SubscribeAudio(subscriberID, 100)
	defer source.UnsubscribeAudio(subscriberID)

	logger.Info("🎵 Subscribed to Opus stream", "subscriberID", subscriberID)

	// Process audio stream
	audioFrameCount := 0

	logger.Info("🎵 Starting to stream Opus data to HTTP client")

	for {
		select {
		case <-r.Context().Done():
			logger.Info("🎵 Opus stream context cancelled")
			return

		case audioSample, ok := <-audioCh:
			if !ok {
				logger.Info("🎵 Opus channel closed")
				return
			}

			audioFrameCount++

			// Log first few frames
			if audioFrameCount <= 5 || audioFrameCount%50 == 0 {
				logger.Info("🎵 Received audio sample for Opus stream", "frame", audioFrameCount, "size", len(audioSample.Data))
			}

			if len(audioSample.Data) > 0 {
				// Handle all packets like WebRTC does - no filtering, just send them
				// Log detailed info for first few packets to understand the stream
				if audioFrameCount <= 10 {
					isOpusHead := h.isOpusConfigPacket(audioSample.Data)
					logger.Info("🎵 Audio packet details", "frame", audioFrameCount, "size", len(audioSample.Data), "isOpusHead", isOpusHead)

					// Show hex dump for very first packet
					if audioFrameCount == 1 {
						hexData := ""
						dataLen := min(len(audioSample.Data), 32)
						for i, b := range audioSample.Data[:dataLen] {
							if i > 0 {
								hexData += " "
							}
							hexData += fmt.Sprintf("%02x", b)
						}
						logger.Info("🔍 First packet hex", "hex", hexData)
					}
				}

				// Send all audio data just like WebRTC does
				if _, err := w.Write(audioSample.Data); err != nil {
					logger.Error("❌ Failed to write Opus data to HTTP response", "error", err, "frame", audioFrameCount)
					return
				}

				// Flush data immediately for low latency
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}

				// Log successful transmission for first few frames
				if audioFrameCount <= 5 {
					logger.Info("✅ Successfully sent Opus data to HTTP client", "frame", audioFrameCount, "size", len(audioSample.Data))
				}
			} else {
				logger.Warn("⚠️ Received empty audio sample for Opus stream", "frame", audioFrameCount)
			}
		}
	}
}

// isOpusConfigPacket checks if the data contains an Opus configuration packet
func (h *StreamingHandlers) isOpusConfigPacket(data []byte) bool {
	// Check for OpusHead signature
	opusHead := []byte("OpusHead")
	if len(data) >= len(opusHead) {
		for i, b := range opusHead {
			if data[i] != b {
				return false
			}
		}
		return true
	}
	return false
}

// handleTestAudioHTTP handles test audio streaming that mimics WebRTC exactly
func (h *StreamingHandlers) handleTestAudioHTTP(w http.ResponseWriter, r *http.Request, deviceSerial string) {
	logger := slog.With("device", deviceSerial)
	logger.Info("🧪 Starting test audio HTTP stream", "url", r.URL.String())

	// Set headers
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Get audio stream from device source
	source := scrcpy.GetSource(deviceSerial)
	if source == nil {
		logger.Error("❌ Device source not found")
		http.Error(w, "Device not connected", http.StatusServiceUnavailable)
		return
	}

	// Subscribe to audio stream - exactly like WebRTC does
	audioCh := source.SubscribeAudio("test_audio_stream", 100)
	defer source.UnsubscribeAudio("test_audio_stream")

	sampleCount := 0
	logger.Info("🧪 Test audio streaming started")

	for {
		select {
		case <-r.Context().Done():
			logger.Info("🧪 Test audio stream cancelled")
			return
		case sample, ok := <-audioCh:
			if !ok {
				logger.Info("🧪 Test audio channel closed")
				return
			}

			// Skip empty samples just like WebRTC does
			if sample.Data == nil || len(sample.Data) == 0 {
				continue
			}

			sampleCount++

			// Log first few samples with detailed analysis
			if sampleCount <= 10 {
				logger.Info("🧪 Test audio sample", "count", sampleCount, "size", len(sample.Data), "pts", sample.PTS)

				// Show hex dump for first few samples
				if sampleCount <= 3 && len(sample.Data) > 0 {
					hexData := ""
					dataLen := min(len(sample.Data), 16)
					for i, b := range sample.Data[:dataLen] {
						if i > 0 {
							hexData += " "
						}
						hexData += fmt.Sprintf("%02x", b)
					}
					logger.Info("🧪 Sample hex", "count", sampleCount, "hex", hexData)

					// Analyze Opus frame structure
					if len(sample.Data) > 0 {
						toc := sample.Data[0]
						config := (toc >> 3) & 0x1F
						stereo := (toc >> 2) & 0x1
						frameCount := toc & 0x3
						logger.Info("🧪 Opus analysis", "count", sampleCount, "toc", fmt.Sprintf("0x%02x", toc), "config", config, "stereo", stereo, "frames", frameCount)
					}
				}
			}

			// Write raw data exactly like WebRTC does
			if _, err := w.Write(sample.Data); err != nil {
				logger.Error("❌ Failed to write test audio data", "error", err)
				return
			}

			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}
}
