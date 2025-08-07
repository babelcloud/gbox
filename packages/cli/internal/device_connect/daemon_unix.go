//go:build !windows

package device_connect

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/babelcloud/gbox/packages/cli/config"
	"github.com/babelcloud/gbox/packages/cli/internal/profile"
)

func StartDeviceProxyService() error {
	binaryPath, err := FindDeviceProxyBinary()
	if err != nil {
		return fmt.Errorf("device proxy binary not found: %v", err)
	}

	// Create device proxy home directory
	deviceProxyHome := config.GetDeviceProxyHome()
	if err := os.MkdirAll(deviceProxyHome, 0755); err != nil {
		return fmt.Errorf("failed to create device proxy home directory: %v", err)
	}

	// Create log file
	logFile := filepath.Join(deviceProxyHome, "device-proxy.log")
	logFd, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to create log file: %v", err)
	}
	defer logFd.Close()

	// Create PID file path
	pidFile := filepath.Join(deviceProxyHome, "device-proxy.pid")

	// Get API key from current profile
	apiKey, err := profile.GetCurrentAPIKey()
	if err != nil {
		return fmt.Errorf("failed to get API key: %v", err)
	}

	// Set up environment variables
	env := setupDeviceProxyEnvironment(apiKey)

	cmd := exec.Command(binaryPath, "--port", "19925", "--on-demand")
	cmd.Stdout = logFd
	cmd.Stderr = logFd
	cmd.Env = env

	// Set process group to make child process independent
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start device proxy service: %v", err)
	}

	// Write PID to file
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0644); err != nil {
		// Try to kill the process if we can't write PID file
		cmd.Process.Kill()
		return fmt.Errorf("failed to write PID file: %v", err)
	}

	time.Sleep(2 * time.Second)
	return nil
}
