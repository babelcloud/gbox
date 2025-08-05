package device_connect

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/babelcloud/gbox/packages/cli/config"
	"github.com/babelcloud/gbox/packages/cli/internal/profile"
)

// EnsureDeviceProxyRunning checks if the service is running, and starts it if not
func EnsureDeviceProxyRunning(isServiceRunning func() (bool, error)) error {
	running, err := isServiceRunning()
	if err != nil {
		return StartDeviceProxyService()
	}
	if running {
		return nil
	}
	return StartDeviceProxyService()
}

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
	env := os.Environ()
	env = append(env, fmt.Sprintf("GBOX_API_KEY=%s", apiKey))

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

func FindDeviceProxyBinary() (string, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %v", err)
	}
	executablePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %v", err)
	}
	executableDir := filepath.Dir(executablePath)
	osName := runtime.GOOS
	arch := runtime.GOARCH

	// Map OS names to match the actual binary naming convention
	binaryOSName := osName
	if osName == "darwin" {
		binaryOSName = "macos"
	}

	specificBinary := fmt.Sprintf("gbox-device-proxy-%s-%s", binaryOSName, arch)
	if osName == "windows" {
		specificBinary += ".exe"
	}
	genericBinary := "gbox-device-proxy"
	if osName == "windows" {
		genericBinary += ".exe"
	}

	// Search in current directory hierarchy
	searchPaths := []string{}
	current := currentDir
	for {
		searchPaths = append(searchPaths, filepath.Join(current, specificBinary))
		searchPaths = append(searchPaths, filepath.Join(current, genericBinary))

		parent := filepath.Dir(current)
		if parent == current {
			break // Reached root directory
		}
		current = parent
	}

	// Also search in executable directory hierarchy
	execCurrent := executableDir
	for {
		searchPaths = append(searchPaths, filepath.Join(execCurrent, specificBinary))
		searchPaths = append(searchPaths, filepath.Join(execCurrent, genericBinary))

		parent := filepath.Dir(execCurrent)
		if parent == execCurrent {
			break // Reached root directory
		}
		execCurrent = parent
	}

	// First, try to find binary in babel-umbrella directory
	babelUmbrellaPath := FindBabelUmbrellaDir(currentDir)
	if babelUmbrellaPath != "" {
		binariesDir := filepath.Join(babelUmbrellaPath, "gbox-device-proxy", "build", "binaries")
		babelBinaryPath := filepath.Join(binariesDir, specificBinary)
		if _, err := os.Stat(babelBinaryPath); err == nil {
			return babelBinaryPath, nil
		}
		if _, err := os.Stat(filepath.Join(binariesDir, genericBinary)); err == nil {
			return filepath.Join(binariesDir, genericBinary), nil
		}
	}

	for _, path := range searchPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("gbox-device-proxy binary not found. Please build it first using 'npm run build' in the gbox-device-proxy directory")
}

func FindBabelUmbrellaDir(startDir string) string {
	current := startDir

	// First, try to find babel-umbrella in the current path hierarchy
	for {
		if filepath.Base(current) == "babel-umbrella" {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	// If not found in hierarchy, try the known relative path
	knownPath := filepath.Join(startDir, "..", "..", "..", "babel-umbrella")
	if _, err := os.Stat(knownPath); err == nil {
		return knownPath
	}

	return ""
}
