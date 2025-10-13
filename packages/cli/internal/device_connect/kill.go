package device_connect

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// FindProcessesOnPort finds processes using a specific port
func FindProcessesOnPort(port int) ([]int, error) {
	var cmd *exec.Cmd
	var output []byte
	var err error

	switch runtime.GOOS {
	case "darwin", "linux":
		// Use lsof to find processes using the port
		cmd = exec.Command("lsof", "-ti", fmt.Sprintf(":%d", port))
		output, err = cmd.Output()
		if err != nil {
			// If lsof fails, try to find gbox-device-proxy processes by name
			return FindGboxDeviceProxyProcesses()
		}
		return parseLsofOutput(string(output))
	case "windows":
		// Use netstat on Windows
		cmd = exec.Command("netstat", "-ano")
		output, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("failed to find processes on port %d: %v", port, err)
		}
		return parseWindowsNetstatOutput(string(output), port)
	default:
		return nil, fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

func parseLsofOutput(output string) ([]int, error) {
	if output == "" {
		return []int{}, nil
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	var pids []int

	for _, line := range lines {
		if line == "" {
			continue
		}
		var pid int
		if _, err := fmt.Sscanf(line, "%d", &pid); err == nil {
			pids = append(pids, pid)
		}
	}

	return pids, nil
}

func parseNetstatOutput(output string, port int) ([]int, error) {
	lines := strings.Split(output, "\n")
	var pids []int

	for _, line := range lines {
		if strings.Contains(line, fmt.Sprintf(":%d", port)) {
			// Extract PID from the last field
			fields := strings.Fields(line)
			if len(fields) > 0 {
				lastField := fields[len(fields)-1]
				if strings.Contains(lastField, "/") {
					parts := strings.Split(lastField, "/")
					if len(parts) > 0 {
						var pid int
						if _, err := fmt.Sscanf(parts[0], "%d", &pid); err == nil {
							pids = append(pids, pid)
						}
					}
				}
			}
		}
	}

	return pids, nil
}

func parseWindowsNetstatOutput(output string, port int) ([]int, error) {
	lines := strings.Split(output, "\n")
	var pids []int

	for _, line := range lines {
		if strings.Contains(line, fmt.Sprintf(":%d", port)) {
			// Extract PID from the last field
			fields := strings.Fields(line)
			if len(fields) > 0 {
				lastField := fields[len(fields)-1]
				var pid int
				if _, err := fmt.Sscanf(lastField, "%d", &pid); err == nil {
					pids = append(pids, pid)
				}
			}
		}
	}

	return pids, nil
}

func FindGboxDeviceProxyProcesses() ([]int, error) {
	// Use ps to find gbox-device-proxy processes
	cmd := exec.Command("ps", "-ef")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to find gbox-device-proxy processes: %v", err)
	}

	lines := strings.Split(string(output), "\n")
	var pids []int

	for _, line := range lines {
		if strings.Contains(line, "gbox-device-proxy") && !strings.Contains(line, "grep") {
			fields := strings.Fields(line)
			if len(fields) > 1 {
				var pid int
				if _, err := fmt.Sscanf(fields[1], "%d", &pid); err == nil {
					pids = append(pids, pid)
				}
			}
		}
	}

	return pids, nil
}

// KillProcess kills a process by PID
func KillProcess(pid int, force bool) error {
    // On Unix, the device proxy is started in its own process group. To ensure
    // all child processes (e.g., frpc) are terminated, send the signal to the
    // process group using a negative PID first, then fall back to the single PID.
    if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
        // Try killing the entire process group
        if force {
            if err := exec.Command("kill", "-9", fmt.Sprintf("-%d", pid)).Run(); err == nil {
                return nil
            }
            // Fall back to killing only the leader
            return exec.Command("kill", "-9", fmt.Sprintf("%d", pid)).Run()
        }
        if err := exec.Command("kill", fmt.Sprintf("-%d", pid)).Run(); err == nil {
            return nil
        }
        return exec.Command("kill", fmt.Sprintf("%d", pid)).Run()
    }

    // Windows behavior unchanged
    var cmd *exec.Cmd
    if force {
        switch runtime.GOOS {
        case "windows":
            cmd = exec.Command("taskkill", "/F", "/PID", fmt.Sprintf("%d", pid))
        default:
            return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
        }
    } else {
        switch runtime.GOOS {
        case "windows":
            cmd = exec.Command("taskkill", "/PID", fmt.Sprintf("%d", pid))
        default:
            return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
        }
    }
    return cmd.Run()
}

// GetProcessCommand returns the command and arguments for a given process ID
func GetProcessCommand(pid int) (string, error) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("ps", "-p", fmt.Sprintf("%d", pid), "-o", "command=")
	case "linux":
		cmd = exec.Command("ps", "-p", fmt.Sprintf("%d", pid), "-o", "args=")
	case "windows":
		cmd = exec.Command("wmic", "process", "where", fmt.Sprintf("ProcessId=%d", pid), "get", "CommandLine", "/value")
	default:
		return "", fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	command := strings.TrimSpace(string(output))
	if command == "" {
		return "", fmt.Errorf("no command found for PID %d", pid)
	}

	return command, nil
}

// IsDeviceProxyProcess checks if a process is a device-proxy process by examining its command
func IsDeviceProxyProcess(pid int) bool {
	command, err := GetProcessCommand(pid)
	if err != nil {
		return false
	}

	// Check if the command contains "gbox-device-proxy"
	return strings.Contains(command, "gbox-device-proxy")
}
