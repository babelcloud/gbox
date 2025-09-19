package handlers

import (
	"io/fs"
	"net/http/httptest"
	"testing"
	"time"
)

// MockServerService for testing
type MockServerService struct {
	bridges map[string]interface{}
}

// Status and info
func (m *MockServerService) IsRunning() bool          { return true }
func (m *MockServerService) GetPort() int             { return 8080 }
func (m *MockServerService) GetUptime() time.Duration { return time.Hour }
func (m *MockServerService) GetBuildID() string       { return "test-build" }
func (m *MockServerService) GetVersion() string       { return "1.0.0" }

// Services status
func (m *MockServerService) IsADBExposeRunning() bool { return false }

// Bridge management
func (m *MockServerService) ListBridges() []string {
	var result []string
	for k := range m.bridges {
		result = append(result, k)
	}
	return result
}

func (m *MockServerService) GetBridge(deviceSerial string) (Bridge, bool) {
	_, exists := m.bridges[deviceSerial]
	if !exists {
		return nil, false
	}
	// Convert to Bridge interface - this is a mock so we'll return a simple implementation
	return &MockBridge{}, true
}

func (m *MockServerService) CreateBridge(deviceSerial string) error {
	// Mock implementation - create a simple bridge
	bridge := make(map[string]interface{})
	m.bridges[deviceSerial] = bridge
	return nil
}

func (m *MockServerService) RemoveBridge(deviceSerial string) {
	delete(m.bridges, deviceSerial)
}

// Static file serving
func (m *MockServerService) GetStaticFS() fs.FS             { return nil }
func (m *MockServerService) FindLiveViewStaticPath() string { return "/static/live-view" }
func (m *MockServerService) FindStaticPath() string         { return "/static" }

// Server lifecycle
func (m *MockServerService) Stop() error { return nil }

// ADB Expose methods
func (m *MockServerService) StartPortForward(boxID string, localPorts, remotePorts []int) error {
	return nil
}
func (m *MockServerService) StopPortForward(boxID string) error { return nil }
func (m *MockServerService) ListPortForwards() interface{}      { return nil }

func NewMockServerService() *MockServerService {
	return &MockServerService{
		bridges: make(map[string]interface{}),
	}
}

func (m *MockServerService) AddBridge(deviceSerial string, bridge interface{}) {
	m.bridges[deviceSerial] = bridge
}

// MockBridge implements the Bridge interface for testing
type MockBridge struct{}

func (m *MockBridge) HandleTouchEvent(msg map[string]interface{})  {}
func (m *MockBridge) HandleKeyEvent(msg map[string]interface{})    {}
func (m *MockBridge) HandleScrollEvent(msg map[string]interface{}) {}

// TestStreamingHandlersCreation tests the creation of streaming handlers
func TestStreamingHandlersCreation(t *testing.T) {
	handlers := NewStreamingHandlers()
	if handlers == nil {
		t.Fatal("Expected handlers to be created")
	}

	// Test setting server service
	mockService := NewMockServerService()
	handlers.SetServerService(mockService)
	if handlers.serverService == nil {
		t.Error("Expected server service to be set")
	}

	// Test setting path prefix
	handlers.SetPathPrefix("/api")
	if handlers.pathPrefix != "/api" {
		t.Error("Expected path prefix to be set")
	}
}

// TestUtilityFunctions tests utility functions
func TestUtilityFunctions(t *testing.T) {
	// Test isValidDeviceSerial
	validSerials := []string{"device123", "abc-def_123", "test.device", "DEVICE-456", "device.789", "device_abc"}
	invalidSerials := []string{"", "a", "device with spaces", "device@invalid", "device#invalid", "device@123", "device/123"}

	for _, serial := range validSerials {
		if !isValidDeviceSerial(serial) {
			t.Errorf("Expected %s to be valid", serial)
		}
	}

	for _, serial := range invalidSerials {
		if isValidDeviceSerial(serial) {
			t.Errorf("Expected %s to be invalid", serial)
		}
	}

}

// TestHandleVideoStream tests video stream handling
func TestHandleVideoStream(t *testing.T) {
	handlers := NewStreamingHandlers()

	tests := []struct {
		path           string
		expectedStatus int
		description    string
	}{
		{"/stream/video/", 400, "Missing device serial"},
		{"/stream/video/device123?mode=invalid", 400, "Invalid mode"},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			req := httptest.NewRequest("GET", test.path, nil)
			w := httptest.NewRecorder()

			handlers.HandleVideoStream(w, req)

			if w.Code != test.expectedStatus {
				t.Errorf("Expected status %d, got %d", test.expectedStatus, w.Code)
			}
		})
	}

	// Note: Tests that require actual device connection are skipped in unit tests
	// They should be run in integration tests with real devices
}

// TestHandleAudioStream tests audio stream handling
func TestHandleAudioStream(t *testing.T) {
	handlers := NewStreamingHandlers()

	tests := []struct {
		path           string
		expectedStatus int
		description    string
	}{
		{"/stream/audio/", 400, "Missing device serial"},
		{"/stream/audio/device123?codec=invalid", 400, "Invalid codec"},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			req := httptest.NewRequest("GET", test.path, nil)
			w := httptest.NewRecorder()

			handlers.HandleAudioStream(w, req)

			if w.Code != test.expectedStatus {
				t.Errorf("Expected status %d, got %d", test.expectedStatus, w.Code)
			}
		})
	}

	// Note: Tests that require actual device connection are skipped in unit tests
	// They should be run in integration tests with real devices
}

// TestHandleControlWebSocket tests control WebSocket handling
func TestHandleControlWebSocket(t *testing.T) {
	handlers := NewStreamingHandlers()

	tests := []struct {
		path           string
		expectedStatus int
		description    string
	}{
		{"/stream/control/device123", 400, "Valid control WebSocket request (will fail without upgrade)"},
		{"/stream/control/", 400, "Missing device serial"},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			req := httptest.NewRequest("GET", test.path, nil)
			w := httptest.NewRecorder()

			handlers.HandleControlWebSocket(w, req)

			if w.Code != test.expectedStatus {
				t.Errorf("Expected status %d, got %d", test.expectedStatus, w.Code)
			}
		})
	}
}

// TestHandleStreamInfo tests stream info handling
func TestHandleStreamInfo(t *testing.T) {
	handlers := NewStreamingHandlers()

	tests := []struct {
		path           string
		expectedStatus int
		description    string
	}{
		{"/stream/info?device=device123", 200, "Valid stream info request"},
		{"/stream/info", 400, "Missing device parameter"},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			req := httptest.NewRequest("GET", test.path, nil)
			w := httptest.NewRecorder()

			handlers.HandleStreamInfo(w, req)

			if w.Code != test.expectedStatus {
				t.Errorf("Expected status %d, got %d", test.expectedStatus, w.Code)
			}
		})
	}
}

// TestHandleStreamConnect tests stream connect handling
func TestHandleStreamConnect(t *testing.T) {
	handlers := NewStreamingHandlers()

	tests := []struct {
		path           string
		method         string
		expectedStatus int
		description    string
	}{
		{"/api/stream/device123/connect", "POST", 200, "Valid connect request"},
		{"/api/stream/device123/disconnect", "POST", 200, "Valid disconnect request"},
		{"/api/stream/device123/connect", "GET", 405, "Invalid method for connect"},
		{"/api/stream/device123/invalid", "POST", 400, "Invalid action"},
		{"/api/stream/", "POST", 400, "Invalid URL format"},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			req := httptest.NewRequest(test.method, test.path, nil)
			w := httptest.NewRecorder()

			handlers.HandleStreamConnect(w, req)

			if w.Code != test.expectedStatus {
				t.Errorf("Expected status %d, got %d", test.expectedStatus, w.Code)
			}
		})
	}
}

// TestBuildURL tests URL building
func TestBuildURL(t *testing.T) {
	handlers := NewStreamingHandlers()

	// Test without prefix
	url := handlers.buildURL("/stream/video/device123")
	if url != "/stream/video/device123" {
		t.Errorf("Expected /stream/video/device123, got %s", url)
	}

	// Test with prefix
	handlers.SetPathPrefix("/api")
	url = handlers.buildURL("/stream/video/device123")
	if url != "/api/stream/video/device123" {
		t.Errorf("Expected /api/stream/video/device123, got %s", url)
	}
}

// TestWebSocketUpgrader tests WebSocket upgrader configuration
func TestWebSocketUpgrader(t *testing.T) {
	// Test that upgrader is configured correctly
	if controlUpgrader.CheckOrigin == nil {
		t.Error("Expected CheckOrigin to be set")
	}

	// Test CheckOrigin function
	req := httptest.NewRequest("GET", "/stream/control/device123", nil)
	if !controlUpgrader.CheckOrigin(req) {
		t.Error("Expected CheckOrigin to return true")
	}
}

// Benchmark tests for performance
func BenchmarkHandleVideoStream(b *testing.B) {
	handlers := NewStreamingHandlers()
	req := httptest.NewRequest("GET", "/stream/video/device123?mode=h264", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handlers.HandleVideoStream(w, req)
	}
}

func BenchmarkHandleAudioStream(b *testing.B) {
	handlers := NewStreamingHandlers()
	req := httptest.NewRequest("GET", "/stream/audio/device123?codec=opus", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handlers.HandleAudioStream(w, req)
	}
}

func BenchmarkUtilityFunctions(b *testing.B) {
	serial := "device123"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		isValidDeviceSerial(serial)
	}
}
