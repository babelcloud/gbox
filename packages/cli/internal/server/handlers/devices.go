package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/control"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/scrcpy"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/transport/audio"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/transport/h264"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/transport/stream"
	"github.com/gorilla/websocket"
)

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

	devices, err := h.getADBDevices()
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

// HandleDeviceAudioDump streams raw AAC frames for debugging.
// Query params:
//
//	adts=1|0    wrap raw AAC with ADTS headers (default 0)
//	sr=48000    sampling rate hint for ADTS (default 48000)
//	ch=2        channel count hint for ADTS (default 2)
func (h *DeviceHandlers) HandleDeviceAudioDump(w http.ResponseWriter, req *http.Request) {
	// Extract device serial from path: /api/devices/{serial}/audio.dump
	path := strings.TrimPrefix(req.URL.Path, "/api/devices/")
	parts := strings.Split(path, "/")
	deviceSerial := parts[0]

	if deviceSerial == "" {
		http.Error(w, "Device serial required", http.StatusBadRequest)
		return
	}

	// Parse query params
	q := req.URL.Query()
	withADTS := q.Get("adts") == "1" || strings.ToLower(q.Get("adts")) == "true"

	// Defaults suitable for scrcpy AAC
	sampleRate := 48000
	channelCount := 2
	if v := q.Get("sr"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			sampleRate = n
		}
	}
	if v := q.Get("ch"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			channelCount = n
		}
	}

	// Set headers
	w.Header().Set("Content-Type", "audio/aac")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	// Use existing streaming handlers pipeline
	streamingHandlers := NewStreamingHandlers()
	streamingHandlers.SetServerService(h.serverService)
	streamingHandlers.SetPathPrefix("/api")

	// Start/attach scrcpy source in AAC/MP4 mode to ensure AAC and subscribe directly
	source := scrcpy.GetOrCreateSourceWithMode(deviceSerial, "mp4")
	scrcpy.StartSourceWithMode(deviceSerial, context.Background(), "mp4")

	// Subscribe to audio stream
	subscriberID := fmt.Sprintf("audio_dump_%s_%d", deviceSerial, time.Now().UnixNano())
	audioCh := source.SubscribeAudio(subscriberID, 1000)
	defer source.UnsubscribeAudio(subscriberID)

	// Prepare ADTS helper if needed
	writeADTS := func(frame []byte) []byte {
		if !withADTS || len(frame) == 0 {
			return frame
		}
		// Build ADTS header (7 bytes, no CRC)
		// AAC LC profile = 2 (in ADTS: profile-1 => 1)
		profile := 1 // AAC LC
		srIdx := adtsSamplingFreqIndex(sampleRate)
		chCfg := channelCount
		aacFrameLen := len(frame) + 7
		hdr := make([]byte, 7)
		hdr[0] = 0xFF
		hdr[1] = 0xF1 // 1111 0001 (sync + MPEG-4, no CRC)
		hdr[2] = byte(((profile & 0x3) << 6) | ((srIdx & 0xF) << 2) | ((chCfg >> 2) & 0x1))
		hdr[3] = byte(((chCfg & 0x3) << 6) | ((aacFrameLen >> 11) & 0x3))
		hdr[4] = byte((aacFrameLen >> 3) & 0xFF)
		hdr[5] = byte(((aacFrameLen & 0x7) << 5) | 0x1F)
		hdr[6] = 0xFC
		return append(hdr, frame...)
	}

	// Stream loop
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
}

// adtsSamplingFreqIndex maps sample rate to ADTS index
func adtsSamplingFreqIndex(sr int) int {
	// Table per ISO/IEC 14496-3
	switch sr {
	case 96000:
		return 0
	case 88200:
		return 1
	case 64000:
		return 2
	case 48000:
		return 3
	case 44100:
		return 4
	case 32000:
		return 5
	case 24000:
		return 6
	case 22050:
		return 7
	case 16000:
		return 8
	case 12000:
		return 9
	case 11025:
		return 10
	case 8000:
		return 11
	case 7350:
		return 12
	default:
		// Fallback to 48000
		return 3
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

func (h *DeviceHandlers) getADBDevices() ([]map[string]interface{}, error) {
	cmd := exec.Command("adb", "devices", "-l")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute adb devices: %v", err)
	}

	lines := strings.Split(string(output), "\n")
	var devices []map[string]interface{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "List of devices") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) >= 2 {
			device := map[string]interface{}{
				"id":     parts[0],
				"status": parts[1],
			}

			// Parse additional device info if available
			if len(parts) > 2 {
				for _, part := range parts[2:] {
					if strings.Contains(part, ":") {
						kv := strings.SplitN(part, ":", 2)
						if len(kv) == 2 {
							device[kv[0]] = kv[1]
						}
					}
				}
			}

			devices = append(devices, device)
		}
	}

	return devices, nil
}
