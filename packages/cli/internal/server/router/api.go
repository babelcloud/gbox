package router

import (
	"net/http"
	"github.com/babelcloud/gbox/packages/cli/internal/server/handlers"
)

// APIRouter handles all /api/* routes
type APIRouter struct{
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

	// Create box handlers separately
	boxHandlers := handlers.NewBoxHandlers(serverService)

	// Health and status endpoints
	mux.HandleFunc("/api/health", r.handlers.HandleHealth)
	mux.HandleFunc("/api/status", r.handlers.HandleStatus)

	// Device management endpoints
	mux.HandleFunc("/api/devices", r.handlers.HandleDeviceList)
	mux.HandleFunc("/api/devices/", r.handlers.HandleDeviceAction)
	mux.HandleFunc("/api/devices/register", r.handlers.HandleDeviceRegister)
	mux.HandleFunc("/api/devices/unregister", r.handlers.HandleDeviceUnregister)

	// Box management endpoints (proxy to remote GBOX API)
	mux.HandleFunc("/api/boxes", boxHandlers.HandleBoxList)

	// Note: Streaming endpoints are handled by StreamingRouter

	// ADB Expose endpoints
	mux.HandleFunc("/api/adb-expose/start", r.handlers.HandleADBExposeStart)
	mux.HandleFunc("/api/adb-expose/stop", r.handlers.HandleADBExposeStop)
	mux.HandleFunc("/api/adb-expose/status", r.handlers.HandleADBExposeStatus)
	mux.HandleFunc("/api/adb-expose/list", r.handlers.HandleADBExposeList)

	// Server management endpoints
	mux.HandleFunc("/api/server/shutdown", r.handlers.HandleServerShutdown)
	mux.HandleFunc("/api/server/info", r.handlers.HandleServerInfo)
}

// GetPathPrefix returns the path prefix for this router
func (r *APIRouter) GetPathPrefix() string {
	return "/api"
}

