package cmd

import (
	"fmt"
	"os"
	"syscall"

	"github.com/babelcloud/gbox/packages/cli/internal/adb_expose"
	"github.com/spf13/cobra"
)

// ExecuteAdbExposeStop stops adb-expose processes for a specific box
func ExecuteAdbExposeStop(cmd *cobra.Command, opts *AdbExposeStopOptions, args []string) error {
	boxID := args[0]
	if boxID == "" {
		return fmt.Errorf("box ID is required. Usage: gbox adb-expose stop <box_id>")
	}

	// Find all running adb-expose processes for this box
	infos, err := adb_expose.ListPidFiles()
	if err != nil {
		return fmt.Errorf("failed to list pid files: %v", err)
	}

	var foundProcesses []int
	for _, info := range infos {
		if info.BoxID == boxID {
			foundProcesses = append(foundProcesses, info.Pid)
		}
	}

	if len(foundProcesses) == 0 {
		return fmt.Errorf("no running adb-expose processes found for box %s", boxID)
	}

	// Stop all processes for this box
	for _, pid := range foundProcesses {
		proc, err := os.FindProcess(pid)
		if err != nil {
			fmt.Printf("Warning: failed to find process %d: %v\n", pid, err)
			continue
		}

		// Find the specific process info for better output
		var processInfo *adb_expose.PidInfo
		for _, info := range infos {
			if info.Pid == pid {
				processInfo = &info
				break
			}
		}

		if processInfo != nil {
			fmt.Printf("Stopping adb-expose process %d for box %s (port %d)\n", pid, boxID, processInfo.LocalPorts[0])
		}

		err = proc.Signal(syscall.SIGTERM)
		if err != nil {
			fmt.Printf("Warning: failed to stop process %d: %v\n", pid, err)
		}
	}

	fmt.Printf("Successfully stopped all adb-expose processes for box %s\n", boxID)
	return nil
}
