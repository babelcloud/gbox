//go:build !windows

package device_connect

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/babelcloud/gbox/packages/cli/config"
)

// serverInstance holds the global server instance
var serverInstance *Server

// StartNativeDeviceProxyService starts the integrated scrcpy server
func StartNativeDeviceProxyService() error {
	// Create device proxy home directory
	deviceProxyHome := config.GetDeviceProxyHome()
	if err := os.MkdirAll(deviceProxyHome, 0755); err != nil {
		return fmt.Errorf("failed to create device proxy home directory: %v", err)
	}

	// Check if already running
	pidFile := filepath.Join(deviceProxyHome, "device-proxy.pid")
	if pidBytes, err := os.ReadFile(pidFile); err == nil {
		var pid int
		if _, err := fmt.Sscanf(string(pidBytes), "%d", &pid); err == nil {
			// Check if process is still running
			if err := syscall.Kill(pid, 0); err == nil {
				return fmt.Errorf("device proxy service already running with PID %d", pid)
			}
		}
		// Remove stale PID file
		os.Remove(pidFile)
	}

	// Fork the process to run in background
	if os.Getenv("GBOX_DEVICE_PROXY_DAEMON") != "1" {
		// Create log file
		logFile := filepath.Join(deviceProxyHome, "device-proxy.log")
		logFd, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to create log file: %v", err)
		}
		defer logFd.Close()

		// Start the server as a subprocess using exec.Command
		cmd := exec.Command(os.Args[0], "device-connect", "start-server")
		cmd.Env = append(os.Environ(), "GBOX_DEVICE_PROXY_DAEMON=1")
		cmd.Stdout = logFd
		cmd.Stderr = logFd
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setsid: true,
		}
		
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to start daemon: %v", err)
		}

		pid := cmd.Process.Pid

		// Write PID file
		if err := os.WriteFile(pidFile, []byte(strconv.Itoa(pid)), 0644); err != nil {
			return fmt.Errorf("failed to write PID file: %v", err)
		}

		log.Printf("Started device proxy service with PID %d", pid)
		
		// Wait a bit to ensure server starts
		time.Sleep(500 * time.Millisecond)
		
		// Verify server is responding
		client := NewClient(DefaultURL)
		for i := 0; i < 10; i++ {
			if _, _, err := client.IsServiceRunning(); err == nil {
				return nil
			}
			time.Sleep(500 * time.Millisecond)
		}
		
		// If we get here, server didn't start properly
		log.Printf("Warning: Server started but not responding on port %d", DefaultPort)
		return nil
	}

	// This is the daemon process
	// Start the integrated device connect server
	server := NewServer(DefaultPort)
	serverInstance = server
	
	if err := server.Start(); err != nil {
		return fmt.Errorf("failed to start device connect server: %v", err)
	}

	// Write our PID
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0644); err != nil {
		log.Printf("Warning: failed to write PID file: %v", err)
	}

	// Keep the process running
	select {}
}

// StopNativeDeviceProxyService stops the integrated scrcpy server
func StopNativeDeviceProxyService() error {
	deviceProxyHome := config.GetDeviceProxyHome()
	pidFile := filepath.Join(deviceProxyHome, "device-proxy.pid")

	// Read PID from file
	pidBytes, err := os.ReadFile(pidFile)
	if err != nil {
		return fmt.Errorf("device proxy service not running")
	}

	var pid int
	if _, err := fmt.Sscanf(string(pidBytes), "%d", &pid); err != nil {
		return fmt.Errorf("invalid PID file")
	}

	// Send SIGTERM to the process
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		// Process might already be dead
		os.Remove(pidFile)
		return fmt.Errorf("failed to stop process: %v", err)
	}

	// Remove PID file
	os.Remove(pidFile)

	log.Printf("Stopped device proxy service (PID %d)", pid)
	return nil
}

// IsNativeServiceRunning checks if the native service is running
func IsNativeServiceRunning() (bool, error) {
	// First check if PID file exists
	deviceProxyHome := config.GetDeviceProxyHome()
	pidFile := filepath.Join(deviceProxyHome, "device-proxy.pid")

	if _, err := os.Stat(pidFile); os.IsNotExist(err) {
		return false, nil
	}

	// Read PID from file
	pidBytes, err := os.ReadFile(pidFile)
	if err != nil {
		return false, nil
	}

	var pid int
	if _, err := fmt.Sscanf(string(pidBytes), "%d", &pid); err != nil {
		return false, nil
	}

	// Check if process is still running
	if err := syscall.Kill(pid, 0); err != nil {
		// Process is not running, remove PID file
		os.Remove(pidFile)
		return false, nil
	}

	// Try to check service status via API
	client := NewClient(DefaultURL)
	running, onDemandEnabled, err := client.IsServiceRunning()
	if err != nil {
		// If API check fails but process exists, assume it's starting up
		return true, nil
	}

	// Check if onDemandEnabled is false and warn user
	if running && !onDemandEnabled {
		fmt.Println("Warning: Device proxy service is running with automatic registration enabled.")
	}

	return running, nil
}