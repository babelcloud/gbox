package audio

import (
	"io"
	"net/http"
)

// AudioService defines the interface for audio streaming services
type AudioService interface {
	// StreamOpus streams Opus audio data to a writer
	StreamOpus(deviceSerial string, writer io.Writer, format string) error

	// StreamWebM streams Opus audio as WebM with HTTP response handling
	StreamWebM(deviceSerial string, w http.ResponseWriter, r *http.Request) error
}

// AudioSource defines the interface for audio data sources
type AudioSource interface {
	// SubscribeAudio subscribes to audio stream for a device
	SubscribeAudio(deviceSerial, subscriberID string, bufferSize int) <-chan AudioSample

	// UnsubscribeAudio unsubscribes from audio stream
	UnsubscribeAudio(subscriberID string)
}

// AudioSample represents a single audio sample
type AudioSample struct {
	Data      []byte
	Timestamp int64
	Format    string
}

// WebMStreamer defines the interface for WebM streaming operations
type WebMStreamer interface {
	// WriteHeader writes WebM container header
	WriteHeader() error

	// WriteOpusFrame writes an Opus audio frame
	WriteOpusFrame(data []byte, timestamp int64) error

	// Close closes the streamer
	Close() error

	// GetStats returns streaming statistics
	GetStats() map[string]interface{}
}
