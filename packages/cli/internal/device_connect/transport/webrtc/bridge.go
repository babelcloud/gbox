package webrtc

import (
	"context"
	"fmt"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/device"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/scrcpy"
	"github.com/pion/webrtc/v4"
)

// Bridge provides backward compatibility with the old WebRTC Bridge interface
// This adapter wraps the new Transport implementation
type Bridge struct {
	transport *Transport
	source    *scrcpy.Source

	// Backward compatibility fields
	DeviceSerial string
	VideoWidth   int
	VideoHeight  int
	WebRTCConn   *webrtc.PeerConnection
	DataChannel  *webrtc.DataChannel
	WSConnection interface{} // For WebSocket connection compatibility
}

// NewBridge creates a new WebRTC bridge for a device (backward compatibility)
func NewBridge(deviceSerial string, adbPath string) (*Bridge, error) {

	// Start scrcpy source with explicit webrtc mode
	src, err := scrcpy.StartSourceWithMode(deviceSerial, context.Background(), "webrtc")
	if err != nil {
		return nil, fmt.Errorf("failed to start scrcpy source: %w", err)
	}

	// Create new transport (pass nil pipeline for now)
	transport, err := NewTransport(deviceSerial, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create WebRTC transport: %w", err)
	}

	// Get device info
	deviceSerial, videoWidth, videoHeight := src.GetConnectionInfo()

	return &Bridge{
		transport:    transport,
		source:       src,
		DeviceSerial: deviceSerial,
		VideoWidth:   videoWidth,
		VideoHeight:  videoHeight,
		WebRTCConn:   transport.GetPeerConnection(),
		DataChannel:  nil, // Will be set when received
	}, nil
}

// Start starts the bridge connection to device
func (b *Bridge) Start() error {
	return b.transport.Start(b.source)
}

// Close closes the bridge and all its connections
func (b *Bridge) Close() error {
	if b.transport != nil {
		b.transport.Close()
	}
	// Clean up the scrcpy source
	if b.source != nil {
		b.source.Stop()
		// Remove from global manager to ensure clean state for reconnection
		scrcpy.RemoveSource(b.DeviceSerial)
	}
	return nil
}

// GetPeerConnection returns the WebRTC peer connection for signaling
func (b *Bridge) GetPeerConnection() *webrtc.PeerConnection {
	return b.transport.GetPeerConnection()
}

// Backward compatibility methods that delegate to transport
func (b *Bridge) SendControlMessage(msg *device.ControlMessage) error {
	// This is now handled by ControlHandler in the transport
	return nil
}

func (b *Bridge) HandleTouchEvent(message map[string]interface{}) {
	// This is now handled by ControlHandler in the transport
}

func (b *Bridge) HandleKeyEvent(message map[string]interface{}) {
	// This is now handled by ControlHandler in the transport
}

func (b *Bridge) HandleScrollEvent(message map[string]interface{}) {
	// This is now handled by ControlHandler in the transport
}

// Additional getters for backward compatibility
func (b *Bridge) GetDeviceSerial() string {
	return b.transport.deviceSerial
}

func (b *Bridge) GetVideoTrack() interface{} {
	return b.transport.videoTrack
}

func (b *Bridge) GetAudioTrack() interface{} {
	return b.transport.audioTrack
}
