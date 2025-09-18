package router

import (
	"io/fs"
	"net/http"

	"github.com/babelcloud/gbox/packages/cli/internal/server/handlers"
)

// AssetsRouter handles all /assets/* routes
type AssetsRouter struct {
	handlers *handlers.AssetsHandlers
}

// RegisterRoutes registers all static asset routes
func (r *AssetsRouter) RegisterRoutes(mux *http.ServeMux, server interface{}) {
	// Get static filesystem from server
	var staticFS fs.FS
	if serverService, ok := server.(handlers.ServerService); ok {
		staticFS = serverService.GetStaticFS()
	}

	// Create handlers instance
	r.handlers = handlers.NewAssetsHandlers(staticFS)

	// Main assets handler
	mux.HandleFunc("/assets/", r.handlers.HandleAssets)
}

// GetPathPrefix returns the path prefix for this router
func (r *AssetsRouter) GetPathPrefix() string {
	return "/assets"
}
