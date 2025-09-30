package stream

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/core"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/scrcpy"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/mpeg4audio"
)

// StreamManager handles high-level streaming operations with protocol abstraction
type StreamManager struct {
	logger *slog.Logger
}

// NewStreamManager creates a new stream manager
func NewStreamManager(logger *slog.Logger) *StreamManager {
	return &StreamManager{
		logger: logger,
	}
}

// StreamConfig contains configuration for starting a stream
type StreamConfig struct {
	DeviceSerial string
	Mode         string // "webm" or "mp4"
	VideoWidth   int
	VideoHeight  int
}

// StreamResult contains the result of starting a stream
type StreamResult struct {
	CodecParams *CodecParams
	VideoCh     <-chan core.VideoSample
	AudioCh     <-chan core.AudioSample
	Source      *scrcpy.Source
	Cleanup     func()
}

// StartStream starts a mixed audio/video stream with protocol abstraction
func (sm *StreamManager) StartStream(ctx context.Context, config StreamConfig) (*StreamResult, error) {
	// Get or create scrcpy source with specified mode
	// Use background context so the shared source is not cancelled when HTTP request ends
	source, err := scrcpy.StartSourceWithMode(config.DeviceSerial, context.Background(), config.Mode)
	if err != nil {
		return nil, fmt.Errorf("failed to start scrcpy source: %w", err)
	}

	// Generate unique subscriber IDs for this connection
	videoSubscriberID := fmt.Sprintf("%s_video_%d", config.Mode, time.Now().UnixNano())
	audioSubscriberID := fmt.Sprintf("%s_audio_%d", config.Mode, time.Now().UnixNano())

	// Subscribe to video and audio streams
	videoCh := source.SubscribeVideo(videoSubscriberID, 1000)
	audioCh := source.SubscribeAudio(audioSubscriberID, 1000)

	// Prepare cleanup function
	cleanup := func() {
		source.UnsubscribeVideo(videoSubscriberID)
		source.UnsubscribeAudio(audioSubscriberID)
	}

	// Prepare codec parameters with protocol-agnostic initialization
	codecParams, err := sm.initializeCodecParams(config, source)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to initialize codec parameters: %w", err)
	}

	return &StreamResult{
		CodecParams: codecParams,
		VideoCh:     videoCh,
		AudioCh:     audioCh,
		Source:      source,
		Cleanup:     cleanup,
	}, nil
}

// initializeCodecParams handles protocol-specific codec parameter initialization
func (sm *StreamManager) initializeCodecParams(config StreamConfig, source *scrcpy.Source) (*CodecParams, error) {
	// Prepare codec parameters
	codecParams := &CodecParams{
		VideoSPS: nil, // Will be extracted from stream
		VideoPPS: nil, // Will be extracted from stream
	}

	// Set audio config for MP4
	if config.Mode == "mp4" {
		codecParams.AudioConfig = mpeg4audio.AudioSpecificConfig{
			Type:         2, // AAC
			SampleRate:   48000,
			ChannelCount: 2,
		}
	}

	// Try to get SPS/PPS from cached data using protocol abstraction
	spsPpsExtractor := NewSpsPpsExtractor(sm.logger)
	sps, pps, err := spsPpsExtractor.ExtractFromCache(source, config.DeviceSerial)
	if err != nil {
		sm.logger.Warn("Failed to extract SPS/PPS from cache", "device", config.DeviceSerial, "error", err)
		// Continue without SPS/PPS - they will be extracted from the first frame
	} else if sps != nil && pps != nil {
		codecParams.VideoSPS = sps
		codecParams.VideoPPS = pps
		sm.logger.Info("SPS/PPS extracted from cache", "device", config.DeviceSerial, "sps_size", len(sps), "pps_size", len(pps))
	}

	return codecParams, nil
}

// ConvertToMuxerSamples converts scrcpy samples to muxer samples
func (sm *StreamManager) ConvertToMuxerSamples(videoSrc <-chan core.VideoSample, audioSrc <-chan core.AudioSample) (chan VideoSample, chan AudioSample) {
	videoCh := make(chan VideoSample, 1000)
	audioCh := make(chan AudioSample, 1000)

	// Start channel converters
	go sm.convertVideoChannel(videoSrc, videoCh)
	go sm.convertAudioChannel(audioSrc, audioCh)

	return videoCh, audioCh
}

// convertVideoChannel converts scrcpy video samples to our VideoSample format
func (sm *StreamManager) convertVideoChannel(src <-chan core.VideoSample, dst chan<- VideoSample) {
	defer close(dst)
	for sample := range src {
		// Simple keyframe detection based on NAL unit type
		isKeyFrame := false
		if len(sample.Data) > 0 {
			nalType := sample.Data[0] & 0x1F
			isKeyFrame = (nalType == 5) // IDR frame
		}

		dst <- VideoSample{
			Data:       sample.Data,
			PTS:        sample.PTS,
			IsKeyFrame: isKeyFrame,
		}
	}
}

// convertAudioChannel converts scrcpy audio samples to our AudioSample format
func (sm *StreamManager) convertAudioChannel(src <-chan core.AudioSample, dst chan<- AudioSample) {
	defer close(dst)
	for sample := range src {
		dst <- AudioSample{
			Data: sample.Data,
			PTS:  sample.PTS,
		}
	}
}
