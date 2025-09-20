package audio

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/scrcpy"
)

// ScrcpyAudioSourceAdapter adapts scrcpy source to AudioSource interface
type ScrcpyAudioSourceAdapter struct {
	logger *slog.Logger
}

// NewScrcpyAudioSourceAdapter creates a new scrcpy audio source adapter
func NewScrcpyAudioSourceAdapter() *ScrcpyAudioSourceAdapter {
	return &ScrcpyAudioSourceAdapter{
		logger: slog.With("component", "scrcpy_audio_adapter"),
	}
}

// SubscribeAudio subscribes to audio stream for a device
func (a *ScrcpyAudioSourceAdapter) SubscribeAudio(deviceSerial, subscriberID string, bufferSize int) <-chan AudioSample {
	// Get audio stream from device source
	source := scrcpy.GetSource(deviceSerial)
	if source == nil {
		a.logger.Error("Device source not found", "device", deviceSerial)
		// Return a closed channel to indicate no source available
		ch := make(chan AudioSample)
		close(ch)
		return ch
	}

	a.logger.Info("Found scrcpy source for audio streaming", "device", deviceSerial)

	// Subscribe to audio stream from scrcpy source
	audioCh := source.SubscribeAudio(subscriberID, bufferSize)

	// Convert scrcpy audio samples to our AudioSample format
	convertedCh := make(chan AudioSample, bufferSize)

	go func() {
		defer close(convertedCh)
		for sample := range audioCh {
			convertedSample := AudioSample{
				Data:      sample.Data,
				Timestamp: 0,      // scrcpy doesn't provide timestamp, we'll calculate it
				Format:    "opus", // Assume Opus format from scrcpy
			}
			select {
			case convertedCh <- convertedSample:
			default:
				// Channel is full, skip this sample
				a.logger.Warn("Audio channel full, dropping sample", "subscriberID", subscriberID)
			}
		}
	}()

	return convertedCh
}

// UnsubscribeAudio unsubscribes from audio stream
func (a *ScrcpyAudioSourceAdapter) UnsubscribeAudio(subscriberID string) {
	// Find the source that has this subscriber
	// Note: This is a simplified implementation
	// In a real implementation, you might need to track sources by subscriber ID
	a.logger.Debug("Unsubscribing from audio stream", "subscriberID", subscriberID)

	// The actual unsubscription is handled by the scrcpy source
	// We can't easily access it here without tracking, but the scrcpy source
	// should handle cleanup when the channel is closed
}

// AudioServiceFactory creates audio services with proper dependencies
type AudioServiceFactory struct {
	sourceAdapter   *ScrcpyAudioSourceAdapter
	streamerFactory *WebMStreamerFactory
}

// NewAudioServiceFactory creates a new audio service factory
func NewAudioServiceFactory() *AudioServiceFactory {
	return &AudioServiceFactory{
		sourceAdapter:   NewScrcpyAudioSourceAdapter(),
		streamerFactory: NewWebMStreamerFactory(),
	}
}

// CreateAudioService creates a new audio service
func (f *AudioServiceFactory) CreateAudioService() AudioService {
	core := NewAudioStreamingCore(f.sourceAdapter, *f.streamerFactory)
	return &AudioServiceImpl{core: core}
}

// AudioServiceImpl implements AudioService interface
type AudioServiceImpl struct {
	core *AudioStreamingCore
}

// StreamOpus streams Opus audio data to a writer
func (s *AudioServiceImpl) StreamOpus(deviceSerial string, writer io.Writer, format string) error {
	return s.core.StreamOpus(deviceSerial, writer, format)
}

// StreamWebM streams Opus audio as WebM with HTTP response handling
func (s *AudioServiceImpl) StreamWebM(deviceSerial string, w http.ResponseWriter, r *http.Request) error {
	// Extract flush function from http.ResponseWriter
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported")
	}

	return s.core.StreamWebM(deviceSerial, w, flusher.Flush)
}
