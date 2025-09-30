package stream

// Muxer defines a unified interface for audio/video muxing
type Muxer interface {
	// Initialize sets up the muxer with video dimensions and codec parameters
	Initialize(width, height int, codecParams *CodecParams) error

	// Stream processes video and audio samples from channels
	Stream(videoCh <-chan VideoSample, audioCh <-chan AudioSample) error

	// Close cleans up resources
	Close() error
}

// VideoSample represents a video frame sample
type VideoSample struct {
	Data       []byte
	PTS        int64
	IsKeyFrame bool
}

// AudioSample represents an audio frame sample
type AudioSample struct {
	Data []byte
	PTS  int64
}

// CodecParams contains codec-specific parameters
type CodecParams struct {
	// Video codec parameters
	VideoSPS []byte
	VideoPPS []byte

	// Audio codec parameters
	AudioConfig interface{} // Can be mpeg4audio.AudioSpecificConfig for MP4, or nil for WebM
}
