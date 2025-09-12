package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	adb_expose "github.com/babelcloud/gbox/packages/cli/internal/adb_expose"
	"github.com/babelcloud/gbox/packages/cli/internal/profile"
)

// ADBExposeService manages ADB port expose for remote boxes
type ADBExposeService struct {
	mu      sync.RWMutex
	running bool
}

// BoxPortForward represents an active port forward for a remote box
type BoxPortForward struct {
	BoxID       string    `json:"box_id"`
	LocalPorts  []int     `json:"local_ports"`
	RemotePorts []int     `json:"remote_ports"`
	Status      string    `json:"status"` // "running", "stopped", "error"
	StartedAt   time.Time `json:"started_at"`
	Error       string    `json:"error,omitempty"`
}

// NewADBExposeService creates a new ADB expose service
func NewADBExposeService() *ADBExposeService {
	return &ADBExposeService{}
}

// Start starts the ADB expose service
func (s *ADBExposeService) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.running = true
	log.Println("ADB Expose service started")
	return nil
}

// Stop stops the ADB expose service
func (s *ADBExposeService) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.running = false
	log.Println("ADB Expose service stopped")
	return nil
}

// Close closes the service
func (s *ADBExposeService) Close() error {
	return s.Stop()
}

// IsRunning returns whether the service is running
func (s *ADBExposeService) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// StartBoxPortForward starts ADB port expose for a remote box
func (s *ADBExposeService) StartBoxPortForward(boxID string, localPorts, remotePorts []int) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.running {
		return fmt.Errorf("service not running")
	}

	// ADB expose is now handled by the main server's HTTP handlers
	// This method is kept for compatibility but the actual work is done
	// by the HTTP handlers that call the new ADB expose implementation

	log.Printf("ADB port expose request for box %s: local ports %v -> remote ports %v", boxID, localPorts, remotePorts)
	return nil
}

// StopBoxPortForward stops ADB port expose for a remote box
func (s *ADBExposeService) StopBoxPortForward(boxID string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.running {
		return fmt.Errorf("service not running")
	}

	log.Printf("ADB port expose stop request for box %s", boxID)
	return nil
}

// ListBoxPortForwards returns all active box port forwards
func (s *ADBExposeService) ListBoxPortForwards() ([]*BoxPortForward, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.running {
		return nil, fmt.Errorf("service not running")
	}

	// ADB expose is now handled by the main server's HTTP handlers
	// Return empty list for compatibility
	return make([]*BoxPortForward, 0), nil
}

// HTTP Handlers for ADB Expose

func (s *GBoxServer) handleADBExposeStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		BoxID       string `json:"box_id"`
		LocalPorts  []int  `json:"local_ports"`
		RemotePorts []int  `json:"remote_ports"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
		return
	}

	// Validate request
	if req.BoxID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "box_id is required",
		})
		return
	}

	if len(req.LocalPorts) == 0 || len(req.RemotePorts) == 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "local_ports and remote_ports are required",
		})
		return
	}

	if len(req.LocalPorts) != len(req.RemotePorts) {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "local_ports and remote_ports must have the same length",
		})
		return
	}

	// Create a request for port forwarding
	adbReq := StartRequest{
		BoxID:       req.BoxID,
		LocalPorts:  req.LocalPorts,
		RemotePorts: req.RemotePorts,
	}

	// Call the ADB expose server's start method
	// We need to get the configuration first
	pm := profile.NewProfileManager()
	if err := pm.Load(); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Failed to load profile manager: " + err.Error(),
		})
		return
	}

	// Get API key
	apiKey, err := pm.GetCurrentAPIKey()
	if err != nil {
		// Try to use the first available profile
		profiles := pm.GetProfiles()
		if len(profiles) == 0 {
			respondJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "No profiles available. Please run 'gbox profile add' to add a profile first",
			})
			return
		}

		var firstProfileID string
		for id := range profiles {
			firstProfileID = id
			break
		}

		if err := pm.Use(firstProfileID); err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "Failed to set profile: " + err.Error(),
			})
			return
		}

		apiKey, err = pm.GetCurrentAPIKey()
		if err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "Failed to get API key: " + err.Error(),
			})
			return
		}
	}

	gboxURL := profile.Default.GetEffectiveBaseURL()
	if gboxURL == "" {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "GBOX base URL not configured",
		})
		return
	}

	// Set up the configuration
	adbReq.Config = adb_expose.Config{
		APIKey:      apiKey,
		BoxID:       req.BoxID,
		GboxURL:     gboxURL,
		LocalAddr:   "127.0.0.1",
		TargetPorts: req.RemotePorts,
	}

	// Start port forwarding directly
	log.Printf("Starting ADB port forward for box %s", req.BoxID)
	forward, err := s.startPortForward(adbReq)
	if err != nil {
		log.Printf("Failed to start ADB port forward: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Failed to start ADB port expose: " + err.Error(),
		})
		return
	}
	log.Printf("ADB port forward started successfully for box %s", req.BoxID)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("ADB port exposed for box %s: %v -> %v", req.BoxID, req.LocalPorts, req.RemotePorts),
		"data":    forward,
	})
}

func (s *GBoxServer) handleADBExposeStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		BoxID string `json:"box_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// If no body, stop all forwards
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "box_id is required",
		})
		return
	}

	// Validate request
	if req.BoxID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "box_id is required",
		})
		return
	}

	// Stop port forwarding for the specific box
	log.Printf("Stopping ADB port forward for box %s", req.BoxID)
	if err := s.stopPortForward(req.BoxID); err != nil {
		log.Printf("Failed to stop ADB port forward: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Failed to stop ADB port expose: " + err.Error(),
		})
		return
	}
	log.Printf("ADB port forward stopped for box %s", req.BoxID)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("ADB port expose stopped for box %s", req.BoxID),
	})
}

func (s *GBoxServer) handleADBExposeStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"running": s.adbExpose.IsRunning(),
	}

	// Get box port forwards if service is running
	if s.adbExpose.IsRunning() {
		forwards, err := s.adbExpose.ListBoxPortForwards()
		if err != nil {
			status["error"] = err.Error()
		} else {
			status["forwards"] = forwards
		}
	} else {
		status["forwards"] = []*BoxPortForward{}
	}

	respondJSON(w, http.StatusOK, status)
}

func (s *GBoxServer) handleADBExposeList(w http.ResponseWriter, r *http.Request) {
	// Get port forwards directly from port manager
	forwards := s.listPortForwards()

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"forwards": forwards,
		"count":    len(forwards),
	})
}

// startPortForward starts port forwarding for a box
func (s *GBoxServer) startPortForward(req StartRequest) (*PortForward, error) {
	// Get or create WebSocket connection
	client, err := s.getOrCreateConnection(req.BoxID, req.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %v", err)
	}

	// Create port forward instance
	forward := &PortForward{
		BoxID:       req.BoxID,
		LocalPorts:  req.LocalPorts,
		RemotePorts: req.RemotePorts,
		StartedAt:   time.Now(),
		Status:      "starting",
		client:      client,
	}

	// Store the port forward in the manager
	s.portManager.mu.Lock()
	s.portManager.forwards[req.BoxID] = forward
	s.portManager.mu.Unlock()

	// Start local listeners for each port
	for i, localPort := range req.LocalPorts {
		remotePort := req.RemotePorts[i]
		go s.startLocalListener(forward, localPort, remotePort)
	}

	forward.Status = "running"
	return forward, nil
}

// stopPortForward stops port forwarding for a box
func (s *GBoxServer) stopPortForward(boxID string) error {
	s.portManager.mu.Lock()
	defer s.portManager.mu.Unlock()

	forward, exists := s.portManager.forwards[boxID]
	if !exists {
		return fmt.Errorf("port forward not found for box %s", boxID)
	}

	// Stop the port forward
	forward.Stop()

	// Close the client connection if it exists
	if forward.client != nil {
		forward.client.Close()
	}

	// Remove from manager
	delete(s.portManager.forwards, boxID)

	return nil
}

// listPortForwards returns all active port forwards
func (s *GBoxServer) listPortForwards() []*BoxPortForward {
	s.portManager.mu.RLock()
	defer s.portManager.mu.RUnlock()

	boxForwards := make([]*BoxPortForward, 0, len(s.portManager.forwards))
	for _, forward := range s.portManager.forwards {
		boxForward := &BoxPortForward{
			BoxID:       forward.BoxID,
			LocalPorts:  forward.LocalPorts,
			RemotePorts: forward.RemotePorts,
			Status:      forward.Status,
			StartedAt:   forward.StartedAt,
			Error:       forward.Error,
		}
		boxForwards = append(boxForwards, boxForward)
	}

	return boxForwards
}

// getOrCreateConnection gets or creates a WebSocket connection for a box
func (s *GBoxServer) getOrCreateConnection(boxID string, config adb_expose.Config) (*adb_expose.MultiplexClient, error) {
	s.connectionPool.mu.Lock()
	defer s.connectionPool.mu.Unlock()

	// Check if connection already exists
	if client, exists := s.connectionPool.connections[boxID]; exists {
		return client, nil
	}

	// Create new connection
	client, err := adb_expose.ConnectWebSocket(config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect WebSocket: %v", err)
	}

	// Start client handler
	go func() {
		if err := client.Run(); err != nil {
			log.Printf("WebSocket connection closed for box %s: %v", boxID, err)
		}
		// Remove from connection pool on error
		s.connectionPool.mu.Lock()
		delete(s.connectionPool.connections, boxID)
		s.connectionPool.mu.Unlock()
	}()

	s.connectionPool.connections[boxID] = client
	return client, nil
}

// startLocalListener starts a local listener for port forwarding
func (s *GBoxServer) startLocalListener(forward *PortForward, localPort, remotePort int) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", localPort))
	if err != nil {
		forward.mu.Lock()
		forward.Status = "error"
		forward.Error = fmt.Sprintf("Failed to listen on port %d: %v", localPort, err)
		forward.mu.Unlock()
		return
	}
	defer listener.Close()

	log.Printf("Listening on port %d for box %s", localPort, forward.BoxID)

	for {
		conn, err := listener.Accept()
		if err != nil {
			// Only log if it's not a normal shutdown
			if forward.Status != "stopped" {
				log.Printf("Failed to accept connection on port %d: %v", localPort, err)
			}
			continue
		}

		// Handle connection in goroutine
		go adb_expose.HandleLocalConnWithClient(conn, forward.client, remotePort)
	}
}
