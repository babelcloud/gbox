package device

import (
	"bytes"
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
	RegId          string `json:"reg_id"`
}

const (
	gboxRegIdSettingKey = "gbox_reg_id"
	gboxDeviceIDFileDir = "/sdcard/.gbox"
	gboxRegIdFilePath   = "/sdcard/.gbox/reg_id"
)

// Identifiers contains key identifiers for a device.
type Identifiers struct {
	SerialNo  string
	AndroidID string
	RegId     string
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

		// Check if device is connected via network
		if strings.Contains(deviceID, "._adb._tcp") {
			// mDNS service name (e.g., "adb-A4RYVB3A20008848._adb._tcp")
			device.ConnectionType = "mdns"
			// Keep the full mDNS name as device ID
		} else if strings.Contains(deviceID, ":") {
			// IP address with port (e.g., "192.168.1.100:5555")
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
		serialNo, err := m.getSerialNo(deviceID)
		if err != nil {
			log.Printf("Failed to get serialno of device %s: %v", deviceID, err)
			// Use device ID as fallback for serial number
			device.SerialNo = deviceID
			device.AndroidID = ""
		} else {
			device.SerialNo = serialNo
			androidID, err := m.getAndroidID(deviceID)
			if err != nil {
				log.Printf("Failed to get android id of device %s: %v", deviceID, err)
				device.AndroidID = ""
			} else {
				device.AndroidID = androidID
			}
		}

		// Get reg_id for this device (non-fatal if fails)
		regId, _ := m.GetRegId(deviceID)
		device.RegId = regId

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
			"model":                   device.Model, // Add model field for easy access
			"ro.product.model":        device.Model,
			"device":                  device.Manufacturer, // Add device field for easy access
			"ro.product.manufacturer": device.Manufacturer,
			"connectionType":          device.ConnectionType,
			"isRegistrable":           device.IsRegistrable,
			"gbox.reg_id":             device.RegId, // Add reg_id field from DeviceInfo
		}
	}

	return result, nil
}

// getSerialNo gets the device serial number
func (m *Manager) getSerialNo(deviceID string) (string, error) {
	cmd := exec.Command(m.adbPath, "-s", deviceID, "shell", "getprop", "ro.serialno")
	output, err := cmd.Output()
	if err != nil {
		return "", errors.Wrapf(err, "failed to get serialno of device %s", deviceID)
	}
	return strings.TrimSpace(string(output)), nil
}

// getAndroidID gets the device Android ID
func (m *Manager) getAndroidID(deviceID string) (string, error) {
	cmd := exec.Command(m.adbPath, "-s", deviceID, "shell", "settings", "get", "secure", "android_id")
	output, err := cmd.Output()
	if err != nil {
		return "", errors.Wrapf(err, "failed to get android id of device %s", deviceID)
	}
	return strings.TrimSpace(string(output)), nil
}

// GetIdentifiers returns device identifiers for the given device.
func (m *Manager) GetIdentifiers(deviceID string) (Identifiers, error) {
	serialNo, err := m.getSerialNo(deviceID)
	if err != nil {
		return Identifiers{}, err
	}

	androidID, err := m.getAndroidID(deviceID)
	if err != nil {
		return Identifiers{}, err
	}

	regId, _ := m.GetRegId(deviceID) // non-fatal
	return Identifiers{
		SerialNo:  serialNo,
		AndroidID: androidID,
		RegId:     regId,
	}, nil
}

// SetRegId writes a registration ID to the device.
// It first tries to write into Android settings (global). If that fails (e.g., permission denied),
// it falls back to writing a file on external storage.
func (m *Manager) SetRegId(deviceID string, regId string) error {
	// Try settings put global first
	putCmd := exec.Command(m.adbPath, "-s", deviceID, "shell", "settings", "put", "global", gboxRegIdSettingKey, regId)
	if err := putCmd.Run(); err == nil {
		// Verify from settings
		getCmd := exec.Command(m.adbPath, "-s", deviceID, "shell", "settings", "get", "global", gboxRegIdSettingKey)
		out, verr := getCmd.Output()
		if verr == nil {
			got := strings.TrimSpace(string(out))
			if got != "" && got != "null" && got == strings.TrimSpace(regId) {
				// Enforce single source of truth: delete file; if deletion fails, report error
				rmCmd := exec.Command(m.adbPath, "-s", deviceID, "shell", "rm", "-f", gboxRegIdFilePath)
				if err := rmCmd.Run(); err != nil {
					return errors.Wrap(err, "failed to delete fallback reg_id file after successful settings write")
				}
				return nil
			}
		}
		// if verification failed, fall through to file fallback
	}

	// Fallback: write to file only (do not attempt settings again)
	shell := fmt.Sprintf("mkdir -p %s && printf %s %s > %s",
		gboxDeviceIDFileDir, "%s", shellQuoteForSingle(regId), gboxRegIdFilePath)
	fileCmd := exec.Command(m.adbPath, "-s", deviceID, "shell", "sh", "-c", shell)
	if err := fileCmd.Run(); err != nil {
		return errors.Wrap(err, "failed to write reg id to file")
	}

	// Verify by reading the file
	readCmd := exec.Command(m.adbPath, "-s", deviceID, "shell", "cat", gboxRegIdFilePath)
	out, err := readCmd.Output()
	if err != nil {
		return errors.Wrap(err, "failed to read back reg id from file")
	}
	got := strings.TrimSpace(string(out))
	if got != strings.TrimSpace(regId) {
		return fmt.Errorf("verification failed (file): expected %q, got %q", regId, got)
	}
	return nil
}

// GetRegId reads the registration ID from settings or fallback file.
func (m *Manager) GetRegId(deviceID string) (string, error) {
	// Prefer file first
	readCmd := exec.Command(m.adbPath, "-s", deviceID, "shell", "cat", gboxRegIdFilePath)
	out, err := readCmd.Output()
	if err == nil {
		v := strings.TrimSpace(string(out))
		if v != "" {
			return v, nil
		}
	}

	// Then try settings
	getCmd := exec.Command(m.adbPath, "-s", deviceID, "shell", "settings", "get", "global", gboxRegIdSettingKey)
	out, err = getCmd.Output()
	if err != nil {
		return "", errors.Wrap(err, "failed to read reg id from settings")
	}
	v := strings.TrimSpace(string(out))
	if v == "null" {
		v = ""
	}
	return v, nil
}

// shellQuoteForSingle returns a single-quoted shell-safe string, handling embedded single quotes.
// e.g., abc'def -> 'abc'"'"'def'
func shellQuoteForSingle(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

type AdbCommandResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exitCode"`
}

func (m *Manager) ExecAdbCommand(deviceID, command string) (*AdbCommandResult, error) {
	cmd := exec.Command("sh", "-c", strings.Join([]string{m.adbPath, "-s", deviceID, command}, " "))

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	if err := cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return &AdbCommandResult{
				Stdout:   stdoutBuf.String(),
				Stderr:   stderrBuf.String(),
				ExitCode: exitError.ExitCode(),
			}, nil
		}
		return nil, errors.Wrapf(err, "failed to exec adb command on device %s", deviceID)
	}

	return &AdbCommandResult{
		Stdout:   stdoutBuf.String(),
		Stderr:   stderrBuf.String(),
		ExitCode: 0,
	}, nil
}
