package device_connect

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/babelcloud/gbox/packages/cli/config"
	"github.com/babelcloud/gbox/packages/cli/internal/version"
)

const (
	deviceProxyRepo       = "babelcloud/gbox-device-proxy"
	deviceProxyPublicRepo = "babelcloud/gbox" // Public repository for device-proxy assets
	githubAPIURL          = "https://api.github.com"
)

// GitHubRelease represents a GitHub release
type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name        string `json:"name"`
		DownloadURL string `json:"browser_download_url"`
		URL         string `json:"url"`
	} `json:"assets"`
}

// findSHA256File finds the SHA256 file for a given asset
func findSHA256File(release *GitHubRelease, assetName string) (string, error) {
	sha256FileName := assetName + ".sha256"

	for _, asset := range release.Assets {
		if asset.Name == sha256FileName {
			if asset.DownloadURL != "" {
				return asset.DownloadURL, nil
			}
			if asset.URL != "" {
				return asset.URL, nil
			}
		}
	}

	return "", fmt.Errorf("SHA256 file not found for asset: %s", assetName)
}

// downloadSHA256File downloads and returns the SHA256 hash from a SHA256 file
func downloadSHA256File(sha256URL string) (string, error) {
	req, err := http.NewRequest("GET", sha256URL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Accept", "text/plain")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download SHA256 file, status: %d", resp.StatusCode)
	}

	// Read the SHA256 hash from the file
	sha256Bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read SHA256 file: %v", err)
	}

	// Extract the hash (format: "hash filename")
	sha256Line := strings.TrimSpace(string(sha256Bytes))
	parts := strings.Fields(sha256Line)
	if len(parts) < 1 {
		return "", fmt.Errorf("invalid SHA256 file format: %s", sha256Line)
	}

	return parts[0], nil
}

// VersionInfo represents version information
type VersionInfo struct {
	TagName    string `json:"tag_name"`
	CommitID   string `json:"commit_id"`
	Downloaded string `json:"downloaded"`
}

// getVersionCachePath returns the path to the version cache file
func getVersionCachePath() string {
	deviceProxyHome := config.GetDeviceProxyHome()
	return filepath.Join(deviceProxyHome, "version.json")
}

// loadVersionInfo loads version information from cache
func loadVersionInfo() (*VersionInfo, error) {
	cachePath := getVersionCachePath()
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, err
	}

	var info VersionInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}

	return &info, nil
}

// saveVersionInfo saves version information to cache
func saveVersionInfo(info *VersionInfo) error {
	cachePath := getVersionCachePath()
	deviceProxyHome := config.GetDeviceProxyHome()

	// Ensure directory exists
	if err := os.MkdirAll(deviceProxyHome, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cachePath, data, 0644)
}

// CheckAndDownloadDeviceProxy checks if update is needed and downloads if necessary
func CheckAndDownloadDeviceProxy() (string, error) {
	deviceProxyHome := config.GetDeviceProxyHome()
	binaryName := "gbox-device-proxy"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(deviceProxyHome, binaryName)

	// Check if binary exists
	if _, err := os.Stat(binaryPath); err != nil {
		// Binary doesn't exist, download it
		return DownloadDeviceProxy()
	}

	// Load cached version info
	cachedInfo, err := loadVersionInfo()
	if err != nil {
		// No cache, download latest
		return DownloadDeviceProxy()
	}

	// Try to find release matching current version first
	currentVersion := version.ClientInfo()["Version"]
	currentCommit := version.ClientInfo()["GitCommit"]

	// First try to find exact version match
	if currentVersion != "dev" {
		release, err := getReleaseByTag(deviceProxyPublicRepo, currentVersion)
		if err == nil {
			assetURL, assetName, err := findDeviceProxyAssetForPlatform(release)
			if err == nil {
				// Found matching version, check if we need to download
				if cachedInfo.TagName == currentVersion {
					// Same version, return existing binary
					return binaryPath, nil
				}
				// Different version, download
				binaryPath, err := downloadAndExtractBinaryWithRetry(assetURL, assetName)
				if err == nil {
					// Save version info
					saveVersionInfo(&VersionInfo{
						TagName:    currentVersion,
						CommitID:   currentCommit,
						Downloaded: time.Now().Format(time.RFC3339),
					})
					return binaryPath, nil
				}
			}
		}
	}

	// If no exact match or failed, try latest release
	release, err := getLatestRelease(deviceProxyPublicRepo)
	if err != nil {
		return "", fmt.Errorf("failed to get latest release: %v", err)
	}

	// Check if we already have this version
	if cachedInfo.TagName == release.TagName {
		return binaryPath, nil
	}

	// Download latest version
	assetURL, assetName, err := findDeviceProxyAssetForPlatform(release)
	if err != nil {
		return "", fmt.Errorf("failed to find device proxy asset: %v", err)
	}

	binaryPath, err = downloadAndExtractBinaryWithRetry(assetURL, assetName)
	if err != nil {
		return "", fmt.Errorf("failed to download device proxy: %v", err)
	}

	// Save version info
	saveVersionInfo(&VersionInfo{
		TagName:    release.TagName,
		CommitID:   currentCommit,
		Downloaded: time.Now().Format(time.RFC3339),
	})

	return binaryPath, nil
}

// DownloadDeviceProxy downloads the gbox-device-proxy binary from GitHub
// It first tries to download from a release matching the current version,
// and falls back to the latest release if no matching version is found
func DownloadDeviceProxy() (string, error) {
	currentVersion := version.ClientInfo()["Version"]
	currentCommit := version.ClientInfo()["GitCommit"]

	var release *GitHubRelease
	var err error

	// First try to find release matching current version
	if currentVersion != "dev" {
		release, err = getReleaseByTag(deviceProxyPublicRepo, currentVersion)
		if err == nil {
			// Found matching version, try to download from it
			assetURL, assetName, err := findDeviceProxyAssetForPlatform(release)
			if err == nil {
				binaryPath, err := downloadAndExtractBinaryWithRetry(assetURL, assetName)
				if err == nil {
					// Save version info for matching version
					saveVersionInfo(&VersionInfo{
						TagName:    release.TagName,
						CommitID:   currentCommit,
						Downloaded: time.Now().Format(time.RFC3339),
					})
					return binaryPath, nil
				}
				// If download failed, continue to try latest release
			}
		}
		// If no matching version found or download failed, continue to latest release
	}

	// Fallback to latest release
	release, err = getLatestRelease(deviceProxyPublicRepo)
	if err != nil {
		return "", fmt.Errorf("failed to get latest release: %v", err)
	}

	assetURL, assetName, err := findDeviceProxyAssetForPlatform(release)
	if err != nil {
		return "", fmt.Errorf("failed to find device proxy asset: %v", err)
	}

	binaryPath, err := downloadAndExtractBinaryWithRetry(assetURL, assetName)
	if err != nil {
		return "", fmt.Errorf("failed to download device proxy: %v", err)
	}

	// Save version info for latest release
	saveVersionInfo(&VersionInfo{
		TagName:    release.TagName,
		CommitID:   currentCommit,
		Downloaded: time.Now().Format(time.RFC3339),
	})

	return binaryPath, nil
}

// getLatestRelease fetches the latest release from GitHub
func getLatestRelease(repo string) (*GitHubRelease, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/latest", githubAPIURL, repo)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "gbox-cli")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status: %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

// getReleaseByTag fetches a specific release by tag from GitHub
func getReleaseByTag(repo, tag string) (*GitHubRelease, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/tags/%s", githubAPIURL, repo, tag)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "gbox-cli")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status: %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

// findDeviceProxyAssetForPlatform finds the device-proxy asset for the current platform
func findDeviceProxyAssetForPlatform(release *GitHubRelease) (string, string, error) {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	// Map runtime.GOOS to asset name format
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

	if platform == "" {
		return "", "", fmt.Errorf("unsupported platform: %s-%s", osName, arch)
	}

	// Find device-proxy asset containing the platform
	for _, asset := range release.Assets {
		if strings.Contains(asset.Name, "gbox-device-proxy") && strings.Contains(asset.Name, platform) {
			// Use browser_download_url for public access
			if asset.DownloadURL != "" {
				return asset.DownloadURL, asset.Name, nil
			}
			// fallback to API URL (may rate-limit/fail)
			if asset.URL != "" {
				return asset.URL, asset.Name, nil
			}
		}
	}

	return "", "", fmt.Errorf("no device-proxy asset found for platform: %s", platform)
}

// downloadAndExtractBinary downloads and extracts the binary file
func downloadAndExtractBinary(assetURL, assetName string) (string, error) {
	// Get device proxy home directory first
	deviceProxyHome := config.GetDeviceProxyHome()
	if err := os.MkdirAll(deviceProxyHome, 0755); err != nil {
		return "", err
	}

	// Download the asset directly to device proxy home directory
	assetPath := filepath.Join(deviceProxyHome, assetName)
	if err := downloadFile(assetURL, assetPath); err != nil {
		return "", err
	}

	// Create temporary directory for extraction
	tempDir, err := os.MkdirTemp("", "gbox-device-proxy-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tempDir)

	// Extract the binary
	binaryPath, err := extractBinary(assetPath, tempDir)
	if err != nil {
		return "", err
	}

	binaryName := "gbox-device-proxy"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}

	finalPath := filepath.Join(deviceProxyHome, binaryName)

	// Remove existing file if it exists (in case it's corrupted)
	if _, err := os.Stat(finalPath); err == nil {
		if err := os.Remove(finalPath); err != nil {
			// Don't fail if we can't remove the file (it might be in use)
			// Just log a warning and continue
			fmt.Fprintf(os.Stderr, "Warning: Could not remove existing binary %s: %v\n", finalPath, err)
		}
	}

	if err := os.Rename(binaryPath, finalPath); err != nil {
		// If rename fails, try copy and remove
		if copyErr := copyFile(binaryPath, finalPath); copyErr != nil {
			return "", fmt.Errorf("failed to move binary to final location: %v (copy failed: %v)", err, copyErr)
		}
		// Try to remove the original, but don't fail if it doesn't work
		os.Remove(binaryPath)
	}

	// Make binary executable
	if err := os.Chmod(finalPath, 0755); err != nil {
		return "", err
	}

	return finalPath, nil
}

// downloadAndExtractBinaryWithRetry downloads and extracts the binary file with retry logic
func downloadAndExtractBinaryWithRetry(assetURL, assetName string) (string, error) {
	var binaryPath string
	var lastErr error
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		binaryPath, lastErr = downloadAndExtractBinary(assetURL, assetName)
		if lastErr == nil {
			break
		}

		if i < maxRetries-1 {
			fmt.Fprintf(os.Stderr, "Download attempt %d failed: %v. Retrying...\n", i+1, lastErr)
			time.Sleep(time.Duration(i+1) * time.Second) // Exponential backoff
		}
	}

	if lastErr != nil {
		return "", fmt.Errorf("failed to download and extract binary after %d attempts: %v", maxRetries, lastErr)
	}

	return binaryPath, nil
}

// downloadFile downloads a file from URL to local path
func downloadFile(url, filepath string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Use a buffer to copy data and check for errors
	buf := make([]byte, 32*1024) // 32KB buffer
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := file.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("write error: %v", writeErr)
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("read error: %v", err)
		}
	}

	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// extractBinary extracts the binary from the downloaded asset
func extractBinary(assetPath, extractDir string) (string, error) {
	if strings.HasSuffix(assetPath, ".tar.gz") {
		return extractTarGz(assetPath, extractDir)
	} else if strings.HasSuffix(assetPath, ".zip") {
		return extractZip(assetPath, extractDir)
	}
	return "", fmt.Errorf("unsupported archive format: %s", assetPath)
}

// extractTarGz extracts a .tar.gz file
func extractTarGz(archivePath, extractDir string) (string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	var binaryPath string

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		// Skip directories
		if header.Typeflag == tar.TypeDir {
			continue
		}

		// Look for device-proxy binary
		if strings.Contains(header.Name, "device-proxy") {
			extractPath := filepath.Join(extractDir, filepath.Base(header.Name))
			extractFile, err := os.Create(extractPath)
			if err != nil {
				return "", err
			}

			if _, err := io.Copy(extractFile, tr); err != nil {
				extractFile.Close()
				return "", err
			}
			extractFile.Close()

			binaryPath = extractPath
			break
		}
	}

	if binaryPath == "" {
		return "", fmt.Errorf("device-proxy binary not found in archive")
	}

	return binaryPath, nil
}

// extractZip extracts a .zip file
func extractZip(archivePath, extractDir string) (string, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	var binaryPath string

	for _, file := range reader.File {
		// Look for device-proxy binary
		if strings.Contains(file.Name, "device-proxy") {
			extractPath := filepath.Join(extractDir, filepath.Base(file.Name))

			// Create the file
			extractFile, err := os.Create(extractPath)
			if err != nil {
				return "", err
			}

			// Open the file in the archive
			archiveFile, err := file.Open()
			if err != nil {
				extractFile.Close()
				return "", err
			}

			// Copy content
			if _, err := io.Copy(extractFile, archiveFile); err != nil {
				extractFile.Close()
				archiveFile.Close()
				return "", err
			}

			extractFile.Close()
			archiveFile.Close()
			binaryPath = extractPath
			break
		}
	}

	if binaryPath == "" {
		return "", fmt.Errorf("device-proxy binary not found in archive")
	}

	return binaryPath, nil
}
