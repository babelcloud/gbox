package device_connect

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/babelcloud/gbox/packages/cli/config"
	"github.com/babelcloud/gbox/packages/cli/internal/profile"
)

// isExecutableFile checks if the given path is an executable file (not a directory)
func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	// Check if it's a directory
	if info.IsDir() {
		return false
	}

	// Check if it has execute permissions
	mode := info.Mode()
	return mode&0111 != 0 // Check if any execute bit is set
}

// calculateSHA256 calculates the SHA256 hash of a file
func calculateSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to read file: %v", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

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

	// Map runtime.GOOS to directory name format
	dirOsName := osName
	if osName == "darwin" {
		dirOsName = "macos"
	}

	binaryName := "gbox-device-proxy"
	if osName == "windows" {
		binaryName += ".exe"
	}

	debug := os.Getenv("DEBUG") == "true"

	// Priority 1: Check current directory first
	currentBinaryPath := filepath.Join(currentDir, binaryName)
	if isExecutableFile(currentBinaryPath) {
		if debug {
			fmt.Fprintf(os.Stderr, "[DEBUG] Found gbox-device-proxy binary in current directory: %s\n", currentBinaryPath)
		}
		return currentBinaryPath, nil
	}

	// Priority 2: Check babel-umbrella directory
	babelUmbrellaPath := FindBabelUmbrellaDir(currentDir)
	if babelUmbrellaPath != "" {
		binariesDir := filepath.Join(babelUmbrellaPath, "gbox-device-proxy", "build", fmt.Sprintf("binaries-%s-%s", dirOsName, arch))
		babelBinaryPath := filepath.Join(binariesDir, binaryName)
		if debug {
			fmt.Fprintf(os.Stderr, "[DEBUG] Checking babel-umbrella path: %s\n", babelBinaryPath)
		}
		if isExecutableFile(babelBinaryPath) {
			if debug {
				fmt.Fprintf(os.Stderr, "[DEBUG] Found gbox-device-proxy binary in babel-umbrella: %s\n", babelBinaryPath)
			}
			return babelBinaryPath, nil
		}
	}

	// Priority 3: Check device proxy home directory (where we download binaries)
	deviceProxyHome := config.GetDeviceProxyHome()
	deviceProxyBinaryPath := filepath.Join(deviceProxyHome, binaryName)
	if debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] Checking device proxy home: %s\n", deviceProxyBinaryPath)
	}
	if isExecutableFile(deviceProxyBinaryPath) {
		if debug {
			fmt.Fprintf(os.Stderr, "[DEBUG] Found gbox-device-proxy binary in device proxy home: %s\n", deviceProxyBinaryPath)
		}

		// Use the new version-aware download function
		// This will check if we need to update and download if necessary
		binaryPath, err := CheckAndDownloadDeviceProxy()
		if err != nil {
			if debug {
				fmt.Printf("Warning: Failed to check/download device proxy: %v\n", err)
			}
			// Return existing binary if download fails
			return deviceProxyBinaryPath, nil
		}

		return binaryPath, nil
	}

	// Priority 4: Check PATH
	if path, err := exec.LookPath("gbox-device-proxy"); err == nil {
		if debug {
			fmt.Fprintf(os.Stderr, "[DEBUG] Found gbox-device-proxy binary in PATH: %s\n", path)
		}
		return path, nil
	}

	// Fallback: Search in directory hierarchy (current and executable directories)
	searchPaths := []string{}
	// Search in current directory hierarchy
	current := currentDir
	for {
		searchPaths = append(searchPaths, filepath.Join(current, binaryName))
		parent := filepath.Dir(current)
		if parent == current {
			break // Reached root directory
		}
		current = parent
	}
	// Search in executable directory hierarchy
	execCurrent := executableDir
	for {
		searchPaths = append(searchPaths, filepath.Join(execCurrent, binaryName))
		parent := filepath.Dir(execCurrent)
		if parent == execCurrent {
			break // Reached root directory
		}
		execCurrent = parent
	}
	for _, path := range searchPaths {
		if isExecutableFile(path) {
			if debug {
				fmt.Fprintf(os.Stderr, "[DEBUG] Found gbox-device-proxy binary in fallback search: %s\n", path)
			}
			return path, nil
		}
	}

	// Final fallback: Try to download from gbox Releases (public), then fallback to private repo
	fmt.Fprintf(os.Stderr, "gbox-device-proxy binary not found. Attempting to download from gbox Releases...\n")

	downloadedPath, err := DownloadDeviceProxy()
	if err != nil {
		return "", fmt.Errorf("gbox-device-proxy binary not found and download failed: %v", err)
	}

	// Run version command after download and print it to console in one line
	versionCmd := exec.Command(downloadedPath, "--version")
	versionCmd.Env = os.Environ()
	if out, verr := versionCmd.CombinedOutput(); verr != nil {
		fmt.Fprintf(os.Stderr, "Binary downloaded to: %s\n, but it's not executable: %v\n", downloadedPath, verr)
	} else {
		fmt.Fprintf(os.Stderr, "Successfully downloaded gbox-device-proxy to: %s version: %s.\n", downloadedPath, strings.TrimSpace(string(out)))
	}

	return downloadedPath, nil
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

// setupDeviceProxyEnvironment sets up environment variables for device proxy service
func setupDeviceProxyEnvironment(apiKey string) []string {
	env := os.Environ()
	env = append(env, "GBOX_PROVIDER_TYPE=org")
	env = append(env, fmt.Sprintf("GBOX_API_KEY=%s", apiKey))

	// Add ANDROID_DEVMGR_ENDPOINT environment variable with effective base URL
	cloudEndpoint, err := profile.GetEffectiveBaseURL()
	if err != nil {
		// Fallback to default if profile is not available
		cloudEndpoint = config.GetDefaultBaseURL()
	}
	androidDevmgrEndpoint := fmt.Sprintf("%s/devmgr", cloudEndpoint)
	env = append(env, fmt.Sprintf("ANDROID_DEVMGR_ENDPOINT=%s", androidDevmgrEndpoint))

	return env
}
