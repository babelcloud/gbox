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
	deviceProxyRepo = "babelcloud/gbox-device-proxy"
	githubAPIURL    = "https://api.github.com"
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

// DownloadDeviceProxy downloads the latest gbox-device-proxy binary from GitHub
func DownloadDeviceProxy() (string, error) {
	// Get GitHub token - try both sources
	token := config.GetGithubToken()
	if token == "" {
		token = config.GetGithubClientSecret()
	}
	if token == "" {
		return "", fmt.Errorf("GitHub token not found. Please set either GITHUB_TOKEN or GBOX_GITHUB_CLIENT_SECRET environment variable")
	}

	// Get latest release
	release, err := getLatestRelease(token)
	if err != nil {
		return "", fmt.Errorf("failed to get latest release: %v", err)
	}

	// Find asset for current platform
	assetURL, assetName, err := findAssetForPlatform(release)
	if err != nil {
		return "", fmt.Errorf("failed to find asset for platform: %v", err)
	}

	// Download and extract binary with retry
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

// getLatestRelease fetches the latest release from GitHub
func getLatestRelease(token string) (*GitHubRelease, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/latest", githubAPIURL, deviceProxyRepo)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

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

// findAssetForPlatform finds the appropriate asset for the current platform
func findAssetForPlatform(release *GitHubRelease) (string, string, error) {
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

	// Find asset containing the platform
	for _, asset := range release.Assets {
		if strings.Contains(asset.Name, platform) {
			// Use URL field (authenticated API URL) like CI does, fallback to browser_download_url
			downloadURL := asset.URL
			if downloadURL == "" {
				downloadURL = asset.DownloadURL
			}
			return downloadURL, asset.Name, nil
		}
	}

	return "", "", fmt.Errorf("no asset found for platform: %s", platform)
}

// downloadAndExtractBinary downloads and extracts the binary file
func downloadAndExtractBinary(assetURL, assetName, token string) (string, error) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "gbox-device-proxy-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tempDir)

	// Download the asset
	assetPath := filepath.Join(tempDir, assetName)
	if err := downloadFile(assetURL, assetPath, token); err != nil {
		return "", err
	}

	// Extract the binary
	binaryPath, err := extractBinary(assetPath, tempDir)
	if err != nil {
		return "", err
	}

	// Move binary to device proxy home directory
	deviceProxyHome := config.GetDeviceProxyHome()
	if err := os.MkdirAll(deviceProxyHome, 0755); err != nil {
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
