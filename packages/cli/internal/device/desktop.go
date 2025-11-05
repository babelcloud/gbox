package device

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// DesktopManager manages desktop devices (implements DeviceManager)
type DesktopManager struct {
	osType string
}

// GetIdentifiers returns device identifiers for desktop devices
func (m *DesktopManager) GetIdentifiers(deviceID string) (Identifiers, error) {
	// Desktop devices don't have Android ID
	// Try to get regId from local file
	regId, _ := m.GetRegId(deviceID) // non-fatal
	return Identifiers{
		SerialNo:  deviceID, // Use deviceID as serialno for desktop
		AndroidID: nil,      // Desktop devices don't have Android ID
		RegId:     regId,
	}, nil
}

// GetDisplayResolution returns the primary/built-in display resolution for desktop devices
func (m *DesktopManager) GetDisplayResolution(deviceID string) (int, int, error) {
	switch m.osType {
	case "macos":
		return getMacOSDisplayResolution()
	case "linux":
		return getLinuxDisplayResolution()
	case "windows":
		return getWindowsDisplayResolution()
	default:
		return 0, 0, fmt.Errorf("unsupported OS type: %s", m.osType)
	}
}

// getMacOSDisplayResolution gets the primary display resolution on macOS
func getMacOSDisplayResolution() (int, int, error) {
	// Use system_profiler to get display info
	// Priority: 1. Built-in display, 2. Main Display, 3. First display
	cmd := exec.Command("system_profiler", "SPDisplaysDataType")
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, err
	}

	lines := strings.Split(string(output), "\n")
	var builtInResolution string
	var mainDisplayResolution string
	var firstResolution string

	// Track current display context
	type displayContext struct {
		isBuiltIn     bool
		isMainDisplay bool
		resolution    string
	}

	var currentDisplay displayContext
	displays := []displayContext{}

	// Parse output line by line
	// Display sections start with "Displays:" and each display name is indented with spaces/tabs
	inDisplaysSection := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Detect when we enter the Displays section
		if strings.Contains(trimmed, "Displays:") {
			inDisplaysSection = true
			continue
		}

		// Only process lines within Displays section
		if !inDisplaysSection {
			continue
		}

		// Detect new display section (indented line ending with colon, like "        Mi 27 NU:")
		// This is a display name line (has indentation and ends with colon, but no other content)
		if strings.HasSuffix(trimmed, ":") && (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")) {
			// Check if this looks like a display name (simple name, not a key-value pair)
			// Display names typically don't have spaces before the colon in the key part
			namePart := strings.TrimSuffix(trimmed, ":")
			if !strings.Contains(namePart, ":") && namePart != "" {
				// Save previous display if it has resolution
				if currentDisplay.resolution != "" {
					displays = append(displays, currentDisplay)
				}
				// Start new display context
				currentDisplay = displayContext{}
				continue
			}
		}

		// Check if current display is built-in
		if strings.Contains(trimmed, "Display Type: Built-in") || strings.Contains(trimmed, "Built-in: Yes") {
			currentDisplay.isBuiltIn = true
		}

		// Check if current display is main display
		if strings.Contains(trimmed, "Main Display: Yes") {
			currentDisplay.isMainDisplay = true
		}

		// Extract resolution from Resolution line
		if strings.Contains(trimmed, "Resolution:") {
			parts := strings.Split(trimmed, ":")
			if len(parts) >= 2 {
				res := strings.TrimSpace(parts[1])
				// Extract resolution (e.g., "3840 x 2160", "3456 x 2234 Retina")
				// Remove text after resolution like "(2160p/4K UHD 1 - Ultra High Definition)" or "Retina"
				resParts := strings.Fields(res)
				// Find the first two numeric fields (skip "x" if present)
				var widthStr, heightStr string
				for _, part := range resParts {
					// Skip non-numeric parts like "x", "Retina", etc.
					if _, err := strconv.Atoi(part); err == nil {
						if widthStr == "" {
							widthStr = part
						} else if heightStr == "" {
							heightStr = part
							break // Found both width and height
						}
					}
				}
				if widthStr != "" && heightStr != "" {
					currentDisplay.resolution = widthStr + "x" + heightStr
				}
			}
		}
	}

	// Save last display if it has resolution
	if currentDisplay.resolution != "" {
		displays = append(displays, currentDisplay)
	}

	// Find resolutions based on priority
	for _, display := range displays {
		if display.resolution == "" {
			continue
		}
		if firstResolution == "" {
			firstResolution = display.resolution
		}
		if display.isBuiltIn && builtInResolution == "" {
			builtInResolution = display.resolution
		}
		if display.isMainDisplay && mainDisplayResolution == "" {
			mainDisplayResolution = display.resolution
		}
	}

	// Use priority: Built-in > Main Display > First
	var resolution string
	if builtInResolution != "" {
		resolution = builtInResolution
	} else if mainDisplayResolution != "" {
		resolution = mainDisplayResolution
	} else if firstResolution != "" {
		resolution = firstResolution
	}

	if resolution == "" {
		return 0, 0, fmt.Errorf("could not determine display resolution")
	}

	// Parse resolution string (e.g., "3840x2160")
	parts := strings.Split(resolution, "x")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid resolution format: %s", resolution)
	}

	width, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid width: %s", parts[0])
	}

	height, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid height: %s", parts[1])
	}

	return width, height, nil
}

// getLinuxDisplayResolution gets the primary display resolution on Linux
func getLinuxDisplayResolution() (int, int, error) {
	// Try xrandr first (most common)
	cmd := exec.Command("xrandr")
	output, err := cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			// Look for line with "*" (current resolution) or "connected primary"
			if strings.Contains(line, "connected primary") || strings.Contains(line, "*") {
				// Parse resolution from line like "   1920x1080     60.00*+"
				fields := strings.Fields(line)
				for _, field := range fields {
					if strings.Contains(field, "x") {
						parts := strings.Split(field, "x")
						if len(parts) == 2 {
							width, err1 := strconv.Atoi(parts[0])
							height, err2 := strconv.Atoi(strings.TrimSuffix(parts[1], "*+"))
							if err1 == nil && err2 == nil {
								return width, height, nil
							}
						}
					}
				}
			}
		}
	}

	// Fallback: try wayland-info or other methods
	return 0, 0, fmt.Errorf("could not determine display resolution")
}

// getWindowsDisplayResolution gets the primary display resolution on Windows
func getWindowsDisplayResolution() (int, int, error) {
	// Use PowerShell to get display resolution
	cmd := exec.Command("powershell", "-Command", "Get-WmiObject -Class Win32_VideoController | Select-Object -First 1 | Select-Object -ExpandProperty CurrentHorizontalResolution, CurrentVerticalResolution")
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, err
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) >= 2 {
		width, err1 := strconv.Atoi(strings.TrimSpace(lines[0]))
		height, err2 := strconv.Atoi(strings.TrimSpace(lines[1]))
		if err1 == nil && err2 == nil {
			return width, height, nil
		}
	}

	return 0, 0, fmt.Errorf("could not determine display resolution")
}

// SetRegId writes regId to local file for desktop devices
func (m *DesktopManager) SetRegId(deviceID string, regId string) error {
	return writeLocalRegId(regId)
}

// GetRegId reads regId from local file for desktop devices
func (m *DesktopManager) GetRegId(deviceID string) (string, error) {
	return readLocalRegId()
}

// GetDevices returns empty list for desktop devices (not applicable)
func (m *DesktopManager) GetDevices() ([]DeviceInfo, error) {
	return []DeviceInfo{}, nil
}

// GetOSVersion returns the OS version for desktop devices
func (m *DesktopManager) GetOSVersion(deviceID string) (string, error) {
	switch m.osType {
	case "macos":
		cmd := exec.Command("sw_vers", "-productVersion")
		output, err := cmd.Output()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(output)), nil
	case "linux":
		// Try to get distribution version from /etc/os-release
		cmd := exec.Command("cat", "/etc/os-release")
		output, err := cmd.Output()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "VERSION_ID=") {
					version := strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), "\"")
					return version, nil
				}
			}
		}
		return "", fmt.Errorf("failed to get Linux version")
	case "windows":
		// Use PowerShell to get Windows version
		cmd := exec.Command("powershell", "-Command", "(Get-CimInstance Win32_OperatingSystem).Version")
		output, err := cmd.Output()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(output)), nil
	default:
		return "", fmt.Errorf("unsupported OS type: %s", m.osType)
	}
}

// GetMemory returns the total memory in GB for desktop devices
func (m *DesktopManager) GetMemory(deviceID string) (string, error) {
	switch m.osType {
	case "macos":
		// Try system_profiler first
		cmd := exec.Command("system_profiler", "SPHardwareDataType")
		output, err := cmd.Output()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.Contains(line, "Memory:") {
					parts := strings.Split(line, ":")
					if len(parts) >= 2 {
						return strings.TrimSpace(parts[1]), nil
					}
				}
			}
		}
		// Fallback: calculate from sysctl
		cmd2 := exec.Command("sysctl", "-n", "hw.memsize")
		output2, err2 := cmd2.Output()
		if err2 == nil {
			memBytes, err := strconv.ParseInt(strings.TrimSpace(string(output2)), 10, 64)
			if err == nil {
				memGB := float64(memBytes) / (1024 * 1024 * 1024)
				return fmt.Sprintf("%.0f GB", memGB), nil
			}
		}
		return "", fmt.Errorf("failed to get macOS memory")
	case "linux":
		// Read from /proc/meminfo
		cmd := exec.Command("cat", "/proc/meminfo")
		output, err := cmd.Output()
		if err != nil {
			return "", err
		}
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "MemTotal:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					memKB, err := strconv.ParseInt(fields[1], 10, 64)
					if err == nil {
						memGB := float64(memKB) / (1024 * 1024)
						return fmt.Sprintf("%.0f GB", memGB), nil
					}
				}
			}
		}
		return "", fmt.Errorf("failed to parse memory information")
	case "windows":
		// Use PowerShell to get total memory
		cmd := exec.Command("powershell", "-Command", "$totalRAM = (Get-CimInstance Win32_ComputerSystem).TotalPhysicalMemory; [math]::Round($totalRAM / 1GB, 0)")
		output, err := cmd.Output()
		if err != nil {
			return "", err
		}
		memGB := strings.TrimSpace(string(output))
		return fmt.Sprintf("%s GB", memGB), nil
	default:
		return "", fmt.Errorf("unsupported OS type: %s", m.osType)
	}
}

// getLocalRegIdPath returns the file path for storing the reg_id on this machine.
func getLocalRegIdPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".gbox")
	return filepath.Join(dir, "reg_id"), nil
}

// readLocalRegId reads reg_id from ~/.gbox/reg_id if exists.
func readLocalRegId() (string, error) {
	path, err := getLocalRegIdPath()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	// Trim trailing spaces/newlines
	s := strings.TrimSpace(string(data))
	return s, nil
}

// writeLocalRegId writes reg_id into ~/.gbox/reg_id, creating directory if needed.
func writeLocalRegId(regId string) error {
	path, err := getLocalRegIdPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(regId+"\n"), 0o600)
}
