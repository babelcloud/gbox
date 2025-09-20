package webm

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestNewWebMMuxer(t *testing.T) {
	var buf bytes.Buffer
	muxer := NewWebMMuxer(&buf)
	
	if muxer == nil {
		t.Fatal("NewWebMMuxer returned nil")
	}
	
	if muxer.writer != &buf {
		t.Error("Writer not set correctly")
	}
	
	if muxer.initialized {
		t.Error("Muxer should not be initialized initially")
	}
	
	if muxer.closed {
		t.Error("Muxer should not be closed initially")
	}
	
	if muxer.frameCount != 0 {
		t.Error("Frame count should start at 0")
	}
	
	if muxer.trackEntry.CodecID != "A_OPUS" {
		t.Error("Default codec should be A_OPUS")
	}
}

func TestWriteHeader(t *testing.T) {
	var buf bytes.Buffer
	muxer := NewWebMMuxer(&buf)
	
	err := muxer.WriteHeader()
	if err != nil {
		t.Errorf("WriteHeader failed: %v", err)
	}
	
	// WriteHeader should be a no-op, so no data should be written yet
	if buf.Len() != 0 {
		t.Error("WriteHeader should not write data immediately")
	}
}

func TestWriteOpusFrame(t *testing.T) {
	var buf bytes.Buffer
	muxer := NewWebMMuxer(&buf)
	
	// Test writing a frame
	opusData := []byte{0x78, 0x9c, 0x95, 0x96, 0x01, 0x00, 0x00, 0x00, 0xff, 0xff}
	timestamp := 20 * time.Millisecond
	
	err := muxer.WriteOpusFrame(opusData, timestamp)
	if err != nil {
		t.Errorf("WriteOpusFrame failed: %v", err)
	}
	
	// Check that muxer was initialized
	if !muxer.initialized {
		t.Error("Muxer should be initialized after first frame")
	}
	
	// Check frame count
	if muxer.frameCount != 1 {
		t.Errorf("Expected frame count 1, got %d", muxer.frameCount)
	}
	
	// Check timestamp
	if muxer.audioTimestamp != timestamp {
		t.Errorf("Expected timestamp %v, got %v", timestamp, muxer.audioTimestamp)
	}
	
	// Check that some data was written
	if buf.Len() == 0 {
		t.Error("Expected some data to be written")
	}
}

func TestWriteOpusFrameMultiple(t *testing.T) {
	var buf bytes.Buffer
	muxer := NewWebMMuxer(&buf)
	
	// Write multiple frames
	frames := [][]byte{
		{0x78, 0x9c, 0x95, 0x96, 0x01, 0x00, 0x00, 0x00, 0xff, 0xff},
		{0x78, 0x9c, 0x95, 0x96, 0x01, 0x00, 0x00, 0x00, 0xff, 0xfe},
		{0x78, 0x9c, 0x95, 0x96, 0x01, 0x00, 0x00, 0x00, 0xff, 0xfd},
	}
	
	for i, frame := range frames {
		timestamp := time.Duration(i*20) * time.Millisecond
		err := muxer.WriteOpusFrame(frame, timestamp)
		if err != nil {
			t.Errorf("WriteOpusFrame %d failed: %v", i, err)
		}
	}
	
	if muxer.frameCount != uint64(len(frames)) {
		t.Errorf("Expected frame count %d, got %d", len(frames), muxer.frameCount)
	}
	
	if buf.Len() == 0 {
		t.Error("Expected some data to be written")
	}
}

func TestWriteOpusFrameAfterClose(t *testing.T) {
	var buf bytes.Buffer
	muxer := NewWebMMuxer(&buf)
	
	// Close the muxer
	err := muxer.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
	
	// Try to write after close
	opusData := []byte{0x78, 0x9c, 0x95, 0x96, 0x01, 0x00, 0x00, 0x00, 0xff, 0xff}
	timestamp := 20 * time.Millisecond
	
	err = muxer.WriteOpusFrame(opusData, timestamp)
	if err != io.ErrClosedPipe {
		t.Errorf("Expected ErrClosedPipe, got %v", err)
	}
}

func TestClose(t *testing.T) {
	var buf bytes.Buffer
	muxer := NewWebMMuxer(&buf)
	
	// Write a frame first
	opusData := []byte{0x78, 0x9c, 0x95, 0x96, 0x01, 0x00, 0x00, 0x00, 0xff, 0xff}
	timestamp := 20 * time.Millisecond
	
	err := muxer.WriteOpusFrame(opusData, timestamp)
	if err != nil {
		t.Errorf("WriteOpusFrame failed: %v", err)
	}
	
	// Close the muxer
	err = muxer.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
	
	if !muxer.closed {
		t.Error("Muxer should be marked as closed")
	}
	
	// Close again should not error
	err = muxer.Close()
	if err != nil {
		t.Errorf("Second close should not error, got %v", err)
	}
}

func TestGetStats(t *testing.T) {
	var buf bytes.Buffer
	muxer := NewWebMMuxer(&buf)
	
	// Get stats before any operations
	stats := muxer.GetStats()
	
	if stats["frames_written"] != uint64(0) {
		t.Error("Initial frame count should be 0")
	}
	
	if stats["closed"] != false {
		t.Error("Initial closed state should be false")
	}
	
	// Write a frame
	opusData := []byte{0x78, 0x9c, 0x95, 0x96, 0x01, 0x00, 0x00, 0x00, 0xff, 0xff}
	timestamp := 20 * time.Millisecond
	
	err := muxer.WriteOpusFrame(opusData, timestamp)
	if err != nil {
		t.Errorf("WriteOpusFrame failed: %v", err)
	}
	
	// Get stats after writing
	stats = muxer.GetStats()
	
	if stats["frames_written"] != uint64(1) {
		t.Error("Frame count should be 1 after writing one frame")
	}
	
	if stats["closed"] != false {
		t.Error("Closed state should be false after writing")
	}
	
	// Close and get stats
	muxer.Close()
	stats = muxer.GetStats()
	
	if stats["closed"] != true {
		t.Error("Closed state should be true after closing")
	}
}

func TestConcurrentWrite(t *testing.T) {
	var buf bytes.Buffer
	muxer := NewWebMMuxer(&buf)
	
	// Test concurrent writes
	done := make(chan bool, 10)
	
	for i := 0; i < 10; i++ {
		go func(i int) {
			defer func() { done <- true }()
			
			opusData := []byte{0x78, 0x9c, 0x95, 0x96, 0x01, 0x00, 0x00, 0x00, 0xff, 0xff}
			timestamp := time.Duration(i*20) * time.Millisecond
			
			err := muxer.WriteOpusFrame(opusData, timestamp)
			if err != nil {
				t.Errorf("Concurrent WriteOpusFrame %d failed: %v", i, err)
			}
		}(i)
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
	
	// Check that all frames were written
	if muxer.frameCount != 10 {
		t.Errorf("Expected frame count 10, got %d", muxer.frameCount)
	}
}

func TestPanicRecovery(t *testing.T) {
	// Create a writer that panics
	panicWriter := &panicWriter{}
	muxer := NewWebMMuxer(panicWriter)
	
	// This should not panic due to our recovery mechanism
	opusData := []byte{0x78, 0x9c, 0x95, 0x96, 0x01, 0x00, 0x00, 0x00, 0xff, 0xff}
	timestamp := 20 * time.Millisecond
	
	err := muxer.WriteOpusFrame(opusData, timestamp)
	if err != nil {
		t.Errorf("WriteOpusFrame should handle panic gracefully, got: %v", err)
	}
	
	// Muxer should be marked as closed after panic
	if !muxer.closed {
		t.Error("Muxer should be closed after panic")
	}
}

// panicWriter is a writer that panics on write
type panicWriter struct{}

func (w *panicWriter) Write(p []byte) (n int, err error) {
	panic("test panic")
}

func (w *panicWriter) Close() error {
	return nil
}

func TestWriteCloserWrapper(t *testing.T) {
	var buf bytes.Buffer
	wrapper := &writeCloserWrapper{Writer: &buf}
	
	// Test write
	testData := []byte("test data")
	n, err := wrapper.Write(testData)
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}
	
	if n != len(testData) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(testData), n)
	}
	
	if !bytes.Equal(buf.Bytes(), testData) {
		t.Error("Data not written correctly")
	}
	
	// Test close (should not error)
	err = wrapper.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func BenchmarkWriteOpusFrame(b *testing.B) {
	var buf bytes.Buffer
	muxer := NewWebMMuxer(&buf)
	
	opusData := []byte{0x78, 0x9c, 0x95, 0x96, 0x01, 0x00, 0x00, 0x00, 0xff, 0xff}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		timestamp := time.Duration(i*20) * time.Millisecond
		err := muxer.WriteOpusFrame(opusData, timestamp)
		if err != nil {
			b.Fatalf("WriteOpusFrame failed: %v", err)
		}
	}
}

func BenchmarkConcurrentWriteOpusFrame(b *testing.B) {
	var buf bytes.Buffer
	muxer := NewWebMMuxer(&buf)
	
	opusData := []byte{0x78, 0x9c, 0x95, 0x96, 0x01, 0x00, 0x00, 0x00, 0xff, 0xff}
	
	b.ResetTimer()
	
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			timestamp := time.Duration(i*20) * time.Millisecond
			err := muxer.WriteOpusFrame(opusData, timestamp)
			if err != nil {
				b.Fatalf("WriteOpusFrame failed: %v", err)
			}
			i++
		}
	})
}

func TestMain(m *testing.M) {
	// Set up logging for tests
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError, // Only show errors in tests
	}))
	slog.SetDefault(logger)
	
	os.Exit(m.Run())
}