package server

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// StaticFileConfig represents configuration for serving static files
type StaticFileConfig struct {
	// BasePath is the base directory to serve files from
	BasePath string
	// FallbackFile is the file to serve if the requested file is not found
	FallbackFile string
	// ContentType is the MIME type to set (if empty, will be auto-detected)
	ContentType string
	// RedirectPath is the path to redirect to if no fallback is available
	RedirectPath string
}

// serveStaticFile serves a static file with the given configuration
func (s *GBoxServer) serveStaticFile(w http.ResponseWriter, r *http.Request, config StaticFileConfig) {
	// Extract the requested file path from the URL
	requestedPath := strings.TrimPrefix(r.URL.Path, "/")
	if requestedPath == "" {
		requestedPath = "index.html"
	}

	// Try to serve the requested file
	filePath := filepath.Join(config.BasePath, requestedPath)
	if s.tryServeFile(w, filePath, config.ContentType) {
		return
	}

	// Try to serve fallback file
	if config.FallbackFile != "" {
		fallbackPath := filepath.Join(config.BasePath, config.FallbackFile)
		if s.tryServeFile(w, fallbackPath, config.ContentType) {
			return
		}
	}

	// Try to serve from server static directory as last resort
	if config.BasePath != s.findStaticPath() {
		serverStaticPath := s.findStaticPath()
		if serverStaticPath != "" {
			serverFilePath := filepath.Join(serverStaticPath, requestedPath)
			if s.tryServeFile(w, serverFilePath, config.ContentType) {
				return
			}

			if config.FallbackFile != "" {
				serverFallbackPath := filepath.Join(serverStaticPath, config.FallbackFile)
				if s.tryServeFile(w, serverFallbackPath, config.ContentType) {
					return
				}
			}
		}
	}

	// If redirect path is specified, redirect
	if config.RedirectPath != "" {
		http.Redirect(w, r, config.RedirectPath, http.StatusTemporaryRedirect)
		return
	}

	// Final fallback: simple error message
	s.serveErrorPage(w, requestedPath)
}

// tryServeFile attempts to serve a file and returns true if successful
func (s *GBoxServer) tryServeFile(w http.ResponseWriter, filePath string, contentType string) bool {
	// Check if file exists
	if _, err := os.Stat(filePath); err != nil {
		return false
	}

	// Open and serve the file
	file, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer file.Close()

	// Set content type
	if contentType == "" {
		contentType = s.getContentType(filePath)
	}
	w.Header().Set("Content-Type", contentType)

	// Copy file content to response
	_, err = io.Copy(w, file)
	return err == nil
}

// tryServeEmbeddedFile attempts to serve a file from embedded FS and returns true if successful
func (s *GBoxServer) tryServeEmbeddedFile(w http.ResponseWriter, fsys fs.FS, filePath string, contentType string) bool {
	// Check if file exists in embedded FS
	if _, err := fs.Stat(fsys, filePath); err != nil {
		return false
	}

	// Open and serve the file
	file, err := fsys.Open(filePath)
	if err != nil {
		return false
	}
	defer file.Close()

	// Set content type
	if contentType == "" {
		contentType = s.getContentType(filePath)
	}
	w.Header().Set("Content-Type", contentType)

	// Copy file content to response
	_, err = io.Copy(w, file)
	return err == nil
}

// getContentType determines the MIME type based on file extension
func (s *GBoxServer) getContentType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "application/javascript; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".ico":
		return "image/x-icon"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	case ".eot":
		return "application/vnd.ms-fontobject"
	default:
		return "text/plain; charset=utf-8"
	}
}

// serveErrorPage serves a simple error page
func (s *GBoxServer) serveErrorPage(w http.ResponseWriter, requestedPath string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Not Found - GBOX Local Server</title>
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        body { 
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: #0a0a0a; color: #fff; margin: 0; padding: 2rem;
            display: flex; align-items: center; justify-content: center; min-height: 100vh;
        }
        .container { text-align: center; max-width: 600px; }
        h1 { color: #ef4444; margin-bottom: 1rem; }
        .back-link { color: #667eea; text-decoration: none; }
        .back-link:hover { text-decoration: underline; }
    </style>
</head>
<body>
    <div class="container">
        <h1>404 - Not Found</h1>
        <p>The requested file "%s" was not found.</p>
        <a href="/" class="back-link">← Back to Home</a>
    </div>
</body>
</html>`, requestedPath)

	fmt.Fprint(w, html)
}

// handleLiveViewHTML serves the live-view.html file
func (s *GBoxServer) handleLiveViewHTML(w http.ResponseWriter, r *http.Request) {
	config := StaticFileConfig{
		BasePath:     s.findLiveViewStaticPath(),
		FallbackFile: "index.html",
		RedirectPath: "/live-view",
	}
	s.serveStaticFile(w, r, config)
}

// handleLiveViewAssets serves assets from the live-view static directory
func (s *GBoxServer) handleLiveViewAssets(w http.ResponseWriter, r *http.Request) {
	liveViewPath := s.findLiveViewStaticPath()
	if liveViewPath == "" {
		http.NotFound(w, r)
		return
	}

	// Remove /assets/ prefix and serve from live-view static directory
	requestedPath := strings.TrimPrefix(r.URL.Path, "/assets/")
	filePath := filepath.Join(liveViewPath, "assets", requestedPath)

	// Set appropriate content type
	contentType := s.getContentType(filePath)

	if s.tryServeFile(w, filePath, contentType) {
		return
	}

	http.NotFound(w, r)
}

// handleLiveView serves the live-view application
func (s *GBoxServer) handleLiveView(w http.ResponseWriter, r *http.Request) {
	liveViewPath := s.findLiveViewStaticPath()

	// Note: embedded files removed - using external files only

	// Fallback to external files or placeholder
	config := StaticFileConfig{
		BasePath:     liveViewPath,
		FallbackFile: "live-view.html",
	}
	s.serveStaticFile(w, r, config)
}

// handleAdbExposeUI serves the ADB Expose management interface
func (s *GBoxServer) handleAdbExposeUI(w http.ResponseWriter, r *http.Request) {
	config := StaticFileConfig{
		BasePath:     s.findStaticPath(),
		FallbackFile: "adb-expose.html",
	}
	s.serveStaticFile(w, r, config)
}

// handleStatic serves general static files from the server static directory
func (s *GBoxServer) handleStatic(w http.ResponseWriter, r *http.Request) {
	config := StaticFileConfig{
		BasePath: s.findStaticPath(),
	}
	s.serveStaticFile(w, r, config)
}
