package stream

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"sync"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/transport/h264"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/mpeg4audio"
	"github.com/bluenviron/mediacommon/v2/pkg/formats/fmp4"
	"github.com/bluenviron/mediacommon/v2/pkg/formats/fmp4/seekablebuffer"
	"github.com/bluenviron/mediacommon/v2/pkg/formats/mp4"
)

// scaleTimestampToTimescale converts a timestamp expressed in microseconds
// into the given MP4 track timescale units.
func scaleTimestampToTimescale(timestampUs int64, timeScale uint32) int64 {
	if timestampUs <= 0 {
		return 0
	}
	// Prevent overflow by doing 64-bit math
	return (timestampUs * int64(timeScale)) / 1_000_000
}

// stripADTSHeader removes the ADTS header if present and returns the raw AAC payload.
// If no ADTS header is detected, returns the original data.
func stripADTSHeader(data []byte) []byte {
	if len(data) < 7 {
		return data
	}
	// ADTS syncword 12 bits: 0xFFF
	if data[0] == 0xFF && (data[1]&0xF0) == 0xF0 {
		// protection_absent is the last bit of byte 1
		headerLen := 7
		if (data[1] & 0x01) == 0 { // CRC present => 2 extra bytes
			headerLen = 9
		}
		if len(data) > headerLen {
			return data[headerLen:]
		}
	}
	return data
}

// FMP4StreamWriter is an fMP4 writer for HTTP streaming.
// It is based on gohlslib but simplified for direct streaming.
type FMP4StreamWriter struct {
	writer         io.Writer
	logger         *slog.Logger
	videoTrack     *fmp4Track
	audioTrack     *fmp4Track
	initSent       bool
	mu             sync.Mutex // protects concurrent writes
	flusher        http.Flusher
	closed         bool
	sequenceNumber uint32
}

type fmp4Track struct {
	id        uint32
	codec     mp4.Codec
	clockRate uint32
	timeScale uint32
	lastDTS   int64 // in track timescale units
	firstDTS  int64 // in track timescale units
	sampleNum uint32
}

// NewFMP4StreamWriter creates a new fMP4 stream writer
func NewFMP4StreamWriter(writer io.Writer, logger *slog.Logger, videoWidth, videoHeight uint32) *FMP4StreamWriter {
	w := &FMP4StreamWriter{
		writer: writer,
		logger: logger,
		videoTrack: &fmp4Track{
			id:        1,
			clockRate: 90000,
			timeScale: 90000,
		},
		audioTrack: &fmp4Track{
			id:        2,
			clockRate: 48000,
			timeScale: 48000,
		},
		sequenceNumber: 1,
	}
	// Capture HTTP flusher if available
	if f, ok := writer.(http.Flusher); ok {
		w.flusher = f
	}
	return w
}

// WriteInitSegment writes the fMP4 initialization segment
func (w *FMP4StreamWriter) WriteInitSegment(videoSPS, videoPPS []byte, audioConfig mpeg4audio.AudioSpecificConfig) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.initSent {
		return nil
	}

	// Create video codec
	videoCodec := &mp4.CodecH264{
		SPS: videoSPS,
		PPS: videoPPS,
	}
	w.videoTrack.codec = videoCodec

	// Create audio codec
	audioCodec := &mp4.CodecMPEG4Audio{
		Config: audioConfig,
	}
	w.audioTrack.codec = audioCodec

	// Create fMP4 init segment
	init := &fmp4.Init{
		Tracks: []*fmp4.InitTrack{
			{
				ID:        int(w.videoTrack.id),
				TimeScale: w.videoTrack.timeScale,
				Codec:     videoCodec,
			},
			{
				ID:        int(w.audioTrack.id),
				TimeScale: w.audioTrack.timeScale,
				Codec:     audioCodec,
			},
		},
	}

	// Serialize init segment
	var buf seekablebuffer.Buffer
	err := init.Marshal(&buf)
	if err != nil {
		return fmt.Errorf("failed to marshal init segment: %w", err)
	}
	initBytes := buf.Bytes()

	// Write init segment
	if _, err := w.writer.Write(initBytes); err != nil {
		return fmt.Errorf("failed to write init segment: %w", err)
	}

	w.initSent = true
	w.logger.Info("fMP4 init segment written", "size", len(initBytes))
	if w.flusher != nil {
		w.flusher.Flush()
	}
	return nil
}

// WriteVideoFrame writes a video frame
func (w *FMP4StreamWriter) WriteVideoFrame(data []byte, pts int64, isKeyFrame bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("writer closed")
	}

	if !w.initSent {
		return fmt.Errorf("init segment not written yet")
	}

	if len(data) == 0 {
		w.logger.Debug("Skipping empty video frame", "pts", pts)
		return nil
	}

	// Convert Annex-B (start codes) to AVCC (length-prefixed) for MP4 compliance
	avcData, err := h264.ConvertAnnexBToAVC(data)
	if err != nil {
		return fmt.Errorf("failed to convert AnnexB to AVCC: %w", err)
	}
	if len(avcData) == 0 {
		w.logger.Debug("Skipping empty converted video frame", "pts", pts)
		return nil
	}

	// For keyframes, prepend SPS/PPS NAL units to improve decoder robustness
	if isKeyFrame {
		if c, ok := w.videoTrack.codec.(*mp4.CodecH264); ok && len(c.SPS) > 0 && len(c.PPS) > 0 {
			avcData = h264.PrependParameterSetsAVCC(avcData, c.SPS, c.PPS)
		}
	}

	// Create fMP4 sample
	sample := &fmp4.Sample{
		IsNonSyncSample: !isKeyFrame,
		Payload:         avcData,
	}

	// Scale PTS (microseconds) into track timescale (e.g., 90000)
	dts := scaleTimestampToTimescale(pts, w.videoTrack.timeScale)
	if w.videoTrack.firstDTS == 0 {
		w.videoTrack.firstDTS = dts
	}

	// Compute sample duration; if first frame, set a reasonable default (~30fps)
	if w.videoTrack.lastDTS != 0 {
		duration := dts - w.videoTrack.lastDTS
		if duration > 0 {
			sample.Duration = uint32(duration)
		}
	}
	if sample.Duration == 0 {
		// default to 30fps when duration not known yet
		sample.Duration = uint32(w.videoTrack.clockRate / 30)
	}

	// Create media part with mfhd/tfdt via Part API (sequence number set later by Marshal)
	segment := &fmp4.Part{
		Tracks: []*fmp4.PartTrack{
			{
				ID:       int(w.videoTrack.id),
				BaseTime: uint64(w.videoTrack.firstDTS), // tfdt for video track
				Samples:  []*fmp4.Sample{sample},
			},
		},
		SequenceNumber: w.sequenceNumber,
	}

	// Serialize media part
	var buf seekablebuffer.Buffer
	err = segment.Marshal(&buf)
	if err != nil {
		return fmt.Errorf("failed to marshal video segment: %w", err)
	}
	segmentBytes := buf.Bytes()

	// Write media part
	if _, err := w.writer.Write(segmentBytes); err != nil {
		w.logger.Error("Failed to write video segment", "error", err, "size", len(segmentBytes))
		return fmt.Errorf("failed to write video segment: %w", err)
	}

	w.videoTrack.lastDTS = dts
	w.videoTrack.sampleNum++
	w.sequenceNumber++

	w.logger.Debug("Video frame written", "pts", pts, "dts", dts, "isKeyFrame", isKeyFrame, "size", len(segmentBytes))
	if w.flusher != nil {
		w.flusher.Flush()
	}
	return nil
}

// WriteAudioFrame writes an audio frame
func (w *FMP4StreamWriter) WriteAudioFrame(data []byte, pts int64) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("writer closed")
	}

	if !w.initSent {
		return fmt.Errorf("init segment not written yet")
	}

	if len(data) == 0 {
		w.logger.Debug("Skipping empty audio frame", "pts", pts)
		return nil
	}

	// Some encoders may output ADTS frames; MP4 samples must be raw AAC
	data = stripADTSHeader(data)

	// Create fMP4 sample
	sample := &fmp4.Sample{
		IsNonSyncSample: false, // audio samples are typically sync samples
		Payload:         data,
	}

	// Compute DTS by converting microseconds to 48k timescale
	dts := scaleTimestampToTimescale(pts, w.audioTrack.timeScale)
	if w.audioTrack.firstDTS == 0 {
		w.audioTrack.firstDTS = dts
	}

	// Compute sample duration; AAC typically 1024 samples per frame
	if w.audioTrack.lastDTS != 0 {
		duration := dts - w.audioTrack.lastDTS
		if duration > 0 {
			sample.Duration = uint32(duration)
		}
	}
	if sample.Duration == 0 {
		sample.Duration = 1024
	}

	// Create media part with normalized base time
	baseAudio := int64(0)
	if w.audioTrack.firstDTS != 0 {
		baseAudio = dts - w.audioTrack.firstDTS
		if baseAudio < 0 {
			baseAudio = 0
		}
	}
	segment := &fmp4.Part{
		Tracks: []*fmp4.PartTrack{
			{
				ID:       int(w.audioTrack.id),
				BaseTime: uint64(baseAudio),
				Samples: []*fmp4.Sample{
					sample,
				},
			},
		},
		SequenceNumber: w.sequenceNumber,
	}

	// Serialize media part
	var buf seekablebuffer.Buffer
	err := segment.Marshal(&buf)
	if err != nil {
		return fmt.Errorf("failed to marshal audio segment: %w", err)
	}
	segmentBytes := buf.Bytes()

	// Write media part
	if _, err := w.writer.Write(segmentBytes); err != nil {
		w.logger.Error("Failed to write audio segment", "error", err, "size", len(segmentBytes))
		return fmt.Errorf("failed to write audio segment: %w", err)
	}

	w.audioTrack.lastDTS = dts
	w.audioTrack.sampleNum++
	w.sequenceNumber++

	w.logger.Debug("Audio frame written", "pts", pts, "dts", dts, "size", len(segmentBytes))
	if w.flusher != nil {
		w.flusher.Flush()
	}
	return nil
}

// WriteMixedFrame writes mixed video and audio frames in one part
func (w *FMP4StreamWriter) WriteMixedFrame(videoData []byte, audioData []byte, videoPTS, audioPTS int64, isKeyFrame bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("writer closed")
	}

	if !w.initSent {
		return fmt.Errorf("init segment not written yet")
	}

	// Convert H.264 Annex-B to AVCC
	if len(videoData) == 0 {
		return nil
	}
	avcData, err := h264.ConvertAnnexBToAVC(videoData)
	if err != nil {
		return fmt.Errorf("failed to convert AnnexB to AVCC: %w", err)
	}
	if len(avcData) == 0 {
		return nil
	}

	// Strip ADTS headers from AAC (MP4 requires raw AAC)
	if len(audioData) > 0 {
		audioData = stripADTSHeader(audioData)
	}

	// For keyframes, prepend SPS/PPS parameter sets
	if isKeyFrame {
		if c, ok := w.videoTrack.codec.(*mp4.CodecH264); ok && len(c.SPS) > 0 && len(c.PPS) > 0 {
			avcData = h264.PrependParameterSetsAVCC(avcData, c.SPS, c.PPS)
		}
	}
	// Create video sample
	videoSample := &fmp4.Sample{
		IsNonSyncSample: !isKeyFrame,
		Payload:         avcData,
	}

	// Create audio sample
	audioSample := &fmp4.Sample{
		IsNonSyncSample: false,
		Payload:         audioData,
	}

	// Compute DTS (scale microseconds into track timescales)
	videoDTS := scaleTimestampToTimescale(videoPTS, w.videoTrack.timeScale)
	audioDTS := scaleTimestampToTimescale(audioPTS, w.audioTrack.timeScale)

	if w.videoTrack.firstDTS == 0 {
		w.videoTrack.firstDTS = videoDTS
	}
	if w.audioTrack.firstDTS == 0 {
		w.audioTrack.firstDTS = audioDTS
	}

	// Compute sample duration
	if w.videoTrack.lastDTS != 0 {
		duration := videoDTS - w.videoTrack.lastDTS
		if duration > 0 {
			videoSample.Duration = uint32(duration)
		}
	}
	if videoSample.Duration == 0 {
		videoSample.Duration = uint32(w.videoTrack.clockRate / 30)
	}

	if w.audioTrack.lastDTS != 0 {
		duration := audioDTS - w.audioTrack.lastDTS
		if duration > 0 {
			audioSample.Duration = uint32(duration)
		}
	}
	if audioSample.Duration == 0 {
		audioSample.Duration = 1024
	}

	// Create mixed media part with base times and sequence number
	vBase := int64(0)
	if w.videoTrack.firstDTS != 0 {
		vBase = videoDTS - w.videoTrack.firstDTS
		if vBase < 0 {
			vBase = 0
		}
	}
	aBase := int64(0)
	if w.audioTrack.firstDTS != 0 {
		aBase = audioDTS - w.audioTrack.firstDTS
		if aBase < 0 {
			aBase = 0
		}
	}
	segment := &fmp4.Part{
		Tracks: []*fmp4.PartTrack{
			{ID: int(w.videoTrack.id), BaseTime: uint64(vBase), Samples: []*fmp4.Sample{videoSample}},
			{ID: int(w.audioTrack.id), BaseTime: uint64(aBase), Samples: []*fmp4.Sample{audioSample}},
		},
		SequenceNumber: w.sequenceNumber,
	}

	// Serialize media part
	var buf seekablebuffer.Buffer
	err = segment.Marshal(&buf)
	if err != nil {
		return fmt.Errorf("failed to marshal mixed segment: %w", err)
	}
	segmentBytes := buf.Bytes()

	// Write media part
	if _, err := w.writer.Write(segmentBytes); err != nil {
		return fmt.Errorf("failed to write mixed segment: %w", err)
	}

	w.videoTrack.lastDTS = videoDTS
	w.audioTrack.lastDTS = audioDTS
	w.videoTrack.sampleNum++
	w.audioTrack.sampleNum++
	w.sequenceNumber++

	if w.flusher != nil {
		w.flusher.Flush()
	}

	w.logger.Debug("Mixed frame written",
		"videoPTS", videoPTS, "audioPTS", audioPTS,
		"isKeyFrame", isKeyFrame, "size", len(segmentBytes))
	return nil
}

// VideoSampleInput represents one video sample to mux
type VideoSampleInput struct {
	Data  []byte
	PTS   int64
	IsKey bool
}

// AudioSampleInput represents one audio sample to mux
type AudioSampleInput struct {
	Data []byte
	PTS  int64
}

// WriteMixedBatch writes multiple video and audio samples in a single fragment
func (w *FMP4StreamWriter) WriteMixedBatch(videoSamples []VideoSampleInput, audioSamples []AudioSampleInput) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("writer closed")
	}
	if !w.initSent {
		return fmt.Errorf("init segment not written yet")
	}
	if len(videoSamples) == 0 && len(audioSamples) == 0 {
		return nil
	}

	// Sort by PTS to ensure monotonic order
	sort.Slice(videoSamples, func(i, j int) bool { return videoSamples[i].PTS < videoSamples[j].PTS })
	sort.Slice(audioSamples, func(i, j int) bool { return audioSamples[i].PTS < audioSamples[j].PTS })

	// Build track samples
	var vSamples []*fmp4.Sample
	var aSamples []*fmp4.Sample

	// Track DTS arrays for duration calculation
	var vDTS []int64
	var aDTS []int64

	// Video: convert to AVCC and scale PTS
	for _, vs := range videoSamples {
		if len(vs.Data) == 0 {
			continue
		}
		avcData, err := h264.ConvertAnnexBToAVC(vs.Data)
		if err != nil || len(avcData) == 0 {
			continue
		}
		dts := scaleTimestampToTimescale(vs.PTS, w.videoTrack.timeScale)
		vDTS = append(vDTS, dts)
		vSamples = append(vSamples, &fmp4.Sample{
			IsNonSyncSample: !vs.IsKey,
			Payload:         avcData,
		})
	}

	// Audio: strip ADTS, scale PTS
	for _, as := range audioSamples {
		if len(as.Data) == 0 {
			continue
		}
		raw := stripADTSHeader(as.Data)
		if len(raw) == 0 {
			continue
		}
		dts := scaleTimestampToTimescale(as.PTS, w.audioTrack.timeScale)
		aDTS = append(aDTS, dts)
		aSamples = append(aSamples, &fmp4.Sample{
			IsNonSyncSample: false,
			Payload:         raw,
		})
	}

	// Assign durations
	if len(vSamples) > 0 {
		prev := w.videoTrack.lastDTS
		for i := 0; i < len(vSamples); i++ {
			cur := vDTS[i]
			var next int64
			if i+1 < len(vDTS) {
				next = vDTS[i+1]
			} else {
				next = cur
			}
			dur := int64(0)
			if prev != 0 {
				dur = cur - prev
			} else if next > cur {
				dur = next - cur
			}
			if dur <= 0 {
				dur = int64(w.videoTrack.clockRate / 30)
			}
			vSamples[i].Duration = uint32(dur)
			prev = cur
		}
		w.videoTrack.lastDTS = vDTS[len(vDTS)-1]
		w.videoTrack.sampleNum += uint32(len(vSamples))
		if w.videoTrack.firstDTS == 0 && len(vDTS) > 0 {
			w.videoTrack.firstDTS = vDTS[0]
		}
	}

	if len(aSamples) > 0 {
		prev := w.audioTrack.lastDTS
		for i := 0; i < len(aSamples); i++ {
			cur := aDTS[i]
			var next int64
			if i+1 < len(aDTS) {
				next = aDTS[i+1]
			} else {
				next = cur
			}
			dur := int64(0)
			if prev != 0 {
				dur = cur - prev
			} else if next > cur {
				dur = next - cur
			}
			if dur <= 0 {
				dur = 1024
			}
			aSamples[i].Duration = uint32(dur)
			prev = cur
		}
		w.audioTrack.lastDTS = aDTS[len(aDTS)-1]
		w.audioTrack.sampleNum += uint32(len(aSamples))
		if w.audioTrack.firstDTS == 0 && len(aDTS) > 0 {
			w.audioTrack.firstDTS = aDTS[0]
		}
	}

	// Build part with both tracks
	part := &fmp4.Part{SequenceNumber: w.sequenceNumber, Tracks: []*fmp4.PartTrack{}}
	if len(vSamples) > 0 {
		// Set BaseTime to first video DTS relative to stream start
		baseTime := uint64(0)
		if w.videoTrack.firstDTS > 0 && len(vDTS) > 0 {
			relativeTime := vDTS[0] - w.videoTrack.firstDTS
			if relativeTime >= 0 {
				baseTime = uint64(relativeTime)
			}
		}
		part.Tracks = append(part.Tracks, &fmp4.PartTrack{
			ID:       int(w.videoTrack.id),
			BaseTime: baseTime,
			Samples:  vSamples,
		})
	}
	if len(aSamples) > 0 {
		// Set BaseTime to first audio DTS relative to stream start
		baseTime := uint64(0)
		if w.audioTrack.firstDTS > 0 && len(aDTS) > 0 {
			relativeTime := aDTS[0] - w.audioTrack.firstDTS
			if relativeTime >= 0 {
				baseTime = uint64(relativeTime)
			}
		}
		part.Tracks = append(part.Tracks, &fmp4.PartTrack{
			ID:       int(w.audioTrack.id),
			BaseTime: baseTime,
			Samples:  aSamples,
		})
	}

	var buf seekablebuffer.Buffer
	if err := part.Marshal(&buf); err != nil {
		return fmt.Errorf("failed to marshal mixed batch: %w", err)
	}
	bytes := buf.Bytes()
	if _, err := w.writer.Write(bytes); err != nil {
		return fmt.Errorf("failed to write mixed batch: %w", err)
	}
	if w.flusher != nil {
		w.flusher.Flush()
	}
	w.logger.Debug("Mixed batch written", "video", len(vSamples), "audio", len(aSamples), "size", len(bytes))
	// advance global sequence number for next fragment
	w.sequenceNumber++
	return nil
}

// Close closes the writer
func (w *FMP4StreamWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	w.closed = true
	w.logger.Info("FMP4 stream writer closed",
		"videoSamples", w.videoTrack.sampleNum,
		"audioSamples", w.audioTrack.sampleNum)
	return nil
}
