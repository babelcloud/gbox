package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// ADBExposeService manages ADB port forwarding
type ADBExposeService struct {
	mu        sync.RWMutex
	forwards  map[string]*PortForward // key: "device:localPort:remotePort"
	running   bool
}

// PortForward represents an active port forward
type PortForward struct {
	DeviceSerial string `json:"device_serial"`
	LocalPort    int    `json:"local_port"`
	RemotePort   int    `json:"remote_port"`
	Protocol     string `json:"protocol"` // tcp or unix
	Active       bool   `json:"active"`
}

// NewADBExposeService creates a new ADB expose service
func NewADBExposeService() *ADBExposeService {
	return &ADBExposeService{
		forwards: make(map[string]*PortForward),
	}
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
	
	// Clear all forwards
	for key, forward := range s.forwards {
		if err := s.removeForward(forward); err != nil {
			log.Printf("Failed to remove forward %s: %v", key, err)
		}
	}
	
	s.forwards = make(map[string]*PortForward)
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

// AddForward adds a new port forward
func (s *ADBExposeService) AddForward(deviceSerial string, localPort, remotePort int, protocol string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if !s.running {
		return fmt.Errorf("service not running")
	}
	
	key := fmt.Sprintf("%s:%d:%d", deviceSerial, localPort, remotePort)
	
	// Check if already exists
	if _, exists := s.forwards[key]; exists {
		return fmt.Errorf("forward already exists")
	}
	
	// Execute adb forward command
	var cmd *exec.Cmd
	if deviceSerial == "" || deviceSerial == "default" {
		cmd = exec.Command("adb", "forward", 
			fmt.Sprintf("tcp:%d", localPort),
			fmt.Sprintf("%s:%d", protocol, remotePort))
	} else {
		cmd = exec.Command("adb", "-s", deviceSerial, "forward",
			fmt.Sprintf("tcp:%d", localPort),
			fmt.Sprintf("%s:%d", protocol, remotePort))
	}
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add forward: %w", err)
	}
	
	forward := &PortForward{
		DeviceSerial: deviceSerial,
		LocalPort:    localPort,
		RemotePort:   remotePort,
		Protocol:     protocol,
		Active:       true,
	}
	
	s.forwards[key] = forward
	log.Printf("Added forward: %s", key)
	
	return nil
}

// RemoveForward removes a port forward
func (s *ADBExposeService) RemoveForward(deviceSerial string, localPort, remotePort int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	key := fmt.Sprintf("%s:%d:%d", deviceSerial, localPort, remotePort)
	
	forward, exists := s.forwards[key]
	if !exists {
		return fmt.Errorf("forward not found")
	}
	
	if err := s.removeForward(forward); err != nil {
		return err
	}
	
	delete(s.forwards, key)
	log.Printf("Removed forward: %s", key)
	
	return nil
}

// removeForward executes adb command to remove forward
func (s *ADBExposeService) removeForward(forward *PortForward) error {
	var cmd *exec.Cmd
	if forward.DeviceSerial == "" || forward.DeviceSerial == "default" {
		cmd = exec.Command("adb", "forward", "--remove",
			fmt.Sprintf("tcp:%d", forward.LocalPort))
	} else {
		cmd = exec.Command("adb", "-s", forward.DeviceSerial, "forward", "--remove",
			fmt.Sprintf("tcp:%d", forward.LocalPort))
	}
	
	return cmd.Run()
}

// ListForwards returns all active forwards
func (s *ADBExposeService) ListForwards() []*PortForward {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	forwards := make([]*PortForward, 0, len(s.forwards))
	for _, forward := range s.forwards {
		forwards = append(forwards, forward)
	}
	
	return forwards
}

// HTTP Handlers for ADB Expose

func (s *GBoxServer) handleADBExposeStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var req struct {
		DeviceSerial string `json:"device_serial"`
		LocalPort    int    `json:"local_port"`
		RemotePort   int    `json:"remote_port"`
		Protocol     string `json:"protocol"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
		return
	}
	
	// Default protocol to tcp
	if req.Protocol == "" {
		req.Protocol = "tcp"
	}
	
	// Start service if not running
	if !s.adbExpose.IsRunning() {
		if err := s.adbExpose.Start(); err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{
				"error": err.Error(),
			})
			return
		}
	}
	
	// Add forward
	if err := s.adbExpose.AddForward(req.DeviceSerial, req.LocalPort, req.RemotePort, req.Protocol); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}
	
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Port forward added: %d -> %d", req.LocalPort, req.RemotePort),
	})
}

func (s *GBoxServer) handleADBExposeStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var req struct {
		DeviceSerial string `json:"device_serial"`
		LocalPort    int    `json:"local_port"`
		RemotePort   int    `json:"remote_port"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// If no body, stop all forwards
		if err := s.adbExpose.Stop(); err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{
				"error": err.Error(),
			})
			return
		}
		
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"message": "All port forwards removed",
		})
		return
	}
	
	// Remove specific forward
	if err := s.adbExpose.RemoveForward(req.DeviceSerial, req.LocalPort, req.RemotePort); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}
	
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Port forward removed",
	})
}

func (s *GBoxServer) handleADBExposeStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"running":  s.adbExpose.IsRunning(),
		"forwards": s.adbExpose.ListForwards(),
	}
	
	respondJSON(w, http.StatusOK, status)
}

func (s *GBoxServer) handleADBExposeList(w http.ResponseWriter, r *http.Request) {
	// Get all adb forwards from system
	cmd := exec.Command("adb", "forward", "--list")
	output, err := cmd.Output()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("Failed to list forwards: %v", err),
		})
		return
	}
	
	// Parse output
	lines := strings.Split(string(output), "\n")
	forwards := []map[string]interface{}{}
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		// Format: serial tcp:local tcp:remote
		parts := strings.Fields(line)
		if len(parts) >= 3 {
			localParts := strings.Split(parts[1], ":")
			remoteParts := strings.Split(parts[2], ":")
			
			forward := map[string]interface{}{
				"device_serial": parts[0],
				"local":         parts[1],
				"remote":        parts[2],
			}
			
			// Try to parse ports
			if len(localParts) == 2 {
				if port, err := strconv.Atoi(localParts[1]); err == nil {
					forward["local_port"] = port
				}
			}
			if len(remoteParts) == 2 {
				if port, err := strconv.Atoi(remoteParts[1]); err == nil {
					forward["remote_port"] = port
				}
			}
			
			forwards = append(forwards, forward)
		}
	}
	
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"forwards": forwards,
		"managed":  s.adbExpose.ListForwards(),
	})
}