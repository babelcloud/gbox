package handlers

import (
	"io"
	"io/fs"
	"net/http"
	"time"
)

// PagesHandlers contains handlers for page routes (/, /live-view, /adb-expose)
type PagesHandlers struct{
	staticFS fs.FS // Static files filesystem
}

// NewPagesHandlers creates a new pages handlers instance
func NewPagesHandlers(staticFS fs.FS) *PagesHandlers {
	return &PagesHandlers{
		staticFS: staticFS,
	}
}

// HandleLiveView handles /live-view and /live-view/
func (h *PagesHandlers) HandleLiveView(w http.ResponseWriter, req *http.Request) {
	// Redirect to built live-view app if available, otherwise serve fallback
	h.serveLiveViewPage(w, req)
}


// serveLiveViewPage serves the live-view page
func (h *PagesHandlers) serveLiveViewPage(w http.ResponseWriter, req *http.Request) {
	// Try to serve embedded live-view index.html
	if h.staticFS != nil {
		file, err := h.staticFS.Open("static/live-view/index.html")
		if err == nil {
			defer file.Close()
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			http.ServeContent(w, req, "index.html", time.Time{}, file.(io.ReadSeeker))
			return
		}
	}

	// Fallback error message
	http.Error(w, "Live view not available. Please rebuild with embedded static files.", http.StatusNotFound)
}

// HandleADBExpose handles /adb-expose and /adb-expose/
func (h *PagesHandlers) HandleADBExpose(w http.ResponseWriter, req *http.Request) {
	// Try to serve adb-expose.html from static files
	if h.staticFS != nil {
		file, err := h.staticFS.Open("static/adb-expose.html")
		if err == nil {
			defer file.Close()
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			http.ServeContent(w, req, "adb-expose.html", time.Time{}, file.(io.ReadSeeker))
			return
		}
	}

	// Fallback error
	http.Error(w, "ADB expose page not available", http.StatusNotFound)
}

// HandleRoot handles / (root path)
func (h *PagesHandlers) HandleRoot(w http.ResponseWriter, req *http.Request) {
	// Only handle exact root path
	if req.URL.Path != "/" {
		http.NotFound(w, req)
		return
	}

	// Try to serve index.html from static files
	if h.staticFS != nil {
		// Try to serve index.html from static subdirectory
		file, err := h.staticFS.Open("static/index.html")
		if err == nil {
			defer file.Close()
			// Set content type
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			// Copy file content to response
			http.ServeContent(w, req, "index.html", time.Time{}, file.(io.ReadSeeker))
			return
		}
	}

	// Fallback: simple status page
	http.Error(w, "Static files not available", http.StatusNotFound)
}