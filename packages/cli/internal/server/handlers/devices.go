package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/babelcloud/gbox/packages/cli/internal/cloud"
	"github.com/babelcloud/gbox/packages/cli/internal/device"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/control"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/transport/audio"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/transport/h264"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/transport/stream"
	serverScripts "github.com/babelcloud/gbox/packages/cli/internal/server/scripts"
	"github.com/babelcloud/gbox/packages/cli/internal/util"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

// DeviceDTO is a strong-typed representation of a device for API responses
type DeviceDTO struct {
	ID             string `json:"id"`
	TransportID    string `json:"transportId"`
	Serialno       string `json:"serialno"`
	AndroidID      string `json:"androidId"`
	Model          string `json:"model"`
	Manufacturer   string `json:"manufacturer"`
	ConnectionType string `json:"connectionType"`
	IsRegistered   bool   `json:"isRegistered"`
	RegId          string `json:"regId"`
}

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
	deviceManager  *device.Manager
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
		deviceManager:  device.NewManager(),
	}
}

// HandleDeviceList handles device listing requests
func (h *DeviceHandlers) HandleDeviceList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Use unified device manager (reused)
	devs, err := h.deviceManager.GetDevices()
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

	// Get all registered devices from cloud in one call
	registeredDevicesMap := make(map[string]*cloud.Device)
	allCloudDevices, err := deviceAPI.GetAll()
	if err != nil {
		log.Printf("Failed to get all devices from cloud: %v", err)
	} else {
		// Build a map of regId -> Device for quick lookup
		for _, cloudDevice := range allCloudDevices.Data {
			if cloudDevice.RegId != "" {
				registeredDevicesMap[cloudDevice.RegId] = cloudDevice
			}
		}
	}

	dtos := make([]DeviceDTO, 0, len(devs))
	for _, d := range devs {
		dto := DeviceDTO{
			ID:             "",
			TransportID:    d.ID,
			Serialno:       d.SerialNo,
			AndroidID:      d.AndroidID,
			Model:          d.Model,
			Manufacturer:   d.Manufacturer,
			ConnectionType: d.ConnectionType,
			IsRegistered:   false,
			RegId:          d.RegId,
		}

		// Check if device is registered by looking up in the map
		if strings.TrimSpace(d.RegId) != "" {
			if cloudDevice, found := registeredDevicesMap[d.RegId]; found {
				dto.IsRegistered = true
				dto.ID = cloudDevice.Id
			}
		}

		dtos = append(dtos, dto)
	}

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success":         true,
		"devices":         dtos,
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
		Type     string `json:"type"`
	}
	if err := decoder.Decode(&reqBody); err != nil {
		http.Error(w, errors.Wrap(err, "failed to parse request body").Error(), http.StatusBadRequest)
		return
	}

	deviceAPI := cloud.NewDeviceAPI()
	var created *cloud.Device
	var err error

	if strings.ToLower(reqBody.Type) == "linux" {
		// Best-effort: initialize xdotool and noVNC environments on Linux hosts
		if err := runLinuxInitXdotoolScript(); err != nil {
			log.Printf("Warning: init-linux-xdotool script failed: %v", err)
		}
		if err := runLinuxInitNoVncScript(); err != nil {
			log.Printf("Warning: init-linux-novnc script failed: %v", err)
		}
		created, err = deviceAPI.Create(&cloud.Device{
			Metadata: struct {
				Serialno   string `json:"serialno,omitempty"`
				AndroidId  string `json:"androidId,omitempty"`
				Type       string `json:"type,omitempty"`
				Resolution string `json:"resolution,omitempty"`
			}{
				Type: "linux",
			},
		})
		if err != nil {
			http.Error(w, errors.Wrap(err, "failed to register linux device").Error(), http.StatusInternalServerError)
			return
		}
	} else {
		// For Android devices, use the detailed registration logic
		devMgr := device.NewManager()
		ids, err := devMgr.GetIdentifiers(reqBody.DeviceId)
		if err != nil {
			http.Error(w, errors.Wrap(err, "failed to resolve device serialno or android_id").Error(), http.StatusInternalServerError)
			return
		}

		// Get Resolution (WxH) for Metadata
		width, height, resErr := devMgr.GetDisplayResolution(reqBody.DeviceId)
		var resolution string
		if resErr == nil {
			resolution = fmt.Sprintf("%dx%d", width, height)
		} else {
			resolution = ""
			log.Printf("failed to get resolution for device %s: %v", reqBody.DeviceId, resErr)
		}

		newDevice := &cloud.Device{
			Metadata: struct {
				Serialno   string `json:"serialno,omitempty"`
				AndroidId  string `json:"androidId,omitempty"`
				Type       string `json:"type,omitempty"`
				Resolution string `json:"resolution,omitempty"`
			}{
				Serialno:   ids.SerialNo,
				AndroidId:  ids.AndroidID,
				Type:       "android",
				Resolution: resolution,
			},
			RegId: ids.RegId,
		}

		created, err = deviceAPI.Create(newDevice)
		if err != nil {
			http.Error(w, errors.Wrap(err, "failed to register device").Error(), http.StatusInternalServerError)
			return
		}

		// Persist the created device ID back to the physical device as RegId
		if created != nil && created.RegId != "" {
			if err := devMgr.SetRegId(reqBody.DeviceId, created.RegId); err != nil {
				// Log the error but don't fail the registration
				log.Printf("Warning: failed to persist RegId to device %s: %v", reqBody.DeviceId, err)
			}
		}
	}

	// Establish connection to Access Point for Android only (Linux requires manual connect)
	if strings.ToLower(reqBody.Type) != "linux" {
		go func() {
			if err := h.serverService.ConnectAP(reqBody.DeviceId); err != nil {
				log.Print(errors.Wrapf(err, "fail to connect device %s to access point", reqBody.DeviceId))
			}
		}()
	}

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    created,
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

	devMgr2 := device.NewManager()
	ids2, err := devMgr2.GetIdentifiers(reqBody.DeviceId)
	serialno := ids2.SerialNo
	androidId := ids2.AndroidID
	regId := ids2.RegId
	if err != nil {
		http.Error(w, errors.Wrap(err, "failed to resolve device serialno or android_id").Error(), http.StatusInternalServerError)
		return
	}

	deviceAPI := cloud.NewDeviceAPI()

	// If regId is available, use it to find and delete the device (most accurate)
	if regId != "" {
		deviceList, err := deviceAPI.GetByRegId(regId)
		if err != nil {
			// If lookup by regId fails, fallback to serialno/androidId lookup
			log.Printf("Warning: failed to get devices by regId %s: %v, falling back to serialno/androidId lookup", regId, err)
		} else if len(deviceList.Data) > 0 {
			// Found device(s) by regId, delete them
			for _, device := range deviceList.Data {
				if err := deviceAPI.Delete(device.Id); err != nil {
					http.Error(w, errors.Wrapf(err, "failed to delete device %s", device.Id).Error(), http.StatusInternalServerError)
					return
				}
			}
			// Successfully deleted by regId, return early
			RespondJSON(w, http.StatusOK, map[string]interface{}{
				"success": true,
			})
			return
		}
	}

	// Fallback: use serialno and androidId to find and delete devices
	deviceList, err := deviceAPI.GetBySerialnoAndAndroidId(serialno, androidId)
	if err != nil {
		http.Error(w, errors.Wrap(err, "failed to get devices").Error(), http.StatusInternalServerError)
		return
	}

	if len(deviceList.Data) > 0 {
		for _, device := range deviceList.Data {
			if err := deviceAPI.Delete(device.Id); err != nil {
				http.Error(w, errors.Wrapf(err, "failed to delete device %s", device.Id).Error(), http.StatusInternalServerError)
				return
			}
		}
	} else {
		// No devices found by serialno/androidId either
		if regId != "" {
			http.Error(w, fmt.Errorf("device not found by regId %s or serialno/androidId", regId).Error(), http.StatusNotFound)
			return
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

// HandleLinuxAPConnect manually connects a linux device to access point
func (h *DeviceHandlers) HandleLinuxAPConnect(w http.ResponseWriter, r *http.Request) {
	var reqBody struct {
		DeviceId string `json:"deviceId"`
		RegId    string `json:"regId"`
	}
	if r.Method == http.MethodPost {
		decoder := json.NewDecoder(r.Body)
		_ = decoder.Decode(&reqBody)
	}

	deviceAPI := cloud.NewDeviceAPI()
	deviceId := strings.TrimSpace(reqBody.DeviceId)
	regId := strings.TrimSpace(reqBody.RegId)
	if deviceId == "" {
		// Try to reuse existing device by regId if provided
		if regId != "" {
			if list, err := deviceAPI.GetByRegId(regId); err == nil && len(list.Data) > 0 {
				deviceId = list.Data[0].Id
			}
		}
		// If still empty, create a linux device on cloud then connect it
		if deviceId == "" {
			dev, err := deviceAPI.Create(&cloud.Device{
				Metadata: struct {
					Serialno   string `json:"serialno,omitempty"`
					AndroidId  string `json:"androidId,omitempty"`
					Type       string `json:"type,omitempty"`
					Resolution string `json:"resolution,omitempty"`
				}{
					Type: "linux",
				},
			})
			if err != nil {
				http.Error(w, errors.Wrap(err, "failed to register linux device").Error(), http.StatusInternalServerError)
				return
			}
			deviceId = dev.Id
			// Best-effort to surface regId for client persistence
			if dev.RegId != "" {
				regId = dev.RegId
			} else {
				regId = dev.Id
			}
		}
	}

	// Best-effort: initialize xdotool and noVNC environments on Linux hosts before connecting
	if err := runLinuxInitXdotoolScript(); err != nil {
		log.Printf("Warning: init-linux-xdotool script failed: %v", err)
	}
	if err := runLinuxInitNoVncScript(); err != nil {
		log.Printf("Warning: init-linux-novnc script failed: %v", err)
	}

	if err := h.serverService.ConnectAPLinux(deviceId); err != nil {
		http.Error(w, errors.Wrapf(err, "failed to connect linux device %s to access point", deviceId).Error(), http.StatusInternalServerError)
		return
	}

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"deviceId": deviceId,
		"regId":    regId,
		"status":   "connected",
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
	if codec == "" {
		codec = "h264+aac"
	}
	format := req.URL.Query().Get("format")
	if format == "" {
		format = "mp4"
	}

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

// HandleDeviceExec executes a shell command on the server, scoped under a device path
// Path: /api/devices/{serial}/exec
// Method: POST
// Body JSON: { "cmd": "echo hello", "timeout_sec": 60 }
// Response JSON: { stdout, stderr, exit_code, duration_ms }
func (h *DeviceHandlers) HandleDeviceExec(w http.ResponseWriter, req *http.Request) {
	// Extract device serial from path
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

	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		Cmd        string `json:"cmd"`
		TimeoutSec int    `json:"timeout_sec"`
	}
	decoder := json.NewDecoder(req.Body)
	if err := decoder.Decode(&payload); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}
	if payload.Cmd == "" {
		http.Error(w, "Field 'cmd' is required", http.StatusBadRequest)
		return
	}
	if payload.TimeoutSec <= 0 {
		payload.TimeoutSec = 60
	}

	ctx, cancel := context.WithTimeout(req.Context(), time.Duration(payload.TimeoutSec)*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", payload.Cmd)
	} else {
		cmd = exec.CommandContext(ctx, "/bin/sh", "-c", payload.Cmd)
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	start := time.Now()
	runErr := cmd.Run()
	duration := time.Since(start)

	exitCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"device":      deviceSerial,
		"stdout":      stdoutBuf.String(),
		"stderr":      stderrBuf.String(),
		"exit_code":   exitCode,
		"duration_ms": duration.Milliseconds(),
	})
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

func (h *DeviceHandlers) HandleDeviceAdb(w http.ResponseWriter, req *http.Request) {
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

	log.Printf("[HandleDeviceAdb] Processing adb request for device: %s", deviceSerial)

	decoder := json.NewDecoder(req.Body)
	var reqBody struct {
		Command string `json:"command"`
	}
	if err := decoder.Decode(&reqBody); err != nil {
		http.Error(w, errors.Wrap(err, "failed to parse request body").Error(), http.StatusBadRequest)
		return
	}

	manager := device.NewManager()
	result, err := manager.ExecAdbCommand(deviceSerial, reqBody.Command)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    result,
	})
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

// runLinuxInitNoVncScript initializes local noVNC environment on Linux hosts.
// Best-effort: returns error but callers may choose to continue.
func runLinuxInitNoVncScript() error {
	if runtime.GOOS != "linux" {
		return nil
	}
	if len(serverScripts.InitLinuxNoVncScript) == 0 {
		return fmt.Errorf("init-linux-novnc script not embedded")
	}
	tmpFile, err := os.CreateTemp("", "gbox-init-novnc-*.sh")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	if _, err := tmpFile.Write(serverScripts.InitLinuxNoVncScript); err != nil {
		tmpFile.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	_ = os.Chmod(tmpPath, 0700)

	cmd := exec.Command("bash", tmpPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	runErr := cmd.Run()
	_ = os.Remove(tmpPath)
	return runErr
}

// runLinuxInitXdotoolScript installs and sets up xdotool environment (best-effort)
func runLinuxInitXdotoolScript() error {
	if runtime.GOOS != "linux" {
		return nil
	}
	if len(serverScripts.InitLinuxXdotoolScript) == 0 {
		return fmt.Errorf("init-linux-xdotool script not embedded")
	}
	tmpFile, err := os.CreateTemp("", "gbox-init-xdotool-*.sh")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	if _, err := tmpFile.Write(serverScripts.InitLinuxXdotoolScript); err != nil {
		tmpFile.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	_ = os.Chmod(tmpPath, 0700)

	cmd := exec.Command("bash", tmpPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	runErr := cmd.Run()
	_ = os.Remove(tmpPath)
	return runErr
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
