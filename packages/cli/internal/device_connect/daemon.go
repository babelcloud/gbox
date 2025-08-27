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
func checkRemoteVersionAndCompare(localBinaryPath string) (bool, error) {
	fmt.Println("检查远程版本并比对SHA256...")
	
	// Get the asset name for current platform
	assetName := getAssetNameForPlatform()
	deviceProxyHome := config.GetDeviceProxyHome()
	localArchivePath := filepath.Join(deviceProxyHome, assetName)
	
	// Check if both local files exist
	if _, err := os.Stat(localArchivePath); err != nil {
		fmt.Println("本地压缩包不存在，需要下载")
		return true, nil
	}
	
	if _, err := os.Stat(localBinaryPath); err != nil {
		fmt.Println("本地二进制文件不存在，需要下载")
		return true, nil
	}
	
	// Both files exist, check SHA256
	fmt.Println("本地文件已存在，开始校验SHA256...")
	
	// Get GitHub token
	token := config.GetGithubToken()
	
	// Try to get latest release from public repository first
	release, err := getLatestRelease(deviceProxyPublicRepo, "")
	if err != nil {
		fmt.Printf("从公共仓库获取最新版本失败: %v，尝试私有仓库\n", err)
		if token != "" {
			release, err = getLatestRelease(deviceProxyRepo, token)
			if err != nil {
				return false, fmt.Errorf("从私有仓库获取最新版本也失败: %v", err)
			}
		} else {
			return false, fmt.Errorf("无法获取远程版本信息: %v", err)
		}
	}
	
	// Find device-proxy asset for current platform
	assetURL, _, err := findDeviceProxyAssetForPlatform(release, token)
	if err != nil {
		return false, fmt.Errorf("找不到平台对应的资源: %v", err)
	}
	
	// Try to get remote SHA256 from SHA256 file first (more efficient)
	fmt.Println("尝试从SHA256文件获取远程压缩包哈希值...")
	remoteSHA256, err := getRemoteSHA256FromFile(release, assetName, token)
	if err != nil {
		fmt.Printf("无法从SHA256文件获取哈希值: %v，回退到下载压缩包方式\n", err)
		// Fallback to downloading the archive for SHA256 calculation
		remoteSHA256, err = getRemoteSHA256FromArchive(assetURL, assetName, token)
		if err != nil {
			return false, fmt.Errorf("获取远程压缩包SHA256失败: %v", err)
		}
	} else {
		fmt.Printf("成功从SHA256文件获取远程压缩包SHA256: %s\n", remoteSHA256)
	}
	
	// Calculate local archive SHA256
	localSHA256, err := calculateSHA256(localArchivePath)
	if err != nil {
		return false, fmt.Errorf("计算本地压缩包SHA256失败: %v", err)
	}
	fmt.Printf("本地压缩包SHA256: %s\n", localSHA256)
	
	// Compare SHA256
	if remoteSHA256 == localSHA256 {
		fmt.Println("SHA256校验通过，本地版本是最新的")
		return false, nil
	} else {
		fmt.Println("SHA256校验失败，远程版本已更新")
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
		return "", fmt.Errorf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	tempArchivePath := filepath.Join(tempDir, assetName)
	fmt.Printf("下载远程压缩包到临时位置进行SHA256校验: %s\n", tempArchivePath)
	
	if err := downloadFile(assetURL, tempArchivePath, token); err != nil {
		return "", fmt.Errorf("下载远程压缩包失败: %v", err)
	}
	
	// Calculate remote archive SHA256
	remoteSHA256, err := calculateSHA256(tempArchivePath)
	if err != nil {
		return "", fmt.Errorf("计算远程压缩包SHA256失败: %v", err)
	}
	fmt.Printf("远程压缩包SHA256: %s\n", remoteSHA256)
	
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
		needUpdate, err := checkRemoteVersionAndCompare(deviceProxyBinaryPath)
		if err != nil {
			fmt.Printf("检查远程版本失败: %v，使用本地版本\n", err)
			return deviceProxyBinaryPath, nil
		}
		
		if needUpdate {
			fmt.Println("检测到远程版本更新，删除本地文件并重新下载")
			
			// Remove local files
			assetName := getAssetNameForPlatform()
			localArchivePath := filepath.Join(deviceProxyHome, assetName)
			
			if _, err := os.Stat(localArchivePath); err == nil {
				if err := os.Remove(localArchivePath); err != nil {
					fmt.Printf("警告: 删除本地压缩包失败: %v\n", err)
				}
			}
			if err := os.Remove(deviceProxyBinaryPath); err != nil {
				fmt.Printf("警告: 删除本地二进制文件失败: %v\n", err)
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
