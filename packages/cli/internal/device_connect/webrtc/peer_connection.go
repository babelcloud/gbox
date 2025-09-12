package webrtc

import (
	"fmt"
	"log"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
)

// PeerConnectionConfig contains WebRTC peer connection configuration
type PeerConnectionConfig struct {
	VideoCodec string
	AudioCodec string
}

// CreatePeerConnection creates a new WebRTC peer connection
func CreatePeerConnection() (*webrtc.PeerConnection, error) {
	// Create a MediaEngine with codecs
	m := &webrtc.MediaEngine{}
	
	// Register video codecs
	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:     webrtc.MimeTypeH264,
			ClockRate:    90000,
			SDPFmtpLine:  "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42001f",
		},
		PayloadType: 96,
	}, webrtc.RTPCodecTypeVideo); err != nil {
		return nil, err
	}

	// Register audio codecs
	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:  webrtc.MimeTypeOpus,
			ClockRate: 48000,
			Channels:  2,
		},
		PayloadType: 111,
	}, webrtc.RTPCodecTypeAudio); err != nil {
		return nil, err
	}

	// Create the API with MediaEngine
	api := webrtc.NewAPI(webrtc.WithMediaEngine(m))

	// Create a new RTCPeerConnection with low latency configuration
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{},
	}

	pc, err := api.NewPeerConnection(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create peer connection: %w", err)
	}

	// Set up connection state logging
	pc.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		log.Printf("WebRTC Connection State: %s", s.String())
	})

	pc.OnICEConnectionStateChange(func(s webrtc.ICEConnectionState) {
		log.Printf("ICE Connection State: %s", s.String())
	})

	return pc, nil
}

// AddVideoTrack adds a video track to the peer connection
func AddVideoTrack(pc *webrtc.PeerConnection, codecType string) (*webrtc.TrackLocalStaticSample, error) {
	var videoTrack *webrtc.TrackLocalStaticSample
	var err error

	switch codecType {
	case "h264":
		videoTrack, err = webrtc.NewTrackLocalStaticSample(
			webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264},
			"video",
			"android-screen",
		)
	default:
		return nil, fmt.Errorf("unsupported video codec: %s", codecType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create video track: %w", err)
	}

	if _, err = pc.AddTrack(videoTrack); err != nil {
		return nil, fmt.Errorf("failed to add video track: %w", err)
	}

	log.Printf("Added %s video track", codecType)
	return videoTrack, nil
}

// AddAudioTrack adds an audio track to the peer connection
func AddAudioTrack(pc *webrtc.PeerConnection, codecType string) (*webrtc.TrackLocalStaticSample, error) {
	var audioTrack *webrtc.TrackLocalStaticSample
	var err error

	switch codecType {
	case "opus":
		audioTrack, err = webrtc.NewTrackLocalStaticSample(
			webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus},
			"audio",
			"android-audio",
		)
	default:
		return nil, fmt.Errorf("unsupported audio codec: %s", codecType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create audio track: %w", err)
	}

	if _, err = pc.AddTrack(audioTrack); err != nil {
		return nil, fmt.Errorf("failed to add audio track: %w", err)
	}

	log.Printf("Added %s audio track", codecType)
	return audioTrack, nil
}

// WriteSample writes a media sample to a track
func WriteSample(track *webrtc.TrackLocalStaticSample, data []byte, duration uint32) error {
	sample := media.Sample{
		Data:     data,
		Duration: time.Duration(duration) * time.Nanosecond,
	}
	
	return track.WriteSample(sample)
}