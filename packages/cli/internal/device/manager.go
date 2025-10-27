package device

import (
	"fmt"
	"log"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
)

// Manager manages Android devices
type Manager struct {
	adbPath string
}

// DeviceInfo contains device information
type DeviceInfo struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	SerialNo       string `json:"ro.serialno"`
	AndroidID      string `json:"android_id"`
	Model          string `json:"ro.product.model"`
	Manufacturer   string `json:"ro.product.manufacturer"`
	ConnectionType string `json:"connectionType"`
	IsRegistrable  bool   `json:"isRegistrable"`
}

// NewManager creates a new device manager
func NewManager() *Manager {
	adbPath, err := exec.LookPath("adb")
	if err != nil {
		adbPath = "adb"
	}

	return &Manager{
		adbPath: adbPath,
	}
}

// GetDevices returns list of connected Android devices
func (m *Manager) GetDevices() ([]DeviceInfo, error) {
	cmd := exec.Command(m.adbPath, "devices", "-l")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run adb devices: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	var devices []DeviceInfo

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "List of devices") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		deviceID := parts[0]
		status := parts[1]

		// Only include devices with "device" status
		if status != "device" {
			continue
		}

		device := DeviceInfo{
			ID:             deviceID,
			Status:         status,
			ConnectionType: "usb", // Default to USB connection
			IsRegistrable:  false, // Default to false, will be updated by caller if needed
		}

		// Check if device is connected via IP (contains ":")
		if strings.Contains(deviceID, ":") {
			device.ConnectionType = "ip"
		}

		// Parse additional device info if available
		if len(parts) > 2 {
			for _, part := range parts[2:] {
				if strings.Contains(part, ":") {
					kv := strings.SplitN(part, ":", 2)
					if len(kv) == 2 {
						// Map common fields to expected names
						switch kv[0] {
						case "model":
							device.Model = kv[1]
						case "device":
							device.Manufacturer = kv[1]
						}
					}
				}
			}
		}

		// Get serial number and Android ID
		serialNo, androidID, err := m.getDeviceSerialnoAndAndroidId(deviceID)
		if err != nil {
			log.Printf("Failed to get serialno of device %s: %v", deviceID, err)
			// Use device ID as fallback for serial number
			device.SerialNo = deviceID
			device.AndroidID = ""
		} else {
			device.SerialNo = serialNo
			device.AndroidID = androidID
		}

		devices = append(devices, device)
	}

	return devices, nil
}

// GetDevicesAsMap returns devices as map[string]interface{} for backward compatibility
func (m *Manager) GetDevicesAsMap() ([]map[string]interface{}, error) {
	devices, err := m.GetDevices()
	if err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, len(devices))
	for i, device := range devices {
		result[i] = map[string]interface{}{
			"id":                      device.ID,
			"udid":                    device.ID, // Use ID as UDID for compatibility
			"status":                  device.Status,
			"state":                   device.Status, // Add state field for compatibility
			"ro.serialno":             device.SerialNo,
			"android_id":              device.AndroidID,
			"ro.product.model":        device.Model,
			"ro.product.manufacturer": device.Manufacturer,
			"connectionType":          device.ConnectionType,
			"isRegistrable":           device.IsRegistrable,
		}
	}

	return result, nil
}

// getDeviceSerialnoAndAndroidId gets serial number and Android ID for a device
func (m *Manager) getDeviceSerialnoAndAndroidId(deviceID string) (serialno string, androidID string, err error) {
	getSerialnoCmd := exec.Command(m.adbPath, "-s", deviceID, "shell", "getprop", "ro.serialno")
	output, err := getSerialnoCmd.Output()
	if err != nil {
		err = errors.Wrapf(err, "failed to get serialno of device %s", deviceID)
		return
	}
	serialno = strings.TrimSpace(string(output))

	getAndroidIdCmd := exec.Command(m.adbPath, "-s", deviceID, "shell", "settings", "get", "secure", "android_id")
	output, err = getAndroidIdCmd.Output()
	if err != nil {
		err = errors.Wrapf(err, "failed to get android id of device %s", deviceID)
		return
	}
	androidID = strings.TrimSpace(string(output))

	return
}

// GetDeviceSerialnoAndAndroidId is a standalone function for backward compatibility
func GetDeviceSerialnoAndAndroidId(deviceID string) (serialno string, androidID string, err error) {
	manager := NewManager()
	return manager.getDeviceSerialnoAndAndroidId(deviceID)
}
