package device

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// Manager manages Android devices
type Manager struct {
	adbPath string
	devices map[string]*DeviceInfo
	mu      sync.RWMutex
}

// DeviceInfo contains device information
type DeviceInfo struct {
	Serial          string
	State           string
	Model           string
	Manufacturer    string
	ConnectionType  string
	IsRegistered    bool
}

// NewManager creates a new device manager
func NewManager() *Manager {
	adbPath, err := exec.LookPath("adb")
	if err != nil {
		adbPath = "adb"
	}

	return &Manager{
		adbPath: adbPath,
		devices: make(map[string]*DeviceInfo),
	}
}

// GetDevices returns list of connected Android devices
func (m *Manager) GetDevices() ([]map[string]interface{}, error) {
	cmd := exec.Command(m.adbPath, "devices", "-l")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run adb devices: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	devices := []map[string]interface{}{}

	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		serial := parts[0]
		state := parts[1]

		if state != "device" {
			continue
		}

		device := map[string]interface{}{
			"id":             serial,
			"udid":           serial,
			"state":          state,
			"ro.serialno":    serial,
			"connectionType": "usb",
			"isRegistrable":  false,
		}

		// Parse additional properties
		if strings.Contains(line, "model:") {
			if idx := strings.Index(line, "model:"); idx != -1 {
				modelPart := line[idx+6:]
				if spaceIdx := strings.Index(modelPart, " "); spaceIdx != -1 {
					device["ro.product.model"] = modelPart[:spaceIdx]
				} else {
					device["ro.product.model"] = modelPart
				}
			}
		}

		if strings.Contains(line, "device:") {
			if idx := strings.Index(line, "device:"); idx != -1 {
				devicePart := line[idx+7:]
				if spaceIdx := strings.Index(devicePart, " "); spaceIdx != -1 {
					device["ro.product.manufacturer"] = devicePart[:spaceIdx]
				}
			}
		}

		if strings.Contains(serial, ":") {
			device["connectionType"] = "ip"
		}

		// Check if device is registered
		m.mu.RLock()
		if info, exists := m.devices[serial]; exists {
			device["isRegistrable"] = info.IsRegistered
		}
		m.mu.RUnlock()

		devices = append(devices, device)
	}

	return devices, nil
}

// RegisterDevice marks a device as registered
func (m *Manager) RegisterDevice(serial string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.devices[serial] == nil {
		m.devices[serial] = &DeviceInfo{
			Serial: serial,
		}
	}
	m.devices[serial].IsRegistered = true
}

// UnregisterDevice marks a device as unregistered
func (m *Manager) UnregisterDevice(serial string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if info, exists := m.devices[serial]; exists {
		info.IsRegistered = false
	}
}

// IsDeviceRegistered checks if a device is registered
func (m *Manager) IsDeviceRegistered(serial string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if info, exists := m.devices[serial]; exists {
		return info.IsRegistered
	}
	return false
}