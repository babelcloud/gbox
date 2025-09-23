package handlers

import (
	"net/http"
	"os"
	"time"
)

// APIHandlers contains handlers for all /api/* routes
type APIHandlers struct {
	serverService     ServerService
	adbExposeHandlers *ADBExposeHandlers
	deviceHandlers    *DeviceHandlers
}

// NewAPIHandlers creates a new API handlers instance
func NewAPIHandlers(serverSvc ServerService) *APIHandlers {
	return &APIHandlers{
		serverService:     serverSvc,
		adbExposeHandlers: NewADBExposeHandlers(),
		deviceHandlers:    NewDeviceHandlers(serverSvc),
	}
}

// Health and status endpoints
func (h *APIHandlers) HandleHealth(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"healthy","service":"gbox-server"}`))
}

func (h *APIHandlers) HandleStatus(w http.ResponseWriter, req *http.Request) {
	if h.serverService == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"running","service":"gbox-server"}`))
		return
	}

	uptime := h.serverService.GetUptime()

	status := map[string]interface{}{
		"running": h.serverService.IsRunning(),
		"port":    h.serverService.GetPort(),
		"uptime":  uptime.String(),
		"services": map[string]interface{}{
			"device_connect": true,
			"adb_expose":     h.serverService.IsADBExposeRunning(),
		},
		"version":  h.serverService.GetVersion(),
		"build_id": h.serverService.GetBuildID(),
	}

	RespondJSON(w, http.StatusOK, status)
}

// Device management endpoints - delegate to dedicated handlers
func (h *APIHandlers) HandleDeviceList(w http.ResponseWriter, req *http.Request) {
	h.deviceHandlers.HandleDeviceList(w, req)
}

func (h *APIHandlers) HandleDeviceRegister(w http.ResponseWriter, req *http.Request) {
	h.deviceHandlers.HandleDeviceRegister(w, req)
}

func (h *APIHandlers) HandleDeviceUnregister(w http.ResponseWriter, req *http.Request) {
	h.deviceHandlers.HandleDeviceUnregister(w, req)
}

// Device-specific handlers
func (h *APIHandlers) HandleDeviceAction(w http.ResponseWriter, req *http.Request) {
	h.deviceHandlers.HandleDeviceAction(w, req)
}

func (h *APIHandlers) HandleDeviceVideo(w http.ResponseWriter, req *http.Request) {
	h.deviceHandlers.HandleDeviceVideo(w, req)
}

func (h *APIHandlers) HandleDeviceAudio(w http.ResponseWriter, req *http.Request) {
	h.deviceHandlers.HandleDeviceAudio(w, req)
}

func (h *APIHandlers) HandleDeviceControl(w http.ResponseWriter, req *http.Request) {
	h.deviceHandlers.HandleDeviceControl(w, req)
}

// ADB Expose endpoints - delegate to dedicated handlers
func (h *APIHandlers) HandleADBExposeStart(w http.ResponseWriter, req *http.Request) {
	h.adbExposeHandlers.HandleADBExposeStart(w, req)
}

func (h *APIHandlers) HandleADBExposeStop(w http.ResponseWriter, req *http.Request) {
	h.adbExposeHandlers.HandleADBExposeStop(w, req)
}

func (h *APIHandlers) HandleADBExposeStatus(w http.ResponseWriter, req *http.Request) {
	h.adbExposeHandlers.HandleADBExposeStatus(w, req)
}

func (h *APIHandlers) HandleADBExposeList(w http.ResponseWriter, req *http.Request) {
	h.adbExposeHandlers.HandleADBExposeList(w, req)
}

// Server management endpoints
func (h *APIHandlers) HandleServerShutdown(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.serverService == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotImplemented)
		w.Write([]byte(`{"message":"Server shutdown not yet implemented in new architecture"}`))
		return
	}

	RespondJSON(w, http.StatusOK, map[string]string{
		"message": "Server shutting down",
	})

	// Shutdown after response
	go func() {
		time.Sleep(100 * time.Millisecond)
		h.serverService.Stop()
		os.Exit(0)
	}()
}

func (h *APIHandlers) HandleServerInfo(w http.ResponseWriter, req *http.Request) {
	if h.serverService == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"name":"gbox-server","version":"dev","message":"Server info not yet fully implemented in new architecture"}`))
		return
	}

	uptime := h.serverService.GetUptime()

	info := map[string]interface{}{
		"version":  h.serverService.GetVersion(),
		"build_id": h.serverService.GetBuildID(),
		"port":     h.serverService.GetPort(),
		"uptime":   uptime.String(),
		"services": []string{
			"device-connect",
			"adb-expose",
		},
	}

	// Set CORS headers for debugging
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	RespondJSON(w, http.StatusOK, info)
}
