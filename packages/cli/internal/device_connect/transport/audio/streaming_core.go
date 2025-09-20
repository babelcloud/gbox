package audio

import (
	"fmt"
	"io"
	"log/slog"
	"time"
)

// AudioStreamingCore handles the core audio streaming logic
type AudioStreamingCore struct {
	source           AudioSource
	streamerFactory  WebMStreamerFactory
	logger           *slog.Logger
}

// NewAudioStreamingCore creates a new audio streaming core
func NewAudioStreamingCore(source AudioSource, factory WebMStreamerFactory) *AudioStreamingCore {
	return &AudioStreamingCore{
		source:          source,
		streamerFactory: factory,
		logger:          slog.With("component", "audio_streaming_core"),
	}
}

// StreamOpus streams Opus audio data to a writer
func (c *AudioStreamingCore) StreamOpus(deviceSerial string, writer io.Writer, format string) error {
	logger := c.logger.With("device", deviceSerial, "format", format)
	logger.Info("Starting Opus audio stream")

	// Only support WebM format
	if format != "webm" {
		logger.Error("Unsupported format", "format", format)
		return fmt.Errorf("unsupported format: %s. Only 'webm' is supported", format)
	}

	// Subscribe to audio stream
	subscriberID := fmt.Sprintf("audio_opus_%p", writer)
	audioCh := c.source.SubscribeAudio(deviceSerial, subscriberID, 100)
	defer c.source.UnsubscribeAudio(subscriberID)

	logger.Info("Subscribed to Opus stream", "subscriberID", subscriberID)

	// Create WebM streamer
	streamer := c.streamerFactory.CreateStreamer(writer)
	defer streamer.Close()

	// Write WebM header
	if err := streamer.WriteHeader(); err != nil {
		logger.Error("Failed to write WebM header", "error", err)
		return err
	}
	logger.Info("WebM container initialized")

	return c.processAudioStream(audioCh, streamer, logger)
}

// processAudioStream processes the audio stream with panic recovery
func (c *AudioStreamingCore) processAudioStream(audioCh <-chan AudioSample, streamer WebMStreamer, logger *slog.Logger) error {
	sampleCount := 0
	startTime := time.Now()

	for sample := range audioCh {
		sampleCount++

		// Skip empty samples
		if len(sample.Data) == 0 {
			continue
		}

		// Calculate timestamp
		timestamp := time.Since(startTime).Nanoseconds()

		// Write frame with panic recovery
		if err := c.writeFrameWithRecovery(streamer, sample.Data, timestamp, sampleCount, logger); err != nil {
			if err == io.ErrClosedPipe {
				logger.Info("Client disconnected, stopping audio stream", "frames_sent", sampleCount)
				break
			}
			return err
		}

		// Log successful transmission for first few frames
		if sampleCount <= 5 {
			logger.Info("Successfully sent WebM Opus data", "count", sampleCount, "size", len(sample.Data))
		}
	}

	return nil
}

// writeFrameWithRecovery writes a frame with comprehensive panic recovery
func (c *AudioStreamingCore) writeFrameWithRecovery(streamer WebMStreamer, data []byte, timestamp int64, frameCount int, logger *slog.Logger) error {
	var writeErr error
	
	func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Warn("WebM write panic recovered", "panic", r, "frame", frameCount)
				writeErr = fmt.Errorf("panic in WebM write: %v", r)
			}
		}()

		writeErr = streamer.WriteOpusFrame(data, timestamp)
	}()

	if writeErr != nil {
		if writeErr == io.ErrClosedPipe {
			return writeErr
		}
		logger.Error("Failed to write WebM frame", "error", writeErr, "frame", frameCount)
		return writeErr
	}

	return nil
}

// StreamWebM streams Opus audio as WebM with HTTP response handling
func (c *AudioStreamingCore) StreamWebM(deviceSerial string, writer io.Writer, flushFunc func()) error {
	logger := c.logger.With("device", deviceSerial)
	logger.Info("Starting WebM audio stream with HTTP response handling")

	// Subscribe to audio stream with larger buffer for HTTP streaming
	subscriberID := fmt.Sprintf("webm_%s_%d", deviceSerial, time.Now().UnixNano())
	audioCh := c.source.SubscribeAudio(deviceSerial, subscriberID, 1000)
	defer c.source.UnsubscribeAudio(subscriberID)

	logger.Info("Subscribed to audio stream", "subscriberID", subscriberID)

	// Create WebM streamer
	streamer := c.streamerFactory.CreateStreamer(writer)
	defer streamer.Close()

	// Write WebM header immediately for HTTP streaming initialization
	if err := streamer.WriteHeader(); err != nil {
		logger.Error("Failed to write WebM header", "error", err)
		return err
	}
	flushFunc()

	logger.Info("WebM header sent, starting audio data stream")

	return c.processHTTPAudioStream(audioCh, streamer, flushFunc, logger)
}

// processHTTPAudioStream processes the audio stream optimized for HTTP streaming
func (c *AudioStreamingCore) processHTTPAudioStream(audioCh <-chan AudioSample, streamer WebMStreamer, flushFunc func(), logger *slog.Logger) error {
	startTime := time.Now()
	frameCount := 0
	var lastFlushTime time.Time

	// Stream audio frames
	for sample := range audioCh {
		// Skip empty samples
		if len(sample.Data) == 0 {
			continue
		}

		// Calculate relative timestamp
		timestamp := time.Since(startTime).Nanoseconds()

		// Write Opus frame to WebM stream with comprehensive protection
		writeSuccess := false
		func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Warn("WebM write panic recovered", "panic", r, "frame", frameCount)
				}
			}()

			if writeErr := streamer.WriteOpusFrame(sample.Data, timestamp); writeErr != nil {
				// Check if this is a client disconnect (expected)
				if writeErr == io.ErrClosedPipe {
					logger.Info("Client disconnected during WebM streaming", "frames_sent", frameCount)
				} else {
					logger.Error("Failed to write Opus frame", "error", writeErr)
				}
			} else {
				writeSuccess = true
			}
		}()

		// If write failed, return error
		if !writeSuccess {
			return io.ErrClosedPipe // Treat as normal termination
		}

		frameCount++

		// Force flush every 100ms for low latency (HTTP streaming optimization)
		now := time.Now()
		if now.Sub(lastFlushTime) >= 100*time.Millisecond {
			flushFunc()
			lastFlushTime = now

			// Log progress every 5 seconds
			if frameCount%250 == 0 { // ~5s at 20ms frames
				stats := streamer.GetStats()
				logger.Info("WebM streaming progress",
					"frames", frameCount,
					"duration", time.Duration(timestamp).Truncate(time.Millisecond),
					"stats", stats)
			}
		}
	}

	return nil
}
