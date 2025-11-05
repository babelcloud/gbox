package util

import (
	"os"
	"os/exec"
	"strings"
)

// GetDesktopSerialNo gets the serial number for a desktop device based on OS type.
// For macOS, it tries to get the hardware serial number from system_profiler.
// For Windows, it tries to get the serial number from wmic.
// For Linux and other platforms, it uses the hostname.
// Returns the serial number or hostname as fallback.
func GetDesktopSerialNo(osType string) string {
	switch strings.ToLower(osType) {
	case "macos", "mac": // Support both "macos" and "mac" for backward compatibility
		// For macOS, get serial number from system_profiler
		cmd := exec.Command("system_profiler", "SPHardwareDataType")
		output, err := cmd.Output()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.Contains(line, "Serial Number") {
					parts := strings.Split(line, ":")
					if len(parts) >= 2 {
						serialno := strings.TrimSpace(parts[1])
						if serialno != "" {
							return serialno
						}
					}
				}
			}
		}
		// Fallback to hostname
		if hostname, err := os.Hostname(); err == nil {
			return hostname
		}
	case "windows":
		// For Windows, try to get serial number from wmic
		cmd := exec.Command("wmic", "bios", "get", "serialnumber")
		output, err := cmd.Output()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" && !strings.EqualFold(line, "SerialNumber") {
					return line
				}
			}
		}
		// Fallback to hostname
		if hostname, err := os.Hostname(); err == nil {
			return hostname
		}
	case "linux":
		// For Linux, try to get board serial number from DMI
		// Try with sudo first
		cmd := exec.Command("sudo", "cat", "/sys/class/dmi/id/board_serial")
		output, err := cmd.Output()
		if err == nil {
			serialno := strings.TrimSpace(string(output))
			if serialno != "" && serialno != "Not Specified" && serialno != "Default string" {
				return serialno
			}
		}
		// Try without sudo as fallback
		cmd = exec.Command("cat", "/sys/class/dmi/id/board_serial")
		output, err = cmd.Output()
		if err == nil {
			serialno := strings.TrimSpace(string(output))
			if serialno != "" && serialno != "Not Specified" && serialno != "Default string" {
				return serialno
			}
		}
		// Fallback to hostname
		if hostname, err := os.Hostname(); err == nil {
			return hostname
		}
	default:
		// Fallback to hostname for any other platform
		if hostname, err := os.Hostname(); err == nil {
			return hostname
		}
	}
	return ""
}

// DetectDesktopDeviceType detects if a desktop device is physical or virtual machine
// Returns "vm" for virtual machines, "physical" for physical devices
func DetectDesktopDeviceType(osType string) string {
	switch osType {
	case "linux":
		// Check DMI product name
		cmd := exec.Command("cat", "/sys/class/dmi/id/product_name")
		output, err := cmd.Output()
		if err == nil {
			productName := strings.ToLower(strings.TrimSpace(string(output)))
			if strings.Contains(productName, "vmware") ||
				strings.Contains(productName, "virtualbox") ||
				strings.Contains(productName, "qemu") ||
				strings.Contains(productName, "kvm") ||
				strings.Contains(productName, "parallels") ||
				strings.Contains(productName, "xen") {
				return "vm"
			}
		}

		// Check DMI sys vendor
		cmd = exec.Command("cat", "/sys/class/dmi/id/sys_vendor")
		output, err = cmd.Output()
		if err == nil {
			vendor := strings.ToLower(strings.TrimSpace(string(output)))
			if strings.Contains(vendor, "vmware") ||
				strings.Contains(vendor, "innotek") || // VirtualBox
				strings.Contains(vendor, "qemu") ||
				strings.Contains(vendor, "xen") {
				return "vm"
			}
		}

	case "macos":
		// Check system_profiler for virtualization
		cmd := exec.Command("system_profiler", "SPHardwareDataType")
		output, err := cmd.Output()
		if err == nil {
			outputStr := strings.ToLower(string(output))
			if strings.Contains(outputStr, "vmware") ||
				strings.Contains(outputStr, "parallels") ||
				strings.Contains(outputStr, "virtualbox") {
				return "vm"
			}
		}

	case "windows":
		// Check via WMI (can be enhanced)
		// For now, default to physical
	}

	return "physical"
}

// DetectAndroidDeviceType detects if an Android device is physical or emulator
// Returns "emulator" for emulators, "physical" for physical devices
func DetectAndroidDeviceType(deviceID, serialNo string) string {
	// Check serial number for emulator indicators
	serialUpper := strings.ToUpper(serialNo)
	if strings.Contains(serialUpper, "EMULATOR") {
		return "emulator"
	}

	// Try to detect via ADB properties
	adbPath, err := exec.LookPath("adb")
	if err != nil {
		adbPath = "adb"
	}

	// Check hardware property
	cmd := exec.Command(adbPath, "-s", deviceID, "shell", "getprop", "ro.hardware")
	output, err := cmd.Output()
	if err == nil {
		hardware := strings.TrimSpace(string(output))
		// Common emulator hardware types
		if hardware == "goldfish" || hardware == "ranchu" {
			return "emulator"
		}
	}

	// Check product brand
	cmd = exec.Command(adbPath, "-s", deviceID, "shell", "getprop", "ro.product.brand")
	output, err = cmd.Output()
	if err == nil {
		brand := strings.ToLower(strings.TrimSpace(string(output)))
		if brand == "generic" || brand == "unknown" {
			return "emulator"
		}
	}

	return "physical"
}

// GetMacOSChip gets the chip information from macOS
// Returns chip name (e.g., "Apple M4 Max") or "Unknown" if not available
func GetMacOSChip() string {
	// Try system_profiler first (more reliable for Apple Silicon)
	cmd := exec.Command("system_profiler", "SPHardwareDataType")
	output, err := cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, "Chip:") {
				parts := strings.Split(line, ":")
				if len(parts) >= 2 {
					return strings.TrimSpace(parts[1])
				}
			}
		}
	}
	// Fallback: try sysctl for Intel Macs
	cmd2 := exec.Command("sysctl", "-n", "machdep.cpu.brand_string")
	output2, err2 := cmd2.Output()
	if err2 == nil {
		return strings.TrimSpace(string(output2))
	}
	return "Unknown"
}
