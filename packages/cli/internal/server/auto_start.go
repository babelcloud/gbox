package server

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/babelcloud/gbox/packages/cli/internal/proc_group"
)

// AutoStartManager manages automatic server startup
type AutoStartManager struct {
	serverPort int
	pidFile    string
	logFile    string
}

// NewAutoStartManager creates a new auto-start manager
func NewAutoStartManager(serverPort int) *AutoStartManager {
	homeDir, _ := os.UserHomeDir()
	gboxDir := filepath.Join(homeDir, ".gbox", "cli")

	return &AutoStartManager{
		serverPort: serverPort,
		pidFile:    filepath.Join(gboxDir, "gbox-server.pid"),
		logFile:    filepath.Join(gboxDir, "server.log"),
	}
}

// EnsureServerRunning ensures the GBOX server is running, starting it if necessary
func (m *AutoStartManager) EnsureServerRunning() error {
	// Check if server is already running
	if m.isServerRunning() {
		return nil
	}

	// Start server in background
	return m.startServerInBackground()
}

// isServerRunning checks if the server is already running
func (m *AutoStartManager) isServerRunning() bool {
	// Check if port is listening
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", m.serverPort), 1*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// startServerInBackground starts the server in background mode
func (m *AutoStartManager) startServerInBackground() error {
	// Get the current executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}

	// Create command to start server in background
	cmd := exec.Command(execPath, "server", "start", "--daemon")

	// Set up process attributes for daemon mode (platform-specific)
	procgroup.SetProcGrp(cmd)

	// Redirect output to log file
	logFile, err := os.OpenFile(m.logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %v", err)
	}
	defer logFile.Close()

	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// Start the process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start server: %v", err)
	}

	// Write PID to file
	pidFile, err := os.OpenFile(m.pidFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create PID file: %v", err)
	}
	defer pidFile.Close()

	if _, err := pidFile.WriteString(strconv.Itoa(cmd.Process.Pid)); err != nil {
		return fmt.Errorf("failed to write PID file: %v", err)
	}

	// Wait a bit for server to start
	time.Sleep(2 * time.Second)

	// Verify server is running
	if !m.isServerRunning() {
		return fmt.Errorf("server failed to start properly")
	}

	log.Printf("GBOX server started in background (PID: %d)", cmd.Process.Pid)
	return nil
}

// StopServer stops the background server
func (m *AutoStartManager) StopServer() error {
	// Read PID from file
	pidBytes, err := os.ReadFile(m.pidFile)
	if err != nil {
		return fmt.Errorf("failed to read PID file: %v", err)
	}

	pid, err := strconv.Atoi(string(pidBytes))
	if err != nil {
		return fmt.Errorf("invalid PID in file: %v", err)
	}

	// Send SIGTERM to the process
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process: %v", err)
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM: %v", err)
	}

	// Wait for process to exit
	time.Sleep(1 * time.Second)

	// Remove PID file
	if err := os.Remove(m.pidFile); err != nil {
		log.Printf("Warning: failed to remove PID file: %v", err)
	}

	log.Printf("GBOX server stopped (PID: %d)", pid)
	return nil
}

// IsServerRunning returns whether the server is running
func (m *AutoStartManager) IsServerRunning() bool {
	return m.isServerRunning()
}
