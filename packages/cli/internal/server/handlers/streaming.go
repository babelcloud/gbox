package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/control"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/scrcpy"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/transport/audio"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/transport/h264"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/transport/stream"
	"github.com/babelcloud/gbox/packages/cli/internal/util"
	"github.com/gorilla/websocket"
)

// setWebMStreamingHeaders sets HTTP headers for WebM audio streaming
func setWebMStreamingHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "audio/webm; codecs=opus")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Range")
}

// setRawOpusStreamingHeaders sets HTTP headers for raw Opus audio streaming
func setRawOpusStreamingHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "audio/opus")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
}

// startStreamingResponse sets headers and starts the streaming response
func startStreamingResponse(w http.ResponseWriter) {
	w.WriteHeader(http.StatusOK)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
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

// HandleVideoStream handles H.264 video streaming
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

	// Debug logging for request parameters

	switch mode {
	case "h264":
		// Check format parameter for AVC vs Annex-B
		if format == "avc" {
			// AVC format H.264 streaming (for WebCodecs)
			handler := h264.NewAVCHTTPHandler(deviceSerial)
			handler.ServeHTTP(w, r)
		} else {
			// Direct H.264 streaming (Annex-B format)
			handler := h264.NewAnnexBHandler(deviceSerial)
			handler.ServeHTTP(w, r)
		}

	case "webm":
		// WebM container streaming with H.264 video and Opus audio
		handler := stream.NewWebMHandler(deviceSerial)
		handler.ServeHTTP(w, r)

	case "mp4":
		// MP4 container streaming with H.264 video and Opus audio
		handler := stream.NewMP4Handler(deviceSerial)
		handler.ServeHTTP(w, r)

	default:
		http.Error(w, "Invalid mode. Supported: h264, webm, mp4", http.StatusBadRequest)
	}
}

// HandleAudioStream handles audio streaming endpoints
func (h *StreamingHandlers) HandleAudioStream(w http.ResponseWriter, r *http.Request) {
	// Extract device serial from path
	path := strings.TrimPrefix(r.URL.Path, "/stream/audio/")
	parts := strings.Split(path, "?")
	deviceSerial := parts[0]

	util.GetLogger().Debug("Processing audio stream", "device", deviceSerial, "url", r.URL.String())

	if deviceSerial == "" {
		http.Error(w, "Device serial required", http.StatusBadRequest)
		return
	}

	if !isValidDeviceSerial(deviceSerial) {
		http.Error(w, "Invalid device serial", http.StatusBadRequest)
		return
	}

	// Parse codec/format parameters
	codec := r.URL.Query().Get("codec")
	if codec == "" {
		codec = "aac" // default to AAC
	}
	format := r.URL.Query().Get("format")

	util.GetLogger().Debug("Audio parameters", "codec", codec, "format", format)

	switch codec {
	case "opus":
		// Determine streaming format and setup
		audioService := audio.GetAudioService()
		if format == "webm" {
			// WebM container streaming
			util.GetLogger().Debug("Using WebM streaming", "device", deviceSerial)
			setWebMStreamingHeaders(w)
			startStreamingResponse(w)
			if err := audioService.StreamWebM(deviceSerial, w, r); err != nil {
				util.GetLogger().Error("WebM streaming error", "error", err)
				http.Error(w, "WebM streaming failed", http.StatusInternalServerError)
			}
			return
		}
		// Raw Opus streaming
		util.GetLogger().Debug("Using raw Opus streaming", "device", deviceSerial)
		setRawOpusStreamingHeaders(w)
		startStreamingResponse(w)
		audioService.StreamOpus(deviceSerial, w)

	case "aac":
		// AAC dump: format=raw (default) or format=adts
		withADTS := strings.ToLower(format) == "adts"

		// Ensure scrcpy source in mp4 mode to use AAC encoder
		source := scrcpy.GetOrCreateSourceWithMode(deviceSerial, "mp4")
		scrcpy.StartSourceWithMode(deviceSerial, context.Background(), "mp4")

		// Subscribe to audio stream
		subscriberID := "audio_stream_" + deviceSerial + "_" + fmt.Sprint(time.Now().UnixNano())
		audioCh := source.SubscribeAudio(subscriberID, 1000)
		defer source.UnsubscribeAudio(subscriberID)

		// Set headers
		w.Header().Set("Content-Type", "audio/aac")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Transfer-Encoding", "chunked")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}

		// ADTS wrapper when requested
		writeADTS := func(frame []byte) []byte {
			if !withADTS || len(frame) == 0 {
				return frame
			}
			// Assume AAC LC, 48kHz, stereo by default
			profile := 1
			srIdx := adtsSamplingFreqIndex(48000)
			chCfg := 2
			aacFrameLen := len(frame) + 7
			hdr := make([]byte, 7)
			hdr[0] = 0xFF
			hdr[1] = 0xF1
			hdr[2] = byte(((profile & 0x3) << 6) | ((srIdx & 0xF) << 2) | ((chCfg >> 2) & 0x1))
			hdr[3] = byte(((chCfg & 0x3) << 6) | ((aacFrameLen >> 11) & 0x3))
			hdr[4] = byte((aacFrameLen >> 3) & 0xFF)
			hdr[5] = byte(((aacFrameLen & 0x7) << 5) | 0x1F)
			hdr[6] = 0xFC
			return append(hdr, frame...)
		}

		flusher, _ := w.(http.Flusher)
		for sample := range audioCh {
			data := writeADTS(sample.Data)
			if len(data) == 0 {
				continue
			}
			if _, err := w.Write(data); err != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}

	default:
		http.Error(w, "Invalid codec. Supported: opus, aac", http.StatusBadRequest)
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
		"supportedModes": []string{"h264", "webrtc"},
		"supportedFormats": map[string][]string{
			"h264":  {"annexb", "avc"},
			"audio": {"opus"},
		},
		"endpoints": map[string]string{
			"video":          h.buildURL(fmt.Sprintf("/stream/video/%s", deviceSerial)),
			"video_h264":     h.buildURL(fmt.Sprintf("/stream/video/%s?mode=h264", deviceSerial)),
			"video_h264_avc": h.buildURL(fmt.Sprintf("/stream/video/%s?mode=h264&format=avc", deviceSerial)),
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
		util.GetLogger().Error("Failed to upgrade control WebSocket", "error", err)
		return
	}
	defer conn.Close()

	util.GetLogger().Debug("Control WebSocket connection established", "device", deviceSerial)

	// Delegate to control service
	controlService := control.GetControlService()

	// Handle WebSocket messages
	for {
		var msg map[string]interface{}
		if err := conn.ReadJSON(&msg); err != nil {
			// Check for normal close conditions
			if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
				util.GetLogger().Debug("Control WebSocket closed normally", "device", deviceSerial)
			} else if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				util.GetLogger().Error("Control WebSocket read error", "error", err)
			}
			break
		}

		msgType, ok := msg["type"].(string)
		if !ok {
			continue
		}

		util.GetLogger().Debug("Control message received", "type", msgType, "device", deviceSerial)

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
			util.GetLogger().Warn("Unknown control message type", "type", msgType)
		}
	}
}

// delegateToWebRTCHandler forwards WebRTC signaling messages to the specialized handler
func (h *StreamingHandlers) delegateToWebRTCHandler(conn *websocket.Conn, msg map[string]interface{}, msgType, deviceSerial string) {
	if h.webrtcHandlers == nil {
		util.GetLogger().Warn("WebRTC handlers not initialized")
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
