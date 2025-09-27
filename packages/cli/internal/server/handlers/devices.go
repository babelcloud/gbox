package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/control"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/transport/audio"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/transport/h264"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/transport/stream"
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

var controlUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now
	},
}

// DeviceHandlers contains handlers for device management
type DeviceHandlers struct {
	serverService  ServerService
	upgrader       websocket.Upgrader
	webrtcHandlers *WebRTCHandlers
}

// NewDeviceHandlers creates a new device handlers instance
func NewDeviceHandlers(serverSvc ServerService) *DeviceHandlers {
	return &DeviceHandlers{
		serverService: serverSvc,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for now
			},
		},
		webrtcHandlers: NewWebRTCHandlers(serverSvc),
	}
}

// HandleDeviceList handles device listing requests
func (h *DeviceHandlers) HandleDeviceList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	devices, err := getADBDevices()
	if err != nil {
		log.Printf("Failed to get devices: %v", err)
		RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
			"devices": []interface{}{},
		})
		return
	}

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success":         true,
		"devices":         devices,
		"onDemandEnabled": true,
	})
}

// HandleDeviceAction handles device action requests (connect/disconnect)
func (h *DeviceHandlers) HandleDeviceAction(w http.ResponseWriter, r *http.Request) {
	// Extract device serial from path: /api/devices/{serial}
	path := strings.TrimPrefix(r.URL.Path, "/api/devices/")
	deviceID := strings.Split(path, "/")[0]

	if deviceID == "" {
		http.Error(w, "Device serial required", http.StatusBadRequest)
		return
	}

	// Handle connect/disconnect based on HTTP method
	switch r.Method {
	case http.MethodPost:
		// POST /api/devices/{serial} - connect device
		h.handleDeviceConnect(w, r, deviceID)
	case http.MethodDelete:
		// DELETE /api/devices/{serial} - disconnect device
		h.handleDeviceDisconnect(w, r, deviceID)
	default:
		RespondJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"success": false,
			"error":   "Method not allowed. Use POST to connect or DELETE to disconnect",
		})
	}
}

// HandleDeviceRegister handles device registration requests
func (h *DeviceHandlers) HandleDeviceRegister(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement device registration
	RespondJSON(w, http.StatusNotImplemented, map[string]interface{}{
		"success": false,
		"error":   "Device registration not yet implemented",
	})
}

// HandleDeviceUnregister handles device unregistration requests
func (h *DeviceHandlers) HandleDeviceUnregister(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement device unregistration
	RespondJSON(w, http.StatusNotImplemented, map[string]interface{}{
		"success": false,
		"error":   "Device unregistration not yet implemented",
	})
}

// Device streaming handlers
func (h *DeviceHandlers) HandleDeviceVideo(w http.ResponseWriter, req *http.Request) {
	// Extract device serial from path: /api/devices/{serial}/video
	path := strings.TrimPrefix(req.URL.Path, "/api/devices/")
	parts := strings.Split(path, "/")
	deviceSerial := parts[0]

	if deviceSerial == "" {
		http.Error(w, "Device serial required", http.StatusBadRequest)
		return
	}

	// Parse mode and format parameters
	query := req.URL.Query()
	codec := query.Get("codec")
	format := query.Get("format")
	mode := query.Get("mode") // Check if mode is already provided

	// Set mode based on codec and format, or use existing mode
	if mode == "" {
		if codec == "h264" {
			if format == "avc" {
				mode = "h264"
				format = "avc"
			} else if format == "annexb" {
				mode = "h264"
				format = "annexb"
			} else if format == "webm" {
				mode = "webm"
			} else {
				mode = "h264" // default to annexb
			}
		} else {
			mode = "h264" // default
		}
	}

	log.Printf("[HandleDeviceVideo] Processing video request for device: %s, mode: %s, format: %s", deviceSerial, mode, format)

	// Direct video streaming implementation
	switch mode {
	case "h264":
		// Check format parameter for AVC vs Annex-B
		if format == "avc" {
			// AVC format H.264 streaming (for WebCodecs)
			handler := h264.NewAVCHTTPHandler(deviceSerial)
			handler.ServeHTTP(w, req)
		} else {
			// Direct H.264 streaming (Annex-B format)
			handler := h264.NewAnnexBHandler(deviceSerial)
			handler.ServeHTTP(w, req)
		}

	case "webm":
		// WebM container streaming with H.264 video and Opus audio
		handler := stream.NewWebMHandler(deviceSerial)
		handler.ServeHTTP(w, req)

	case "mp4":
		// MP4 container streaming with H.264 video and Opus audio
		handler := stream.NewMP4Handler(deviceSerial)
		handler.ServeHTTP(w, req)

	default:
		http.Error(w, "Invalid mode. Supported: h264, webm, mp4", http.StatusBadRequest)
	}
}

func (h *DeviceHandlers) HandleDeviceAudio(w http.ResponseWriter, req *http.Request) {
	// Extract device serial from path: /api/devices/{serial}/audio
	path := strings.TrimPrefix(req.URL.Path, "/api/devices/")
	parts := strings.Split(path, "/")
	deviceSerial := parts[0]

	log.Printf("[HandleDeviceAudio] Processing audio request for device: %s, URL: %s", deviceSerial, req.URL.String())

	if deviceSerial == "" {
		http.Error(w, "Device serial required", http.StatusBadRequest)
		return
	}

	if !isValidDeviceSerial(deviceSerial) {
		http.Error(w, "Invalid device serial", http.StatusBadRequest)
		return
	}

	// Parse codec/format parameters
	codec := req.URL.Query().Get("codec")
	if codec == "" {
		codec = "aac" // default to AAC
	}
	format := req.URL.Query().Get("format")

	log.Printf("[HandleDeviceAudio] Audio parameters - codec: %s, format: %s", codec, format)

	// Direct audio streaming implementation
	switch codec {
	case "opus":
		// Determine streaming format and setup
		audioService := audio.GetAudioService()
		if format == "webm" {
			// WebM container streaming
			log.Printf("[HandleDeviceAudio] Using WebM streaming for device: %s", deviceSerial)
			setWebMStreamingHeaders(w)
			startStreamingResponse(w)
			if err := audioService.StreamWebM(deviceSerial, w, req); err != nil {
				log.Printf("[HandleDeviceAudio] WebM streaming error: %v", err)
				http.Error(w, "WebM streaming failed", http.StatusInternalServerError)
			}
			return
		}
		// Raw Opus streaming
		log.Printf("[HandleDeviceAudio] Using raw Opus streaming for device: %s", deviceSerial)
		setRawOpusStreamingHeaders(w)
		startStreamingResponse(w)
		audioService.StreamOpus(deviceSerial, w)

	case "aac":
		// AAC dump: format=raw (default) or format=adts
		withADTS := strings.ToLower(format) == "adts"

		// Ensure scrcpy source in mp4 mode to use AAC encoder
		// This is handled by the audio service
		log.Printf("[HandleDeviceAudio] Using AAC streaming for device: %s, withADTS: %v", deviceSerial, withADTS)

		// Set appropriate headers for AAC streaming
		if withADTS {
			w.Header().Set("Content-Type", "audio/aac")
		} else {
			w.Header().Set("Content-Type", "audio/aac")
		}
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		startStreamingResponse(w)
		// Note: AAC streaming is not implemented in the current audio service
		// This would need to be implemented if AAC streaming is required
		http.Error(w, "AAC streaming not implemented", http.StatusNotImplemented)

	default:
		http.Error(w, "Invalid codec. Supported: opus, aac", http.StatusBadRequest)
	}
}

func (h *DeviceHandlers) HandleDeviceStream(w http.ResponseWriter, req *http.Request) {
	// Extract device serial from path: /api/devices/{serial}/stream
	path := strings.TrimPrefix(req.URL.Path, "/api/devices/")
	parts := strings.Split(path, "/")
	deviceSerial := parts[0]

	if deviceSerial == "" {
		http.Error(w, "Device serial required", http.StatusBadRequest)
		return
	}

	log.Printf("[HandleDeviceStream] Processing stream request for device: %s", deviceSerial)

	// Parse query parameters
	codec := req.URL.Query().Get("codec")
	format := req.URL.Query().Get("format")

	log.Printf("[HandleDeviceStream] Parameters - codec: %s, format: %s", codec, format)

	// Validate parameters - Go's url.Query().Get() automatically decodes URL encoding
	if codec != "h264+opus" && codec != "h264+aac" {
		http.Error(w, "Invalid codec. Only 'h264+opus' and 'h264+aac' are supported for mixed streams", http.StatusBadRequest)
		return
	}

	if format != "webm" && format != "mp4" {
		http.Error(w, "Invalid format. Only 'webm' and 'mp4' are supported for mixed streams", http.StatusBadRequest)
		return
	}

	// Direct mixed stream implementation
	log.Printf("[HandleDeviceStream] Using %s format for mixed stream", format)

	switch format {
	case "webm":
		// WebM container streaming with H.264 video and Opus audio
		handler := stream.NewWebMHandler(deviceSerial)
		handler.ServeHTTP(w, req)

	case "mp4":
		// MP4 container streaming with H.264 video and AAC audio
		handler := stream.NewMP4Handler(deviceSerial)
		handler.ServeHTTP(w, req)

	default:
		http.Error(w, "Invalid format. Supported: webm, mp4", http.StatusBadRequest)
	}
}

func (h *DeviceHandlers) HandleDeviceControl(w http.ResponseWriter, req *http.Request) {
	// Extract device serial from path: /api/devices/{serial}/control
	path := strings.TrimPrefix(req.URL.Path, "/api/devices/")
	parts := strings.Split(path, "/")
	deviceSerial := parts[0]

	if deviceSerial == "" {
		http.Error(w, "Device serial required", http.StatusBadRequest)
		return
	}

	if !isValidDeviceSerial(deviceSerial) {
		http.Error(w, "Invalid device serial", http.StatusBadRequest)
		return
	}

	log.Printf("[HandleDeviceControl] Processing control WebSocket request for device: %s", deviceSerial)

	// Direct control WebSocket implementation
	conn, err := controlUpgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Printf("[HandleDeviceControl] Failed to upgrade control WebSocket: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("[HandleDeviceControl] Control WebSocket connection established for device: %s", deviceSerial)

	// Delegate to control service
	controlService := control.GetControlService()

	// Handle WebSocket messages
	for {
		var msg map[string]interface{}
		if err := conn.ReadJSON(&msg); err != nil {
			// Check for normal close conditions
			if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
				log.Printf("[HandleDeviceControl] Control WebSocket closed normally for device: %s", deviceSerial)
			} else if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[HandleDeviceControl] Control WebSocket read error for device: %s: %v", deviceSerial, err)
			}
			break
		}

		msgType, ok := msg["type"].(string)
		if !ok {
			continue
		}

		log.Printf("[HandleDeviceControl] Control message received for device: %s, type: %s", deviceSerial, msgType)

		switch msgType {
		// WebRTC signaling messages
		case "ping", "offer", "answer", "ice-candidate":
			h.handleWebRTCMessage(conn, msg, msgType, deviceSerial)

		// Device control messages
		case "key":
			// Handle key events
			controlService.HandleKeyEvent(msg, deviceSerial)

		case "text":
			// Handle text input (clipboard event)
			clipboardMsg := map[string]interface{}{
				"text":  msg["text"],
				"paste": true,
			}
			controlService.HandleClipboardEvent(clipboardMsg, deviceSerial)

		case "touch":
			// Handle touch events
			controlService.HandleTouchEvent(msg, deviceSerial)

		case "scroll":
			// Handle scroll events
			controlService.HandleScrollEvent(msg, deviceSerial)

		case "clipboard_set":
			controlService.HandleClipboardEvent(msg, deviceSerial)

		case "reset_video":
			controlService.HandleVideoResetEvent(msg, deviceSerial)

		default:
			log.Printf("[HandleDeviceControl] Unknown message type for device: %s: %s", deviceSerial, msgType)
		}
	}
}

// handleWebRTCMessage handles WebRTC signaling messages
func (h *DeviceHandlers) handleWebRTCMessage(conn *websocket.Conn, msg map[string]interface{}, msgType, deviceSerial string) {
	if h.webrtcHandlers == nil {
		log.Printf("[HandleDeviceControl] WebRTC handlers not initialized")
		return
	}

	log.Printf("[HandleDeviceControl] Delegating WebRTC message to handler: type=%s, device=%s", msgType, deviceSerial)

	switch msgType {
	case "ping":
		h.webrtcHandlers.HandlePing(conn, msg)
	case "offer":
		h.webrtcHandlers.HandleOffer(conn, msg, deviceSerial)
	case "answer":
		h.webrtcHandlers.HandleAnswer(conn, msg, deviceSerial)
	case "ice-candidate":
		h.webrtcHandlers.HandleIceCandidate(conn, msg, deviceSerial)
	default:
		log.Printf("[HandleDeviceControl] Unknown WebRTC message type: %s", msgType)
	}
}

// HandleWebSocket handles WebSocket connections for device communication
func (h *DeviceHandlers) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	log.Println("WebSocket connection established")

	for {
		var msg map[string]interface{}
		err := conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Handle different message types
		msgType, ok := msg["type"].(string)
		if !ok {
			log.Printf("Invalid message format: missing type")
			continue
		}

		switch msgType {
		case "connect":
			h.handleWebSocketConnect(conn, msg)
		case "offer":
			h.handleWebSocketOffer(conn, msg)
		case "ice-candidate":
			h.handleWebSocketICECandidate(conn, msg)
		case "disconnect":
			h.handleWebSocketDisconnect(conn, msg)
		default:
			log.Printf("Unknown message type: %s", msgType)
		}
	}
}

// Private helper methods

func (h *DeviceHandlers) handleDeviceConnect(w http.ResponseWriter, r *http.Request, deviceID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// TODO: Implement actual device connection via bridge manager
	if h.serverService != nil {
		err := h.serverService.CreateBridge(deviceID)
		if err != nil {
			RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"success": false,
				"error":   fmt.Sprintf("Failed to connect to device: %v", err),
			})
			return
		}
	}

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"device_id": deviceID,
		"status":    "connected",
	})
}

func (h *DeviceHandlers) handleDeviceDisconnect(w http.ResponseWriter, r *http.Request, deviceID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// TODO: Implement actual device disconnection
	if h.serverService != nil {
		h.serverService.RemoveBridge(deviceID)
	}

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"device_id": deviceID,
		"status":    "disconnected",
	})
}

func (h *DeviceHandlers) handleWebSocketConnect(conn *websocket.Conn, msg map[string]interface{}) {
	// TODO: Implement WebSocket connect handling
	response := map[string]interface{}{
		"type":    "connect-response",
		"success": true,
	}
	conn.WriteJSON(response)
}

func (h *DeviceHandlers) handleWebSocketOffer(conn *websocket.Conn, msg map[string]interface{}) {
	// TODO: Implement WebRTC offer handling
	log.Printf("Received WebRTC offer: %v", msg)
}

func (h *DeviceHandlers) handleWebSocketICECandidate(conn *websocket.Conn, msg map[string]interface{}) {
	// TODO: Implement ICE candidate handling
	log.Printf("Received ICE candidate: %v", msg)
}

func (h *DeviceHandlers) handleWebSocketDisconnect(conn *websocket.Conn, msg map[string]interface{}) {
	// TODO: Implement WebSocket disconnect handling
	log.Printf("WebSocket disconnect: %v", msg)
}
