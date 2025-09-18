package handlers

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

// ADBExposeHandlers contains handlers for ADB expose functionality
type ADBExposeHandlers struct {
	portManager   *PortManager
	connectionPool *ConnectionPool
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

// PortForward manages a single port forwarding session
type PortForward struct {
	BoxID       string                         `json:"box_id"`
	LocalPorts  []int                         `json:"local_ports"`
	RemotePorts []int                         `json:"remote_ports"`
	StartedAt   time.Time                     `json:"started_at"`
	Status      string                        `json:"status"`
	Error       string                        `json:"error,omitempty"`
	client      *adb_expose.MultiplexClient
	listeners   []net.Listener
	mu          sync.RWMutex
}

// Stop stops the port forward
func (pf *PortForward) Stop() {
	pf.mu.Lock()
	defer pf.mu.Unlock()

	pf.Status = "stopped"
	for _, listener := range pf.listeners {
		listener.Close()
	}
}

// PortManager manages multiple port forwards
type PortManager struct {
	forwards map[string]*PortForward
	mu       sync.RWMutex
}

// ConnectionPool manages WebSocket connections
type ConnectionPool struct {
	connections map[string]*adb_expose.MultiplexClient
	mu          sync.RWMutex
}

// StartRequest represents a start port forward request
type StartRequest struct {
	BoxID       string            `json:"box_id"`
	LocalPorts  []int             `json:"local_ports"`
	RemotePorts []int             `json:"remote_ports"`
	Config      adb_expose.Config `json:"config"`
}

// NewADBExposeHandlers creates a new ADB expose handlers instance
func NewADBExposeHandlers() *ADBExposeHandlers {
	return &ADBExposeHandlers{
		portManager: &PortManager{
			forwards: make(map[string]*PortForward),
		},
		connectionPool: &ConnectionPool{
			connections: make(map[string]*adb_expose.MultiplexClient),
		},
	}
}

// HandleADBExposeStart handles ADB expose start requests
func (h *ADBExposeHandlers) HandleADBExposeStart(w http.ResponseWriter, r *http.Request) {
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
		RespondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
		return
	}

	// Validate request
	if req.BoxID == "" {
		RespondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "box_id is required",
		})
		return
	}

	if len(req.LocalPorts) == 0 || len(req.RemotePorts) == 0 {
		RespondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "local_ports and remote_ports are required",
		})
		return
	}

	if len(req.LocalPorts) != len(req.RemotePorts) {
		RespondJSON(w, http.StatusBadRequest, map[string]string{
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
		RespondJSON(w, http.StatusInternalServerError, map[string]string{
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
			RespondJSON(w, http.StatusInternalServerError, map[string]string{
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
			RespondJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "Failed to set profile: " + err.Error(),
			})
			return
		}

		apiKey, err = pm.GetCurrentAPIKey()
		if err != nil {
			RespondJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "Failed to get API key: " + err.Error(),
			})
			return
		}
	}

	gboxURL := profile.Default.GetEffectiveBaseURL()
	if gboxURL == "" {
		RespondJSON(w, http.StatusInternalServerError, map[string]string{
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
	forward, err := h.startPortForward(adbReq)
	if err != nil {
		log.Printf("Failed to start ADB port forward: %v", err)
		RespondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Failed to start ADB port expose: " + err.Error(),
		})
		return
	}
	log.Printf("ADB port forward started successfully for box %s", req.BoxID)

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("ADB port exposed for box %s: %v -> %v", req.BoxID, req.LocalPorts, req.RemotePorts),
		"data":    forward,
	})
}

// HandleADBExposeStop handles ADB expose stop requests
func (h *ADBExposeHandlers) HandleADBExposeStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		BoxID string `json:"box_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// If no body, stop all forwards
		RespondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "box_id is required",
		})
		return
	}

	// Validate request
	if req.BoxID == "" {
		RespondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "box_id is required",
		})
		return
	}

	// Stop port forwarding for the specific box
	log.Printf("Stopping ADB port forward for box %s", req.BoxID)
	if err := h.stopPortForward(req.BoxID); err != nil {
		log.Printf("Failed to stop ADB port forward: %v", err)
		RespondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Failed to stop ADB port expose: " + err.Error(),
		})
		return
	}
	log.Printf("ADB port forward stopped for box %s", req.BoxID)

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("ADB port expose stopped for box %s", req.BoxID),
	})
}

// HandleADBExposeStatus handles ADB expose status requests
func (h *ADBExposeHandlers) HandleADBExposeStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"running": true, // Always running as part of main server
	}

	// Get box port forwards
	forwards := h.listPortForwards()
	status["forwards"] = forwards

	RespondJSON(w, http.StatusOK, status)
}

// HandleADBExposeList handles ADB expose list requests
func (h *ADBExposeHandlers) HandleADBExposeList(w http.ResponseWriter, r *http.Request) {
	// Get port forwards directly from port manager
	forwards := h.listPortForwards()

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"forwards": forwards,
		"count":    len(forwards),
	})
}

// startPortForward starts port forwarding for a box
func (h *ADBExposeHandlers) startPortForward(req StartRequest) (*PortForward, error) {
	// Get or create WebSocket connection
	client, err := h.getOrCreateConnection(req.BoxID, req.Config)
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
	h.portManager.mu.Lock()
	h.portManager.forwards[req.BoxID] = forward
	h.portManager.mu.Unlock()

	// Start local listeners for each port
	for i, localPort := range req.LocalPorts {
		remotePort := req.RemotePorts[i]
		go h.startLocalListener(forward, localPort, remotePort)
	}

	forward.Status = "running"
	return forward, nil
}

// stopPortForward stops port forwarding for a box
func (h *ADBExposeHandlers) stopPortForward(boxID string) error {
	h.portManager.mu.Lock()
	defer h.portManager.mu.Unlock()

	forward, exists := h.portManager.forwards[boxID]
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
	delete(h.portManager.forwards, boxID)

	return nil
}

// listPortForwards returns all active port forwards
func (h *ADBExposeHandlers) listPortForwards() []*BoxPortForward {
	h.portManager.mu.RLock()
	defer h.portManager.mu.RUnlock()

	boxForwards := make([]*BoxPortForward, 0, len(h.portManager.forwards))
	for _, forward := range h.portManager.forwards {
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
func (h *ADBExposeHandlers) getOrCreateConnection(boxID string, config adb_expose.Config) (*adb_expose.MultiplexClient, error) {
	h.connectionPool.mu.Lock()
	defer h.connectionPool.mu.Unlock()

	// Check if connection already exists
	if client, exists := h.connectionPool.connections[boxID]; exists {
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
		h.connectionPool.mu.Lock()
		delete(h.connectionPool.connections, boxID)
		h.connectionPool.mu.Unlock()
	}()

	h.connectionPool.connections[boxID] = client
	return client, nil
}

// startLocalListener starts a local listener for port forwarding
func (h *ADBExposeHandlers) startLocalListener(forward *PortForward, localPort, remotePort int) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", localPort))
	if err != nil {
		forward.mu.Lock()
		forward.Status = "error"
		forward.Error = fmt.Sprintf("Failed to listen on port %d: %v", localPort, err)
		forward.mu.Unlock()
		return
	}
	defer listener.Close()

	// Store listener for cleanup
	forward.mu.Lock()
	forward.listeners = append(forward.listeners, listener)
	forward.mu.Unlock()

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