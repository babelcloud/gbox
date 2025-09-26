package stream

import (
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/at-wat/ebml-go/mkvcore"
	"github.com/at-wat/ebml-go/webm"
)

// WebMMuxer provides WebM container for mixed audio and video streaming
type WebMMuxer struct {
	writer         io.Writer
	audioWriter    webm.BlockWriteCloser
	videoWriter    webm.BlockWriteCloser
	logger         *slog.Logger
	initialized    bool
	audioTimestamp time.Duration
	videoTimestamp time.Duration
	videoWidth     int
	videoHeight    int
}

// NewWebMMuxer creates a new WebM muxer for mixed streams
func NewWebMMuxer(writer io.Writer) *WebMMuxer {
	return &WebMMuxer{
		writer: writer,
		logger: slog.With("component", "webm_mixed_muxer"),
	}
}

// NewWebMMuxerWithDimensions creates a new WebM muxer with specific video dimensions
func NewWebMMuxerWithDimensions(writer io.Writer, width, height int) *WebMMuxer {
	return &WebMMuxer{
		writer:      writer,
		logger:      slog.With("component", "webm_mixed_muxer"),
		videoWidth:  width,
		videoHeight: height,
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

// WriteHeader initializes the WebM container with audio and video tracks
func (m *WebMMuxer) WriteHeader() error {
	if m.initialized {
		return nil
	}

	m.logger.Info("🎬 Initializing WebM container for mixed audio/video stream")

	// Wrap writer with basic error handling
	writeCloser := &writerCloser{
		writer: m.writer,
		logger: m.logger,
		closed: false,
	}

	// Create WebM writer with audio and video track configuration
	writers, err := webm.NewSimpleBlockWriter(writeCloser, []webm.TrackEntry{
		{
			Name:            "Video",
			TrackNumber:     1,
			TrackUID:        1,
			CodecID:         "V_MPEG4/ISO/AVC", // H.264
			TrackType:       1,                 // Video track type
			DefaultDuration: 33333333,          // ~30fps in nanoseconds
			Video: &webm.Video{
				PixelWidth:  uint64(m.getVideoWidth()),
				PixelHeight: uint64(m.getVideoHeight()),
			},
		},
		{
			Name:            "Audio",
			TrackNumber:     2,
			TrackUID:        2,
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
		m.videoWriter = nil
	}))

	if err != nil {
		m.logger.Error("Failed to create WebM writer", "error", err)
		return err
	}

	// Get the video and audio writers from the slice
	m.videoWriter = writers[0] // Video is track 1
	m.audioWriter = writers[1] // Audio is track 2
	m.initialized = true

	m.logger.Info("✅ WebM mixed stream container initialized successfully")
	return nil
}

// WriteVideoFrame writes an H.264 video frame to the WebM container
func (m *WebMMuxer) WriteVideoFrame(h264Data []byte, timestamp time.Duration) error {
	if !m.initialized || m.videoWriter == nil {
		return fmt.Errorf("WebM muxer not initialized")
	}

	if len(h264Data) == 0 {
		return nil
	}

	// Convert timestamp to nanoseconds
	ns := uint64(timestamp.Nanoseconds())

	// WebM container expects Annex-B format, so write H.264 data directly
	_, err := m.videoWriter.Write(true, int64(ns), h264Data)
	if err != nil {
		m.logger.Error("Failed to write video frame", "error", err, "size", len(h264Data))
		return err
	}

	m.videoTimestamp = timestamp
	m.logger.Debug("Video frame written", "size", len(h264Data), "timestamp", timestamp)
	return nil
}

// WriteAudioFrame writes an Opus audio frame to the WebM container
func (m *WebMMuxer) WriteAudioFrame(opusData []byte, timestamp time.Duration) error {
	if !m.initialized || m.audioWriter == nil {
		return fmt.Errorf("WebM muxer not initialized")
	}

	if len(opusData) == 0 {
		return nil
	}

	// Convert timestamp to nanoseconds
	ns := uint64(timestamp.Nanoseconds())

	// Write Opus data to audio track
	_, err := m.audioWriter.Write(true, int64(ns), opusData)
	if err != nil {
		m.logger.Error("Failed to write audio frame", "error", err, "size", len(opusData))
		return err
	}

	m.audioTimestamp = timestamp
	m.logger.Debug("Audio frame written", "size", len(opusData), "timestamp", timestamp)
	return nil
}

// Close finalizes the WebM container
func (m *WebMMuxer) Close() error {
	if m.videoWriter != nil {
		if err := m.videoWriter.Close(); err != nil {
			m.logger.Warn("Video writer close error", "error", err)
		}
		m.videoWriter = nil
	}

	if m.audioWriter != nil {
		m.logger.Info("🎵 Finalizing WebM mixed stream container",
			"video_timestamp", m.videoTimestamp.Truncate(time.Millisecond),
			"audio_timestamp", m.audioTimestamp.Truncate(time.Millisecond))

		if err := m.audioWriter.Close(); err != nil {
			m.logger.Warn("Audio writer close error", "error", err)
		}
		m.audioWriter = nil
	}

	m.initialized = false
	m.logger.Info("✅ WebM mixed stream muxer closed successfully")
	return nil
}

// getVideoWidth returns the video width, defaulting to 1920 if not set
func (m *WebMMuxer) getVideoWidth() int {
	if m.videoWidth > 0 {
		return m.videoWidth
	}
	return 1920 // Default width
}

// getVideoHeight returns the video height, defaulting to 1080 if not set
func (m *WebMMuxer) getVideoHeight() int {
	if m.videoHeight > 0 {
		return m.videoHeight
	}
	return 1080 // Default height
}
