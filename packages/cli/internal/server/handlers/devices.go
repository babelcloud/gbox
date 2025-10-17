package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/babelcloud/gbox/packages/cli/internal/cloud"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/control"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/transport/audio"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/transport/h264"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/transport/stream"
	"github.com/babelcloud/gbox/packages/cli/internal/util"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
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

	deviceAPI := cloud.NewDeviceAPI()
	for _, device := range devices {
		serialno, ok := device["ro.serialno"].(string)
		if !ok {
			continue
		}
		androindId, ok := device["android_id"].(string)
		if !ok {
			continue
		}
		deviceList, err := deviceAPI.GetBySerialnoAndAndroidId(serialno, androindId)
		if err != nil {
			log.Print(errors.Wrapf(err, "fail to get device with serialno %s and androidId %s", serialno, androindId))
			continue
		}
		if deviceList.Total > 0 {
			device["gbox.device_id"] = deviceList.Data[0].Id
		}
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
	decoder := json.NewDecoder(r.Body)
	var reqBody struct {
		DeviceId string `json:"deviceId"`
	}
	if err := decoder.Decode(&reqBody); err != nil {
		http.Error(w, errors.Wrap(err, "failed to parse request body").Error(), http.StatusBadRequest)
		return
	}

	serialno, androidId, err := GetDeviceSerialnoAndAndroidId(reqBody.DeviceId)
	if err != nil {
		http.Error(w, errors.Wrap(err, "failed to resolve device serialno or android_id").Error(), http.StatusInternalServerError)
		return
	}

	deviceAPI := cloud.NewDeviceAPI()
	device, err := deviceAPI.Create(&cloud.Device{
		Metadata: struct {
			Serialno  string `json:"serialno,omitempty"`
			AndroidId string `json:"androidId,omitempty"`
		}{
			Serialno:  serialno,
			AndroidId: androidId,
		},
	})
	if err != nil {
		http.Error(w, errors.Wrap(err, "failed to register device").Error(), http.StatusInternalServerError)
		return
	}

	go func() {
		if err := h.serverService.ConnectAP(reqBody.DeviceId); err != nil {
			log.Print(errors.Wrapf(err, "fail to connect device %s to access point", reqBody.DeviceId))
		}
	}()

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    device,
	})
}

// HandleDeviceUnregister handles device unregistration requests
func (h *DeviceHandlers) HandleDeviceUnregister(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var reqBody struct {
		DeviceId string `json:"deviceId"`
	}
	if err := decoder.Decode(&reqBody); err != nil {
		http.Error(w, errors.Wrap(err, "failed to parse request body").Error(), http.StatusBadRequest)
		return
	}

	serialno, androidId, err := GetDeviceSerialnoAndAndroidId(reqBody.DeviceId)
	if err != nil {
		http.Error(w, errors.Wrap(err, "failed to resolve device serialno or android_id").Error(), http.StatusInternalServerError)
		return
	}

	deviceAPI := cloud.NewDeviceAPI()

	deviceList, err := deviceAPI.GetBySerialnoAndAndroidId(serialno, androidId)
	if err != nil {
		http.Error(w, errors.Wrap(err, "failed to get devices").Error(), http.StatusInternalServerError)
		return
	}

	if deviceList.Total > 0 {
		for _, device := range deviceList.Data {
			if err := deviceAPI.Delete(device.Id); err != nil {
				http.Error(w, errors.Wrapf(err, "failed to delete device %s", device.Id).Error(), http.StatusInternalServerError)
				return
			}
		}
	}

	go func() {
		if err := h.serverService.DisconnectAP(reqBody.DeviceId); err != nil {
			log.Print(errors.Wrapf(err, "fail to disconnect device %s from access point", reqBody.DeviceId))
		}
	}()

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// Device streaming handlers
func (h *DeviceHandlers) HandleDeviceVideo(w http.ResponseWriter, req *http.Request) {
	// Extract device serial from path: /api/devices/{serial}/video
	path := strings.TrimPrefix(req.URL.Path, "/api/devices/")
	parts := strings.Split(path, "/")
	deviceSerial := parts[0]

	if strings.Contains(req.Header.Get("via"), "gbox-device-ap") {
		deviceSerial = h.serverService.GetAdbSerialByGboxDeviceId(deviceSerial)
	}

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
		h.HandleWebMStream(w, req, deviceSerial)

	case "mp4":
		// MP4 container streaming with H.264 video and Opus audio
		h.HandleMP4Stream(w, req, deviceSerial)

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

	if strings.Contains(req.Header.Get("via"), "gbox-device-ap") {
		deviceSerial = h.serverService.GetAdbSerialByGboxDeviceId(deviceSerial)
	}

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

	if strings.Contains(req.Header.Get("via"), "gbox-device-ap") {
		deviceSerial = h.serverService.GetAdbSerialByGboxDeviceId(deviceSerial)
	}

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
		h.HandleWebMStream(w, req, deviceSerial)

	case "mp4":
		// MP4 container streaming with H.264 video and AAC audio
		h.HandleMP4Stream(w, req, deviceSerial)

	default:
		http.Error(w, "Invalid format. Supported: webm, mp4", http.StatusBadRequest)
	}
}

func (h *DeviceHandlers) HandleDeviceControl(w http.ResponseWriter, req *http.Request) {
	// Extract device serial from path: /api/devices/{serial}/control
	path := strings.TrimPrefix(req.URL.Path, "/api/devices/")
	parts := strings.Split(path, "/")
	deviceSerial := parts[0]

	if strings.Contains(req.Header.Get("via"), "gbox-device-ap") {
		deviceSerial = h.serverService.GetAdbSerialByGboxDeviceId(deviceSerial)
	}

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

		slog.Debug("Control message received", "device", deviceSerial, "type", msgType)

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

func (h *DeviceHandlers) HandleDeviceTestHttp(w http.ResponseWriter, req *http.Request) {
	path := strings.TrimPrefix(req.URL.Path, "/api/devices/")
	parts := strings.Split(path, "/")
	deviceSerial := parts[0]

	if strings.Contains(req.Header.Get("via"), "gbox-device-ap") {
		deviceSerial = h.serverService.GetAdbSerialByGboxDeviceId(deviceSerial)
	}

	if deviceSerial == "" {
		http.Error(w, "Device serial required", http.StatusBadRequest)
		return
	}

	if !isValidDeviceSerial(deviceSerial) {
		http.Error(w, "Invalid device serial", http.StatusBadRequest)
		return
	}

	fmt.Fprintf(w, "connect local device %s at %s", deviceSerial, time.Now().Format(time.RFC3339))
}

func (h *DeviceHandlers) HandleDeviceTestWs(w http.ResponseWriter, req *http.Request) {
	path := strings.TrimPrefix(req.URL.Path, "/api/devices/")
	parts := strings.Split(path, "/")
	deviceSerial := parts[0]

	if strings.Contains(req.Header.Get("via"), "gbox-device-ap") {
		deviceSerial = h.serverService.GetAdbSerialByGboxDeviceId(deviceSerial)
	}

	if deviceSerial == "" {
		http.Error(w, "Device serial required", http.StatusBadRequest)
		return
	}

	if !isValidDeviceSerial(deviceSerial) {
		http.Error(w, "Invalid device serial", http.StatusBadRequest)
		return
	}

	upgrader := websocket.Upgrader{}
	c, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		http.Error(w, fmt.Sprint("unable to upgrade websocket", err), http.StatusInternalServerError)
		return
	}
	defer c.Close()

	go func() {
		for range time.Tick(5 * time.Second) {
			if err := c.WriteMessage(websocket.TextMessage, []byte(time.Now().Format(time.RFC3339))); err != nil {
				log.Println("test websocket write error:", err)
				break
			}
		}
	}()

	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			log.Println("test websocket read error:", err)
			break
		}
		err = c.WriteMessage(mt, message)
		if err != nil {
			log.Println("test websocket write error:", err)
			break
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

// HandleWebMStream handles WebM streaming
func (h *DeviceHandlers) HandleWebMStream(w http.ResponseWriter, r *http.Request, deviceSerial string) {
	logger := util.GetLogger()
	logger.Info("Starting WebM mixed stream", "device", deviceSerial)

	// Set headers for WebM streaming
	w.Header().Set("Content-Type", "video/webm; codecs=avc1.42E01E,opus")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create WebM stream writer
	writer := stream.NewWebMMuxer(w)
	defer writer.Close()

	// Start streaming with the writer
	if err := h.startStream(deviceSerial, writer, "webm"); err != nil {
		logger.Error("Failed to start WebM stream", "device", deviceSerial, "error", err)
		http.Error(w, fmt.Sprintf("Failed to start stream: %v", err), http.StatusInternalServerError)
		return
	}
}

// HandleMP4Stream handles MP4 container streaming
func (h *DeviceHandlers) HandleMP4Stream(w http.ResponseWriter, r *http.Request, deviceSerial string) {
	logger := util.GetLogger()
	logger.Info("Starting fMP4 stream", "device", deviceSerial)

	// Set headers for fMP4 streaming
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create MP4 stream writer
	writer := stream.NewFMP4Muxer(w, logger)
	defer writer.Close()

	// Start streaming with the writer
	if err := h.startStream(deviceSerial, writer, "mp4"); err != nil {
		logger.Error("Failed to start MP4 stream", "device", deviceSerial, "error", err)
		http.Error(w, fmt.Sprintf("Failed to start stream: %v", err), http.StatusInternalServerError)
		return
	}
}

// startStream starts a mixed audio/video stream with the given writer using StreamManager
func (h *DeviceHandlers) startStream(deviceSerial string, writer stream.Muxer, mode string) error {
	logger := util.GetLogger()

	// Create stream manager for protocol abstraction
	streamManager := stream.NewStreamManager(logger)

	// Configure stream
	config := stream.StreamConfig{
		DeviceSerial: deviceSerial,
		Mode:         mode,
		VideoWidth:   1920, // Will be updated from source
		VideoHeight:  1080, // Will be updated from source
	}

	// Start stream using stream manager
	result, err := streamManager.StartStream(context.Background(), config)
	if err != nil {
		return fmt.Errorf("failed to start stream: %w", err)
	}
	defer result.Cleanup()

	// Get actual device dimensions
	_, videoWidth, videoHeight := result.Source.GetConnectionInfo()
	logger.Info("Device video dimensions", "width", videoWidth, "height", videoHeight)

	// Initialize the stream writer with device dimensions
	if err := writer.Initialize(videoWidth, videoHeight, result.CodecParams); err != nil {
		return fmt.Errorf("failed to initialize stream writer: %w", err)
	}

	// Convert channels to muxer format
	videoSampleCh, audioSampleCh := streamManager.ConvertToMuxerSamples(result.VideoCh, result.AudioCh)

	// Start streaming
	logger.Info("Mixed stream started", "device", deviceSerial, "mode", mode)

	// Use the writer's streaming method
	return writer.Stream(videoSampleCh, audioSampleCh)
}
