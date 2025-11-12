package audio

import (
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/at-wat/ebml-go/mkvcore"
	"github.com/at-wat/ebml-go/webm"
)

// WebMMuxer provides WebM container using at-wat/ebml-go
// Based on the official Pion WebRTC save-to-webm example
type WebMMuxer struct {
	writer         io.Writer
	audioWriter    webm.BlockWriteCloser
	logger         *slog.Logger
	initialized    bool
	frameCount     uint64
	audioTimestamp time.Duration
}

// NewWebMMuxer creates a new WebM muxer
func NewWebMMuxer(writer io.Writer) *WebMMuxer {
	return &WebMMuxer{
		writer: writer,
		logger: slog.With("component", "webm_muxer"),
	}
}

// writerCloser wraps an io.Writer with basic error handling
type writerCloser struct {
	writer io.Writer
	logger *slog.Logger
	closed bool
}

func (wc *writerCloser) Write(p []byte) (n int, err error) {
	if wc.closed {
		return 0, io.ErrClosedPipe
	}

	n, err = wc.writer.Write(p)
	if err != nil {
		wc.logger.Warn("Write error detected, marking writer as closed",
			"error", err,
			"error_type", fmt.Sprintf("%T", err),
			"data_size", len(p),
			"bytes_written", n)
		wc.closed = true
	}
	return n, err
}

func (wc *writerCloser) Close() error {
	wc.closed = true
	return nil
}

// WriteHeader initializes the WebM container with audio track
func (m *WebMMuxer) WriteHeader() error {
	if m.initialized {
		return nil
	}

	m.logger.Info("🎵 Initializing WebM container based on Pion example")

	// Wrap writer with basic error handling
	writeCloser := &writerCloser{
		writer: m.writer,
		logger: m.logger,
		closed: false,
	}

	// Create WebM writer with audio track configuration (matching Pion's example)
	// Use custom fatal error handler to avoid panic
	writers, err := webm.NewSimpleBlockWriter(writeCloser, []webm.TrackEntry{
		{
			Name:            "Audio",
			TrackNumber:     1,
			TrackUID:        1, // Use simple UID to avoid conflicts
			CodecID:         "A_OPUS",
			TrackType:       2,        // Audio track type
			DefaultDuration: 20000000, // 20ms in nanoseconds (typical Opus frame duration)
			Audio: &webm.Audio{
				SamplingFrequency: 48000.0, // 48kHz
				Channels:          2,       // Stereo
			},
		},
	}, mkvcore.WithOnFatalHandler(func(err error) {
		m.logger.Warn("WebM error occurred, will trigger client reconnect", "error", err)
		// Reset state for clean reconnection
		m.initialized = false
		m.audioWriter = nil
	}))

	if err != nil {
		m.logger.Error("Failed to create WebM writer", "error", err)
		return err
	}

	// Get the audio writer from the slice
	m.audioWriter = writers[0]
	m.initialized = true

	m.logger.Info("✅ WebM container initialized successfully")
	return nil
}

// WriteOpusFrame writes an Opus frame to the WebM container
func (m *WebMMuxer) WriteOpusFrame(opusData []byte, timestamp time.Duration) error {
	// Early validation checks
	if len(opusData) == 0 {
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

	// Update audio timestamp (cumulative duration)
	// Using fixed 20ms duration for Opus frames (typical)
	frameTimestamp := 20 * time.Millisecond
	m.audioTimestamp += frameTimestamp

	// Write to WebM container
	// Parameters: isKeyframe (true for audio), timestamp in milliseconds, data
	_, err := m.audioWriter.Write(true, int64(m.audioTimestamp/time.Millisecond), opusData)

	if err != nil {
		if err == io.ErrClosedPipe {
			m.logger.Info("Audio stream closed, stopping WebM stream", "frame", m.frameCount)
		} else {
			m.logger.Error("Failed to write Opus frame to WebM", "error", err, "frame", m.frameCount)
			// Mark writer as closed on write error to prevent further attempts
			m.audioWriter = nil
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
func (m *WebMMuxer) Close() error {
	if m.audioWriter != nil {
		m.logger.Info("🎵 Finalizing WebM container", "total_frames", m.frameCount)

		if err := m.audioWriter.Close(); err != nil {
			m.logger.Warn("WebM writer close error (expected if stream ended)", "error", err)
		}

		m.audioWriter = nil
	}

	m.logger.Info("✅ WebM muxer closed successfully")
	return nil
}

// GetStats returns muxer statistics
func (m *WebMMuxer) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"frames_written":    m.frameCount,
		"audio_duration_ms": int64(m.audioTimestamp / time.Millisecond),
		"initialized":       m.initialized,
		"type":              "webm",
	}
}
