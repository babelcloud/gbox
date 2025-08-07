package device_connect

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

	specificBinary := fmt.Sprintf("gbox-device-proxy-%s-%s", osName, arch)
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

	// Check PATH for Homebrew installation
	if path, err := exec.LookPath("gbox-device-proxy"); err == nil {
		return path, nil
	}

	for _, path := range searchPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("gbox-device-proxy binary not found")
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
