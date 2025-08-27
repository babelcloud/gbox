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


// checkRemoteVersionAndCompare checks if remote version is different from local
func checkRemoteVersionAndCompare(localBinaryPath string, debug bool) (bool, error) {
	// Get the asset name for current platform
	assetName := getAssetNameForPlatform()
	deviceProxyHome := config.GetDeviceProxyHome()
	localArchivePath := filepath.Join(deviceProxyHome, assetName)
	
	// Check if both local files exist
	if _, err := os.Stat(localArchivePath); err != nil {
		return true, nil
	}
	
	if _, err := os.Stat(localBinaryPath); err != nil {
		return true, nil
	}
	
	// Both files exist, check SHA256
	
	// Get GitHub token
	token := config.GetGithubToken()
	
	// Try to get latest release from public repository first
	release, err := getLatestRelease(deviceProxyPublicRepo, "")
	if err != nil {
		if debug {
			fmt.Printf("Failed to get latest version from public repository: %v, trying private repository\n", err)
		}
		if token != "" {
			release, err = getLatestRelease(deviceProxyRepo, token)
			if err != nil {
				return false, fmt.Errorf("Failed to get latest version from private repository: %v", err)
			}
		} else {
			return false, fmt.Errorf("Unable to get remote version information: %v", err)
		}
	}
	
	// Find device-proxy asset for current platform
	assetURL, _, err := findDeviceProxyAssetForPlatform(release, token)
	if err != nil {
		return false, fmt.Errorf("Cannot find platform-specific asset: %v", err)
	}
	
	// Try to get remote SHA256 from SHA256 file first (more efficient)
	remoteSHA256, err := getRemoteSHA256FromFile(release, assetName, token)
	if err != nil {
		if debug {
			fmt.Printf("Cannot get hash from SHA256 file: %v, falling back to downloading archive\n", err)
		}
		// Fallback to downloading the archive for SHA256 calculation
		remoteSHA256, err = getRemoteSHA256FromArchive(assetURL, assetName, token)
		if err != nil {
			return false, fmt.Errorf("Failed to get remote archive SHA256: %v", err)
		}
	}
	
	// Calculate local archive SHA256
	localSHA256, err := calculateSHA256(localArchivePath)
	if err != nil {
		return false, fmt.Errorf("Failed to calculate local archive SHA256: %v", err)
	}
	
	// Compare SHA256
	if remoteSHA256 == localSHA256 {
		return false, nil
	} else {
		return true, nil
	}
}

// getRemoteSHA256FromFile gets the remote SHA256 hash from the SHA256 file
func getRemoteSHA256FromFile(release *GitHubRelease, assetName, token string) (string, error) {
	sha256URL, err := findSHA256File(release, assetName)
	if err != nil {
		return "", err
	}
	
	return downloadSHA256File(sha256URL, token)
}

// getRemoteSHA256FromArchive downloads the remote archive to get its SHA256 (fallback method)
func getRemoteSHA256FromArchive(assetURL, assetName, token string) (string, error) {
	// Download remote archive to temp to get SHA256
	tempDir, err := os.MkdirTemp("", "gbox-device-proxy-temp-*")
	if err != nil {
		return "", fmt.Errorf("Failed to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	tempArchivePath := filepath.Join(tempDir, assetName)
	
	if err := downloadFile(assetURL, tempArchivePath, token); err != nil {
		return "", fmt.Errorf("Failed to download remote archive: %v", err)
	}
	
	// Calculate remote archive SHA256
	remoteSHA256, err := calculateSHA256(tempArchivePath)
	if err != nil {
		return "", fmt.Errorf("Failed to calculate remote archive SHA256: %v", err)
	}
	
	return remoteSHA256, nil
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
		
		// Check if remote version is different
		needUpdate, err := checkRemoteVersionAndCompare(deviceProxyBinaryPath, debug)
		if err != nil {
			return deviceProxyBinaryPath, nil
		}
		
		if needUpdate {			
			// Remove local files
			assetName := getAssetNameForPlatform()
			localArchivePath := filepath.Join(deviceProxyHome, assetName)
			
			if _, err := os.Stat(localArchivePath); err == nil {
				if err := os.Remove(localArchivePath); err != nil {
					if debug {
						fmt.Printf("Warning: Failed to remove local archive: %v\n", err)
					}
				}
			}
			if err := os.Remove(deviceProxyBinaryPath); err != nil {
				if debug {
					fmt.Printf("Warning: Failed to remove local binary file: %v\n", err)
				}
			}
		} else {
			return deviceProxyBinaryPath, nil
		}
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

// getAssetNameForPlatform returns the asset name for current platform
func getAssetNameForPlatform() string {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	var platform string
	switch osName {
	case "darwin":
		if arch == "amd64" {
			platform = "darwin-amd64"
		} else if arch == "arm64" {
			platform = "darwin-arm64"
		}
	case "linux":
		if arch == "amd64" {
			platform = "linux-amd64"
		} else if arch == "arm64" {
			platform = "linux-arm64"
		}
	case "windows":
		if arch == "amd64" {
			platform = "windows-amd64"
		} else if arch == "arm64" {
			platform = "windows-arm64"
		}
	}

	return fmt.Sprintf("gbox-device-proxy-%s.tar.gz", platform)
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
