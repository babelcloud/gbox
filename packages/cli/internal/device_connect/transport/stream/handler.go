package stream

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h264"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/mpeg4audio"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/scrcpy"
	"github.com/babelcloud/gbox/packages/cli/internal/util"
)

// parseAvccForSpsPps extracts SPS/PPS from avcC box payload
func parseAvccForSpsPps(avcc []byte) (sps, pps []byte, ok bool) {
	if len(avcc) < 7 || avcc[0] != 0x01 {
		return nil, nil, false
	}
	// avcC layout: version(1)=1, profile(1), compatibility(1), level(1), lengthSizeMinusOne(1), numOfSPS(1 & 0x1F), then SPS (2 bytes len + data)... then numOfPPS(1), PPS...
	i := 5
	if i >= len(avcc) {
		return nil, nil, false
	}
	numSps := int(avcc[i] & 0x1F)
	i++
	for n := 0; n < numSps && i+2 <= len(avcc); n++ {
		l := int(avcc[i])<<8 | int(avcc[i+1])
		i += 2
		if i+l > len(avcc) {
			return nil, nil, false
		}
		if l > 0 && sps == nil {
			sps = append([]byte{}, avcc[i:i+l]...)
		}
		i += l
	}
	if i >= len(avcc) {
		return sps, nil, sps != nil
	}
	if i < len(avcc) {
		numPps := int(avcc[i])
		i++
		for n := 0; n < numPps && i+2 <= len(avcc); n++ {
			l := int(avcc[i])<<8 | int(avcc[i+1])
			i += 2
			if i+l > len(avcc) {
				return sps, pps, sps != nil && pps != nil
			}
			if l > 0 && pps == nil {
				pps = append([]byte{}, avcc[i:i+l]...)
			}
			i += l
		}
	}
	return sps, pps, sps != nil && pps != nil
}

// WebMHandler handles HTTP-based WebM container streaming with H.264 video and Opus audio
type WebMHandler struct {
	deviceSerial string
}

// MP4Handler handles HTTP-based MP4 container streaming with H.264 video and Opus audio
type MP4Handler struct {
	deviceSerial string
}

// NewWebMHandler creates a new HTTP handler for WebM container streaming
func NewWebMHandler(deviceSerial string) *WebMHandler {
	return &WebMHandler{
		deviceSerial: deviceSerial,
	}
}

// NewMP4Handler creates a new HTTP handler for MP4 container streaming
func NewMP4Handler(deviceSerial string) *MP4Handler {
	return &MP4Handler{
		deviceSerial: deviceSerial,
	}
}

// ServeHTTP implements http.Handler for WebM container streaming
func (h *WebMHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := util.GetLogger()
	logger.Info("Starting WebM mixed stream", "device", h.deviceSerial)

	// Set headers for WebM streaming
	w.Header().Set("Content-Type", "video/webm; codecs=avc1.42E01E,opus")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Get or create scrcpy source with WebM mode (to select proper codecs)
	// Use background context so the shared source is not cancelled when this HTTP request ends
	source, err := scrcpy.StartSourceWithMode(h.deviceSerial, context.Background(), "webm")
	if err != nil {
		logger.Error("Failed to start scrcpy source", "device", h.deviceSerial, "error", err)
		http.Error(w, fmt.Sprintf("Failed to start stream: %v", err), http.StatusInternalServerError)
		return
	}

	// Generate unique subscriber IDs for this connection
	videoSubscriberID := fmt.Sprintf("webm_video_%d", time.Now().UnixNano())
	audioSubscriberID := fmt.Sprintf("webm_audio_%d", time.Now().UnixNano())

	// Subscribe to video and audio streams (increase buffer to reduce backpressure)
	videoCh := source.SubscribeVideo(videoSubscriberID, 1000)
	defer source.UnsubscribeVideo(videoSubscriberID)

	audioCh := source.SubscribeAudio(audioSubscriberID, 1000)
	defer source.UnsubscribeAudio(audioSubscriberID)

	// Get device video dimensions from scrcpy source
	_, videoWidth, videoHeight := source.GetConnectionInfo()
	logger.Info("Device video dimensions", "width", videoWidth, "height", videoHeight)

	// Create WebM muxer with actual device dimensions
	webmMuxer := NewWebMMuxerWithDimensions(w, videoWidth, videoHeight)
	defer webmMuxer.Close()

	// Initialize WebM muxer with video and audio tracks
	if err := webmMuxer.WriteHeader(); err != nil {
		logger.Error("Failed to initialize WebM muxer", "device", h.deviceSerial, "error", err)
		http.Error(w, "Failed to initialize WebM muxer", http.StatusInternalServerError)
		return
	}

	// Start streaming
	logger.Info("WebM mixed stream started", "device", h.deviceSerial)

	// Channel to coordinate video and audio streaming
	done := make(chan struct{})

	// Start video streaming goroutine
	go func() {
		defer close(done)

		for {
			select {
			case <-r.Context().Done():
				logger.Info("WebM video stream context cancelled", "device", h.deviceSerial)
				return

			case sample, ok := <-videoCh:
				if !ok {
					logger.Info("WebM video channel closed", "device", h.deviceSerial)
					return
				}

				// Write H.264 video data to WebM container
				if err := webmMuxer.WriteVideoFrame(sample.Data, time.Duration(sample.PTS)*time.Nanosecond); err != nil {
					logger.Error("Failed to write video data to WebM", "device", h.deviceSerial, "error", err)
					return
				}
			}
		}
	}()

	// Start audio streaming goroutine
	go func() {
		for {
			select {
			case <-r.Context().Done():
				logger.Info("WebM audio stream context cancelled", "device", h.deviceSerial)
				return

			case sample, ok := <-audioCh:
				if !ok {
					logger.Info("WebM audio channel closed", "device", h.deviceSerial)
					return
				}

				// Write Opus audio data to WebM container
				if err := webmMuxer.WriteAudioFrame(sample.Data, time.Duration(sample.PTS)*time.Nanosecond); err != nil {
					logger.Error("Failed to write audio data to WebM", "device", h.deviceSerial, "error", err)
					return
				}
			}
		}
	}()

	// Wait for completion
	<-done
	logger.Info("WebM mixed stream completed", "device", h.deviceSerial)
}

// ServeHTTP implements http.Handler for MP4 container streaming
func (h *MP4Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := util.GetLogger()
	logger.Info("Starting fMP4 stream", "device", h.deviceSerial)

	// Set headers for fMP4 streaming (avoid locking codecs string)
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Get or create scrcpy source with MP4 mode (to select proper codecs)
	// Use background context so the shared source is not cancelled when this HTTP request ends
	source, err := scrcpy.StartSourceWithMode(h.deviceSerial, context.Background(), "mp4")
	if err != nil {
		logger.Error("Failed to start scrcpy source", "device", h.deviceSerial, "error", err)
		http.Error(w, fmt.Sprintf("Failed to start stream: %v", err), http.StatusInternalServerError)
		return
	}

	// Generate unique subscriber IDs for this connection
	videoSubscriberID := fmt.Sprintf("fmp4_video_%d", time.Now().UnixNano())
	audioSubscriberID := fmt.Sprintf("fmp4_audio_%d", time.Now().UnixNano())

	// Subscribe to video and audio streams (larger buffers to reduce backpressure)
	videoCh := source.SubscribeVideo(videoSubscriberID, 1000)
	defer source.UnsubscribeVideo(videoSubscriberID)

	audioCh := source.SubscribeAudio(audioSubscriberID, 1000)
	defer source.UnsubscribeAudio(audioSubscriberID)

	// Get device video dimensions from scrcpy source
	_, videoWidth, videoHeight := source.GetConnectionInfo()
	logger.Info("Device video dimensions", "width", videoWidth, "height", videoHeight)

	// Create fMP4 stream writer
	fmp4Writer := NewFMP4StreamWriter(w, logger, uint32(videoWidth), uint32(videoHeight))

	// Try to get SPS/PPS from cached data first (like WebRTC does)
	var sps, pps []byte
	var spsPpsExtracted bool

	// Poll cached SPS/PPS for a short time to avoid forcing keyframe/reset
	pollStart := time.Now()
	for !spsPpsExtracted && time.Since(pollStart) < 3*time.Second {
		spsPpsData := source.GetSpsPps()
		if len(spsPpsData) > 0 {
			// Parse SPS/PPS from cached data, support both Annex-B and avcC
			if len(spsPpsData) > 0 && spsPpsData[0] == 0x01 {
				// avcC format
				if ps, pp, ok := parseAvccForSpsPps(spsPpsData); ok {
					sps, pps = ps, pp
				}
			}
			if len(sps) == 0 || len(pps) == 0 {
				// Try Annex-B split
				startCode := []byte{0x00, 0x00, 0x00, 0x01}
				parts := bytes.Split(spsPpsData, startCode)
				for i := 1; i < len(parts); i++ {
					nal := parts[i]
					if len(nal) == 0 {
						continue
					}
					nalType := nal[0] & 0x1F
					switch nalType {
					case 7:
						sps = nal
						logger.Debug("Found SPS from cache", "device", h.deviceSerial, "size", len(sps))
					case 8:
						pps = nal
						logger.Debug("Found PPS from cache", "device", h.deviceSerial, "size", len(pps))
					}
				}
			}
			if len(sps) > 0 && len(pps) > 0 {
				spsPpsExtracted = true
				logger.Info("SPS/PPS extracted from cache", "device", h.deviceSerial, "sps_size", len(sps), "pps_size", len(pps))
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}

	if !spsPpsExtracted {
		// Fallback: extract SPS/PPS from video stream
		logger.Info("No cached SPS/PPS, extracting from video stream", "device", h.deviceSerial)

		maxAttempts := 500 // allow more attempts for SPS/PPS
	extractLoop:
		for i := 0; i < maxAttempts; i++ {
			select {
			case sample, ok := <-videoCh:
				if !ok {
					logger.Error("No video sample received", "device", h.deviceSerial)
					http.Error(w, "No video sample received", http.StatusInternalServerError)
					return
				}

				// Parse H.264 data to extract SPS/PPS
				var annexB h264.AnnexB
				err = annexB.Unmarshal(sample.Data)
				if err == nil {
					for _, nalu := range annexB {
						naluType := h264.NALUType(nalu[0] & 0x1F)
						switch naluType {
						case h264.NALUTypeSPS:
							sps = nalu
							logger.Debug("Found SPS from stream", "device", h.deviceSerial, "size", len(sps))
						case h264.NALUTypePPS:
							pps = nalu
							logger.Debug("Found PPS from stream", "device", h.deviceSerial, "size", len(pps))
						}
					}
				} else if len(sample.Data) > 0 && sample.Data[0] == 0x01 {
					// Possibly avcC config packet
					if ps, pp, ok := parseAvccForSpsPps(sample.Data); ok {
						sps, pps = ps, pp
						logger.Info("Extracted SPS/PPS from avcC config packet", "device", h.deviceSerial, "sps_size", len(sps), "pps_size", len(pps))
					} else {
						logger.Debug("Failed to parse H.264 data, trying next sample", "device", h.deviceSerial, "error", err)
					}
				} else {
					logger.Debug("Failed to parse H.264 data, trying next sample", "device", h.deviceSerial, "error", err)
				}

				// If both SPS and PPS are found, exit the loop
				if len(sps) > 0 && len(pps) > 0 {
					spsPpsExtracted = true
					logger.Info("SPS/PPS extracted from stream", "device", h.deviceSerial, "sps_size", len(sps), "pps_size", len(pps))
					break extractLoop
				}

			case <-r.Context().Done():
				logger.Error("Context cancelled while waiting for SPS/PPS", "device", h.deviceSerial)
				http.Error(w, "Context cancelled", http.StatusRequestTimeout)
				return
			}
		}
	}

	if len(sps) == 0 || len(pps) == 0 {
		logger.Error("Failed to extract SPS/PPS", "device", h.deviceSerial)
		http.Error(w, "Failed to extract SPS/PPS", http.StatusInternalServerError)
		return
	}

	// Wait for first video sample
	firstVideoSample, ok := <-videoCh
	if !ok {
		logger.Error("No video sample received", "device", h.deviceSerial)
		http.Error(w, "No video sample received", http.StatusInternalServerError)
		return
	}

	// Configure audio codec for AAC
	audioConfig := mpeg4audio.AudioSpecificConfig{
		Type:         2, // AAC
		SampleRate:   48000,
		ChannelCount: 2,
	}

	// Write initialization segment
	err = fmp4Writer.WriteInitSegment(sps, pps, audioConfig)
	if err != nil {
		logger.Error("Failed to write init segment", "device", h.deviceSerial, "error", err)
		http.Error(w, "Failed to write init segment", http.StatusInternalServerError)
		return
	}

	// Flush headers + init segment to client to reduce startup latency
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	// Start streaming
	logger.Info("fMP4 stream started", "device", h.deviceSerial)

	// Use context cancellation to coordinate streaming
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	var wg sync.WaitGroup

	// Aggregator: align audio/video into ~150ms chunks, batching all samples in window
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer logger.Info("fMP4 muxed aggregator ended", "device", h.deviceSerial)

		// helper: detect IDR
		isIDR := func(data []byte) bool {
			var ab h264.AnnexB
			if ab.Unmarshal(data) == nil {
				for _, n := range ab {
					if h264.NALUType(n[0]&0x1F) == h264.NALUTypeIDR {
						return true
					}
				}
			}
			return false
		}

		// staging buffers (batch all samples within window)
		videoBatch := make([]VideoSampleInput, 0, 32)
		audioBatch := make([]AudioSampleInput, 0, 32)
		videoBatch = append(videoBatch, VideoSampleInput{Data: firstVideoSample.Data, PTS: firstVideoSample.PTS, IsKey: isIDR(firstVideoSample.Data)})

		ticker := time.NewTicker(150 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case s, ok := <-videoCh:
				if !ok {
					return
				}
				videoBatch = append(videoBatch, VideoSampleInput{Data: s.Data, PTS: s.PTS, IsKey: isIDR(s.Data)})
			case s, ok := <-audioCh:
				if !ok {
					return
				}
				audioBatch = append(audioBatch, AudioSampleInput{Data: s.Data, PTS: s.PTS})
			case <-ticker.C:
				// emit batch when we have anything
				if len(videoBatch) == 0 && len(audioBatch) == 0 {
					continue
				}
				if err := fmp4Writer.WriteMixedBatch(videoBatch, audioBatch); err != nil {
					logger.Error("Failed to write mixed batch", "error", err)
					cancel()
					return
				}
				videoBatch = videoBatch[:0]
				audioBatch = audioBatch[:0]
			}
		}
	}()

	<-ctx.Done()
	// Wait aggregator goroutine to exit before closing writer
	wg.Wait()
	_ = fmp4Writer.Close()
	logger.Info("fMP4 stream completed", "device", h.deviceSerial)
}
