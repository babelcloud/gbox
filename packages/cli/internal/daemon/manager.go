package daemon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

const (
	DefaultPort = 29888  // New port for unified gbox server
	ServerURL   = "http://localhost:29888"
)

// Manager handles the gbox server daemon lifecycle
type Manager struct {
	port int
	url  string
}

// NewManager creates a new daemon manager
func NewManager() *Manager {
	return &Manager{
		port: DefaultPort,
		url:  ServerURL,
	}
}

// EnsureServerRunning ensures the gbox server is running
// Similar to 'adb start-server' - starts server if not running
func (m *Manager) EnsureServerRunning() error {
	// Check if server is already running
	if m.IsServerRunning() {
		return nil
	}

	// Start server in background
	return m.StartServer()
}

// IsServerRunning checks if the server is running
func (m *Manager) IsServerRunning() bool {
	// First check PID file
	pidFile := m.getPIDFile()
	if pidBytes, err := os.ReadFile(pidFile); err == nil {
		var pid int
		if _, err := fmt.Sscanf(string(pidBytes), "%d", &pid); err == nil {
			// Check if process is still alive
			if isProcessAlive(pid) {
				// Process exists, now check if it's responding to HTTP
				if m.checkHTTPHealth() {
					return true
				}
			}
		}
		// PID file exists but process is dead or not responding
		os.Remove(pidFile)
	}

	// Double-check with HTTP even without PID file
	// (server might be running from another source)
	return m.checkHTTPHealth()
}

// checkHTTPHealth checks if server is responding to HTTP requests
func (m *Manager) checkHTTPHealth() bool {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("%s/health", m.url))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// StartServer starts the gbox server daemon
func (m *Manager) StartServer() error {
	// Clean up any old servers first
	m.CleanupOldServers()
	
	// Create daemon home directory
	daemonHome := filepath.Join(getHomeDir(), ".gbox", "cli")
	if err := os.MkdirAll(daemonHome, 0755); err != nil {
		return fmt.Errorf("failed to create daemon home: %v", err)
	}

	// Create log file
	logFile := filepath.Join(daemonHome, "server.log")
	logFd, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to create log file: %v", err)
	}
	defer logFd.Close()

	// Start server as subprocess
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}

	cmd := exec.Command(exePath, "server", "--internal-daemon")
	cmd.Stdout = logFd
	cmd.Stderr = logFd
	cmd.Env = append(os.Environ(), "GBOX_SERVER_DAEMON=1")
	setSysProcAttr(cmd)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start server daemon: %v", err)
	}

	pid := cmd.Process.Pid

	// Write PID file
	pidFile := m.getPIDFile()
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(pid)), 0644); err != nil {
		log.Printf("Warning: failed to write PID file: %v", err)
	}

	// Wait for server to be ready
	for i := 0; i < 20; i++ {
		time.Sleep(250 * time.Millisecond)
		if m.checkHTTPHealth() {
			log.Printf("GBox server started successfully (PID: %d)", pid)
			log.Printf("Web UI available at: http://localhost:%d", m.port)
			return nil
		}
	}

	// Server didn't start properly
	return fmt.Errorf("server started but not responding on port %d", m.port)
}

// StopServer stops the gbox server daemon
func (m *Manager) StopServer() error {
	// Try graceful shutdown via API first
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Post(fmt.Sprintf("%s/api/server/shutdown", m.url), "application/json", nil)
	if err == nil {
		resp.Body.Close()
		time.Sleep(500 * time.Millisecond)
		return nil
	}

	// Fall back to PID-based termination
	pidFile := m.getPIDFile()
	pidBytes, err := os.ReadFile(pidFile)
	if err != nil {
		return fmt.Errorf("server not running")
	}

	var pid int
	if _, err := fmt.Sscanf(string(pidBytes), "%d", &pid); err != nil {
		return fmt.Errorf("invalid PID file")
	}

	// Send SIGTERM
	if err := killProcess(pid, syscall.SIGTERM); err != nil {
		os.Remove(pidFile)
		return fmt.Errorf("failed to stop server: %v", err)
	}

	os.Remove(pidFile)
	log.Printf("GBox server stopped (PID: %d)", pid)
	return nil
}

// CleanupOldServers cleans up any old server processes
func (m *Manager) CleanupOldServers() {
	// Clean up old PID files and processes
	oldPidFiles := []string{
		filepath.Join(getHomeDir(), ".gbox", "device-proxy", "gbox-server.pid"),
		filepath.Join(getHomeDir(), ".gbox", "device-proxy", "device-proxy.pid"),
	}
	
	for _, pidFile := range oldPidFiles {
		if pidBytes, err := os.ReadFile(pidFile); err == nil {
			var pid int
			if _, err := fmt.Sscanf(string(pidBytes), "%d", &pid); err == nil {
				killProcess(pid, syscall.SIGTERM)
			}
			os.Remove(pidFile)
		}
	}
	
	// Kill any stray server processes
	exec.Command("pkill", "-f", "gbox.*server.*--internal-daemon").Run()
	exec.Command("pkill", "-f", "device-connect start-server").Run()
}

// getPIDFile returns the path to the PID file
func (m *Manager) getPIDFile() string {
	return filepath.Join(getHomeDir(), ".gbox", "cli", "server.pid")
}

// CallAPI makes an API call to the server
func (m *Manager) CallAPI(method, endpoint string, body interface{}, result interface{}) error {
	// Ensure server is running
	if err := m.EnsureServerRunning(); err != nil {
		return fmt.Errorf("failed to start server: %v", err)
	}

	url := fmt.Sprintf("%s%s", m.url, endpoint)
	
	var bodyReader io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %v", err)
		}
		bodyReader = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("API call failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("failed to decode response: %v", err)
		}
	}

	return nil
}

// Global instance for convenience
var DefaultManager = NewManager()