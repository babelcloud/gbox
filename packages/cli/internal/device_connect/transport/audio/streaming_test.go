package audio

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/core"
)

// MockAudioSource implements core.Source for testing
type MockAudioSource struct {
	audioChannel chan core.AudioSample
	subscribers  map[string]chan core.AudioSample
}

func NewMockAudioSource() *MockAudioSource {
	return &MockAudioSource{
		audioChannel: make(chan core.AudioSample, 100),
		subscribers:  make(map[string]chan core.AudioSample),
	}
}

func (m *MockAudioSource) SubscribeAudio(subscriberID string, bufferSize int) <-chan core.AudioSample {
	ch := make(chan core.AudioSample, bufferSize)
	m.subscribers[subscriberID] = ch
	return ch
}

func (m *MockAudioSource) UnsubscribeAudio(subscriberID string) {
	if ch, exists := m.subscribers[subscriberID]; exists {
		close(ch)
		delete(m.subscribers, subscriberID)
	}
}

func (m *MockAudioSource) GetConnectionInfo() map[string]interface{} {
	return map[string]interface{}{
		"device_serial": "test_device",
		"status":        "connected",
	}
}

func (m *MockAudioSource) SendAudioSample(sample core.AudioSample) {
	for _, ch := range m.subscribers {
		select {
		case ch <- sample:
		default:
			// Channel is full, skip this sample
		}
	}
}

func (m *MockAudioSource) Close() {
	for _, ch := range m.subscribers {
		close(ch)
	}
}

// MockWriter implements io.Writer and tracks writes
type MockWriter struct {
	bytes.Buffer
	writeErrors []error
	errorIndex  int
}

func NewMockWriter() *MockWriter {
	return &MockWriter{}
}

func (m *MockWriter) SetWriteErrors(errors []error) {
	m.writeErrors = errors
	m.errorIndex = 0
}

func (m *MockWriter) Write(p []byte) (n int, err error) {
	// Simulate write errors if configured
	if m.errorIndex < len(m.writeErrors) {
		err = m.writeErrors[m.errorIndex]
		m.errorIndex++
		if err != nil {
			return 0, err
		}
	}

	return m.Buffer.Write(p)
}

// TestWebMMuxer tests WebM muxer functionality
func TestWebMMuxer(t *testing.T) {
	t.Run("WriteHeader", func(t *testing.T) {
		writer := NewMockWriter()
		muxer := NewWebMMuxer(writer)

		err := muxer.WriteHeader()
		if err != nil {
			t.Fatalf("WriteHeader failed: %v", err)
		}

		if !muxer.initialized {
			t.Error("Muxer should be initialized after WriteHeader")
		}

		// Check that some data was written
		if writer.Len() == 0 {
			t.Error("No data written to buffer")
		}
	})

	t.Run("WriteOpusFrame", func(t *testing.T) {
		writer := NewMockWriter()
		muxer := NewWebMMuxer(writer)

		// Initialize muxer
		if err := muxer.WriteHeader(); err != nil {
			t.Fatalf("WriteHeader failed: %v", err)
		}

		// Write a test frame
		testData := []byte("test opus data")
		err := muxer.WriteOpusFrame(testData, 20*time.Millisecond)
		if err != nil {
			t.Fatalf("WriteOpusFrame failed: %v", err)
		}

		stats := muxer.GetStats()
		if stats["frames_written"] != uint64(1) {
			t.Errorf("Expected 1 frame written, got %v", stats["frames_written"])
		}
	})

	t.Run("WriteOpusFrameWithoutHeader", func(t *testing.T) {
		writer := NewMockWriter()
		muxer := NewWebMMuxer(writer)

		// Try to write frame without initializing
		testData := []byte("test opus data")
		err := muxer.WriteOpusFrame(testData, 20*time.Millisecond)
		if err != nil {
			t.Fatalf("WriteOpusFrame should auto-initialize: %v", err)
		}

		if !muxer.initialized {
			t.Error("Muxer should be auto-initialized")
		}
	})

	t.Run("Close", func(t *testing.T) {
		writer := NewMockWriter()
		muxer := NewWebMMuxer(writer)

		err := muxer.Close()
		if err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	})
}

// TestWriterCloser tests the writer wrapper
func TestWriterCloser(t *testing.T) {
	t.Run("NormalWrite", func(t *testing.T) {
		writer := NewMockWriter()
		swc := &writerCloser{
			writer: writer,
			logger: slog.Default(),
			closed: false,
		}

		testData := []byte("test data")
		n, err := swc.Write(testData)
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}

		if n != len(testData) {
			t.Errorf("Expected to write %d bytes, wrote %d", len(testData), n)
		}
	})

	t.Run("WriteAfterClose", func(t *testing.T) {
		writer := NewMockWriter()
		swc := &writerCloser{
			writer: writer,
			logger: slog.Default(),
			closed: true,
		}

		testData := []byte("test data")
		_, err := swc.Write(testData)
		if err != io.ErrClosedPipe {
			t.Errorf("Expected ErrClosedPipe, got %v", err)
		}
	})

	t.Run("WriteError", func(t *testing.T) {
		writer := NewMockWriter()
		writer.SetWriteErrors([]error{io.ErrShortWrite})

		// Create a simple logger to avoid nil pointer panic
		logger := slog.Default()

		swc := &writerCloser{
			writer: writer,
			logger: logger,
			closed: false,
		}

		testData := []byte("test data")
		_, err := swc.Write(testData)
		if err != io.ErrShortWrite {
			t.Errorf("Expected ErrShortWrite, got %v", err)
		}

		if !swc.closed {
			t.Error("Writer should be marked as closed after error")
		}
	})
}

// TestStreamWebM tests the WebM streaming functionality
func TestStreamWebM(t *testing.T) {
	t.Run("DeviceNotFound", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/stream/audio/test_device?codec=opus&format=webm", nil)
		req = req.WithContext(context.Background())
		rr := httptest.NewRecorder()

		service := NewAudioStreamingService()
		err := service.StreamWebM("test_device", rr, req)

		if err == nil {
			t.Error("Expected error for missing device, got nil")
		}

		if !strings.Contains(err.Error(), "device source not found") {
			t.Errorf("Expected 'device source not found' error, got: %v", err)
		}
	})
}

// TestConnectionHealthMonitor tests the connection health monitoring
func TestConnectionHealthMonitor(t *testing.T) {
	t.Run("HealthyConnection", func(t *testing.T) {
		rr := httptest.NewRecorder()

		monitor := &ConnectionHealthMonitor{
			writer:   rr,
			flusher:  rr,
			logger:   slog.Default(),
			interval: 100 * time.Millisecond,
		}

		monitor.Start()
		defer monitor.Stop()

		// Wait a bit and check health
		time.Sleep(200 * time.Millisecond)

		if !monitor.IsHealthy() {
			t.Error("Connection should be healthy")
		}
	})

	t.Run("StartStop", func(t *testing.T) {
		rr := httptest.NewRecorder()

		monitor := &ConnectionHealthMonitor{
			writer:   rr,
			flusher:  rr,
			logger:   slog.Default(),
			interval: 100 * time.Millisecond,
		}

		monitor.Start()
		monitor.Stop()

		// Stop should not panic
		monitor.Stop()
	})
}

// Benchmark tests
func BenchmarkWebMMuxerWriteOpusFrame(b *testing.B) {
	writer := NewMockWriter()
	muxer := NewWebMMuxer(writer)

	if err := muxer.WriteHeader(); err != nil {
		b.Fatalf("WriteHeader failed: %v", err)
	}

	testData := []byte("test opus data")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		timestamp := time.Duration(i) * 20 * time.Millisecond
		muxer.WriteOpusFrame(testData, timestamp)
	}
}
