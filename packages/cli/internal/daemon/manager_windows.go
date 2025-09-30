//go:build windows

package daemon

import (
	"os"
	"os/exec"
	"syscall"
)

// killProcess sends a signal to a process on Windows
func killProcess(pid int, signal syscall.Signal) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Kill()
}

// isProcessAlive checks if a process is still running on Windows
func isProcessAlive(pid int) bool {
	_, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Windows, FindProcess always succeeds for valid PIDs
	// We need to actually try to open the process to check if it exists
	handle, err := syscall.OpenProcess(syscall.PROCESS_QUERY_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	syscall.CloseHandle(handle)
	return true
}

// setSysProcAttr sets platform-specific process attributes for Windows
func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}
}

// getHomeDir returns the user's home directory on Windows
func getHomeDir() string {
	if home := os.Getenv("USERPROFILE"); home != "" {
		return home
	}
	return os.Getenv("HOME") // fallback
}