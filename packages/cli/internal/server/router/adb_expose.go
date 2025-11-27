package router

import (
	"net/http"

	"github.com/babelcloud/gbox/packages/cli/internal/server/handlers"
)

// ADBExposeRouter handles all ADB expose routes
type ADBExposeRouter struct {
	handlers *handlers.ADBExposeHandlers
}

// RegisterRoutes registers all ADB expose routes
func (r *ADBExposeRouter) RegisterRoutes(mux *http.ServeMux, server interface{}) {
	// Create handlers instance
	r.handlers = handlers.NewADBExposeHandlers()

	// Create pattern router for ADB expose endpoints
	adbExposeRouter := NewPatternRouter()
	adbExposeRouter.HandleFunc("/api/adb-expose/start", r.handlers.HandleADBExposeStart)
	adbExposeRouter.HandleFunc("/api/adb-expose/stop", r.handlers.HandleADBExposeStop)
	adbExposeRouter.HandleFunc("/api/adb-expose/status", r.handlers.HandleADBExposeStatus)
	adbExposeRouter.HandleFunc("/api/adb-expose/list", r.handlers.HandleADBExposeList)

	// Register pattern router
	mux.HandleFunc("/api/adb-expose/", adbExposeRouter.ServeHTTP)
}

// GetPathPrefix returns the path prefix for this router
func (r *ADBExposeRouter) GetPathPrefix() string {
	return "/api/adb-expose"
}
