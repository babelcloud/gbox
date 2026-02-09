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

	// Create a unified pattern router for all /api/* routes
	apiRouter := NewPatternRouter()

	// Health and status endpoints
	apiRouter.HandleFunc("/api/health", r.handlers.HandleHealth)
	apiRouter.HandleFunc("/api/status", r.handlers.HandleStatus)

	// Device management endpoints
	apiRouter.HandleFunc("/api/devices", deviceHandlers.HandleDeviceList)
	apiRouter.HandleFunc("/api/devices/register", deviceHandlers.HandleDeviceRegister)
	apiRouter.HandleFunc("/api/devices/unregister", deviceHandlers.HandleDeviceUnregister)

	// Device-specific endpoints with path parameters
	apiRouter.HandleFunc("/api/devices/{serial}", deviceHandlers.HandleDeviceAction)
	apiRouter.HandleFunc("/api/devices/{serial}/video", deviceHandlers.HandleDeviceVideo)
	apiRouter.HandleFunc("/api/devices/{serial}/audio", deviceHandlers.HandleDeviceAudio)
	apiRouter.HandleFunc("/api/devices/{serial}/stream", deviceHandlers.HandleDeviceStream)
	apiRouter.HandleFunc("/api/devices/{serial}/control", deviceHandlers.HandleDeviceControl)
	apiRouter.HandleFunc("/api/devices/{serial}/adb", deviceHandlers.HandleDeviceAdb)
	apiRouter.HandleFunc("/api/devices/{serial}/exec", deviceHandlers.HandleDeviceExec)
	apiRouter.HandleFunc("/api/devices/{serial}/appium", deviceHandlers.HandleDeviceAppium)
	apiRouter.HandleFunc("/api/devices/{serial}/appium/{path:.*}", deviceHandlers.HandleDeviceAppium)
	apiRouter.HandleFunc("/api/devices/{serial}/screenshot", deviceHandlers.HandleDeviceScreenshot)

	// File operations endpoints
	apiRouter.HandleFunc("/api/devices/{serial}/files", deviceHandlers.HandleDeviceFiles)
	apiRouter.HandleFunc("/api/devices/{serial}/files/{action}", deviceHandlers.HandleDeviceFiles)

	// Box management endpoints (proxy to remote GBOX API)
	apiRouter.HandleFunc("/api/boxes", boxHandlers.HandleBoxList)

	// Server management endpoints
	apiRouter.HandleFunc("/api/server/shutdown", r.handlers.HandleServerShutdown)
	apiRouter.HandleFunc("/api/server/info", r.handlers.HandleServerInfo)

	// Register the unified API router
	// Note: ADB Expose endpoints are handled separately by ADBExposeRouter
	mux.HandleFunc("/api/", apiRouter.ServeHTTP)
}

// GetPathPrefix returns the path prefix for this router
func (r *APIRouter) GetPathPrefix() string {
	return "/api"
}
