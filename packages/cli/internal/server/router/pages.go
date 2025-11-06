package router

import (
	"io/fs"
	"net/http"

	"github.com/babelcloud/gbox/packages/cli/internal/server/handlers"
)

// PagesRouter handles page routes (/, /live-view, /adb-expose)
type PagesRouter struct {
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

	// Create pattern router for page routes
	pagesRouter := NewPatternRouter()

	// Page routes with optional trailing slash
	pagesRouter.HandleFunc("/live-view", r.handlers.HandleLiveView)
	pagesRouter.HandleFunc("/live-view/{path:.*}", r.handlers.HandleLiveView)
	pagesRouter.HandleFunc("/adb-expose", r.handlers.HandleADBExpose)
	pagesRouter.HandleFunc("/adb-expose/{path:.*}", r.handlers.HandleADBExpose)

	// Root handler (catches all unmatched routes)
	pagesRouter.HandleFunc("/{path:.*}", r.handlers.HandleRoot)

	// Register pattern router
	mux.HandleFunc("/", pagesRouter.ServeHTTP)
}

// GetPathPrefix returns the path prefix for this router
func (r *PagesRouter) GetPathPrefix() string {
	return "/"
}
