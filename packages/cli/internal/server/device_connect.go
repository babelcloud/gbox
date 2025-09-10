package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now
	},
}

// Device Connect API handlers

func (s *GBoxServer) handleDevices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	devices, err := s.getADBDevices()
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

func (s *GBoxServer) handleDeviceAction(w http.ResponseWriter, r *http.Request) {
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

func (s *GBoxServer) handleDeviceConnect(w http.ResponseWriter, r *http.Request, deviceID string) {
	// Register the device with WebRTC manager
	bridge, err := s.webrtcManager.CreateBridge(deviceID)
	if err != nil {
		log.Printf("Failed to create bridge for device %s: %v", deviceID, err)
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"deviceId": deviceID,
		"bridgeId": bridge.DeviceSerial,
		"message":  "Device connected successfully",
	})
}

func (s *GBoxServer) handleDeviceDisconnect(w http.ResponseWriter, r *http.Request, deviceID string) {
	// Unregister the device from WebRTC manager
	s.webrtcManager.RemoveBridge(deviceID)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"deviceId": deviceID,
		"message":  "Device disconnected successfully",
	})
}

func (s *GBoxServer) handleRegisterDevice(w http.ResponseWriter, r *http.Request) {
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

	log.Printf("Successfully registered device %s", req.DeviceID)
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"device_id": bridge.DeviceSerial,
		"message":   "Device registered successfully",
	})
}

func (s *GBoxServer) handleUnregisterDevice(w http.ResponseWriter, r *http.Request) {
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

	log.Printf("Successfully unregistered device %s", req.DeviceID)
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Device unregistered successfully",
	})
}

func (s *GBoxServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Proxy WebSocket to device_connect server
	// For now, use the existing webrtc manager logic
	// TODO: Implement proper proxy to device_connect server

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade WebSocket: %v", err)
		return
	}
	defer conn.Close()

	log.Println("WebSocket connection established (main server)")

	for {
		var msg map[string]interface{}
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket read error: %v", err)
			}
			break
		}

		msgType, ok := msg["type"].(string)
		if !ok {
			continue
		}

		switch msgType {
		case "connect":
			s.handleWebSocketConnect(conn, msg)
		case "offer":
			s.handleWebSocketOffer(conn, msg)
		case "ice-candidate":
			s.handleWebSocketICECandidate(conn, msg)
		case "disconnect":
			s.handleWebSocketDisconnect(conn, msg)
		}
	}
}

func (s *GBoxServer) handleWebSocketConnect(conn *websocket.Conn, msg map[string]interface{}) {
	deviceSerial, ok := msg["deviceSerial"].(string)
	if !ok {
		conn.WriteJSON(map[string]interface{}{
			"type":  "error",
			"error": "Device serial required",
		})
		return
	}

	bridge, exists := s.webrtcManager.GetBridge(deviceSerial)
	if !exists {
		var err error
		bridge, err = s.webrtcManager.CreateBridge(deviceSerial)
		if err != nil {
			log.Printf("Failed to create bridge: %v", err)
			conn.WriteJSON(map[string]interface{}{
				"type":  "error",
				"error": err.Error(),
			})
			return
		}
	}

	bridge.WSConnection = conn

	conn.WriteJSON(map[string]interface{}{
		"type":         "connected",
		"deviceSerial": deviceSerial,
	})
}

func (s *GBoxServer) handleWebSocketOffer(conn *websocket.Conn, msg map[string]interface{}) {
	deviceSerial, ok := msg["deviceSerial"].(string)
	if !ok {
		return
	}

	offerData, ok := msg["offer"].(map[string]interface{})
	if !ok {
		return
	}

	sdp, ok := offerData["sdp"].(string)
	if !ok {
		return
	}

	// Get or create bridge for the device
	bridge, exists := s.webrtcManager.GetBridge(deviceSerial)
	if !exists {
		log.Printf("Bridge not found for device %s, creating new bridge", deviceSerial)
		var err error
		bridge, err = s.webrtcManager.CreateBridge(deviceSerial)
		if err != nil {
			log.Printf("Failed to create bridge: %v", err)
			conn.WriteJSON(map[string]interface{}{
				"type":  "error",
				"error": fmt.Sprintf("Failed to connect to device: %v", err),
			})
			return
		}
	}

	// Check signaling state - only recreate if truly necessary
	signalingState := bridge.WebRTCConn.SignalingState()
	connState := bridge.WebRTCConn.ConnectionState()

	log.Printf("Bridge state for device %s: signaling=%s, connection=%s", deviceSerial, signalingState, connState)

	// Only recreate bridge if connection is truly closed or failed
	if connState == webrtc.PeerConnectionStateClosed || connState == webrtc.PeerConnectionStateFailed {
		log.Printf("WebRTC connection is %s for device %s, recreating bridge", connState, deviceSerial)
		s.webrtcManager.RemoveBridge(deviceSerial)

		// Create new bridge
		var err error
		bridge, err = s.webrtcManager.CreateBridge(deviceSerial)
		if err != nil {
			log.Printf("Failed to recreate bridge: %v", err)
			conn.WriteJSON(map[string]interface{}{
				"type":  "error",
				"error": fmt.Sprintf("Failed to reconnect to device: %v", err),
			})
			return
		}
	} else if signalingState == webrtc.SignalingStateClosed {
		// Only recreate if signaling is closed but connection is still active
		log.Printf("Signaling state is closed for device %s, recreating bridge", deviceSerial)
		s.webrtcManager.RemoveBridge(deviceSerial)

		var err error
		bridge, err = s.webrtcManager.CreateBridge(deviceSerial)
		if err != nil {
			log.Printf("Failed to recreate bridge: %v", err)
			conn.WriteJSON(map[string]interface{}{
				"type":  "error",
				"error": fmt.Sprintf("Failed to reset connection: %v", err),
			})
			return
		}
	}

	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  sdp,
	}

	if err := bridge.WebRTCConn.SetRemoteDescription(offer); err != nil {
		log.Printf("Failed to set remote description: %v", err)
		conn.WriteJSON(map[string]interface{}{
			"type":  "error",
			"error": err.Error(),
		})
		return
	}

	answer, err := bridge.WebRTCConn.CreateAnswer(nil)
	if err != nil {
		log.Printf("Failed to create answer: %v", err)
		conn.WriteJSON(map[string]interface{}{
			"type":  "error",
			"error": err.Error(),
		})
		return
	}

	if err := bridge.WebRTCConn.SetLocalDescription(answer); err != nil {
		log.Printf("Failed to set local description: %v", err)
		conn.WriteJSON(map[string]interface{}{
			"type":  "error",
			"error": err.Error(),
		})
		return
	}

	conn.WriteJSON(map[string]interface{}{
		"type": "answer",
		"answer": map[string]interface{}{
			"type": "answer",
			"sdp":  answer.SDP,
		},
	})

	bridge.WebRTCConn.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}

		candidateJSON := candidate.ToJSON()
		conn.WriteJSON(map[string]interface{}{
			"type": "ice-candidate",
			"candidate": map[string]interface{}{
				"candidate":     candidateJSON.Candidate,
				"sdpMLineIndex": candidateJSON.SDPMLineIndex,
				"sdpMid":        candidateJSON.SDPMid,
			},
		})
	})

	// Device info is not needed by frontend, video dimensions will be available through video track
}

func (s *GBoxServer) handleWebSocketICECandidate(conn *websocket.Conn, msg map[string]interface{}) {
	deviceSerial, ok := msg["deviceSerial"].(string)
	if !ok {
		return
	}

	candidateData, ok := msg["candidate"].(map[string]interface{})
	if !ok {
		return
	}

	bridge, exists := s.webrtcManager.GetBridge(deviceSerial)
	if !exists {
		return
	}

	candidate := webrtc.ICECandidateInit{
		Candidate: candidateData["candidate"].(string),
	}

	if sdpMLineIndex, ok := candidateData["sdpMLineIndex"].(float64); ok {
		index := uint16(sdpMLineIndex)
		candidate.SDPMLineIndex = &index
	}

	if sdpMid, ok := candidateData["sdpMid"].(string); ok {
		candidate.SDPMid = &sdpMid
	}

	if err := bridge.WebRTCConn.AddICECandidate(candidate); err != nil {
		log.Printf("Failed to add ICE candidate: %v", err)
	}
}

func (s *GBoxServer) handleWebSocketDisconnect(conn *websocket.Conn, msg map[string]interface{}) {
	deviceSerial, ok := msg["deviceSerial"].(string)
	if !ok {
		return
	}

	s.webrtcManager.RemoveBridge(deviceSerial)

	conn.WriteJSON(map[string]interface{}{
		"type": "disconnected",
	})
}

func (s *GBoxServer) getADBDevices() ([]map[string]interface{}, error) {
	adbPath, err := exec.LookPath("adb")
	if err != nil {
		return nil, fmt.Errorf("adb not found in PATH")
	}

	cmd := exec.Command(adbPath, "devices", "-l")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run adb devices: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	devices := []map[string]interface{}{}

	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		serial := parts[0]
		state := parts[1]

		if state != "device" {
			continue
		}

		device := map[string]interface{}{
			"id":             serial,
			"udid":           serial,
			"state":          state,
			"ro.serialno":    serial,
			"connectionType": "usb",
			"isRegistrable":  false,
		}

		if strings.Contains(line, "model:") {
			if idx := strings.Index(line, "model:"); idx != -1 {
				modelPart := line[idx+6:]
				if spaceIdx := strings.Index(modelPart, " "); spaceIdx != -1 {
					device["ro.product.model"] = modelPart[:spaceIdx]
				} else {
					device["ro.product.model"] = modelPart
				}
			}
		}

		if strings.Contains(line, "device:") {
			if idx := strings.Index(line, "device:"); idx != -1 {
				devicePart := line[idx+7:]
				if spaceIdx := strings.Index(devicePart, " "); spaceIdx != -1 {
					device["ro.product.manufacturer"] = devicePart[:spaceIdx]
				}
			}
		}

		if strings.Contains(serial, ":") {
			device["connectionType"] = "ip"
		}

		if _, exists := s.webrtcManager.GetBridge(serial); exists {
			device["isRegistrable"] = true
		}

		devices = append(devices, device)
	}

	return devices, nil
}
