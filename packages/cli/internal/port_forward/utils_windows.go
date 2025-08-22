//go:build windows

package port_forward

import (
	"fmt"
	"os"
	"syscall"
)

// DaemonizeIfNeeded forks to background if foreground==false and not already daemonized.
// logPath: if not empty, background process logs to this file.
// Returns (shouldReturn, err): if shouldReturn==true, caller should return immediately (parent process or error).
func DaemonizeIfNeeded(foreground bool, logPath string) (bool, error) {
	if foreground || os.Getenv("GBOX_PORT_FORWARD_DAEMON") != "" {
		return false, nil
	}
	// open log file
	logFile := os.Stdout
	if logPath != "" {
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return true, fmt.Errorf("failed to open log file: %v", err)
		}
		logFile = f
		defer f.Close()
	}
	attr := &os.ProcAttr{
		Dir:   "",
		Env:   append(os.Environ(), "GBOX_PORT_FORWARD_DAEMON=1"),
		Files: []*os.File{os.Stdin, logFile, logFile},
		Sys:   &syscall.SysProcAttr{},
	}
	args := os.Args
	// Remove -f/--foreground from args if present
	newArgs := []string{}
	for i := 0; i < len(args); i++ {
		if args[i] == "-f" || args[i] == "--foreground" {
			continue
		}
		newArgs = append(newArgs, args[i])
	}
	proc, err := os.StartProcess(args[0], newArgs, attr)
	if err != nil {
		return true, fmt.Errorf("failed to daemonize: %v", err)
	}
	fmt.Printf("[gbox] Port-forward started in background (pid=%d). Logs: %s\nUse 'gbox port-forward list' to view, 'gbox port-forward kill <pid>' to stop.\n", proc.Pid, logPath)
	return true, nil
}
