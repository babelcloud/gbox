package audio

import (
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/at-wat/ebml-go/webm"
)

// ProfessionalWebMMuxer provides professional WebM container using at-wat/ebml-go
// Based on the official Pion WebRTC save-to-webm example
type ProfessionalWebMMuxer struct {
	writer       io.Writer
	audioWriter  webm.BlockWriteCloser
	logger       *slog.Logger
	initialized  bool
	frameCount   uint64
	audioTimestamp time.Duration
}

// NewProfessionalWebMMuxer creates a new professional WebM muxer
func NewProfessionalWebMMuxer(writer io.Writer) *ProfessionalWebMMuxer {
	return &ProfessionalWebMMuxer{
		writer: writer,
		logger: slog.With("component", "professional_webm_muxer"),
	}
}

// safeWriterCloser wraps an io.Writer with comprehensive panic recovery
type safeWriterCloser struct {
	writer io.Writer
	logger *slog.Logger
	closed bool
}

func (swc *safeWriterCloser) Write(p []byte) (n int, err error) {
	// Double-check closed state
	if swc.closed {
		return 0, io.ErrClosedPipe
	}

	// Additional safety check - verify writer is still valid
	if swc.writer == nil {
		swc.closed = true
		return 0, io.ErrClosedPipe
	}

	// Comprehensive panic recovery
	defer func() {
		if r := recover(); r != nil {
			swc.logger.Warn("Write operation panic recovered", "panic", r)
			swc.closed = true
			err = io.ErrClosedPipe
			n = 0
		}
	}()

	// Additional safety check before write
	if swc.closed {
		return 0, io.ErrClosedPipe
	}

	n, err = swc.writer.Write(p)
	if err != nil {
		swc.logger.Warn("Write error detected, marking writer as closed", "error", err)
		swc.closed = true
	}
	return n, err
}

func (swc *safeWriterCloser) Close() error {
	swc.closed = true
	return nil // No-op close
}

// WriteHeader initializes the WebM container with audio track
func (m *ProfessionalWebMMuxer) WriteHeader() error {
	if m.initialized {
		return nil
	}

	m.logger.Info("🎵 Initializing professional WebM container based on Pion example")

	// Comprehensive panic recovery for the entire initialization
	var initErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				m.logger.Error("WebM initialization panic recovered", "panic", r)
				initErr = fmt.Errorf("WebM initialization failed due to panic: %v", r)
			}
		}()

		// Wrap writer with comprehensive panic recovery
		writeCloser := &safeWriterCloser{
			writer: m.writer,
			logger: m.logger,
			closed: false,
		}

		// Create WebM writer with audio track configuration (matching Pion's example)
		writers, err := webm.NewSimpleBlockWriter(writeCloser, []webm.TrackEntry{
			{
				Name:            "Audio",
				TrackNumber:     1,
				TrackUID:        12345,
				CodecID:         "A_OPUS",
				TrackType:       2,                // Audio track type
				DefaultDuration: 20000000,         // 20ms in nanoseconds (typical Opus frame duration)
				Audio: &webm.Audio{
					SamplingFrequency: 48000.0,    // 48kHz
					Channels:          2,          // Stereo
				},
			},
		})

		if err != nil {
			m.logger.Error("Failed to create WebM writer", "error", err)
			initErr = err
			return
		}

		// Get the audio writer from the slice
		m.audioWriter = writers[0]
		m.initialized = true
	}()

	if initErr != nil {
		return initErr
	}

	m.logger.Info("✅ Professional WebM container initialized successfully")
	return nil
}

// WriteOpusFrame writes an Opus frame to the WebM container
func (m *ProfessionalWebMMuxer) WriteOpusFrame(opusData []byte, timestamp time.Duration) error {
	// Early validation checks
	if opusData == nil || len(opusData) == 0 {
		return nil // Skip empty frames
	}

	if !m.initialized {
		if err := m.WriteHeader(); err != nil {
			return err
		}
	}

	// Check if audioWriter is still valid (in case of stream closure)
	if m.audioWriter == nil {
		m.logger.Warn("WebM writer is closed, cannot write frame")
		return io.ErrClosedPipe
	}

	// Additional safety check for muxer state
	defer func() {
		if r := recover(); r != nil {
			m.logger.Warn("WriteOpusFrame panic recovered", "panic", r, "frame", m.frameCount)
			// Mark writer as closed to prevent further writes
			m.audioWriter = nil
		}
	}()

	// Update audio timestamp (cumulative duration)
	// Using fixed 20ms duration for Opus frames (typical)
	frameTimestamp := 20 * time.Millisecond
	m.audioTimestamp += frameTimestamp

	// Safely write to WebM container with panic recovery
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				m.logger.Warn("WebM write panic recovered", "panic", r, "frame", m.frameCount)
				err = io.ErrClosedPipe
				// Mark writer as closed to prevent further writes
				m.audioWriter = nil
			}
		}()

		// Write to WebM container
		// Parameters: isKeyframe (true for audio), timestamp in milliseconds, data
		_, err = m.audioWriter.Write(true, int64(m.audioTimestamp/time.Millisecond), opusData)
	}()

	if err != nil {
		if err == io.ErrClosedPipe {
			m.logger.Info("Audio stream closed, stopping WebM stream", "frame", m.frameCount)
		} else {
			m.logger.Error("Failed to write Opus frame to WebM", "error", err, "frame", m.frameCount)
		}
		return err
	}

	m.frameCount++

	// Log progress every 250 frames (~5 seconds at 20ms per frame)
	if m.frameCount%250 == 0 {
		m.logger.Debug("🎵 WebM audio progress",
			"frames", m.frameCount,
			"duration", m.audioTimestamp.Truncate(time.Millisecond),
			"data_size", len(opusData))
	}

	return nil
}

// Close finalizes the WebM container
func (m *ProfessionalWebMMuxer) Close() error {
	if m.audioWriter != nil {
		m.logger.Info("🎵 Finalizing WebM container", "total_frames", m.frameCount)

		// Safe close with panic recovery
		func() {
			defer func() {
				if r := recover(); r != nil {
					m.logger.Warn("WebM close panic recovered", "panic", r)
				}
			}()

			if err := m.audioWriter.Close(); err != nil {
				m.logger.Warn("WebM writer close error (expected if stream ended)", "error", err)
			}
		}()

		m.audioWriter = nil
	}

	m.logger.Info("✅ Professional WebM muxer closed successfully")
	return nil
}

// GetStats returns muxer statistics
func (m *ProfessionalWebMMuxer) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"frames_written":    m.frameCount,
		"audio_duration_ms": int64(m.audioTimestamp / time.Millisecond),
		"initialized":       m.initialized,
		"type":             "professional_webm",
	}
}