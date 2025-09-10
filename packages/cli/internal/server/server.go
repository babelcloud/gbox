package server

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/webrtc"
)

//go:embed all:static
var staticFiles embed.FS

// GBoxServer is the unified server for all gbox services
type GBoxServer struct {
	port         int
	httpServer   *http.Server
	mux          *http.ServeMux
	
	// Services
	webrtcManager *webrtc.Manager
	adbExpose     *ADBExposeService
	
	// State
	mu           sync.RWMutex
	running      bool
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewGBoxServer creates a new unified gbox server
func NewGBoxServer(port int) *GBoxServer {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &GBoxServer{
		port:          port,
		mux:           http.NewServeMux(),
		webrtcManager: webrtc.NewManager("adb"),
		adbExpose:     NewADBExposeService(),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start starts the unified server
func (s *GBoxServer) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.running {
		return fmt.Errorf("server already running")
	}
	
	// Setup routes
	s.setupRoutes()
	
	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: s.mux,
	}
	
	// Start server in background
	go func() {
		log.Printf("Starting GBox server on port %d", s.port)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()
	
	s.running = true
	log.Printf("GBox server started successfully on http://localhost:%d", s.port)
	return nil
}

// Stop stops the server
func (s *GBoxServer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if !s.running {
		return nil
	}
	
	s.cancel()
	
	// Shutdown HTTP server
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.httpServer.Shutdown(ctx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
		}
	}
	
	// Cleanup services
	s.webrtcManager.Close()
	s.adbExpose.Close()
	
	s.running = false
	log.Println("GBox server stopped")
	return nil
}

// IsRunning returns whether the server is running
func (s *GBoxServer) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// setupRoutes sets up all HTTP routes
func (s *GBoxServer) setupRoutes() {
	// Health check
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/api/status", s.handleStatus)
	
	// Device Connect API (scrcpy/WebRTC)
	s.mux.HandleFunc("/api/devices", s.handleDevices)
	s.mux.HandleFunc("/api/devices/", s.handleDeviceAction) // Handles /api/devices/{id}/connect and /api/devices/{id}/disconnect
	s.mux.HandleFunc("/api/devices/register", s.handleRegisterDevice)
	s.mux.HandleFunc("/api/devices/unregister", s.handleUnregisterDevice)
	s.mux.HandleFunc("/ws", s.handleWebSocket)
	
	// ADB Expose API
	s.mux.HandleFunc("/api/adb-expose/start", s.handleADBExposeStart)
	s.mux.HandleFunc("/api/adb-expose/stop", s.handleADBExposeStop)
	s.mux.HandleFunc("/api/adb-expose/status", s.handleADBExposeStatus)
	s.mux.HandleFunc("/api/adb-expose/list", s.handleADBExposeList)
	
	// Server management API
	s.mux.HandleFunc("/api/server/shutdown", s.handleShutdown)
	s.mux.HandleFunc("/api/server/info", s.handleServerInfo)
	
	// Sub-applications - handle both with and without trailing slash
	s.mux.HandleFunc("/live-view", s.handleLiveView)
	s.mux.HandleFunc("/live-view/", s.handleLiveView) 
	s.mux.HandleFunc("/live-view.html", s.handleLiveViewHTML)
	s.mux.HandleFunc("/adb-expose", s.handleAdbExposeUI)
	s.mux.HandleFunc("/adb-expose/", s.handleAdbExposeUI)
	
	// Static files and web UI routes - must be last
	s.setupStaticFiles()
}

// setupStaticFiles sets up static file serving
func (s *GBoxServer) setupStaticFiles() {
	// First, try to serve live-view static files if available
	liveViewPath := s.findLiveViewStaticPath()
	if liveViewPath != "" {
		// Serve assets directory for CSS/JS files
		assetsPath := filepath.Join(liveViewPath, "assets")
		if _, err := os.Stat(assetsPath); err == nil {
			s.mux.Handle("/assets/", http.StripPrefix("/assets/", s.serveStaticWithMIME(http.Dir(assetsPath))))
		}
		// Also handle root static files
		s.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(liveViewPath))))
	}
	
	// Try embedded files
	staticFS, err := fs.Sub(staticFiles, "static")
	if err == nil {
		s.mux.Handle("/", http.FileServer(http.FS(staticFS)))
		log.Println("Serving embedded static files")
	} else {
		// Fallback to a simple status page
		s.mux.HandleFunc("/", s.handleRoot)
	}
}

// serveStaticWithMIME wraps a file server to set correct MIME types
func (s *GBoxServer) serveStaticWithMIME(fs http.FileSystem) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set correct MIME types based on file extension
		if strings.HasSuffix(r.URL.Path, ".css") {
			w.Header().Set("Content-Type", "text/css")
		} else if strings.HasSuffix(r.URL.Path, ".js") {
			w.Header().Set("Content-Type", "application/javascript")
		} else if strings.HasSuffix(r.URL.Path, ".html") {
			w.Header().Set("Content-Type", "text/html")
		}
		http.FileServer(fs).ServeHTTP(w, r)
	})
}

// findLiveViewStaticPath finds the live-view build output
func (s *GBoxServer) findLiveViewStaticPath() string {
	// Try various possible locations for the live-view static files
	possiblePaths := []string{
		// Relative to gbox binary location
		"../../live-view/static",
		"../live-view/static",
		"packages/live-view/static",
		// In gbox workspace
		"/Users/duwan/Workspaces/babelcloud/gbox/packages/live-view/static",
		// In user's home directory
		filepath.Join(os.Getenv("HOME"), ".gbox", "live-view-static"),
		// Development paths
		"./packages/live-view/static",
		"../../../gbox/packages/live-view/static",
	}
	
	for _, path := range possiblePaths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		if info, err := os.Stat(absPath); err == nil && info.IsDir() {
			if _, err := os.Stat(filepath.Join(absPath, "index.html")); err == nil {
				log.Printf("Found live-view static files at: %s", absPath)
				return absPath
			}
		}
	}
	
	log.Printf("Warning: Live-view static files not found, using default status page")
	return ""
}

// API Handlers

func (s *GBoxServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (s *GBoxServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"running":      s.IsRunning(),
		"port":         s.port,
		"services": map[string]interface{}{
			"device_connect": true,
			"adb_expose":     s.adbExpose.IsRunning(),
		},
		"version": "1.0.0",
	}
	
	respondJSON(w, http.StatusOK, status)
}

func (s *GBoxServer) handleRoot(w http.ResponseWriter, r *http.Request) {
	// Only serve root page for exact path match
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	
	html := `<!DOCTYPE html>
<html>
<head>
    <title>GBox Server</title>
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { 
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: linear-gradient(135deg, #0f0f0f 0%, #1a1a1a 100%);
            color: #fff;
            min-height: 100vh;
            display: flex;
            flex-direction: column;
            align-items: center;
            justify-content: center;
            padding: 2rem;
        }
        .container {
            max-width: 900px;
            width: 100%;
        }
        .header {
            text-align: center;
            margin-bottom: 3rem;
        }
        h1 { 
            font-size: 3rem;
            margin-bottom: 0.5rem;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
        }
        .subtitle {
            color: #888;
            font-size: 1.2rem;
        }
        .cards {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(350px, 1fr));
            gap: 2rem;
            margin-top: 3rem;
        }
        .card {
            background: rgba(255, 255, 255, 0.05);
            backdrop-filter: blur(10px);
            border: 1px solid rgba(255, 255, 255, 0.1);
            border-radius: 16px;
            padding: 2rem;
            transition: transform 0.3s, box-shadow 0.3s;
            cursor: pointer;
            text-decoration: none;
            color: inherit;
            display: block;
        }
        .card:hover {
            transform: translateY(-5px);
            box-shadow: 0 20px 40px rgba(102, 126, 234, 0.2);
            border-color: rgba(102, 126, 234, 0.5);
        }
        .card-icon {
            font-size: 3rem;
            margin-bottom: 1rem;
        }
        .card-title {
            font-size: 1.5rem;
            margin-bottom: 0.5rem;
            color: #fff;
        }
        .card-description {
            color: #aaa;
            line-height: 1.6;
        }
        .status {
            position: fixed;
            top: 2rem;
            right: 2rem;
            padding: 0.5rem 1rem;
            background: rgba(74, 222, 128, 0.1);
            border: 1px solid #4ade80;
            border-radius: 20px;
            font-size: 0.875rem;
            color: #4ade80;
        }
    </style>
</head>
<body>
    <div class="status">
        🟢 Server Running
    </div>
    
    <div class="container">
        <div class="header">
            <h1>GBox Server</h1>
            <p class="subtitle">Choose a service to continue</p>
        </div>
        
        <div class="cards">
            <a href="/live-view" class="card">
                <div class="card-icon">📱</div>
                <h2 class="card-title">Live View</h2>
                <p class="card-description">
                    Real-time Android device streaming and control via WebRTC
                </p>
            </a>
            
            <a href="/adb-expose" class="card">
                <div class="card-icon">🔌</div>
                <h2 class="card-title">ADB Expose</h2>
                <p class="card-description">
                    Manage ADB port forwarding for Android devices
                </p>
            </a>
        </div>
    </div>
</body>
</html>`
	
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, html)
}

func (s *GBoxServer) handleShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Server shutting down",
	})
	
	// Shutdown after response
	go func() {
		time.Sleep(100 * time.Millisecond)
		s.Stop()
		os.Exit(0)
	}()
}

func (s *GBoxServer) handleServerInfo(w http.ResponseWriter, r *http.Request) {
	info := map[string]interface{}{
		"version":    "1.0.0",
		"port":       s.port,
		"uptime":     time.Since(time.Now()).String(), // TODO: track actual start time
		"services": []string{
			"device-connect",
			"adb-expose",
		},
	}
	
	respondJSON(w, http.StatusOK, info)
}

// Helper function to send JSON responses
func respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}