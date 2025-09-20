package webrtc

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/core"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/pipeline"
	"github.com/babelcloud/gbox/packages/cli/internal/util"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
)

// Transport implements WebRTC streaming transport
type Transport struct {
	deviceSerial   string
	pipeline       *pipeline.Pipeline
	peerConnection *webrtc.PeerConnection
	dataChannel    *webrtc.DataChannel

	// Tracks
	videoTrack *webrtc.TrackLocalStaticSample
	audioTrack *webrtc.TrackLocalStaticSample

	// Control handler
	controlHandler *ControlHandlerWrapper


	// Control flow
	ctx    context.Context
	cancel context.CancelFunc

	// Synchronization
	mu     sync.Mutex
	closed bool
}

// NewTransport creates a new WebRTC transport
func NewTransport(deviceSerial string, pipeline *pipeline.Pipeline) (*Transport, error) {
	log.Printf("Creating WebRTC transport for device: %s", deviceSerial)
	ctx, cancel := context.WithCancel(context.Background())

	// Create WebRTC peer connection
	pc, err := createPeerConnection()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create peer connection: %w", err)
	}

	// Create transport
	transport := &Transport{
		deviceSerial:   deviceSerial,
		pipeline:       pipeline,
		peerConnection: pc,
		ctx:            ctx,
		cancel:         cancel,
	}

	// Set up data channel receiver (frontend will create the data channel)
	pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		if dc.Label() == "control" {
			transport.dataChannel = dc
			log.Printf("Control DataChannel connected")
			// Set up control handler when DataChannel is received
			if transport.controlHandler != nil {
				transport.controlHandler.UpdateDataChannel(dc)
				transport.controlHandler.HandleIncomingMessages()
			}
		}
	})

	// Create control handler (DataChannel will be assigned when received)
	transport.controlHandler = NewControlHandlerWrapper(nil, 1080, 1920)

	// Pre-create video and audio tracks for WebRTC negotiation
	videoTrack, err := addVideoTrack(pc, "h264")
	if err != nil {
		pc.Close()
		cancel()
		return nil, fmt.Errorf("failed to add video track: %w", err)
	}
	transport.videoTrack = videoTrack
	// H.264 video track configured

	// Add audio track
	audioTrack, err := addAudioTrack(pc, "opus")
	if err != nil {
		pc.Close()
		cancel()
		return nil, fmt.Errorf("failed to add audio track: %w", err)
	}
	transport.audioTrack = audioTrack
	// Opus audio track configured

	return transport, nil
}


// Start starts the WebRTC transport using pipeline
func (t *Transport) Start(source core.Source) error {
	// Video: forward Annex-B samples to WebRTC video track
	go func() {
		videoCh := source.SubscribeVideo("webrtc_transport", 1000)
		defer source.UnsubscribeVideo("webrtc_transport")

		var lastVideoTimestamp int64 = 0
		var h264Sps []byte
		var h264Pps []byte
		startCode := []byte{0x00, 0x00, 0x00, 0x01}
		decoderReady := false
		firstFrameSent := false

		for sample := range videoCh {
			if sample.Data == nil || len(sample.Data) == 0 || t.videoTrack == nil {
				continue
			}

			// Calculate duration between frames
			timestamp := sample.PTS
			var duration time.Duration
			if lastVideoTimestamp > 0 && timestamp > lastVideoTimestamp {
				duration = time.Duration(timestamp-lastVideoTimestamp) * time.Microsecond
				duration = min(duration, 33*time.Millisecond) // Cap at 30 FPS
			}
			lastVideoTimestamp = timestamp

			// Initialize SPS/PPS from cached data if not done yet
			if len(h264Sps) == 0 || len(h264Pps) == 0 {
				spsPpsData := source.GetSpsPps()
				if len(spsPpsData) > 0 {
					parts := bytes.Split(spsPpsData, startCode)
					for i := 1; i < len(parts); i++ {
						nal := parts[i]
						if len(nal) == 0 {
							continue
						}
						nalType := nal[0] & 0x1F
						switch nalType {
						case 7: // SPS
							h264Sps = append([]byte{0x00, 0x00, 0x00, 0x01}, nal...)
						case 8: // PPS
							h264Pps = append([]byte{0x00, 0x00, 0x00, 0x01}, nal...)
						}
					}
				}
			}

			// For keyframes, send SPS/PPS first
			if sample.IsKey && len(h264Sps) > 0 && len(h264Pps) > 0 {
				t.videoTrack.WriteSample(media.Sample{Data: h264Sps, Duration: 0})
				t.videoTrack.WriteSample(media.Sample{Data: h264Pps, Duration: 0})
				decoderReady = true
			}

			// Decide whether to send frame
			shouldSendFrame := false
			if sample.IsKey {
				shouldSendFrame = true
				if !firstFrameSent {
					firstFrameSent = true
					decoderReady = true
				}
			} else if decoderReady {
				shouldSendFrame = true
			}

			if shouldSendFrame {
				frameSample := media.Sample{
					Data:     sample.Data,
					Duration: duration,
				}
				if err := t.videoTrack.WriteSample(frameSample); err != nil {
					log.Printf("Failed to write video sample: %v", err)
					return
				}
			}
		}
	}()

	// Audio: forward Opus packets as 20ms samples
	go func() {
		audioCh := source.SubscribeAudio("webrtc_transport", 100)
		defer source.UnsubscribeAudio("webrtc_transport")
		log.Printf("WebRTC audio processing started for device: %s", t.deviceSerial)

		sampleCount := 0
		for sample := range audioCh {
			if sample.Data == nil || len(sample.Data) == 0 || t.audioTrack == nil {
				continue
			}

			sampleCount++
			// Log every 5000th audio sample for debugging (roughly every 100 seconds) and only in verbose mode
			if sampleCount%5000 == 0 {
				logger := util.GetLogger()
				logger.Debug("WebRTC audio samples processed", "count", sampleCount)
			}

			if err := t.audioTrack.WriteSample(media.Sample{Data: sample.Data, Duration: 20 * time.Millisecond}); err != nil {
				log.Printf("Failed to write audio sample: %v", err)
				return
			}
		}
		log.Printf("WebRTC audio processing stopped for device: %s", t.deviceSerial)
	}()

	// Control: handle control messages via the core.Source interface
	if t.controlHandler != nil {
		// Set the source for control message sending
		t.controlHandler.SetSource(source)

		// Update screen dimensions from source (with retry if not available yet)
		_, width, height := source.GetConnectionInfo()
		if width == 0 || height == 0 {
			// Screen dimensions not available yet, will retry
			// Start a goroutine to update dimensions when available
			go func() {
				for i := 0; i < 10; i++ {
					time.Sleep(500 * time.Millisecond)
					_, w, h := source.GetConnectionInfo()
					if w > 0 && h > 0 {
						t.controlHandler.UpdateScreenDimensions(w, h)
						// Screen dimensions updated
						return
					}
				}
				log.Printf("Failed to get screen dimensions after retries")
			}()
		} else {
			t.controlHandler.UpdateScreenDimensions(width, height)
			log.Printf("Control handler configured with source and screen dimensions: %dx%d", width, height)
		}
	}

	return nil
}

// GetPeerConnection returns the WebRTC peer connection
func (t *Transport) GetPeerConnection() *webrtc.PeerConnection {
	return t.peerConnection
}

// Close closes the transport and all its connections
func (t *Transport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}
	t.closed = true

	// Cancel context
	if t.cancel != nil {
		t.cancel()
	}

	// Close WebRTC connection
	if t.peerConnection != nil {
		t.peerConnection.Close()
	}

	log.Printf("WebRTC transport closed for device: %s", t.deviceSerial)
	return nil
}


// min returns the minimum of two durations
func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
