//go:build !windows

package daemon

import (
	"os"
	"os/exec"
	"syscall"
)

// killProcess sends a signal to a process
func killProcess(pid int, signal syscall.Signal) error {
	return syscall.Kill(pid, signal)
}

// isProcessAlive checks if a process is still running
func isProcessAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

// setSysProcAttr sets platform-specific process attributes
func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
}

// getHomeDir returns the user's home directory
func getHomeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	return os.Getenv("USERPROFILE") // fallback for Windows-like environments
}