package device_connect

import (
	"os"
	"path/filepath"
	"runtime"
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
	env := setupDeviceProxyEnvironment(apiKey)

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

	t.Logf("Environment variables set: %d", len(env))
}
