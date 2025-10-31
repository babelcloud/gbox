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

	client "github.com/babelcloud/gbox/packages/cli/internal/client"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/control"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/transport/webrtc"
	"github.com/babelcloud/gbox/packages/cli/internal/server/handlers"
	"github.com/babelcloud/gbox/packages/cli/internal/server/router"
	"github.com/pkg/errors"
)

//go:embed all:static
var staticFiles embed.FS

// GBoxServer is the unified server for all gbox services
type GBoxServer struct {
	port       int
	httpServer *http.Server
	mux        *http.ServeMux

	// Services
	bridgeManager *webrtc.Manager
	deviceKeeper  *DeviceKeeper

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

	// Initialize control service
	control.SetControlService()

	return &GBoxServer{
		port:          port,
		mux:           http.NewServeMux(),
		bridgeManager: webrtc.NewManager("adb"),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start starts the unified server
func (s *GBoxServer) Start() error {
	// Set start time and build ID
	s.startTime = time.Now()
	s.buildID = GetBuildID()

	// Setup routes
	s.setupRoutes()

	if err := s.startDeviceKeeper(); err != nil {
		return err
	}

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      s.mux,
		ReadTimeout:  0, // No read timeout for streaming connections
		WriteTimeout: 0, // No write timeout for streaming connections
		IdleTimeout:  0, // No idle timeout for streaming connections
	}

	return s.httpServer.ListenAndServe()
}

// Stop stops the server
func (s *GBoxServer) Stop() error {
	s.cancel()

	// Shutdown HTTP server with longer timeout
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := s.httpServer.Shutdown(ctx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
			// Force close if graceful shutdown fails
			if err := s.httpServer.Close(); err != nil {
				log.Printf("HTTP server force close error: %v", err)
			}
		}
	}

	// Cleanup services
	s.bridgeManager.Close()
	s.deviceKeeper.Close()

	log.Println("GBox server stopped")
	return nil
}

func (s *GBoxServer) startDeviceKeeper() error {
	var err error
	s.deviceKeeper, err = NewDeviceKeeper()
	if err != nil {
		return errors.Wrap(err, "failed to create device keeper")
	}
	if err := s.deviceKeeper.Start(); err != nil {
		return errors.Wrap(err, "failed to start device keeper")
	}
	return nil
}

// IsRunning returns whether the server is running
func (s *GBoxServer) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// setupRoutes sets up all HTTP routes using the new router system
func (s *GBoxServer) setupRoutes() {
	// Register routers in order of specificity (most specific first)
	routers := []router.Router{
		&router.APIRouter{},
		&router.ADBExposeRouter{},
		&router.AssetsRouter{},
		&router.PagesRouter{}, // Must be last as it includes root handler
	}

	// Register all routes
	for _, r := range routers {
		r.RegisterRoutes(s.mux, s)
	}

	// Setup static files as fallback (must be last)
	s.setupStaticFiles()
}

// setupStaticFiles sets up static file serving
// Note: Root path is now handled by PagesRouter
func (s *GBoxServer) setupStaticFiles() {
	// Root path is handled by PagesRouter, no additional setup needed
	// This function is kept for potential future static file setup
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
			"adb_expose":     true, // Always available through handlers
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

// ServerService interface implementations for handlers

// GetPort returns the server port
func (s *GBoxServer) GetPort() int {
	return s.port
}

// GetUptime returns server uptime
func (s *GBoxServer) GetUptime() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return time.Since(s.startTime)
}

// GetBuildID returns build ID
func (s *GBoxServer) GetBuildID() string {
	return s.buildID
}

// GetVersion returns version info
func (s *GBoxServer) GetVersion() string {
	return BuildInfo.Version
}

// IsADBExposeRunning returns ADB expose status
func (s *GBoxServer) IsADBExposeRunning() bool {
	return true // Always available through handlers
}

// ListBridges returns list of bridge device serials
func (s *GBoxServer) ListBridges() []string {
	return s.bridgeManager.ListBridges()
}

// CreateBridge creates a bridge for device
func (s *GBoxServer) CreateBridge(deviceSerial string) error {
	_, err := s.bridgeManager.CreateBridge(deviceSerial)
	return err
}

// RemoveBridge removes a bridge
func (s *GBoxServer) RemoveBridge(deviceSerial string) {
	s.bridgeManager.RemoveBridge(deviceSerial)
}

// GetBridge gets a bridge by device serial
func (s *GBoxServer) GetBridge(deviceSerial string) (handlers.Bridge, bool) {
	bridge, exists := s.bridgeManager.GetBridge(deviceSerial)
	return bridge, exists
}

// GetStaticFS returns static file system
func (s *GBoxServer) GetStaticFS() fs.FS {
	return staticFiles
}

// FindLiveViewStaticPath returns live view static path (deprecated - now embedded)
func (s *GBoxServer) FindLiveViewStaticPath() string {
	return "" // Live-view files are now embedded, not external
}

// FindStaticPath returns static path
func (s *GBoxServer) FindStaticPath() string {
	return s.findStaticPath()
}

// StartPortForward starts port forwarding for ADB expose
// This method is kept for ServerService interface compatibility
// but ADB functionality is now handled by ADBExposeHandlers
func (s *GBoxServer) StartPortForward(boxID string, localPorts, remotePorts []int) error {
	return fmt.Errorf("ADB port forwarding is now handled through API endpoints")
}

// StopPortForward stops port forwarding for ADB expose
// This method is kept for ServerService interface compatibility
func (s *GBoxServer) StopPortForward(boxID string) error {
	return fmt.Errorf("ADB port forwarding is now handled through API endpoints")
}

// ListPortForwards lists all active port forwards
// This method is kept for ServerService interface compatibility
func (s *GBoxServer) ListPortForwards() interface{} {
	return map[string]interface{}{
		"forwards": []interface{}{},
		"count":    0,
		"message":  "ADB port forwarding is now handled through API endpoints",
	}
}

func (s *GBoxServer) ConnectAP(serial string) error {
	return s.deviceKeeper.connectAP(serial)
}

func (s *GBoxServer) ConnectAPLinux(deviceId string) error {
	return s.deviceKeeper.connectAPByDeviceId(deviceId)
}

func (s *GBoxServer) DisconnectAP(serial string) error {
	return s.deviceKeeper.disconnectAPForce(serial)
}

func (s *GBoxServer) GetAdbSerialByGboxDeviceId(deviceId string) string {
	return s.deviceKeeper.getAdbSerialByGboxDeviceId(deviceId)
}

// Helper function to send JSON responses
func respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}
