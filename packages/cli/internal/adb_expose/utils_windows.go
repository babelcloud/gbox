//go:build windows

package adb_expose

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// DaemonizeIfNeeded forks to background if foreground==false and not already daemonized.
// logPath: if not empty, background process logs to this file.
// boxID: the box ID for startup message.
// fromInteractive: indicates if this is called from interactive mode.
// Returns (shouldReturn, err): if shouldReturn==true, caller should return immediately (parent process or error).
func DaemonizeIfNeeded(foreground bool, logPath string, boxID string, fromInteractive bool) (bool, error) {
	if foreground || os.Getenv("GBOX_ADB_EXPOSE_DAEMON") != "" {
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
	// Prepare environment for child process with GBOX environment variables preserved
	env := PrepareGBOXEnvironment()
	env = append(env, "GBOX_ADB_EXPOSE_DAEMON=1")

	attr := &os.ProcAttr{
		Dir:   "",
		Env:   env,
		Files: []*os.File{os.Stdin, logFile, logFile},
		Sys:   &syscall.SysProcAttr{},
	}
	// For daemon mode, determine the command based on context
	var args []string
	if fromInteractive {
		// If called from interactive mode, use start subcommand
		args = []string{os.Args[0], "adb-expose", "start", boxID}
	} else {
		// If called from start subcommand, use the same command but with daemon flag
		args = os.Args
	}
	// Remove -f/--foreground from args if present
	newArgs := []string{}
	for i := 0; i < len(args); i++ {
		if args[i] == "-f" || args[i] == "--foreground" {
			continue
		}
		newArgs = append(newArgs, args[i])
	}

	// Resolve executable path robustly (PATH lookup + recursive symlink resolution)
	execPath := args[0]
	if !filepath.IsAbs(execPath) {
		if lp, err := exec.LookPath(execPath); err == nil {
			execPath = lp
		}
	}
	if abs, err := filepath.Abs(execPath); err == nil {
		if resolved, err := filepath.EvalSymlinks(abs); err == nil {
			execPath = resolved
		} else {
			execPath = abs
		}
	}

	proc, err := os.StartProcess(execPath, newArgs, attr)
	if err != nil {
		return true, fmt.Errorf("failed to daemonize: %v", err)
	}
	PrintStartupMessage(proc.Pid, logPath, boxID)
	return true, nil
}
