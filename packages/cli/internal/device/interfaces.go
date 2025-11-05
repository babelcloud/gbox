package device

import (
	"os/exec"
	"strings"
)

// DeviceManager is the interface for device management operations
type DeviceManager interface {
	GetIdentifiers(deviceID string) (Identifiers, error)
	GetDisplayResolution(deviceID string) (int, int, error)
	GetOSVersion(deviceID string) (string, error)
	GetMemory(deviceID string) (string, error)
	SetRegId(deviceID string, regId string) error
	GetRegId(deviceID string) (string, error)
	GetDevices() ([]DeviceInfo, error)
}

// DeviceInfo contains device information
type DeviceInfo struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	SerialNo       string `json:"serialNo"`
	AndroidID      string `json:"androidId"`
	Model          string `json:"model"`
	Manufacturer   string `json:"manufacturer"`
	ConnectionType string `json:"connectionType"`
	IsRegistrable  bool   `json:"isRegistrable"`
	RegId          string `json:"regId"`
}

// Identifiers contains key identifiers for a device.
// For Android devices, all fields are populated.
// For Desktop devices, AndroidID is nil (not applicable).
type Identifiers struct {
	SerialNo  string
	AndroidID *string // nil for desktop devices, non-nil for Android devices
	RegId     string
}

// NewManager creates a new device manager based on osType
// osType can be "android", "linux", "windows", "macos", or empty (defaults to "android")
func NewManager(osType string) DeviceManager {
	if osType == "" {
		osType = "android"
	}

	switch strings.ToLower(osType) {
	case "android":
		adbPath, err := exec.LookPath("adb")
		if err != nil {
			adbPath = "adb"
		}
		return &AndroidManager{
			adbPath: adbPath,
		}
	case "linux", "windows", "macos":
		return &DesktopManager{
			osType: strings.ToLower(osType),
		}
	default:
		// Default to Android for backward compatibility
		adbPath, err := exec.LookPath("adb")
		if err != nil {
			adbPath = "adb"
		}
		return &AndroidManager{
			adbPath: adbPath,
		}
	}
}
