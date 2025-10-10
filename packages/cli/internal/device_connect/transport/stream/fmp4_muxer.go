package stream

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h264"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/mpeg4audio"
)

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// FMP4Muxer adapts FMP4StreamWriter to Muxer interface
type FMP4Muxer struct {
	*FMP4StreamWriter
	initialized bool
	mu          sync.Mutex
	codecParams *CodecParams
}

// NewFMP4Muxer creates a new FMP4 muxer
func NewFMP4Muxer(writer io.Writer, logger *slog.Logger) *FMP4Muxer {
	return &FMP4Muxer{
		FMP4StreamWriter: NewFMP4StreamWriter(writer, logger, 1920, 1080), // Default dimensions
	}
}

func (w *FMP4Muxer) Initialize(width, height int, params *CodecParams) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.initialized {
		return nil
	}

	// 更新尺寸
	w.videoTrack.clockRate = 90000
	w.audioTrack.clockRate = 48000

	// 如果已经有 SPS/PPS，直接初始化
	if params != nil && params.VideoSPS != nil && params.VideoPPS != nil && params.AudioConfig != nil {
		if audioConfig, ok := params.AudioConfig.(mpeg4audio.AudioSpecificConfig); ok {
			err := w.WriteInitSegment(params.VideoSPS, params.VideoPPS, audioConfig)
			if err == nil {
				w.initialized = true
			}
			return err
		}
	}

	// 保存参数，稍后在 Stream 中初始化
	w.codecParams = params

	w.initialized = true
	return nil
}

func (w *FMP4Muxer) Stream(videoCh <-chan VideoSample, audioCh <-chan AudioSample) error {
	w.mu.Lock()
	if !w.initialized {
		w.mu.Unlock()
		return fmt.Errorf("muxer not initialized")
	}
	w.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)

	// 使用现有的 WriteMixedBatch 逻辑进行批量处理
	go func() {
		defer wg.Done()
		defer w.logger.Info("FMP4 muxed aggregator ended")

		// 批量处理逻辑
		videoBatch := make([]VideoSampleInput, 0, 32)
		audioBatch := make([]AudioSampleInput, 0, 32)
		initDone := false

		ticker := time.NewTicker(150 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case sample, ok := <-videoCh:
				if !ok {
					return
				}

				// 如果还没有初始化，尝试从第一个视频帧中提取 SPS/PPS
				if !initDone && w.codecParams != nil {
					w.logger.Info("Attempting to initialize from video frame", "data_len", len(sample.Data), "first_bytes", sample.Data[:min(len(sample.Data), 10)])
					if err := w.initializeFromVideoFrame(sample.Data); err == nil {
						initDone = true
						w.logger.Info("FMP4 writer initialized from video frame")
					} else {
						w.logger.Info("Failed to initialize from video frame", "error", err)
					}
				}

				videoBatch = append(videoBatch, VideoSampleInput{
					Data:  sample.Data,
					PTS:   sample.PTS,
					IsKey: sample.IsKeyFrame,
				})
			case sample, ok := <-audioCh:
				if !ok {
					return
				}
				audioBatch = append(audioBatch, AudioSampleInput{
					Data: sample.Data,
					PTS:  sample.PTS,
				})
			case <-ticker.C:
				// 当有数据时发送批次
				if len(videoBatch) == 0 && len(audioBatch) == 0 {
					continue
				}
				if err := w.WriteMixedBatch(videoBatch, audioBatch); err != nil {
					w.logger.Error("Failed to write mixed batch", "error", err)
					cancel()
					return
				}
				videoBatch = videoBatch[:0]
				audioBatch = audioBatch[:0]
			}
		}
	}()

	wg.Wait()
	return nil
}

// initializeFromVideoFrame tries to extract SPS/PPS from video frame and initialize the writer
func (w *FMP4Muxer) initializeFromVideoFrame(data []byte) error {
	if w.codecParams == nil {
		return fmt.Errorf("no codec params available")
	}

	// Try to extract SPS/PPS from the video frame
	sps, pps := w.extractSpsPpsFromFrame(data)

	// If we have both SPS and PPS, initialize the writer
	if len(sps) > 0 && len(pps) > 0 && w.codecParams.AudioConfig != nil {
		if audioConfig, ok := w.codecParams.AudioConfig.(mpeg4audio.AudioSpecificConfig); ok {
			return w.WriteInitSegment(sps, pps, audioConfig)
		}
	}

	return fmt.Errorf("could not extract SPS/PPS from video frame")
}

// extractSpsPpsFromFrame extracts SPS/PPS from a video frame
func (w *FMP4Muxer) extractSpsPpsFromFrame(data []byte) ([]byte, []byte) {
	var sps, pps []byte

	w.logger.Info("Extracting SPS/PPS from frame", "data_len", len(data), "first_bytes", data[:min(len(data), 10)])

	// Try to parse as Annex-B format first (most common)
	if len(data) > 0 && data[0] == 0x00 {
		w.logger.Info("Detected Annex-B format")
		// Annex-B format - use the h264 package to parse
		var annexB h264.AnnexB
		err := annexB.Unmarshal(data)
		if err == nil {
			w.logger.Info("Successfully parsed Annex-B", "nalu_count", len(annexB))
			for _, nalu := range annexB {
				naluType := h264.NALUType(nalu[0] & 0x1F)
				w.logger.Info("Found NALU", "type", naluType, "size", len(nalu))
				switch naluType {
				case h264.NALUTypeSPS:
					sps = nalu
					w.logger.Info("Found SPS", "size", len(sps))
				case h264.NALUTypePPS:
					pps = nalu
					w.logger.Info("Found PPS", "size", len(pps))
				}
			}
		} else {
			w.logger.Warn("Failed to parse Annex-B", "error", err)
		}
	} else if len(data) > 0 && data[0] == 0x01 {
		// avcC format
		w.logger.Info("Detected avcC format")
		if ps, pp, ok := ParseAvccForSpsPps(data); ok {
			sps, pps = ps, pp
			w.logger.Info("Extracted SPS/PPS from avcC", "sps_size", len(sps), "pps_size", len(pps))
		} else {
			w.logger.Warn("Failed to parse avcC format")
		}
	} else {
		w.logger.Warn("Unknown format", "first_byte", data[0])
	}

	w.logger.Info("SPS/PPS extraction result", "sps_size", len(sps), "pps_size", len(pps))
	return sps, pps
}

// ParseAvccForSpsPps extracts SPS/PPS from avcC box payload
func ParseAvccForSpsPps(avcc []byte) (sps, pps []byte, ok bool) {
	if len(avcc) < 7 || avcc[0] != 0x01 {
		return nil, nil, false
	}
	// avcC layout: version(1)=1, profile(1), compatibility(1), level(1), lengthSizeMinusOne(2 bits), reserved(3 bits), numOfSPS(3 bits), then SPS (2 bytes len + data)... then numOfPPS(1), PPS...
	i := 5
	if i >= len(avcc) {
		return nil, nil, false
	}
	// Extract numOfSPS from the lower 3 bits
	numSps := int(avcc[i] & 0x07)
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
	// Check if we have enough data for PPS
	if i >= len(avcc) {
		// No PPS data available
		return sps, nil, false
	}

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

	// Only return ok=true if we have both SPS and PPS
	return sps, pps, sps != nil && pps != nil
}
