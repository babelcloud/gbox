package device_connect

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/babelcloud/gbox/packages/cli/config"
)

func TestVersionMatchingLogic(t *testing.T) {
	// Test the version matching logic with different scenarios
	tempDir, err := os.MkdirTemp("", "gbox-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Temporarily override device proxy home for testing
	originalHome := config.GetDeviceProxyHome()
	defer func() {
		os.Setenv("DEVICE_PROXY_HOME", originalHome)
	}()

	os.Setenv("DEVICE_PROXY_HOME", tempDir)

	// Test scenario 1: No binary exists - should download latest
	t.Run("NoBinaryExists", func(t *testing.T) {
		// Ensure no binary exists
		binaryName := "gbox-device-proxy"
		if runtime.GOOS == "windows" {
			binaryName += ".exe"
		}
		binaryPath := filepath.Join(tempDir, binaryName)
		os.Remove(binaryPath)
		os.Remove(getVersionCachePath())

		// Should download latest
		path, err := CheckAndDownloadDeviceProxy()
		if err != nil {
			t.Fatalf("Failed to download when no binary exists: %v", err)
		}

		if path != binaryPath {
			t.Errorf("Expected binary path %s, got %s", binaryPath, path)
		}

		// Verify version cache was created
		if _, err := os.Stat(getVersionCachePath()); err != nil {
			t.Errorf("Version cache should exist: %v", err)
		}
	})

	// Test scenario 2: Binary exists with matching version - should not download
	t.Run("BinaryExistsWithMatchingVersion", func(t *testing.T) {
		// Create a fake binary
		binaryName := "gbox-device-proxy"
		if runtime.GOOS == "windows" {
			binaryName += ".exe"
		}
		binaryPath := filepath.Join(tempDir, binaryName)
		if err := os.WriteFile(binaryPath, []byte("fake binary"), 0755); err != nil {
			t.Fatalf("Failed to create fake binary: %v", err)
		}

		// Create version cache with current latest version
		latestRelease, err := getLatestRelease(deviceProxyPublicRepo)
		if err != nil {
			t.Fatalf("Failed to get latest release: %v", err)
		}

		cacheInfo := &VersionInfo{
			TagName:    latestRelease.TagName,
			CommitID:   "test-commit",
			Downloaded: time.Now().Format(time.RFC3339),
		}
		if err := saveVersionInfo(cacheInfo); err != nil {
			t.Fatalf("Failed to save version info: %v", err)
		}

		// Should return existing binary without downloading
		path, err := CheckAndDownloadDeviceProxy()
		if err != nil {
			t.Fatalf("Failed to check version: %v", err)
		}

		if path != binaryPath {
			t.Errorf("Expected binary path %s, got %s", binaryPath, path)
		}

		// Verify the binary wasn't replaced (content should still be "fake binary")
		content, err := os.ReadFile(binaryPath)
		if err != nil {
			t.Fatalf("Failed to read binary: %v", err)
		}

		if string(content) != "fake binary" {
			t.Error("Binary should not have been replaced")
		}
	})

	// Test scenario 3: Binary exists with different version - should download
	t.Run("BinaryExistsWithDifferentVersion", func(t *testing.T) {
		// Create a fake binary
		binaryName := "gbox-device-proxy"
		if runtime.GOOS == "windows" {
			binaryName += ".exe"
		}
		binaryPath := filepath.Join(tempDir, binaryName)
		if err := os.WriteFile(binaryPath, []byte("old fake binary"), 0755); err != nil {
			t.Fatalf("Failed to create fake binary: %v", err)
		}

		// Create version cache with old version
		cacheInfo := &VersionInfo{
			TagName:    "v0.0.1", // Old version
			CommitID:   "old-commit",
			Downloaded: time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
		}
		if err := saveVersionInfo(cacheInfo); err != nil {
			t.Fatalf("Failed to save version info: %v", err)
		}

		// Should download new version
		path, err := CheckAndDownloadDeviceProxy()
		if err != nil {
			t.Fatalf("Failed to check version: %v", err)
		}

		if path != binaryPath {
			t.Errorf("Expected binary path %s, got %s", binaryPath, path)
		}

		// Verify the binary was replaced (should be a real binary now)
		info, err := os.Stat(binaryPath)
		if err != nil {
			t.Fatalf("Failed to stat binary: %v", err)
		}

		if info.Size() < 1000 { // Real binary should be much larger
			t.Error("Binary should have been replaced with real binary")
		}

		// Verify version cache was updated
		newCacheInfo, err := loadVersionInfo()
		if err != nil {
			t.Fatalf("Failed to load version info: %v", err)
		}

		if newCacheInfo.TagName == "v0.0.1" {
			t.Error("Version cache should have been updated")
		}
	})
}

func TestVersionCacheOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gbox-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Temporarily override device proxy home for testing
	originalHome := config.GetDeviceProxyHome()
	defer func() {
		os.Setenv("DEVICE_PROXY_HOME", originalHome)
	}()

	os.Setenv("DEVICE_PROXY_HOME", tempDir)

	// Test saving version info
	t.Run("SaveVersionInfo", func(t *testing.T) {
		testInfo := &VersionInfo{
			TagName:    "v1.2.3",
			CommitID:   "abc123def456",
			Downloaded: "2023-12-01T10:30:00Z",
		}

		err := saveVersionInfo(testInfo)
		if err != nil {
			t.Fatalf("Failed to save version info: %v", err)
		}

		// Verify file was created
		cachePath := getVersionCachePath()
		if _, err := os.Stat(cachePath); err != nil {
			t.Errorf("Version cache file should exist: %v", err)
		}

		// Verify content
		data, err := os.ReadFile(cachePath)
		if err != nil {
			t.Fatalf("Failed to read cache file: %v", err)
		}

		if !contains(string(data), "v1.2.3") {
			t.Error("Cache file should contain version v1.2.3")
		}

		if !contains(string(data), "abc123def456") {
			t.Error("Cache file should contain commit ID")
		}
	})

	// Test loading version info
	t.Run("LoadVersionInfo", func(t *testing.T) {
		// First save some data
		testInfo := &VersionInfo{
			TagName:    "v2.0.0",
			CommitID:   "xyz789",
			Downloaded: "2023-12-02T15:45:00Z",
		}

		if err := saveVersionInfo(testInfo); err != nil {
			t.Fatalf("Failed to save version info: %v", err)
		}

		// Then load it
		loadedInfo, err := loadVersionInfo()
		if err != nil {
			t.Fatalf("Failed to load version info: %v", err)
		}

		if loadedInfo.TagName != testInfo.TagName {
			t.Errorf("Expected tag name %s, got %s", testInfo.TagName, loadedInfo.TagName)
		}

		if loadedInfo.CommitID != testInfo.CommitID {
			t.Errorf("Expected commit ID %s, got %s", testInfo.CommitID, loadedInfo.CommitID)
		}

		if loadedInfo.Downloaded != testInfo.Downloaded {
			t.Errorf("Expected downloaded time %s, got %s", testInfo.Downloaded, loadedInfo.Downloaded)
		}
	})

	// Test loading non-existent file
	t.Run("LoadNonExistentFile", func(t *testing.T) {
		// Remove cache file
		os.Remove(getVersionCachePath())

		// Try to load
		_, err := loadVersionInfo()
		if err == nil {
			t.Error("Expected error when loading non-existent file")
		}
	})
}

func TestGetReleaseByTagErrorHandling(t *testing.T) {
	// Test getting a non-existent tag
	t.Run("NonExistentTag", func(t *testing.T) {
		_, err := getReleaseByTag(deviceProxyPublicRepo, "v999.999.999")
		if err == nil {
			t.Error("Expected error when getting non-existent tag")
		}
	})

	// Test getting a valid tag
	t.Run("ValidTag", func(t *testing.T) {
		release, err := getReleaseByTag(deviceProxyPublicRepo, "v0.1.7")
		if err != nil {
			t.Fatalf("Failed to get valid tag: %v", err)
		}

		if release.TagName != "v0.1.7" {
			t.Errorf("Expected tag name v0.1.7, got %s", release.TagName)
		}
	})
}

func TestVersionMatchingPriority(t *testing.T) {
	// This test verifies the version matching priority logic
	tempDir, err := os.MkdirTemp("", "gbox-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Temporarily override device proxy home for testing
	originalHome := config.GetDeviceProxyHome()
	defer func() {
		os.Setenv("DEVICE_PROXY_HOME", originalHome)
	}()

	os.Setenv("DEVICE_PROXY_HOME", tempDir)

	// Test scenario: Current version is "dev" - should download latest
	t.Run("DevVersionDownloadsLatest", func(t *testing.T) {
		// Remove any existing binary and cache
		binaryName := "gbox-device-proxy"
		if runtime.GOOS == "windows" {
			binaryName += ".exe"
		}
		binaryPath := filepath.Join(tempDir, binaryName)
		os.Remove(binaryPath)
		os.Remove(getVersionCachePath())

		// Download with "dev" version (current test environment)
		binaryPath, err := DownloadDeviceProxy()
		if err != nil {
			// Check if it's a rate limit error and skip the test
			if strings.Contains(err.Error(), "403") || strings.Contains(err.Error(), "rate limit") {
				t.Skip("GitHub API rate limit reached, skipping download test")
			}
			t.Fatalf("Failed to download device proxy: %v", err)
		}

		// Verify binary was downloaded
		if _, err := os.Stat(binaryPath); err != nil {
			t.Fatalf("Binary should exist: %v", err)
		}

		// Verify version cache was created
		versionInfo, err := loadVersionInfo()
		if err != nil {
			t.Fatalf("Failed to load version info: %v", err)
		}

		// Should have downloaded from latest release
		if versionInfo.TagName == "" {
			t.Error("Version info should have a tag name")
		}

		t.Logf("Downloaded from version: %s", versionInfo.TagName)
	})

	// Test scenario: Current version matches existing release
	t.Run("MatchingVersionDownloadsFromRelease", func(t *testing.T) {
		// Remove any existing binary and cache
		binaryName := "gbox-device-proxy"
		if runtime.GOOS == "windows" {
			binaryName += ".exe"
		}
		binaryPath := filepath.Join(tempDir, binaryName)
		os.Remove(binaryPath)
		os.Remove(getVersionCachePath())

		// Create a fake binary to simulate existing installation
		if err := os.WriteFile(binaryPath, []byte("fake binary"), 0755); err != nil {
			t.Fatalf("Failed to create fake binary: %v", err)
		}

		// Create version cache with a specific version that exists
		cacheInfo := &VersionInfo{
			TagName:    "v0.1.7",
			CommitID:   "test-commit",
			Downloaded: time.Now().Format(time.RFC3339),
		}
		if err := saveVersionInfo(cacheInfo); err != nil {
			t.Fatalf("Failed to save version info: %v", err)
		}

		// Use CheckAndDownloadDeviceProxy which should respect the cached version
		newBinaryPath, err := CheckAndDownloadDeviceProxy()
		if err != nil {
			// Check if it's a rate limit error and skip the test
			if strings.Contains(err.Error(), "403") || strings.Contains(err.Error(), "rate limit") {
				t.Skip("GitHub API rate limit reached, skipping download test")
			}
			t.Fatalf("Failed to check and download device proxy: %v", err)
		}

		// Should return existing binary path
		if newBinaryPath != binaryPath {
			t.Errorf("Expected binary path %s, got %s", binaryPath, newBinaryPath)
		}

		// Verify the binary wasn't replaced (content should still be "fake binary")
		content, err := os.ReadFile(binaryPath)
		if err != nil {
			t.Fatalf("Failed to read binary: %v", err)
		}

		if string(content) != "fake binary" {
			t.Error("Binary should not have been replaced")
		}

		t.Logf("Successfully used cached version without downloading")
	})
}
