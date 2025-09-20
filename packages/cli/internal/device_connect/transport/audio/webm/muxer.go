package webm

import (
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/at-wat/ebml-go/webm"
)

// WebMMuxer provides a panic-safe wrapper around ebml-go webm functionality
type WebMMuxer struct {
	writer         io.Writer
	logger         *slog.Logger
	initialized    bool
	frameCount     uint64
	audioTimestamp time.Duration
	closed         bool
	mu             sync.RWMutex
	blockWriter    webm.BlockWriteCloser
	trackEntry     webm.TrackEntry
}

// NewWebMMuxer creates a new WebM muxer with panic protection
func NewWebMMuxer(writer io.Writer) *WebMMuxer {
	return &WebMMuxer{
		writer: writer,
		logger: slog.With("component", "webm_muxer"),
		trackEntry: webm.TrackEntry{
			TrackNumber: 1,
			TrackUID:    12345,
			CodecID:     "A_OPUS",
			TrackType:   2, // Audio track
			Audio: &webm.Audio{
				SamplingFrequency: 48000.0,
				Channels:          2,
			},
		},
	}
}

// WriteHeader writes the WebM container header
func (m *WebMMuxer) WriteHeader() error {
	// This is a no-op for our use case as headers are written automatically
	// by the underlying ebml-go webm implementation
	return nil
}

// WriteOpusFrame writes an Opus audio frame with panic protection
func (m *WebMMuxer) WriteOpusFrame(opusData []byte, timestamp time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return io.ErrClosedPipe
	}

	if !m.initialized {
		if err := m.initialize(); err != nil {
			m.logger.Error("Failed to initialize WebM muxer", "error", err)
			m.closed = true
			return err
		}
	}

	// Use a goroutine with recover to protect against panics from ebml-go
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

// Close closes the WebM muxer
func (m *WebMMuxer) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}

	if m.blockWriter != nil {
		if err := m.blockWriter.Close(); err != nil {
			m.logger.Error("Failed to close block writer", "error", err)
			return err
		}
	}

	m.closed = true
	return nil
}

// GetStats returns muxer statistics
func (m *WebMMuxer) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"frames_written": m.frameCount,
		"closed":         m.closed,
		"audio_duration": m.audioTimestamp,
	}
}

// initialize sets up the underlying ebml-go webm block writer
func (m *WebMMuxer) initialize() error {
	// Use a goroutine with recover to protect against panics from ebml-go
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				m.logger.Error("Panic in initialize", "panic", r)
				err = &PanicError{Panic: r}
			}
		}()

		// Create a writeCloser wrapper for our writer
		wc := &writeCloserWrapper{Writer: m.writer}

		// Use ebml-go webm directly
		writers, writeErr := webm.NewSimpleBlockWriter(wc, []webm.TrackEntry{m.trackEntry})
		if writeErr != nil {
			err = writeErr
			return
		}

		if len(writers) == 0 {
			err = &InitializationError{Message: "no block writers created"}
			return
		}

		m.blockWriter = writers[0]
		m.initialized = true
	}()

	return err
}

// PanicError represents a panic that occurred during initialization
type PanicError struct {
	Panic interface{}
}

func (e *PanicError) Error() string {
	return "panic during initialization"
}

// InitializationError represents an initialization error
type InitializationError struct {
	Message string
}

func (e *InitializationError) Error() string {
	return e.Message
}

// writeCloserWrapper wraps an io.Writer to implement io.WriteCloser
type writeCloserWrapper struct {
	io.Writer
}

func (w *writeCloserWrapper) Close() error {
	// If the underlying writer implements io.Closer, call its Close method
	if closer, ok := w.Writer.(io.Closer); ok {
		return closer.Close()
	}
	// No-op for writers that don't implement io.Closer
	return nil
}
