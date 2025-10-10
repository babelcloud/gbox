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

	// ADB expose endpoints
	mux.HandleFunc("/api/adb-expose/start", r.handlers.HandleADBExposeStart)
	mux.HandleFunc("/api/adb-expose/stop", r.handlers.HandleADBExposeStop)
	mux.HandleFunc("/api/adb-expose/status", r.handlers.HandleADBExposeStatus)
	mux.HandleFunc("/api/adb-expose/list", r.handlers.HandleADBExposeList)
}

// GetPathPrefix returns the path prefix for this router
func (r *ADBExposeRouter) GetPathPrefix() string {
	return "/api/adb-expose"
}
