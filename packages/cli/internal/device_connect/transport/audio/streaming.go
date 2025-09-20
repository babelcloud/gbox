package audio

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/core"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/scrcpy"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/transport/audio/webm"
)

// AudioStreamingService 音频流服务
type AudioStreamingService struct {
	source core.Source
}

// NewAudioStreamingService creates a new audio streaming service
func NewAudioStreamingService() *AudioStreamingService {
	return &AudioStreamingService{}
}

// SetSource sets the audio source
func (s *AudioStreamingService) SetSource(source core.Source) {
	s.source = source
}

// StreamOpus 流式处理 Opus 音频 - 只支持WebM格式
func (s *AudioStreamingService) StreamOpus(deviceSerial string, writer io.Writer, format string) error {
	logger := slog.With("device", deviceSerial, "format", format)
	logger.Info("🎵 Starting Opus audio stream", "format", format)

	// Only support WebM format
	if format != "webm" {
		logger.Error("❌ Unsupported format", "format", format)
		return fmt.Errorf("unsupported format: %s. Only 'webm' is supported", format)
	}

	// Get audio stream from device source
	source := scrcpy.GetSource(deviceSerial)
	if source == nil {
		logger.Error("❌ Device source not found - is scrcpy running for this device?")
		return fmt.Errorf("device not connected")
	}

	logger.Info("✅ Found scrcpy source for Opus streaming")

	// Subscribe to audio stream
	subscriberID := fmt.Sprintf("audio_opus_%p", writer)
	audioCh := source.SubscribeAudio(subscriberID, 100)
	defer source.UnsubscribeAudio(subscriberID)

	logger.Info("🎵 Subscribed to Opus stream", "subscriberID", subscriberID)

	// Create WebM muxer
	muxer := webm.NewWebMMuxer(writer)
	defer muxer.Close()

	// Write WebM header
	if err := muxer.WriteHeader(); err != nil {
		logger.Error("❌ Failed to write WebM header", "error", err)
		return err
	}
	logger.Info("✅ WebM container initialized")

	sampleCount := 0
	startTime := time.Now()
	for sample := range audioCh {
		sampleCount++

		// Skip empty samples
		if len(sample.Data) == 0 {
			continue
		}

		// Write frame using professional WebM muxer with comprehensive error recovery
		timestamp := time.Since(startTime)

		// Add comprehensive panic and error protection
		func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Warn("🎵 WebM write panic recovered at streaming level", "panic", r, "frame", sampleCount)
					// Mark muxer as failed to prevent further writes
					muxer = nil
				}
			}()

			if muxer != nil {
				if writeErr := muxer.WriteOpusFrame(sample.Data, timestamp); writeErr != nil {
					if writeErr == io.ErrClosedPipe {
						logger.Info("🎵 Client disconnected, stopping audio stream", "frames_sent", sampleCount)
						// Mark muxer as closed to prevent further writes
						muxer = nil
						return
					} else {
						logger.Error("❌ Failed to write WebM frame", "error", writeErr, "frame", sampleCount)
					}
				}
			}
		}()

		// If muxer is closed, break out of the loop
		if muxer == nil {
			logger.Info("🎵 Muxer closed, stopping stream", "frames_sent", sampleCount)
			break
		}

		// Log successful transmission for first few frames
		if sampleCount <= 5 {
			logger.Info("✅ Successfully sent WebM Opus data", "count", sampleCount, "size", len(sample.Data))
		}
	}

	return nil
}

// StreamWebMForMSE streams Opus audio as WebM optimized for MSE consumption
func (s *AudioStreamingService) StreamWebMForMSE(deviceSerial string, w http.ResponseWriter, r *http.Request) error {
	logger := slog.With("component", "mse_streaming", "device", deviceSerial)
	logger.Info("🎵 Starting MSE-optimized WebM audio stream")

	// Set HTTP headers for MSE streaming
	w.Header().Set("Content-Type", "audio/webm; codecs=opus")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Range")

	// Start streaming immediately
	w.WriteHeader(http.StatusOK)

	// Ensure we can flush chunks
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported")
	}
	flusher.Flush()

	// Get audio stream from device source
	source := scrcpy.GetSource(deviceSerial)
	if source == nil {
		return fmt.Errorf("device source not found: %s", deviceSerial)
	}

	// Subscribe to audio stream with larger buffer for MSE stability
	subscriberID := fmt.Sprintf("mse_webm_%s_%d", deviceSerial, time.Now().UnixNano())
	audioCh := source.SubscribeAudio(subscriberID, 1000)
	defer source.UnsubscribeAudio(subscriberID)

	logger.Info("🎵 Subscribed to audio stream", "subscriberID", subscriberID)

	// Create WebM muxer
	muxer := webm.NewWebMMuxer(w)
	defer muxer.Close()

	// Write WebM header immediately for MSE initialization
	if err := muxer.WriteHeader(); err != nil {
		logger.Error("Failed to write WebM header", "error", err)
		return err
	}
	flusher.Flush()

	logger.Info("✅ WebM header sent, starting audio data stream")

	startTime := time.Now()
	frameCount := 0
	var lastFlushTime time.Time

	// Stream audio frames
	for {
		select {
		case sample, ok := <-audioCh:
			if !ok {
				logger.Info("🎵 Audio channel closed")
				return nil
			}

			// Skip empty samples
			if len(sample.Data) == 0 {
				continue
			}

			// Calculate relative timestamp
			timestamp := time.Since(startTime)

			// Write Opus frame to WebM stream with comprehensive protection
			writeSuccess := false
			func() {
				defer func() {
					if r := recover(); r != nil {
						logger.Warn("🎵 MSE WebM write panic recovered", "panic", r, "frame", frameCount)
						// Mark muxer as failed to prevent further writes
						muxer = nil
					}
				}()

				if muxer != nil {
					if writeErr := muxer.WriteOpusFrame(sample.Data, timestamp); writeErr != nil {
						// Check if this is a client disconnect (expected)
						if writeErr == io.ErrClosedPipe {
							logger.Info("🎵 Client disconnected during MSE streaming", "frames_sent", frameCount)
							// Mark muxer as closed to prevent further writes
							muxer = nil
						} else {
							logger.Error("Failed to write Opus frame", "error", writeErr)
						}
					} else {
						writeSuccess = true
					}
				}
			}()

			// If muxer was set to nil due to panic, stop streaming
			if muxer == nil {
				logger.Info("🎵 MSE muxer failed due to panic, stopping stream", "frames_sent", frameCount)
				return nil
			}

			// If write failed, return error
			if !writeSuccess {
				return io.ErrClosedPipe // Treat as normal termination
			}

			frameCount++

			// Force flush every 100ms for low latency (MSE optimization)
			now := time.Now()
			if now.Sub(lastFlushTime) >= 100*time.Millisecond {
				flusher.Flush()
				lastFlushTime = now

				// Log progress every 5 seconds
				if frameCount%250 == 0 { // ~5s at 20ms frames
					stats := muxer.GetStats()
					logger.Info("🎵 MSE WebM streaming progress",
						"frames", frameCount,
						"duration", timestamp.Truncate(time.Millisecond),
						"stats", stats)
				}
			}

		case <-r.Context().Done():
			logger.Info("🎵 Client disconnected", "frames_sent", frameCount)
			return nil
		}
	}
}

// 全局音频流服务实例
var audioService *AudioStreamingService

// GetAudioService 获取音频流服务实例
func GetAudioService() *AudioStreamingService {
	if audioService == nil {
		audioService = NewAudioStreamingService()
	}
	return audioService
}
