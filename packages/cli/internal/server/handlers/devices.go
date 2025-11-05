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
	ID           string                 `json:"id"`
	TransportID  string                 `json:"transportId"`
	Serialno     string                 `json:"serialno"`
	Platform     string                 `json:"platform"`   // mobile, desktop
	OS           string                 `json:"os"`         // android, linux, windows, macos
	DeviceType   string                 `json:"deviceType"` // physical, emulator, vm
	IsRegistered bool                   `json:"isRegistered"`
	RegId        string                 `json:"regId"`
	IsLocal      bool                   `json:"isLocal"`  // true if this is the local desktop device
	Metadata     map[string]interface{} `json:"metadata"` // Device-specific metadata
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
	deviceManager  device.DeviceManager
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
		deviceManager:  device.NewManager("android"),
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

	// Add Android devices
	for _, d := range devs {
		// Get device information for Android device
		androidMgr := device.NewManager("android")
		width, height, resErr := androidMgr.GetDisplayResolution(d.ID)
		displayResolution := ""
		if resErr == nil {
			displayResolution = fmt.Sprintf("%dx%d", width, height)
		}

		osVersion, _ := androidMgr.GetOSVersion(d.ID) // non-fatal
		memory, _ := androidMgr.GetMemory(d.ID)       // non-fatal

		// Build Android-specific metadata
		metadata := make(map[string]interface{})
		metadata["model"] = d.Model
		metadata["manufacturer"] = d.Manufacturer
		metadata["connectionType"] = d.ConnectionType
		metadata["androidId"] = d.AndroidID // Android-specific field
		if displayResolution != "" {
			metadata["resolution"] = displayResolution
		}
		if osVersion != "" {
			metadata["osVersion"] = osVersion
		}
		if memory != "" {
			metadata["memory"] = memory
		}

		dto := DeviceDTO{
			ID:           "",
			TransportID:  d.ID,
			Serialno:     d.SerialNo,
			Platform:     "mobile",  // Android devices are mobile
			OS:           "android", // Android devices
			DeviceType:   util.DetectAndroidDeviceType(d.ID, d.SerialNo),
			IsRegistered: false,
			RegId:        d.RegId,
			Metadata:     metadata,
		}

		// Check if device is registered by looking up in the map
		if strings.TrimSpace(d.RegId) != "" {
			if cloudDevice, found := registeredDevicesMap[d.RegId]; found {
				dto.IsRegistered = true
				dto.ID = cloudDevice.Id
				// Update Platform and OS from cloud device metadata if available
				if cloudDevice.Metadata.DeviceType != "" {
					dto.Platform = cloudDevice.Metadata.DeviceType
				}
				if cloudDevice.Metadata.OsType != "" {
					dto.OS = cloudDevice.Metadata.OsType
				}
			}
		}

		// Update device info cache with complete device information
		h.serverService.UpdateDeviceInfo(&dto)

		dtos = append(dtos, dto)
	}

	// Add desktop device (always show local machine)
	// Map runtime.GOOS to osType for device manager
	var osType string
	switch runtime.GOOS {
	case "darwin":
		osType = "macos"
	case "linux", "windows":
		osType = strings.ToLower(runtime.GOOS)
	default:
		osType = "linux" // Default fallback
	}

	desktopMgr := device.NewManager(osType)
	localRegId, _ := desktopMgr.GetRegId("") // non-fatal
	serialno := util.GetDesktopSerialNo(osType)

	// Get device information for desktop device
	width, height, resErr := desktopMgr.GetDisplayResolution("")
	displayResolution := ""
	if resErr == nil {
		displayResolution = fmt.Sprintf("%dx%d", width, height)
	}

	osVersion, _ := desktopMgr.GetOSVersion("") // non-fatal
	memory, _ := desktopMgr.GetMemory("")       // non-fatal

	// Build desktop-specific metadata
	metadata := make(map[string]interface{})
	if osType == "macos" {
		metadata["chip"] = util.GetMacOSChip() // macOS-specific field
	}
	if osVersion != "" {
		metadata["osVersion"] = osVersion
	}
	if memory != "" {
		metadata["memory"] = memory
	}
	if displayResolution != "" {
		metadata["resolution"] = displayResolution
	}
	// Add hostname for desktop devices
	hostname, err := os.Hostname()
	if err == nil && hostname != "" {
		metadata["hostname"] = hostname
	}

	var desktopDTO DeviceDTO
	deviceType := util.DetectDesktopDeviceType(osType)

	if err == nil && localRegId != "" {
		// Check if this desktop device is registered in cloud
		if cloudDevice, found := registeredDevicesMap[localRegId]; found {
			// Desktop device is registered
			desktopDTO = DeviceDTO{
				ID:           cloudDevice.Id,
				TransportID:  "local",
				Serialno:     cloudDevice.Metadata.Serialno,
				Platform:     "desktop",
				OS:           osType,
				DeviceType:   deviceType,
				IsRegistered: true,
				RegId:        localRegId,
				IsLocal:      true,
				Metadata:     metadata,
			}
			// Update Platform and OS from cloud device metadata if available
			if cloudDevice.Metadata.DeviceType != "" {
				desktopDTO.Platform = cloudDevice.Metadata.DeviceType
			}
			if cloudDevice.Metadata.OsType != "" {
				desktopDTO.OS = cloudDevice.Metadata.OsType
			}
		} else {
			// Desktop device exists locally but not registered
			desktopDTO = DeviceDTO{
				ID:           "",
				TransportID:  "local",
				Serialno:     serialno,
				Platform:     "desktop",
				OS:           osType,
				DeviceType:   deviceType,
				IsRegistered: false,
				RegId:        localRegId,
				IsLocal:      true,
				Metadata:     metadata,
			}
		}
	} else {
		// No local regId, show desktop device as unregistered
		desktopDTO = DeviceDTO{
			ID:           "",
			TransportID:  "local",
			Serialno:     serialno,
			Platform:     "desktop",
			OS:           osType,
			DeviceType:   deviceType,
			IsRegistered: false,
			RegId:        "",
			IsLocal:      true,
			Metadata:     metadata,
		}
	}

	// Update device info cache with complete desktop device information
	h.serverService.UpdateDeviceInfo(&desktopDTO)

	dtos = append(dtos, desktopDTO)

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

// validateDeviceTypeAndOsType validates deviceType and osType parameters and their combination.
// Returns normalized osType (with default value applied) and error if validation fails.
func validateDeviceTypeAndOsType(deviceType, osType string) (string, error) {
	// Validate deviceType
	if deviceType != "mobile" && deviceType != "desktop" {
		return "", fmt.Errorf("invalid deviceType: %s, must be 'mobile' or 'desktop'", deviceType)
	}

	// Validate osType based on deviceType
	switch deviceType {
	case "mobile":
		// Mobile devices only support android
		if osType != "" && osType != "android" {
			return "", fmt.Errorf("mobile device type only supports 'android' osType, got: %s", osType)
		}
		// Default to android if not specified
		if osType == "" {
			osType = "android"
		}
	case "desktop":
		// Desktop devices support linux, windows, macos
		validDesktopOsTypes := map[string]bool{
			"linux":   true,
			"windows": true,
			"macos":   true,
		}
		if osType != "" && !validDesktopOsTypes[osType] {
			return "", fmt.Errorf("desktop device type only supports 'linux', 'windows', or 'macos' osType, got: %s", osType)
		}
	default:
		return "", fmt.Errorf("invalid deviceType: %s, must be 'mobile' or 'desktop'", deviceType)
	}

	return osType, nil
}

// HandleDeviceRegister handles device registration requests
func (h *DeviceHandlers) HandleDeviceRegister(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var reqBody struct {
		DeviceId   string `json:"deviceId"`
		DeviceType string `json:"deviceType"` // mobile, desktop
		OsType     string `json:"osType"`     // android, linux, windows, macos
	}
	if err := decoder.Decode(&reqBody); err != nil {
		http.Error(w, errors.Wrap(err, "failed to parse request body").Error(), http.StatusBadRequest)
		return
	}

	// Normalize deviceType and osType
	deviceType := strings.ToLower(reqBody.DeviceType)
	osType := strings.ToLower(reqBody.OsType)

	// Validate deviceType and osType, and get normalized osType
	normalizedOsType, err := validateDeviceTypeAndOsType(deviceType, osType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	osType = normalizedOsType

	deviceAPI := cloud.NewDeviceAPI()
	created, err := h.registerDevice(deviceAPI, reqBody.DeviceId, deviceType, osType)
	if err != nil {
		// Check if it's a validation error (400) or server error (500)
		if strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "only supports") {
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Connect to access point asynchronously (for both desktop and mobile)
	go func() {
		if err := h.serverService.ConnectAP(created.Id); err != nil {
			log.Print(errors.Wrapf(err, "fail to connect device %s to access point", created.Id))
		}
	}()

	// Return response
	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    created,
	})
}

// registerDevice registers a device based on deviceType and osType
func (h *DeviceHandlers) registerDevice(deviceAPI *cloud.DeviceAPI, deviceId, deviceType, osType string) (*cloud.Device, error) {
	// Initialize xdotool and noVNC environments for Linux desktop devices
	if deviceType == "desktop" && osType == "linux" {
		if err := runLinuxInitXdotoolScript(); err != nil {
			log.Printf("Warning: init-linux-xdotool script failed: %v", err)
		}
		if err := runLinuxInitNoVncScript(); err != nil {
			log.Printf("Warning: init-linux-novnc script failed: %v", err)
		}
	}

	// Prepare device metadata based on device type
	var serialno, androidId, regId string
	var err error
	metadata := make(map[string]interface{})

	if deviceType == "desktop" {
		// Desktop device: get serialno from system
		serialno = util.GetDesktopSerialNo(osType)
		// Try to read existing regId from local file using DesktopManager
		desktopMgr := device.NewManager(osType)
		if regIdFromFile, readErr := desktopMgr.GetRegId(""); readErr == nil && regIdFromFile != "" {
			regId = regIdFromFile
		}

		// Get device information for desktop device
		width, height, resErr := desktopMgr.GetDisplayResolution("")
		displayResolution := ""
		if resErr == nil {
			displayResolution = fmt.Sprintf("%dx%d", width, height)
		}

		osVersion, _ := desktopMgr.GetOSVersion("") // non-fatal
		memory, _ := desktopMgr.GetMemory("")       // non-fatal

		// Build desktop-specific metadata
		if osType == "macos" {
			metadata["chip"] = util.GetMacOSChip() // macOS-specific field
		}
		if osVersion != "" {
			metadata["osVersion"] = osVersion
		}
		if memory != "" {
			metadata["memory"] = memory
		}
		if displayResolution != "" {
			metadata["resolution"] = displayResolution
		}
		// Add hostname for desktop devices
		hostname, hostnameErr := os.Hostname()
		if hostnameErr == nil && hostname != "" {
			metadata["hostname"] = hostname
		}
	} else {
		// Mobile device: get identifiers from ADB
		devMgr := device.NewManager("android")
		ids, err := devMgr.GetIdentifiers(deviceId)
		if err != nil {
			return nil, errors.Wrap(err, "failed to resolve device serialno or android_id")
		}
		serialno = ids.SerialNo
		if ids.AndroidID != nil {
			androidId = *ids.AndroidID
			metadata["androidId"] = androidId
		}
		regId = ids.RegId

		// Get device information for Android device
		width, height, resErr := devMgr.GetDisplayResolution(deviceId)
		displayResolution := ""
		if resErr == nil {
			displayResolution = fmt.Sprintf("%dx%d", width, height)
		}

		osVersion, _ := devMgr.GetOSVersion(deviceId) // non-fatal
		memory, _ := devMgr.GetMemory(deviceId)       // non-fatal

		// Get device info to get model, manufacturer, connectionType
		devices, err := devMgr.GetDevices()
		if err == nil {
			for _, d := range devices {
				if d.ID == deviceId {
					if d.Model != "" {
						metadata["model"] = d.Model
					}
					if d.Manufacturer != "" {
						metadata["manufacturer"] = d.Manufacturer
					}
					if d.ConnectionType != "" {
						metadata["connectionType"] = d.ConnectionType
					}
					break
				}
			}
		}

		// Build Android-specific metadata
		if displayResolution != "" {
			metadata["resolution"] = displayResolution
		}
		if osVersion != "" {
			metadata["osVersion"] = osVersion
		}
		if memory != "" {
			metadata["memory"] = memory
		}
	}

	// Extract fields from metadata map
	resolution := ""
	if res, ok := metadata["resolution"].(string); ok {
		resolution = res
	}
	hostname := ""
	if hn, ok := metadata["hostname"].(string); ok {
		hostname = hn
	}
	chip := ""
	if c, ok := metadata["chip"].(string); ok {
		chip = c
	}
	osVersion := ""
	if ov, ok := metadata["osVersion"].(string); ok {
		osVersion = ov
	}
	memory := ""
	if m, ok := metadata["memory"].(string); ok {
		memory = m
	}
	model := ""
	if m, ok := metadata["model"].(string); ok {
		model = m
	}
	manufacturer := ""
	if m, ok := metadata["manufacturer"].(string); ok {
		manufacturer = m
	}
	connectionType := ""
	if ct, ok := metadata["connectionType"].(string); ok {
		connectionType = ct
	}

	// Create device in cloud
	newDevice := &cloud.Device{
		Metadata: struct {
			Serialno       string `json:"serialno,omitempty"`
			AndroidId      string `json:"androidId,omitempty"`
			Type           string `json:"type,omitempty"`
			DeviceType     string `json:"deviceType,omitempty"`
			OsType         string `json:"osType,omitempty"`
			Resolution     string `json:"resolution,omitempty"`
			Hostname       string `json:"hostname,omitempty"`
			Chip           string `json:"chip,omitempty"`
			OsVersion      string `json:"osVersion,omitempty"`
			Memory         string `json:"memory,omitempty"`
			Model          string `json:"model,omitempty"`
			Manufacturer   string `json:"manufacturer,omitempty"`
			ConnectionType string `json:"connectionType,omitempty"`
		}{
			Serialno:       serialno,
			AndroidId:      androidId,
			Type:           osType, // Set Type field for backward compatibility with remote API
			DeviceType:     deviceType,
			OsType:         osType,
			Resolution:     resolution,
			Hostname:       hostname,
			Chip:           chip,
			OsVersion:      osVersion,
			Memory:         memory,
			Model:          model,
			Manufacturer:   manufacturer,
			ConnectionType: connectionType,
		},
		RegId: regId,
	}

	created, err := deviceAPI.Create(newDevice)
	if err != nil {
		return nil, errors.Wrap(err, "failed to register device")
	}

	// Persist the created device RegId back to the device
	if created != nil && created.RegId != "" {
		var devMgr device.DeviceManager
		if deviceType == "desktop" {
			devMgr = device.NewManager(osType)
		} else {
			devMgr = device.NewManager("android")
		}
		if err := devMgr.SetRegId(deviceId, created.RegId); err != nil {
			log.Printf("Warning: failed to persist RegId to device %s: %v", deviceId, err)
		}
	}

	return created, nil
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

	deviceAPI := cloud.NewDeviceAPI()

	// First, try to find device by regId (works for both Android and Desktop devices)
	// If reqBody.DeviceId looks like a UUID/regId, try to find device by regId first
	deviceList, err := deviceAPI.GetByRegId(reqBody.DeviceId)
	if err == nil && len(deviceList.Data) > 0 {
		// Found device(s) by regId, delete them
		for _, device := range deviceList.Data {
			if err := deviceAPI.Delete(device.Id); err != nil {
				http.Error(w, errors.Wrapf(err, "failed to delete device %s", device.Id).Error(), http.StatusInternalServerError)
				return
			}
		}
		// Successfully deleted by regId, return early
		go func() {
			if err := h.serverService.DisconnectAP(reqBody.DeviceId); err != nil {
				log.Print(errors.Wrapf(err, "fail to disconnect device %s from access point", reqBody.DeviceId))
			}
		}()
		RespondJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
		})
		return
	}

	// If not found by regId, try to resolve as Android device (for backward compatibility)
	devMgr2 := device.NewManager("android")
	ids2, err := devMgr2.GetIdentifiers(reqBody.DeviceId)
	if err != nil {
		// If GetIdentifiers fails, it might be a desktop device or invalid deviceId
		// Try one more time to find by regId (in case it's a regId that wasn't found above)
		deviceList, err2 := deviceAPI.GetByRegId(reqBody.DeviceId)
		if err2 == nil && len(deviceList.Data) > 0 {
			for _, device := range deviceList.Data {
				if err := deviceAPI.Delete(device.Id); err != nil {
					http.Error(w, errors.Wrapf(err, "failed to delete device %s", device.Id).Error(), http.StatusInternalServerError)
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
			return
		}
		http.Error(w, errors.Wrap(err, "failed to resolve device identifiers").Error(), http.StatusInternalServerError)
		return
	}

	serialno := ids2.SerialNo
	var androidId string
	if ids2.AndroidID != nil {
		androidId = *ids2.AndroidID
	}
	regId := ids2.RegId

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
			go func() {
				if err := h.serverService.DisconnectAP(reqBody.DeviceId); err != nil {
					log.Print(errors.Wrapf(err, "fail to disconnect device %s from access point", reqBody.DeviceId))
				}
			}()
			RespondJSON(w, http.StatusOK, map[string]interface{}{
				"success": true,
			})
			return
		}
	}

	// Fallback: use serialno and androidId to find and delete devices (Android only)
	deviceList, err = deviceAPI.GetBySerialnoAndAndroidId(serialno, androidId)
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
		http.Error(w, fmt.Errorf("device not found").Error(), http.StatusNotFound)
		return
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
		deviceSerial = h.serverService.GetSerialByDeviceId(deviceSerial)
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
		deviceSerial = h.serverService.GetSerialByDeviceId(deviceSerial)
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
		deviceSerial = h.serverService.GetSerialByDeviceId(deviceSerial)
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
		deviceSerial = h.serverService.GetSerialByDeviceId(deviceSerial)
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
		deviceSerial = h.serverService.GetSerialByDeviceId(deviceSerial)
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

	// Determine device type by looking up device information
	devicePlatform := h.getDevicePlatform(deviceSerial)

	var cmd *exec.Cmd
	if devicePlatform == "mobile" {
		// Execute command on Android device via adb shell
		adbPath, err := exec.LookPath("adb")
		if err != nil {
			adbPath = "adb"
		}
		cmd = exec.CommandContext(ctx, adbPath, "-s", deviceSerial, "shell", payload.Cmd)
	} else {
		// Execute command locally on desktop device
		if runtime.GOOS == "windows" {
			cmd = exec.CommandContext(ctx, "cmd", "/C", payload.Cmd)
		} else {
			cmd = exec.CommandContext(ctx, "/bin/sh", "-c", payload.Cmd)
		}
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

// getDevicePlatform determines the platform type (mobile or desktop) for a given device serial
// Returns "mobile" for Android devices, "desktop" for desktop devices
func (h *DeviceHandlers) getDevicePlatform(deviceSerial string) string {
	// First, try to get from device keeper cache (supports serialno, deviceId, or regId lookup)
	deviceInfo := h.serverService.GetDeviceInfo(deviceSerial)
	if deviceInfo != nil {
		if dto, ok := deviceInfo.(*DeviceDTO); ok && dto.Platform != "" {
			return dto.Platform
		}
	}

	// If not in cache, try to determine from local device list
	// This uses local API (not remote API) to get device information
	dtos := h.getLocalDeviceList()
	for _, dto := range dtos {
		// Match by serialno, TransportID, or ID
		if dto.Serialno == deviceSerial || dto.TransportID == deviceSerial || dto.ID == deviceSerial {
			platform := dto.Platform
			if platform == "" {
				// Fallback: determine from OS type
				if dto.OS == "android" {
					platform = "mobile"
				} else {
					platform = "desktop"
				}
			}
			// Update cache with complete device info for future use
			h.serverService.UpdateDeviceInfo(&dto)
			return platform
		}
	}

	// Default to desktop if we can't determine (safer for local execution)
	return "desktop"
}

// getLocalDeviceList gets device list locally without making remote API calls
// This is a helper function to avoid duplicating HandleDeviceList logic
func (h *DeviceHandlers) getLocalDeviceList() []DeviceDTO {
	devs, err := h.deviceManager.GetDevices()
	if err != nil {
		return []DeviceDTO{}
	}

	dtos := make([]DeviceDTO, 0, len(devs))

	// Add Android devices
	for _, d := range devs {
		dto := DeviceDTO{
			ID:           "",
			TransportID:  d.ID,
			Serialno:     d.SerialNo,
			Platform:     "mobile",  // Android devices are mobile
			OS:           "android", // Android devices
			DeviceType:   util.DetectAndroidDeviceType(d.ID, d.SerialNo),
			IsRegistered: false,
			RegId:        d.RegId,
		}
		dtos = append(dtos, dto)
	}

	// Add desktop device (always show local machine)
	var osType string
	switch runtime.GOOS {
	case "darwin":
		osType = "macos"
	case "linux", "windows":
		osType = strings.ToLower(runtime.GOOS)
	default:
		osType = "linux"
	}

	desktopMgr := device.NewManager(osType)
	localRegId, _ := desktopMgr.GetRegId("") // non-fatal
	serialno := util.GetDesktopSerialNo(osType)

	desktopDTO := DeviceDTO{
		ID:           "",
		TransportID:  "local",
		Serialno:     serialno,
		Platform:     "desktop",
		OS:           osType,
		DeviceType:   util.DetectDesktopDeviceType(osType),
		IsRegistered: false,
		RegId:        localRegId,
		IsLocal:      true,
		Metadata:     make(map[string]interface{}),
	}

	dtos = append(dtos, desktopDTO)
	return dtos
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
		deviceSerial = h.serverService.GetSerialByDeviceId(deviceSerial)
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

	manager := device.NewManager("android")
	// ExecAdbCommand is only available on AndroidManager, not DeviceManager interface
	// Cast to *AndroidManager to access ExecAdbCommand
	androidMgr, ok := manager.(*device.AndroidManager)
	if !ok {
		http.Error(w, "ExecAdbCommand is only available for Android devices", http.StatusBadRequest)
		return
	}
	result, err := androidMgr.ExecAdbCommand(deviceSerial, reqBody.Command)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	RespondJSON(w, http.StatusOK, result)
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
