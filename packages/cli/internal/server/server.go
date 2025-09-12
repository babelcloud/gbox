package server

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	adb_expose "github.com/babelcloud/gbox/packages/cli/internal/adb_expose"
	client "github.com/babelcloud/gbox/packages/cli/internal/client"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/webrtc"
)

//go:embed all:static
var staticFiles embed.FS

// PortManager manages all port forwarding instances
type PortManager struct {
	forwards map[string]*PortForward // key: boxID
	mu       sync.RWMutex
}

// PortForward represents a port forwarding instance
type PortForward struct {
	BoxID       string    `json:"boxid"`
	LocalPorts  []int     `json:"localports"`
	RemotePorts []int     `json:"remoteports"`
	StartedAt   time.Time `json:"started_at"`
	Status      string    `json:"status"` // "running", "stopped", "error"
	Error       string    `json:"error,omitempty"`
	client      *adb_expose.MultiplexClient
	mu          sync.RWMutex
}

// ConnectionPool manages WebSocket connections to remote servers
type ConnectionPool struct {
	connections map[string]*adb_expose.MultiplexClient // key: boxID
	mu          sync.RWMutex
}

// StartRequest represents a request to start port forwarding
type StartRequest struct {
	BoxID       string            `json:"boxid"`
	LocalPorts  []int             `json:"localports"`
	RemotePorts []int             `json:"remoteports"`
	Config      adb_expose.Config `json:"config"`
}

// Stop stops the port forward
func (pf *PortForward) Stop() {
	pf.mu.Lock()
	defer pf.mu.Unlock()

	if pf.Status == "stopped" {
		return
	}

	pf.Status = "stopped"
	if pf.client != nil {
		pf.client.Close()
	}
}

// GBoxServer is the unified server for all gbox services
type GBoxServer struct {
	port       int
	httpServer *http.Server
	mux        *http.ServeMux

	// Services
	webrtcManager *webrtc.Manager
	adbExpose     *ADBExposeService

	// ADB Expose functionality integrated directly
	portManager    *PortManager
	connectionPool *ConnectionPool

	// State
	mu        sync.RWMutex
	running   bool
	startTime time.Time
	buildID   string // Store build ID at startup
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewGBoxServer creates a new unified gbox server
func NewGBoxServer(port int) *GBoxServer {
	ctx, cancel := context.WithCancel(context.Background())

	return &GBoxServer{
		port:           port,
		mux:            http.NewServeMux(),
		webrtcManager:  webrtc.NewManager("adb"),
		adbExpose:      NewADBExposeService(),
		portManager:    &PortManager{forwards: make(map[string]*PortForward)},
		connectionPool: &ConnectionPool{connections: make(map[string]*adb_expose.MultiplexClient)},
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Start starts the unified server
func (s *GBoxServer) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("server already running")
	}

	// Set start time and build ID
	s.startTime = time.Now()
	s.buildID = GetBuildID()

	// ADB expose functionality is now integrated directly into this server

	// Setup routes
	s.setupRoutes()

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: s.mux,
	}

	// Start server in background
	go func() {
		// Starting GBox server
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	s.running = true
	// Server started successfully (no log needed here)
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

	// Cleanup ADB expose functionality
	s.portManager.mu.Lock()
	for _, forward := range s.portManager.forwards {
		forward.Stop()
	}
	s.portManager.forwards = make(map[string]*PortForward)
	s.portManager.mu.Unlock()

	s.connectionPool.mu.Lock()
	for _, client := range s.connectionPool.connections {
		client.Close()
	}
	s.connectionPool.connections = make(map[string]*adb_expose.MultiplexClient)
	s.connectionPool.mu.Unlock()

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
	// Health check and status
	s.mux.HandleFunc("/health", s.handleStatus)
	s.mux.HandleFunc("/api/status", s.handleStatus)

	// Device Connect API (scrcpy/WebRTC)
	s.mux.HandleFunc("/api/devices", s.handleDevices)
	s.mux.HandleFunc("/api/devices/", s.handleDeviceAction) // Handles /api/devices/{id}/connect and /api/devices/{id}/disconnect
	s.mux.HandleFunc("/api/devices/register", s.handleRegisterDevice)
	s.mux.HandleFunc("/api/devices/unregister", s.handleUnregisterDevice)
	s.mux.HandleFunc("/ws", s.handleWebSocket)

	// Box API
	s.mux.HandleFunc("/api/boxes", s.handleBoxList)

	// ADB Expose API
	s.mux.HandleFunc("/api/adb-expose/start", s.handleADBExposeStart)
	s.mux.HandleFunc("/api/adb-expose/stop", s.handleADBExposeStop)
	s.mux.HandleFunc("/api/adb-expose/status", s.handleADBExposeStatus)
	s.mux.HandleFunc("/api/adb-expose/list", s.handleADBExposeList)

	// Server management API
	s.mux.HandleFunc("/api/server/shutdown", s.handleShutdown)
	s.mux.HandleFunc("/api/server/info", s.handleServerInfo)

	// Live-view assets - serve from live-view static directory
	s.mux.HandleFunc("/assets/", s.handleLiveViewAssets)

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
	// Try embedded server static files
	staticFS, err := fs.Sub(staticFiles, "static")
	if err == nil {
		s.mux.Handle("/", http.FileServer(http.FS(staticFS)))
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
	// Note: embedded files removed - using external files only

	// Fallback to external files for development
	possiblePaths := []string{
		// Relative to gbox binary location
		"../../live-view/static",
		"../live-view/static",
		"packages/live-view/static",
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
				return absPath
			}
		}
	}

	log.Printf("Warning: Live-view static files not found, using default status page")
	return ""
}

// findStaticPath finds the server static files directory
func (s *GBoxServer) findStaticPath() string {
	// Try various possible locations for the server static files
	possiblePaths := []string{
		// Relative to gbox binary location
		"../../cli/internal/server/static",
		"../cli/internal/server/static",
		"packages/cli/internal/server/static",
		// Development paths
		"./packages/cli/internal/server/static",
		"../../../gbox/packages/cli/internal/server/static",
		// Current directory
		"./static",
		"static",
	}

	for _, path := range possiblePaths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		if info, err := os.Stat(absPath); err == nil && info.IsDir() {
			return absPath
		}
	}

	log.Printf("Warning: Server static files not found")
	return ""
}

// getScrcpyServerPath returns the path to scrcpy-server.jar
func (s *GBoxServer) getScrcpyServerPath() string {
	// Check external file
	possiblePaths := []string{
		"assets/scrcpy-server.jar",
		"../assets/scrcpy-server.jar",
		"../../assets/scrcpy-server.jar",
		"packages/cli/assets/scrcpy-server.jar",
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// API Handlers

func (s *GBoxServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	uptime := time.Since(s.startTime)
	s.mu.RUnlock()

	status := map[string]interface{}{
		"running": s.IsRunning(),
		"port":    s.port,
		"uptime":  uptime.String(),
		"services": map[string]interface{}{
			"device_connect": true,
			"adb_expose":     s.adbExpose.IsRunning(),
		},
		"version":  BuildInfo.Version,
		"build_id": GetBuildID(),
	}

	respondJSON(w, http.StatusOK, status)
}

func (s *GBoxServer) handleRoot(w http.ResponseWriter, r *http.Request) {
	// Only serve root page for exact path match
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Serve the index.html file from static directory
	s.serveStaticFileSimple(w, r, "index.html")
}

// serveStaticFileSimple serves a file from the embedded static files
func (s *GBoxServer) serveStaticFileSimple(w http.ResponseWriter, r *http.Request, filename string) {
	// Read the file from embedded filesystem
	file, err := staticFiles.Open("static/" + filename)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()

	// Set appropriate content type
	if strings.HasSuffix(filename, ".html") {
		w.Header().Set("Content-Type", "text/html")
	} else if strings.HasSuffix(filename, ".css") {
		w.Header().Set("Content-Type", "text/css")
	} else if strings.HasSuffix(filename, ".js") {
		w.Header().Set("Content-Type", "application/javascript")
	} else if strings.HasSuffix(filename, ".svg") {
		w.Header().Set("Content-Type", "image/svg+xml")
	}

	// Copy file content to response
	io.Copy(w, file)
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
	s.mu.RLock()
	uptime := time.Since(s.startTime)
	s.mu.RUnlock()

	info := map[string]interface{}{
		"version":  BuildInfo.Version,
		"build_id": s.buildID, // Use stored build ID from startup
		"port":     s.port,
		"uptime":   uptime.String(),
		"services": []string{
			"device-connect",
			"adb-expose",
		},
	}

	// Set CORS headers for debugging
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	respondJSON(w, http.StatusOK, info)
}

func (s *GBoxServer) handleBoxList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters
	query := r.URL.Query()
	typeFilter := query.Get("type") // e.g., ?type=android

	// Create GBOX client from profile
	sdkClient, err := client.NewClientFromProfile()
	if err != nil {
		log.Printf("Failed to create GBOX client: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to initialize GBOX client",
		})
		return
	}

	// Call GBOX API to get real box list
	boxesData, err := client.ListBoxesRawData(sdkClient, []string{})
	if err != nil {
		log.Printf("Failed to list boxes from GBOX API: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to fetch boxes from GBOX API",
		})
		return
	}

	// Convert to the expected format and add name field
	var allBoxes []map[string]interface{}
	for _, box := range boxesData {
		// Add name field if not present (use ID as fallback)
		if _, ok := box["name"]; !ok {
			if id, ok := box["id"].(string); ok {
				box["name"] = id
			}
		}
		allBoxes = append(allBoxes, box)
	}

	// Filter boxes by type if specified
	var filteredBoxes []map[string]interface{}
	if typeFilter != "" {
		for _, box := range allBoxes {
			if boxType, ok := box["type"].(string); ok && boxType == typeFilter {
				filteredBoxes = append(filteredBoxes, box)
			}
		}
	} else {
		filteredBoxes = allBoxes
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"boxes": filteredBoxes,
		"filter": map[string]interface{}{
			"type": typeFilter,
		},
	})
}

// Helper function to send JSON responses
func respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}
