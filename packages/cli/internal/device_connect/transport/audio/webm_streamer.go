package audio

import (
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/transport/audio/webm"
)

// WebMStreamerImpl implements WebMStreamer interface
type WebMStreamerImpl struct {
	muxer  *webm.WebMMuxer
	logger *slog.Logger
}

// NewWebMStreamer creates a new WebM streamer
func NewWebMStreamer(writer io.Writer) *WebMStreamerImpl {
	return &WebMStreamerImpl{
		muxer:  webm.NewWebMMuxer(writer),
		logger: slog.With("component", "webm_streamer"),
	}
}

// WriteHeader writes WebM container header
func (s *WebMStreamerImpl) WriteHeader() error {
	if err := s.muxer.WriteHeader(); err != nil {
		s.logger.Error("Failed to write WebM header", "error", err)
		return fmt.Errorf("failed to write WebM header: %w", err)
	}
	s.logger.Debug("WebM header written successfully")
	return nil
}

// WriteOpusFrame writes an Opus audio frame
func (s *WebMStreamerImpl) WriteOpusFrame(data []byte, timestamp int64) error {
	if len(data) == 0 {
		return nil // Skip empty frames
	}

	// Convert timestamp from nanoseconds to duration
	timestampDuration := time.Duration(timestamp) * time.Nanosecond
	
	if err := s.muxer.WriteOpusFrame(data, timestampDuration); err != nil {
		s.logger.Error("Failed to write Opus frame", "error", err, "data_size", len(data))
		return fmt.Errorf("failed to write Opus frame: %w", err)
	}
	
	return nil
}

// Close closes the streamer
func (s *WebMStreamerImpl) Close() error {
	if err := s.muxer.Close(); err != nil {
		s.logger.Error("Failed to close WebM muxer", "error", err)
		return fmt.Errorf("failed to close WebM muxer: %w", err)
	}
	s.logger.Debug("WebM muxer closed successfully")
	return nil
}

// GetStats returns streaming statistics
func (s *WebMStreamerImpl) GetStats() map[string]interface{} {
	return s.muxer.GetStats()
}

// WebMStreamerFactory creates WebM streamers
type WebMStreamerFactory struct{}

// NewWebMStreamerFactory creates a new WebM streamer factory
func NewWebMStreamerFactory() *WebMStreamerFactory {
	return &WebMStreamerFactory{}
}

// CreateStreamer creates a new WebM streamer for the given writer
func (f *WebMStreamerFactory) CreateStreamer(writer io.Writer) WebMStreamer {
	return NewWebMStreamer(writer)
}
