package device_connect

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/babelcloud/gbox/packages/cli/config"
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

	// Priority 3: Check CLI cache directory (where we download binaries)
	cliCacheHome := config.GetCliCacheHome()
	deviceProxyBinaryPath := filepath.Join(cliCacheHome, binaryName)
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
func setupDeviceProxyEnvironment(apiKey, baseURL string) []string {
	env := os.Environ()
	env = append(env, "GBOX_PROVIDER_TYPE=org")
	env = append(env, fmt.Sprintf("GBOX_API_KEY=%s", apiKey))

	baseEndpoint := strings.TrimSuffix(baseURL, "/")
	baseEndpoint = strings.TrimSuffix(baseEndpoint, "/api/v1")

	// Add ANDROID_DEVMGR_ENDPOINT environment variable
	env = append(env, fmt.Sprintf("ANDROID_DEVMGR_ENDPOINT=%s/devmgr", baseEndpoint))

	// Also add GBOX_BASE_URL for consistency
	env = append(env, fmt.Sprintf("GBOX_BASE_URL=%s", baseEndpoint))

	// Work around for frp not supporting no_proxy
	env = handleNoProxyWorkaround(env, baseURL)
	
	return env
}

// handleNoProxyWorkaround handles the case where frp doesn't support no_proxy
// If the target domain is in no_proxy list, remove proxy environment variables
func handleNoProxyWorkaround(env []string, baseURL string) []string {
	// Parse baseURL to get domain
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		// If we can't parse the URL, return original env unchanged
		return env
	}

	domain := parsedURL.Hostname()
	if domain == "" {
		return env
	}

	// Check if domain is in no_proxy list
	noProxyList := getNoProxyList()
	if !isDomainInNoProxyList(domain, noProxyList) {
		return env
	}

	// Domain is in no_proxy list, remove proxy environment variables
	return removeProxyEnvironmentVariables(env)
}

// getNoProxyList gets the no_proxy list from environment variables
func getNoProxyList() []string {
	noProxy := os.Getenv("no_proxy")
	if noProxy == "" {
		noProxy = os.Getenv("NO_PROXY")
	}

	if noProxy == "" {
		return nil
	}

	// Split by comma and trim spaces
	domains := strings.Split(noProxy, ",")
	var result []string
	for _, domain := range domains {
		trimmed := strings.TrimSpace(domain)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// isDomainInNoProxyList checks if a domain matches any pattern in no_proxy list
func isDomainInNoProxyList(domain string, noProxyList []string) bool {
	for _, pattern := range noProxyList {
		if matchesNoProxyPattern(domain, pattern) {
			return true
		}
	}
	return false
}

// matchesNoProxyPattern checks if a domain matches a no_proxy pattern
// Supports exact match and wildcard (*.example.com)
func matchesNoProxyPattern(domain, pattern string) bool {
	// Exact match
	if domain == pattern {
		return true
	}

	// Wildcard pattern (*.example.com)
	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[2:] // Remove "*."
		if strings.HasSuffix(domain, suffix) {
			return true
		}
	}

	// Localhost and local domains
	if pattern == "localhost" || pattern == "127.0.0.1" || pattern == "::1" {
		if domain == "localhost" || domain == "127.0.0.1" || domain == "::1" {
			return true
		}
	}

	return false
}

// removeProxyEnvironmentVariables removes proxy-related environment variables
func removeProxyEnvironmentVariables(env []string) []string {
	var result []string

	for _, envVar := range env {
		// Only remove http_proxy and https_proxy (both lowercase and uppercase)
		if strings.HasPrefix(envVar, "http_proxy=") ||
			strings.HasPrefix(envVar, "HTTP_PROXY=") ||
			strings.HasPrefix(envVar, "https_proxy=") ||
			strings.HasPrefix(envVar, "HTTPS_PROXY=") {
			continue // Skip this environment variable
		}
		result = append(result, envVar)
	}

	return result
}
