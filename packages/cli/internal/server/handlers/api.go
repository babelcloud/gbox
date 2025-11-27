package handlers

import (
	"net/http"
	"os"
	"time"
)

// APIHandlers contains handlers for general API routes (health, status, server management)
type APIHandlers struct {
	serverService ServerService
}

// NewAPIHandlers creates a new API handlers instance
func NewAPIHandlers(serverSvc ServerService) *APIHandlers {
	return &APIHandlers{
		serverService: serverSvc,
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

// Device management endpoints are now handled directly by DeviceHandlers in the router
// ADB Expose endpoints are now handled directly by ADBExposeHandlers in the ADBExposeRouter

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

	// Set version headers for client verification
	w.Header().Set("X-GBOX-Version", h.serverService.GetVersion())
	w.Header().Set("X-GBOX-Build-ID", h.serverService.GetBuildID())

	RespondJSON(w, http.StatusOK, info)
}
