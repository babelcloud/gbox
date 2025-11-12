package core

// VideoSample represents a single video frame/sample.
type VideoSample struct {
	Data  []byte // H.264 NAL unit data
	IsKey bool   // Whether this is a keyframe (IDR)
	PTS   int64  // Presentation timestamp
}

// AudioSample represents a single audio frame/sample.
type AudioSample struct {
	Data []byte // Audio data (e.g., Opus)
	PTS  int64  // Presentation timestamp
}

// ControlMessage represents a control command or event.
type ControlMessage struct {
	Type int32  // Message type
	Data []byte // Message payload
}
