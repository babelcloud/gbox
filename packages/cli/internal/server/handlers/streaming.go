package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/control"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/transport/audio"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/transport/h264"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/transport/mse"
	"github.com/gorilla/websocket"
)

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

	// Parse format parameter
	format := r.URL.Query().Get("format")

	// Check for MSE-optimized WebM streaming
	mseOptimized := r.URL.Query().Get("mse") == "true"

	// Handle MSE-optimized WebM streaming (new approach)
	if codec == "opus" && format == "webm" && mseOptimized {
		audioService := audio.GetAudioService()
		if err := audioService.StreamWebMForMSE(deviceSerial, w, r); err != nil {
			log.Printf("MSE WebM streaming error: %v", err)
			http.Error(w, "MSE streaming failed", http.StatusInternalServerError)
		}
		return
	}

	// Only support Opus codec with WebM format
	if codec != "opus" {
		http.Error(w, "Invalid codec. Only 'opus' is supported", http.StatusBadRequest)
		return
	}

	// Set WebM/Opus content type with chunked encoding for binary streaming
	w.Header().Set("Content-Type", "audio/webm; codecs=opus")
	w.Header().Set("Transfer-Encoding", "chunked") // Critical for binary data streaming
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Write headers immediately to start the stream
	w.WriteHeader(http.StatusOK)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	// Stream Opus audio with WebM container (forced for consistency)
	audioService := audio.GetAudioService()
	audioService.StreamOpus(deviceSerial, w, "webm")
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
			"h264":  {"annexb", "avc"},
			"mse":   {"fmp4"},
			"audio": {"opus"},
		},
		"endpoints": map[string]string{
			"video":          h.buildURL(fmt.Sprintf("/stream/video/%s", deviceSerial)),
			"video_h264":     h.buildURL(fmt.Sprintf("/stream/video/%s?mode=h264", deviceSerial)),
			"video_h264_avc": h.buildURL(fmt.Sprintf("/stream/video/%s?mode=h264&format=avc", deviceSerial)),
			"video_mse":      h.buildURL(fmt.Sprintf("/stream/video/%s?mode=mse", deviceSerial)),
			"audio":          h.buildURL(fmt.Sprintf("/stream/audio/%s?codec=opus", deviceSerial)),
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

	// Delegate to control service
	controlService := control.GetControlService()

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

		log.Printf("[DEBUG] Control message received: type=%s, device=%s", msgType, deviceSerial)

		switch msgType {
		// WebRTC signaling messages - delegate to WebRTC handler
		case "ping", "offer", "answer", "ice-candidate":
			h.delegateToWebRTCHandler(conn, msg, msgType, deviceSerial)

		// Device control messages
		case "touch":
			controlService.HandleTouchEvent(msg, deviceSerial)

		case "key":
			controlService.HandleKeyEvent(msg, deviceSerial)

		case "scroll":
			controlService.HandleScrollEvent(msg, deviceSerial)

		case "clipboard_set":
			controlService.HandleClipboardEvent(msg, deviceSerial)

		case "reset_video":
			controlService.HandleVideoResetEvent(msg, deviceSerial)

		default:
			log.Printf("Unknown control message type: %s", msgType)
		}
	}
}

// delegateToWebRTCHandler forwards WebRTC signaling messages to the specialized handler
func (h *StreamingHandlers) delegateToWebRTCHandler(conn *websocket.Conn, msg map[string]interface{}, msgType, deviceSerial string) {
	if h.webrtcHandlers == nil {
		log.Printf("WebRTC handlers not initialized")
		return
	}

	switch msgType {
	case "ping":
		h.webrtcHandlers.HandlePing(conn, msg)
	case "offer":
		h.webrtcHandlers.HandleOffer(conn, msg, deviceSerial)
	case "answer":
		h.webrtcHandlers.HandleAnswer(conn, msg, deviceSerial)
	case "ice-candidate":
		h.webrtcHandlers.HandleIceCandidate(conn, msg, deviceSerial)
	}
}

var controlUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now
	},
}
