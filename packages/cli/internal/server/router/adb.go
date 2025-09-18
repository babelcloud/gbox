package router

import (
	"net/http"
	"github.com/babelcloud/gbox/packages/cli/internal/server/handlers"
)

// ADBRouter handles all ADB expose routes
type ADBRouter struct {
	handlers *handlers.ADBExposeHandlers
}

// RegisterRoutes registers all ADB expose routes
func (r *ADBRouter) RegisterRoutes(mux *http.ServeMux, server interface{}) {
	// Create handlers instance
	r.handlers = handlers.NewADBExposeHandlers()

	// ADB expose endpoints
	mux.HandleFunc("/api/adb/expose/start", r.handlers.HandleADBExposeStart)
	mux.HandleFunc("/api/adb/expose/stop", r.handlers.HandleADBExposeStop)
	mux.HandleFunc("/api/adb/expose/status", r.handlers.HandleADBExposeStatus)
	mux.HandleFunc("/api/adb/expose/list", r.handlers.HandleADBExposeList)
}

// GetPathPrefix returns the path prefix for this router
func (r *ADBRouter) GetPathPrefix() string {
	return "/api/adb"
}