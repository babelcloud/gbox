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

	// Server lifecycle
	Stop() error

	// ADB Expose methods
	StartPortForward(boxID string, localPorts, remotePorts []int) error
	StopPortForward(boxID string) error
	ListPortForwards() interface{}

	ConnectAP(serial string) error
	// ConnectAPWithDeviceId connects to AP using known serialKey (session key) and deviceId (UUID for token API). Use after register when both are known to avoid passing non-UUID to backend.
	ConnectAPWithDeviceId(serialKey, deviceId, deviceType, osType string) error
	DisconnectAP(serial string) error
	GetSerialByDeviceId(deviceId string) string        // Gets device serialno by device ID (supports both Android and desktop)
	GetDeviceInfo(serial string) interface{}           // Returns DeviceDTO or nil
	UpdateDeviceInfo(device interface{})               // Accepts DeviceDTO
	IsDeviceConnected(serial string) bool              // Checks if device is currently connected to AP
	GetDeviceReconnectState(serial string) interface{} // Returns reconnect state (isReconnecting, attempt, maxRetry)
	ReconnectRegisteredDevices() error                 // Reconnects all registered devices on server start
}

// Bridge defines the interface for device bridge operations
type Bridge interface {
	// Control event handlers
	HandleTouchEvent(msg map[string]interface{})
	HandleKeyEvent(msg map[string]interface{})
	HandleScrollEvent(msg map[string]interface{})
}
