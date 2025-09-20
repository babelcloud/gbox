package webm

import (
	"io"
	"log/slog"
	"sync"
	"time"
)

type WebMMuxer struct {
	writer         io.Writer
	logger         *slog.Logger
	initialized    bool
	frameCount     uint64
	audioTimestamp time.Duration
	closed         bool
	mu             sync.RWMutex
	blockWriter BlockWriteCloser
	trackEntry  TrackEntry
}

// NewWebMMuxer creates a new panic-safe WebM muxer
func NewWebMMuxer(writer io.Writer) *WebMMuxer {
	return &WebMMuxer{
		writer: writer,
		logger: slog.With("component", "webm_muxer"),
		trackEntry: TrackEntry{
			TrackNumber: 1,
			TrackUID:    12345,
			CodecID:     "A_OPUS",
			TrackType:   2, // Audio track
			Audio: &Audio{
				SamplingFrequency: 48000.0,
				Channels:          2,
			},
		},
	}
}

// WriteHeader writes the WebM header (no-op for this implementation)
func (m *WebMMuxer) WriteHeader() error {
	// Header is written automatically when the first frame is written
	return nil
}

// WriteOpusFrame writes an Opus frame to the WebM container with panic protection
func (m *WebMMuxer) WriteOpusFrame(opusData []byte, timestamp time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return io.ErrClosedPipe
	}

	// Initialize on first write
	if !m.initialized {
		if err := m.initialize(); err != nil {
			return err
		}
	}

	// Write frame with panic protection
	func() {
		defer func() {
			if r := recover(); r != nil {
				m.logger.Error("Panic in WriteOpusFrame", "panic", r)
				m.closed = true
			}
		}()

		// BlockWriteCloser.Write signature: Write(keyframe bool, timestamp int64, data []byte) (int, error)
		_, err := m.blockWriter.Write(true, int64(timestamp.Nanoseconds()/1000000), opusData)
		if err != nil {
			m.logger.Error("Failed to write Opus frame", "error", err)
			m.closed = true
			return
		}

		m.frameCount++
		m.audioTimestamp = timestamp
	}()

	return nil
}

// GetStats returns the current statistics
func (m *WebMMuxer) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"frames_written": m.frameCount,
		"closed":         m.closed,
		"audio_duration": m.audioTimestamp,
	}
}

// Close closes the WebM muxer with panic protection
func (m *WebMMuxer) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}

	m.closed = true

	// Close with panic protection
	func() {
		defer func() {
			if r := recover(); r != nil {
				m.logger.Error("Panic in Close", "panic", r)
			}
		}()

		if m.blockWriter != nil {
			m.blockWriter.Close()
		}
	}()

	m.logger.Info("WebM muxer closed", "frames_written", m.frameCount)
	return nil
}

// initialize initializes the WebM container with panic protection
func (m *WebMMuxer) initialize() error {
	// Initialize with panic protection
	func() {
		defer func() {
			if r := recover(); r != nil {
				m.logger.Error("Panic in initialize", "panic", r)
				m.closed = true
			}
		}()

		// Create a WriteCloser wrapper
		wc := &writeCloserWrapper{Writer: m.writer}

		// Create block writer
		writers, err := NewSimpleBlockWriter(wc, []TrackEntry{m.trackEntry})
		if err != nil {
			m.logger.Error("Failed to create block writer", "error", err)
			m.closed = true
			return
		}

		if len(writers) == 0 {
			m.logger.Error("No block writers created")
			m.closed = true
			return
		}

		m.blockWriter = writers[0]
		m.initialized = true
		m.logger.Info("WebM container initialized successfully")
	}()

	return nil
}

// writeCloserWrapper wraps an io.Writer to implement io.WriteCloser
type writeCloserWrapper struct {
	io.Writer
}

func (w *writeCloserWrapper) Close() error {
	// No-op for our use case
	return nil
}
