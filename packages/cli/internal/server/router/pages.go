package router

import (
	"io/fs"
	"net/http"
	"github.com/babelcloud/gbox/packages/cli/internal/server/handlers"
)

// PagesRouter handles page routes (/, /live-view, /adb-expose)
type PagesRouter struct{
	handlers *handlers.PagesHandlers
}

// RegisterRoutes registers all page routes
func (r *PagesRouter) RegisterRoutes(mux *http.ServeMux, server interface{}) {
	// Create handlers instance with static filesystem from server
	var staticFS fs.FS = nil
	if serverService, ok := server.(handlers.ServerService); ok {
		staticFS = serverService.GetStaticFS()
	}
	r.handlers = handlers.NewPagesHandlers(staticFS)

	// Main pages
	mux.HandleFunc("/live-view", r.handlers.HandleLiveView)
	mux.HandleFunc("/live-view/", r.handlers.HandleLiveView)
	mux.HandleFunc("/adb-expose", r.handlers.HandleADBExpose)
	mux.HandleFunc("/adb-expose/", r.handlers.HandleADBExpose)

	// Root handler (must be registered last)
	mux.HandleFunc("/", r.handlers.HandleRoot)
}


// GetPathPrefix returns the path prefix for this router
func (r *PagesRouter) GetPathPrefix() string {
	return "/"
}