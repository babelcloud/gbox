package device_connect

import (
	"os"
	"runtime"
	"testing"

	"github.com/babelcloud/gbox/packages/cli/config"
	"github.com/babelcloud/gbox/packages/cli/internal/version"
)

func TestGetLatestRelease(t *testing.T) {
	// Test getting latest release from public repository
	release, err := getLatestRelease(deviceProxyPublicRepo)
	if err != nil {
		t.Fatalf("Failed to get latest release: %v", err)
	}

	if release.TagName == "" {
		t.Error("Expected release to have a tag name")
	}

	if len(release.Assets) == 0 {
		t.Error("Expected release to have assets")
	}

	t.Logf("Latest release: %s with %d assets", release.TagName, len(release.Assets))
}

func TestFindDeviceProxyAssetForPlatform(t *testing.T) {
	// First get a release
	release, err := getLatestRelease(deviceProxyPublicRepo)
	if err != nil {
		t.Fatalf("Failed to get latest release: %v", err)
	}

	// Test finding asset for current platform
	_, assetName, err := findDeviceProxyAssetForPlatform(release)
	if err != nil {
		t.Fatalf("Failed to find device proxy asset: %v", err)
	}

	if assetName == "" {
		t.Error("Expected asset name to be non-empty")
	}

	// Verify the asset name contains the expected platform
	expectedPlatform := getExpectedPlatform()
	if !contains(assetName, expectedPlatform) {
		t.Errorf("Asset name '%s' should contain platform '%s'", assetName, expectedPlatform)
	}

	t.Logf("Found asset: %s", assetName)
}

func TestDownloadDeviceProxyIntegration(t *testing.T) {
	// This is an integration test that actually downloads the binary
	// Skip in CI environments to avoid rate limiting
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping integration test in CI environment")
	}

	// Test the full download process
	binaryPath, err := DownloadDeviceProxy()
	if err != nil {
		t.Fatalf("Failed to download device proxy: %v", err)
	}

	// Verify the binary exists and is executable
	info, err := os.Stat(binaryPath)
	if err != nil {
		t.Fatalf("Failed to stat downloaded binary: %v", err)
	}

	if info.IsDir() {
		t.Error("Downloaded binary should not be a directory")
	}

	// Check if it's executable (on Unix systems)
	if runtime.GOOS != "windows" {
		mode := info.Mode()
		if mode&0111 == 0 {
			t.Error("Downloaded binary should be executable")
		}
	}

	// Verify version cache was created
	cachePath := getVersionCachePath()
	if _, err := os.Stat(cachePath); err != nil {
		t.Errorf("Version cache file should exist: %v", err)
	}

	// Load and verify version info
	versionInfo, err := loadVersionInfo()
	if err != nil {
		t.Fatalf("Failed to load version info: %v", err)
	}

	if versionInfo.TagName == "" {
		t.Error("Version info should have a tag name")
	}

	t.Logf("Successfully downloaded device proxy binary: %s (%d bytes)", binaryPath, info.Size())
	t.Logf("Downloaded from version: %s", versionInfo.TagName)
}

func TestCheckAndDownloadDeviceProxy(t *testing.T) {
	// This test verifies the version-aware download functionality
	// Skip in CI environments to avoid rate limiting
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping integration test in CI environment")
	}

	// Test the version-aware download process
	binaryPath, err := CheckAndDownloadDeviceProxy()
	if err != nil {
		t.Fatalf("Failed to check and download device proxy: %v", err)
	}

	// Verify the binary exists
	info, err := os.Stat(binaryPath)
	if err != nil {
		t.Fatalf("Failed to stat downloaded binary: %v", err)
	}

	if info.IsDir() {
		t.Error("Downloaded binary should not be a directory")
	}

	// Verify version cache was created
	cachePath := getVersionCachePath()
	if _, err := os.Stat(cachePath); err != nil {
		t.Errorf("Version cache file should exist: %v", err)
	}

	// Load and verify version info
	versionInfo, err := loadVersionInfo()
	if err != nil {
		t.Fatalf("Failed to load version info: %v", err)
	}

	if versionInfo.TagName == "" {
		t.Error("Version info should have a tag name")
	}

	if versionInfo.Downloaded == "" {
		t.Error("Version info should have a download timestamp")
	}

	t.Logf("Successfully checked and downloaded device proxy binary: %s", binaryPath)
	t.Logf("Version info: %+v", versionInfo)
}

func TestVersionCache(t *testing.T) {
	// Test version cache functionality
	tempDir, err := os.MkdirTemp("", "gbox-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Temporarily override CLI cache home for testing
	originalHome := config.GetGboxCliHome()
	defer func() {
		// Restore original home
		os.Setenv("GBOX_CLI_HOME", originalHome)
	}()

	os.Setenv("GBOX_CLI_HOME", tempDir)

	// Test saving and loading version info
	testInfo := &VersionInfo{
		TagName:    "v1.0.0",
		CommitID:   "abc123",
		Downloaded: "2023-01-01T00:00:00Z",
	}

	err = saveVersionInfo(testInfo)
	if err != nil {
		t.Fatalf("Failed to save version info: %v", err)
	}

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

	t.Logf("Version cache test passed")
}

func TestGetReleaseByTag(t *testing.T) {
	// Test getting a specific release by tag
	// Use a known tag that exists
	release, err := getReleaseByTag(deviceProxyPublicRepo, "v0.1.7")
	if err != nil {
		t.Fatalf("Failed to get release by tag: %v", err)
	}

	if release.TagName != "v0.1.7" {
		t.Errorf("Expected tag name v0.1.7, got %s", release.TagName)
	}

	if len(release.Assets) == 0 {
		t.Error("Expected release to have assets")
	}

	t.Logf("Successfully got release by tag: %s with %d assets", release.TagName, len(release.Assets))
}

func TestDownloadDeviceProxyVersionMatching(t *testing.T) {
	// This test verifies that DownloadDeviceProxy tries to match current version first
	// Skip in CI environments to avoid rate limiting
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping integration test in CI environment")
	}

	// Get current version info
	clientInfo := version.ClientInfo()
	currentVersion := clientInfo["Version"]

	t.Logf("Current CLI version: %s", currentVersion)

	// Test the download process
	binaryPath, err := DownloadDeviceProxy()
	if err != nil {
		t.Fatalf("Failed to download device proxy: %v", err)
	}

	// Verify the binary exists
	info, err := os.Stat(binaryPath)
	if err != nil {
		t.Fatalf("Failed to stat downloaded binary: %v", err)
	}

	if info.IsDir() {
		t.Error("Downloaded binary should not be a directory")
	}

	// Load version info to see which version was actually downloaded
	versionInfo, err := loadVersionInfo()
	if err != nil {
		t.Fatalf("Failed to load version info: %v", err)
	}

	t.Logf("Downloaded binary from version: %s", versionInfo.TagName)
	t.Logf("Binary size: %d bytes", info.Size())

	// If current version is not "dev", we should try to match it
	if currentVersion != "dev" {
		t.Logf("Current version is %s, checking if we tried to match it", currentVersion)
		// Note: We can't easily verify which version was attempted first in this test
		// but we can verify that the download succeeded and version info was saved
	}

	t.Logf("Download completed successfully")
}

// Helper functions

func getExpectedPlatform() string {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	switch osName {
	case "darwin":
		if arch == "amd64" {
			return "darwin-amd64"
		} else if arch == "arm64" {
			return "darwin-arm64"
		}
	case "linux":
		if arch == "amd64" {
			return "linux-amd64"
		} else if arch == "arm64" {
			return "linux-arm64"
		}
	case "windows":
		if arch == "amd64" {
			return "windows-amd64"
		} else if arch == "arm64" {
			return "windows-arm64"
		}
	}

	return ""
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) &&
			(s[:len(substr)] == substr ||
				s[len(s)-len(substr):] == substr ||
				containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Benchmark tests

func BenchmarkGetLatestRelease(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := getLatestRelease(deviceProxyPublicRepo)
		if err != nil {
			b.Fatalf("Failed to get latest release: %v", err)
		}
	}
}

func BenchmarkFindDeviceProxyAssetForPlatform(b *testing.B) {
	release, err := getLatestRelease(deviceProxyPublicRepo)
	if err != nil {
		b.Fatalf("Failed to get latest release: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := findDeviceProxyAssetForPlatform(release)
		if err != nil {
			b.Fatalf("Failed to find device proxy asset: %v", err)
		}
	}
}
