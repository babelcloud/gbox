package core

import (
	"context"
	"io"
)

// Source defines the interface for device video/audio/control sources.
type Source interface {
	// Start begins the source operation
	Start(ctx context.Context, deviceSerial string) error

	// Stop stops the source operation
	Stop() error

	// SubscribeVideo returns a channel for video samples
	SubscribeVideo(subscriberID string, bufferSize int) <-chan VideoSample

	// UnsubscribeVideo removes a video subscriber
	UnsubscribeVideo(subscriberID string)

	// SubscribeAudio returns a channel for audio samples
	SubscribeAudio(subscriberID string, bufferSize int) <-chan AudioSample

	// UnsubscribeAudio removes an audio subscriber
	UnsubscribeAudio(subscriberID string)

	// SubscribeControl returns a channel for control messages
	SubscribeControl(subscriberID string, bufferSize int) <-chan ControlMessage

	// UnsubscribeControl removes a control subscriber
	UnsubscribeControl(subscriberID string)

	// SendControl sends a control message to the device
	SendControl(msg ControlMessage) error

	// GetSpsPps returns cached SPS/PPS data for H.264 streams
	GetSpsPps() []byte

	// GetConnectionInfo returns device connection information
	GetConnectionInfo() (deviceSerial string, videoWidth, videoHeight int)
}

// StreamWriter defines the interface for writing stream data.
type StreamWriter interface {
	io.Writer
	io.Closer
}

// StreamReader defines the interface for reading stream data.
type StreamReader interface {
	io.Reader
	io.Closer
}
