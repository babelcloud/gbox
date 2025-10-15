package handlers

import (
	"io/fs"
	"time"
)

// ServerService defines the interface for server operations that handlers need
type ServerService interface {
	// Status and info
	IsRunning() bool
	GetPort() int
	GetUptime() time.Duration
	GetBuildID() string
	GetVersion() string

	// Services status
	IsADBExposeRunning() bool

	// Bridge management
	ListBridges() []string
	CreateBridge(deviceSerial string) error
	RemoveBridge(deviceSerial string)
	GetBridge(deviceSerial string) (Bridge, bool)

	// Static file serving
	GetStaticFS() fs.FS
	FindLiveViewStaticPath() string
	FindStaticPath() string

	// Server lifecycle
	Stop() error

	// ADB Expose methods
	StartPortForward(boxID string, localPorts, remotePorts []int) error
	StopPortForward(boxID string) error
	ListPortForwards() interface{}

	ConnectAP(serial string) error
	DisconnectAP(serial string) error
	GetAdbSerialByGboxDeviceId(deviceId string) string
}

// Bridge defines the interface for device bridge operations
type Bridge interface {
	// Control event handlers
	HandleTouchEvent(msg map[string]interface{})
	HandleKeyEvent(msg map[string]interface{})
	HandleScrollEvent(msg map[string]interface{})
}
