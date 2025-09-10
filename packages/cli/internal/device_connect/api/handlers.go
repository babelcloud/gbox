package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

// handleDevices handles GET /api/devices
func (s *Server) handleDevices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	devices, err := s.deviceManager.GetDevices()
	if err != nil {
		log.Printf("Failed to get devices: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
			"devices": []interface{}{},
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":         true,
		"devices":         devices,
		"onDemandEnabled": true,
	})
}

// handleDeviceAction handles /api/devices/{id}/{action}
func (s *Server) handleDeviceAction(w http.ResponseWriter, r *http.Request) {
	// Parse URL path: /api/devices/{id}/{action}
	path := strings.TrimPrefix(r.URL.Path, "/api/devices/")
	parts := strings.Split(path, "/")
	
	if len(parts) != 2 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	
	deviceID := parts[0]
	action := parts[1]
	
	switch action {
	case "connect":
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleDeviceConnect(w, r, deviceID)
	case "disconnect":
		if r.Method != http.MethodDelete {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleDeviceDisconnect(w, r, deviceID)
	default:
		http.Error(w, "Unknown action", http.StatusNotFound)
	}
}

// handleDeviceConnect handles POST /api/devices/{id}/connect
func (s *Server) handleDeviceConnect(w http.ResponseWriter, r *http.Request, deviceID string) {
	// Create WebRTC bridge for the device
	bridge, err := s.webrtcManager.CreateBridge(deviceID)
	if err != nil {
		log.Printf("Failed to create bridge for device %s: %v", deviceID, err)
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}
	
	// Mark device as registered
	s.deviceManager.RegisterDevice(deviceID)
	
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"deviceId": deviceID,
		"proxyId":  bridge.DeviceSerial,
		"message":  "Device connected successfully",
	})
}

// handleDeviceDisconnect handles DELETE /api/devices/{id}/disconnect
func (s *Server) handleDeviceDisconnect(w http.ResponseWriter, r *http.Request, deviceID string) {
	// Remove WebRTC bridge
	s.webrtcManager.RemoveBridge(deviceID)
	
	// Mark device as unregistered
	s.deviceManager.UnregisterDevice(deviceID)
	
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"deviceId": deviceID,
		"message":  "Device disconnected successfully",
	})
}

// handleRegisterDevice handles POST /api/register-device
func (s *Server) handleRegisterDevice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		DeviceID string `json:"deviceId"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	bridge, err := s.webrtcManager.CreateBridge(req.DeviceID)
	if err != nil {
		log.Printf("Failed to create bridge for device %s: %v", req.DeviceID, err)
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	s.deviceManager.RegisterDevice(req.DeviceID)

	log.Printf("Successfully registered device %s", req.DeviceID)
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"device_id": bridge.DeviceSerial,
		"message":   "Device registered successfully",
	})
}

// handleUnregisterDevice handles POST /api/unregister-device
func (s *Server) handleUnregisterDevice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		DeviceID string `json:"deviceId"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	s.webrtcManager.RemoveBridge(req.DeviceID)
	s.deviceManager.UnregisterDevice(req.DeviceID)

	log.Printf("Successfully unregistered device %s", req.DeviceID)
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Device unregistered successfully",
	})
}