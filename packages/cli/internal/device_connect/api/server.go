package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/device"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/webrtc"
)

// Server handles HTTP API and WebSocket connections
type Server struct {
	port          int
	server        *http.Server
	deviceManager *device.Manager
	webrtcManager *webrtc.Manager
	isRunning     bool
}

// NewServer creates a new API server
func NewServer(port int) *Server {
	deviceManager := device.NewManager()
	
	// Get ADB path for WebRTC manager
	adbPath := "adb"
	webrtcManager := webrtc.NewManager(adbPath)
	
	return &Server{
		port:          port,
		deviceManager: deviceManager,
		webrtcManager: webrtcManager,
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	if s.isRunning {
		return fmt.Errorf("server already running")
	}

	// Setup routes
	mux := http.NewServeMux()
	
	// API routes
	mux.HandleFunc("/api/devices", s.handleDevices)
	mux.HandleFunc("/api/devices/", s.handleDeviceAction)
	mux.HandleFunc("/api/register-device", s.handleRegisterDevice)
	mux.HandleFunc("/api/unregister-device", s.handleUnregisterDevice)
	
	// WebSocket route
	mux.HandleFunc("/ws", s.handleWebSocket)
	
	// Static files
	staticPath := s.findLiveViewStaticPath()
	if staticPath != "" {
		log.Printf("Serving static files from: %s", staticPath)
		fs := http.FileServer(http.Dir(staticPath))
		mux.Handle("/", fs)
	} else {
		log.Println("Warning: Live-view static files not found")
		// Return 404 for static files if not found
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		})
	}

	// Create HTTP server
	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Start server
	log.Printf("Starting API server on port %d", s.port)
	s.isRunning = true
	
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
			s.isRunning = false
		}
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)
	
	// Test if server is accessible
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/devices", s.port))
	if err != nil {
		s.isRunning = false
		return fmt.Errorf("server failed to start: %w", err)
	}
	resp.Body.Close()

	log.Printf("API server started successfully on http://localhost:%d", s.port)
	return nil
}

// Stop stops the HTTP server
func (s *Server) Stop() error {
	if !s.isRunning {
		return nil
	}

	log.Println("Stopping API server...")
	
	// Close WebRTC manager
	if s.webrtcManager != nil {
		s.webrtcManager.Close()
	}
	
	// Shutdown HTTP server
	if s.server != nil {
		if err := s.server.Close(); err != nil {
			log.Printf("Error closing server: %v", err)
		}
	}

	s.isRunning = false
	log.Println("API server stopped")
	
	return nil
}

// IsRunning returns whether the server is running
func (s *Server) IsRunning() bool {
	return s.isRunning
}

// findLiveViewStaticPath finds the live-view static files
func (s *Server) findLiveViewStaticPath() string {
	searchPaths := []string{
		"../../live-view/dist/static",
		"../live-view/dist/static",
		"packages/live-view/dist/static",
		"live-view/dist/static",
		"dist/static",
		"static",
	}

	// Also check relative to executable
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		searchPaths = append([]string{
			filepath.Join(exeDir, "static"),
			filepath.Join(exeDir, "..", "live-view", "dist", "static"),
			filepath.Join(exeDir, "..", "..", "live-view", "dist", "static"),
		}, searchPaths...)
	}

	for _, path := range searchPaths {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			absPath, _ := filepath.Abs(path)
			return absPath
		}
	}

	return ""
}

// respondJSON sends a JSON response
func respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	// Use json encoder to write response
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Failed to encode JSON response: %v", err)
	}
}