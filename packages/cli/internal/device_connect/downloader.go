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
func downloadSHA256File(sha256URL, token string) (string, error) {
	req, err := http.NewRequest("GET", sha256URL, nil)
	if err != nil {
		return "", err
	}

	if token != "" {
		req.Header.Set("Authorization", "token "+token)
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

// DownloadDeviceProxy downloads the latest gbox-device-proxy binary from GitHub
func DownloadDeviceProxy() (string, error) {
	// Get GitHub token - try both sources
	token := config.GetGithubToken()

	// Try to download from public repository first (no token required)
	release, err := getLatestRelease(deviceProxyPublicRepo, "")
	if err == nil {
		assetURL, assetName, err := findDeviceProxyAssetForPlatform(release, "")
		if err == nil {
			binaryPath, err := downloadAndExtractBinaryWithRetry(assetURL, assetName, "")
			if err == nil {
				return binaryPath, nil
			}
			fmt.Fprintf(os.Stderr, "Failed to download from public repository: %v\n", err)
		}
	}

	// If public repository fails and we have a token, try private repository
	if token != "" {
		fmt.Println("Trying private repository with authentication...")
		release, err = getLatestRelease(deviceProxyRepo, token)
		if err != nil {
			return "", fmt.Errorf("failed to get latest release from private repository: %v", err)
		}

		assetURL, assetName, err := findDeviceProxyAssetForPlatform(release, token)
		if err != nil {
			return "", fmt.Errorf("failed to find asset for platform in private repository: %v", err)
		}

		binaryPath, err := downloadAndExtractBinaryWithRetry(assetURL, assetName, token)
		if err != nil {
			return "", fmt.Errorf("failed to download from private repository: %v", err)
		}

		return binaryPath, nil
	}

	return "", fmt.Errorf("failed to download device-proxy: public repository unavailable and no authentication token provided")
}

// getLatestRelease fetches the latest release from GitHub
func getLatestRelease(repo, token string) (*GitHubRelease, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/latest", githubAPIURL, repo)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if token != "" {
		req.Header.Set("Authorization", "token "+token)
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
func findDeviceProxyAssetForPlatform(release *GitHubRelease, token string) (string, string, error) {
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
			// If unauthenticated, prefer browser_download_url; otherwise prefer API asset URL
			if token == "" {
				if asset.DownloadURL != "" {
					return asset.DownloadURL, asset.Name, nil
				}
				// fallback to API URL even without token (may rate-limit/fail)
				if asset.URL != "" {
					return asset.URL, asset.Name, nil
				}
			} else {
				if asset.URL != "" {
					return asset.URL, asset.Name, nil
				}
				if asset.DownloadURL != "" {
					return asset.DownloadURL, asset.Name, nil
				}
			}
		}
	}

	return "", "", fmt.Errorf("no device-proxy asset found for platform: %s", platform)
}

// downloadAndExtractBinary downloads and extracts the binary file
func downloadAndExtractBinary(assetURL, assetName, token string) (string, error) {
	// Get device proxy home directory first
	deviceProxyHome := config.GetDeviceProxyHome()
	if err := os.MkdirAll(deviceProxyHome, 0755); err != nil {
		return "", err
	}

	// Download the asset directly to device proxy home directory
	assetPath := filepath.Join(deviceProxyHome, assetName)
	if err := downloadFile(assetURL, assetPath, token); err != nil {
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
func downloadAndExtractBinaryWithRetry(assetURL, assetName, token string) (string, error) {
	var binaryPath string
	var lastErr error
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		binaryPath, lastErr = downloadAndExtractBinary(assetURL, assetName, token)
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
func downloadFile(url, filepath string, token string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "token "+token)
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
