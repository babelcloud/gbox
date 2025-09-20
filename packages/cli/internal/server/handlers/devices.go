package handlers

import (
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"

	"github.com/gorilla/websocket"
)

// DeviceHandlers contains handlers for device management
type DeviceHandlers struct {
	serverService ServerService
	upgrader      websocket.Upgrader
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
	// Parse URL path: /api/devices/{id}/{action}
	path := strings.TrimPrefix(r.URL.Path, "/api/devices/")
	parts := strings.Split(path, "/")

	if len(parts) < 2 {
		RespondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Invalid device action URL format",
		})
		return
	}

	deviceID := parts[0]
	action := parts[1]

	switch action {
	case "connect":
		h.handleDeviceConnect(w, r, deviceID)
	case "disconnect":
		h.handleDeviceDisconnect(w, r, deviceID)
	default:
		RespondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Unknown action: %s", action),
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