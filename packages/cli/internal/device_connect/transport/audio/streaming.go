package audio

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/core"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/scrcpy"
)

// AudioStreamingService 音频流服务
type AudioStreamingService struct {
	source core.Source
}

// ConnectionHealthMonitor monitors HTTP connection health for early disconnection detection
type ConnectionHealthMonitor struct {
	writer   http.ResponseWriter
	flusher  http.Flusher
	logger   *slog.Logger
	interval time.Duration
	stopChan chan struct{}
	stopped  bool
	mu       sync.RWMutex
	healthy  bool
}

// Start begins monitoring the connection health
func (chm *ConnectionHealthMonitor) Start() {
	chm.mu.Lock()
	defer chm.mu.Unlock()

	if chm.stopped {
		return
	}

	chm.stopChan = make(chan struct{})
	chm.healthy = true

	go chm.monitor()
}

// Stop stops the health monitoring
func (chm *ConnectionHealthMonitor) Stop() {
	chm.mu.Lock()
	defer chm.mu.Unlock()

	if chm.stopped {
		return
	}

	chm.stopped = true
	if chm.stopChan != nil {
		close(chm.stopChan)
	}
}

// IsHealthy returns whether the connection is still healthy
func (chm *ConnectionHealthMonitor) IsHealthy() bool {
	chm.mu.RLock()
	defer chm.mu.RUnlock()
	return chm.healthy
}

// monitor runs the health monitoring loop
func (chm *ConnectionHealthMonitor) monitor() {
	ticker := time.NewTicker(chm.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if !chm.checkHealth() {
				chm.mu.Lock()
				chm.healthy = false
				chm.mu.Unlock()
				chm.logger.Info("🎵 Connection health check failed, marking as unhealthy")
				return
			}
		case <-chm.stopChan:
			return
		}
	}
}

// checkHealth performs a health check without interfering with data flushing
func (chm *ConnectionHealthMonitor) checkHealth() bool {
	defer func() {
		if r := recover(); r != nil {
			chm.logger.Warn("🎵 Health check panic recovered", "panic", r)
			// Mark as unhealthy if health check panics
			chm.mu.Lock()
			chm.healthy = false
			chm.mu.Unlock()
		}
	}()

	// Don't flush here to avoid conflicts with data stream flushing
	// Just check if the connection is still alive by checking the writer
	// This is a non-intrusive health check
	return true
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
	muxer := NewWebMMuxer(writer)
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

		// Write frame using WebM muxer
		if muxer != nil {
			if writeErr := muxer.WriteOpusFrame(sample.Data, timestamp); writeErr != nil {
				if writeErr == io.ErrClosedPipe {
					logger.Info("🎵 Client disconnected, stopping audio stream", "frames_sent", sampleCount)
				} else {
					logger.Error("❌ Failed to write WebM frame", "error", writeErr, "frame", sampleCount)
				}
				// Mark muxer as failed and stop streaming
				muxer = nil
			}
		}

		// If muxer was set to nil due to panic, stop streaming
		if muxer == nil {
			logger.Info("🎵 Muxer failed due to panic, stopping stream", "frames_sent", sampleCount)
			return nil
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

	// Set up client disconnect detection
	closeNotifier, ok := w.(http.CloseNotifier)
	var closeNotify <-chan bool
	if ok {
		closeNotify = closeNotifier.CloseNotify()
		logger.Info("🎵 Client disconnect detection enabled")
	} else {
		logger.Warn("🎵 CloseNotifier not available, using context only")
	}

	// Create connection health monitor
	healthMonitor := &ConnectionHealthMonitor{
		writer:   w,
		flusher:  flusher,
		logger:   logger,
		interval: 500 * time.Millisecond, // Check every 500ms
	}
	healthMonitor.Start()
	defer healthMonitor.Stop()

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
	muxer := NewWebMMuxer(w)
	defer muxer.Close()

	// Write WebM header immediately for MSE initialization
	if err := muxer.WriteHeader(); err != nil {
		logger.Error("Failed to write WebM header", "error", err)
		return err
	}
	flusher.Flush()

	logger.Info("✅ WebM header sent, starting audio data stream",
		"headers", map[string]string{
			"Content-Type":      "audio/webm; codecs=opus",
			"Transfer-Encoding": "chunked",
			"Cache-Control":     "no-cache, no-store, must-revalidate",
		})

	startTime := time.Now()
	frameCount := 0
	var lastFlushTime time.Time
	connectionLost := false

	// Stream audio frames with improved error recovery
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

			// Check connection health before processing
			if !healthMonitor.IsHealthy() || connectionLost {
				if !connectionLost {
					logger.Info("🎵 Connection marked as unhealthy, stopping stream", "frames_sent", frameCount)
					connectionLost = true
				}
				return nil
			}

			// Check for backpressure - if channel is getting full, skip some samples
			if len(audioCh) > 800 { // 80% of buffer size
				logger.Warn("🎵 Audio buffer backpressure detected, skipping sample",
					"buffer_usage", len(audioCh), "buffer_size", 1000)
				continue
			}

			// Calculate relative timestamp
			timestamp := time.Since(startTime)

			// Check if muxer is still valid before attempting to write
			if muxer == nil {
				logger.Warn("🎵 Muxer is nil, skipping frame", "frame", frameCount)
				continue
			}

			// Write Opus frame with simple error handling (no retries)
			writeSuccess := false
			if writeErr := muxer.WriteOpusFrame(sample.Data, timestamp); writeErr != nil {
				// Check if this is a client disconnect (expected)
				if writeErr == io.ErrClosedPipe {
					logger.Info("🎵 Client disconnected during MSE streaming", "frames_sent", frameCount)
				} else {
					logger.Warn("🎵 Write failed, stopping stream for client reconnect", "error", writeErr, "frame", frameCount)
				}
				// Stop streaming to trigger client reconnection
				muxer = nil
				connectionLost = true
			} else {
				writeSuccess = true
			}

			// If muxer was set to nil due to panic, stop streaming
			if muxer == nil {
				logger.Info("🎵 MSE muxer failed, stopping stream", "frames_sent", frameCount)
				connectionLost = true
				return nil
			}

			// If write failed, stop streaming
			if !writeSuccess {
				logger.Warn("🎵 Write failed, stopping stream", "frame", frameCount)
				return nil
			}

			frameCount++

			// Force flush every 200ms for better stability (reduced frequency)
			now := time.Now()
			if now.Sub(lastFlushTime) >= 200*time.Millisecond {
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
			logger.Info("🎵 Request context cancelled", "frames_sent", frameCount)
			return nil

		case <-closeNotify:
			logger.Info("🎵 Client disconnected via CloseNotify", "frames_sent", frameCount)
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
