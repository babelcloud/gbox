package handlers

import (
	"io"
	"io/fs"
	"net/http"
	"strings"
	"time"
)

// AssetsHandlers contains handlers for all /assets/* routes
type AssetsHandlers struct {
	staticFS fs.FS
}

// NewAssetsHandlers creates a new assets handlers instance
func NewAssetsHandlers(staticFS fs.FS) *AssetsHandlers {
	return &AssetsHandlers{
		staticFS: staticFS,
	}
}

// HandleAssets serves static assets
func (h *AssetsHandlers) HandleAssets(w http.ResponseWriter, req *http.Request) {
	// Extract the asset path
	assetPath := strings.TrimPrefix(req.URL.Path, "/assets/")

	// Try to serve from embedded live-view assets
	if h.staticFS != nil {
		// First try the assets directory
		file, err := h.staticFS.Open("static/live-view/assets/" + assetPath)
		if err == nil {
			defer file.Close()
			// Set appropriate content type
			if strings.HasSuffix(assetPath, ".js") {
				w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			} else if strings.HasSuffix(assetPath, ".css") {
				w.Header().Set("Content-Type", "text/css; charset=utf-8")
			}
			http.ServeContent(w, req, assetPath, time.Time{}, file.(io.ReadSeeker))
			return
		}

	}

	http.NotFound(w, req)
}
