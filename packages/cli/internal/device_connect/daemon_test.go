package device_connect

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestIsExecutableFile(t *testing.T) {
	// Test with a non-existent file
	if isExecutableFile("/non/existent/file") {
		t.Error("Non-existent file should not be considered executable")
	}

	// Test with a directory
	tempDir, err := os.MkdirTemp("", "gbox-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	if isExecutableFile(tempDir) {
		t.Error("Directory should not be considered executable")
	}

	// Test with a regular file
	tempFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(tempFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	if isExecutableFile(tempFile) {
		t.Error("Regular file should not be considered executable")
	}

	// Test with an executable file (on Unix systems)
	if runtime.GOOS != "windows" {
		execFile := filepath.Join(tempDir, "test.sh")
		if err := os.WriteFile(execFile, []byte("#!/bin/sh\necho test"), 0755); err != nil {
			t.Fatalf("Failed to create executable file: %v", err)
		}

		if !isExecutableFile(execFile) {
			t.Error("Executable file should be considered executable")
		}
	}
}

func TestFindBabelUmbrellaDir(t *testing.T) {
	// Test with current directory (should not find babel-umbrella)
	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}

	result := FindBabelUmbrellaDir(currentDir)

	// In the test environment, we might not have babel-umbrella
	// So we just test that the function doesn't crash
	if result != "" {
		t.Logf("Found babel-umbrella directory: %s", result)
	} else {
		t.Log("No babel-umbrella directory found (expected in test environment)")
	}
}

func TestSetupDeviceProxyEnvironment(t *testing.T) {
	apiKey := "test-api-key-12345"
	baseURL := "https://test.example.com"
	env := setupDeviceProxyEnvironment(apiKey, baseURL)

	// Check that required environment variables are set
	found := false
	for _, envVar := range env {
		if contains(envVar, "GBOX_API_KEY="+apiKey) {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected GBOX_API_KEY to be set in environment")
	}

	found = false
	for _, envVar := range env {
		if contains(envVar, "GBOX_PROVIDER_TYPE=org") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected GBOX_PROVIDER_TYPE to be set in environment")
	}

	found = false
	for _, envVar := range env {
		if contains(envVar, "ANDROID_DEVMGR_ENDPOINT=") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected ANDROID_DEVMGR_ENDPOINT to be set in environment")
	}

	found = false
	for _, envVar := range env {
		if contains(envVar, "GBOX_BASE_URL="+baseURL) {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected GBOX_BASE_URL to be set in environment")
	}

	t.Logf("Environment variables set: %d", len(env))
}

func TestNoProxyWorkaround(t *testing.T) {
	// Test case 1: Domain not in no_proxy list
	env := []string{
		"http_proxy=http://proxy.example.com:8080",
		"https_proxy=https://proxy.example.com:8080",
		"no_proxy=localhost,127.0.0.1",
		"PATH=/usr/bin",
	}

	result := handleNoProxyWorkaround(env, "https://gbox.ai")

	// Should keep proxy variables since gbox.ai is not in no_proxy
	proxyFound := false
	for _, envVar := range result {
		if strings.HasPrefix(envVar, "http_proxy=") {
			proxyFound = true
			break
		}
	}
	if !proxyFound {
		t.Error("Expected http_proxy to be kept when domain not in no_proxy")
	}

	// Test case 2: Domain in no_proxy list (localhost)
	result = handleNoProxyWorkaround(env, "https://localhost:8080")

	// Should remove proxy variables since localhost is in no_proxy
	proxyFound = false
	for _, envVar := range result {
		if strings.HasPrefix(envVar, "http_proxy=") {
			proxyFound = true
			break
		}
	}
	if proxyFound {
		t.Error("Expected http_proxy to be removed when domain in no_proxy")
	}

	// Test case 3: Wildcard pattern
	env = []string{
		"http_proxy=http://proxy.example.com:8080",
		"no_proxy=*.example.com,localhost",
		"PATH=/usr/bin",
	}

	result = handleNoProxyWorkaround(env, "https://api.example.com")

	// Should remove proxy variables since api.example.com matches *.example.com
	proxyFound = false
	for _, envVar := range result {
		if strings.HasPrefix(envVar, "http_proxy=") {
			proxyFound = true
			break
		}
	}
	if proxyFound {
		t.Error("Expected http_proxy to be removed when domain matches wildcard pattern")
	}

	// Test case 4: Check that only http_proxy and https_proxy are removed
	env = []string{
		"http_proxy=http://proxy.example.com:8080",
		"https_proxy=https://proxy.example.com:8080",
		"ftp_proxy=ftp://proxy.example.com:8080",
		"all_proxy=socks5://proxy.example.com:8080",
		"no_proxy=localhost",
		"PATH=/usr/bin",
	}

	result = handleNoProxyWorkaround(env, "https://localhost:8080")

	// Should only remove http_proxy and https_proxy, keep ftp_proxy and all_proxy
	ftpProxyFound := false
	allProxyFound := false
	for _, envVar := range result {
		if strings.HasPrefix(envVar, "ftp_proxy=") {
			ftpProxyFound = true
		}
		if strings.HasPrefix(envVar, "all_proxy=") {
			allProxyFound = true
		}
	}
	if !ftpProxyFound {
		t.Error("Expected ftp_proxy to be kept")
	}
	if !allProxyFound {
		t.Error("Expected all_proxy to be kept")
	}
}
