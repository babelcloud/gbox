package router

import (
	"net/http"

	"github.com/babelcloud/gbox/packages/cli/internal/server/handlers"
)

// APIRouter handles all /api/* routes
type APIRouter struct {
	handlers *handlers.APIHandlers
}

// RegisterRoutes registers all API routes
func (r *APIRouter) RegisterRoutes(mux *http.ServeMux, server interface{}) {
	// Cast server to ServerService
	var serverService handlers.ServerService
	if srv, ok := server.(handlers.ServerService); ok {
		serverService = srv
	}

	// Create handlers instance with actual server service
	r.handlers = handlers.NewAPIHandlers(serverService)

	// Create device handlers separately for direct routing
	deviceHandlers := handlers.NewDeviceHandlers(serverService)

	// Create box handlers separately
	boxHandlers := handlers.NewBoxHandlers(serverService)

	// Health and status endpoints
	mux.HandleFunc("/api/health", r.handlers.HandleHealth)
	mux.HandleFunc("/api/status", r.handlers.HandleStatus)

	// Device management endpoints - direct routing to device handlers
	mux.HandleFunc("/api/devices", deviceHandlers.HandleDeviceList)
	mux.HandleFunc("/api/devices/register", deviceHandlers.HandleDeviceRegister)
	mux.HandleFunc("/api/devices/unregister", deviceHandlers.HandleDeviceUnregister)

	// Device-specific endpoints with path patterns - direct routing to device handlers
	mux.HandleFunc("/api/devices/{serial}", deviceHandlers.HandleDeviceAction)
	mux.HandleFunc("/api/devices/{serial}/video", deviceHandlers.HandleDeviceVideo)
	mux.HandleFunc("/api/devices/{serial}/audio", deviceHandlers.HandleDeviceAudio)
	mux.HandleFunc("/api/devices/{serial}/stream", deviceHandlers.HandleDeviceStream)
	mux.HandleFunc("/api/devices/{serial}/control", deviceHandlers.HandleDeviceControl)
	mux.HandleFunc("/api/devices/{serial}/test/http", deviceHandlers.HandleDeviceTestHttp)
	mux.HandleFunc("/api/devices/{serial}/test/ws", deviceHandlers.HandleDeviceTestWs)

	// Box management endpoints (proxy to remote GBOX API)
	mux.HandleFunc("/api/boxes", boxHandlers.HandleBoxList)

	// Note: ADB Expose endpoints are handled by ADBExposeRouter

	// Server management endpoints
	mux.HandleFunc("/api/server/shutdown", r.handlers.HandleServerShutdown)
	mux.HandleFunc("/api/server/info", r.handlers.HandleServerInfo)
}

// GetPathPrefix returns the path prefix for this router
func (r *APIRouter) GetPathPrefix() string {
	return "/api"
}
