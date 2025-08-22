package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/babelcloud/gbox/packages/cli/internal/adb_expose"
	"github.com/spf13/cobra"
)

// ExecuteAdbExposeList lists all running adb-expose processes
func ExecuteAdbExposeList(cmd *cobra.Command, opts *AdbExposeListOptions) error {
	// Step 1: Find all running gbox adb-expose processes (cross-platform, best effort)
	psCmd := exec.Command("ps", "aux")
	psOut, err := psCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to run ps aux: %v", err)
	}
	lines := strings.Split(string(psOut), "\n")
	var runningPids = make(map[int]bool)
	for _, line := range lines {
		if strings.Contains(line, "gbox adb-expose") && !strings.Contains(line, "grep") {
			// ignore gbox adb-expose list process itself
			if strings.Contains(line, "gbox adb-expose list") {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) > 1 {
				pid, err := strconv.Atoi(fields[1])
				if err == nil {
					runningPids[pid] = true
				}
			}
		}
	}
	// Step 2: List all pid files (registered adb-exposes)
	infos, err := adb_expose.ListPidFiles()
	if err != nil {
		return err
	}
	registeredPids := make(map[int]adb_expose.PidInfo)
	for _, info := range infos {
		registeredPids[info.Pid] = info
	}
	// Step 3: Check for running processes not in pid files
	for pid := range runningPids {
		if _, ok := registeredPids[pid]; !ok {
			fmt.Printf("[WARN] Found running adb-expose process (pid=%d) not in registry. If you want to stop it, run: gbox adb-expose stop <box_id>\n\n", pid)
		}
	}
	// Step 4: Check for pid files whose process is not running, and clean up
	for pid, info := range registeredPids {
		if !runningPids[pid] && !adb_expose.IsProcessAlive(pid) {
			fmt.Printf("[CLEANUP] Removing stale pid file for dead process (pid=%d, boxId=%s, localPorts=%v)\n", pid, info.BoxID, info.LocalPorts)
			for _, lp := range info.LocalPorts {
				adb_expose.RemovePidFile(info.BoxID, lp)
				adb_expose.RemoveLogFile(info.BoxID, lp)
			}
		}
	}
	// Step 5: For those pid files exist and process is running, check the box status, if the box is not running, clean up the pid file and kill the process
	for pid, info := range registeredPids {
		if runningPids[pid] && adb_expose.IsProcessAlive(pid) {
			if !boxValid(info.BoxID) {
				fmt.Printf("[CLEANUP] Box %s is not running, killing adb-expose process (pid=%d) and removing pid file(s)\n", info.BoxID, pid)
				proc, err := os.FindProcess(pid)
				if err == nil {
					proc.Kill()
				}
				for _, lp := range info.LocalPorts {
					adb_expose.RemovePidFile(info.BoxID, lp)
					adb_expose.RemoveLogFile(info.BoxID, lp)
				}
			}
		}
	}

	// Step 6: Print the current valid adb-exposes
	updatedInfos, err := adb_expose.ListPidFiles()
	if err != nil {
		return fmt.Errorf("failed to list pid files after cleanup: %v", err)
	}

	// Output based on format
	if opts.OutputFormat == "json" {
		printAdbExposeJSON(updatedInfos)
	} else {
		printAdbExposeTable(updatedInfos)
	}
	return nil
}

// printAdbExposeTable prints the ADB expose table in a formatted way
func printAdbExposeTable(infos []adb_expose.PidInfo) {
	if len(infos) == 0 {
		fmt.Println("No ADB port exposures found")
		return
	}

	fmt.Printf("| %-8s | %-36s | %-10s | %-8s | %-20s |\n", "PID", "BoxID", "Port", "Status", "StartedAt")
	fmt.Println("|----------|--------------------------------------|------------|----------|----------------------|")
	for _, info := range infos {
		status := "Dead"
		if adb_expose.IsProcessAlive(info.Pid) {
			status = "Alive"
		}
		for i := 0; i < len(info.LocalPorts); i++ {
			fmt.Printf("| %-8d | %-36s | %-10d | %-8s | %-20s |\n", info.Pid, info.BoxID, info.LocalPorts[i], status, info.StartedAt.Format("2006-01-02 15:04:05"))
		}
	}
}

// printAdbExposeJSON prints the ADB expose information in JSON format
func printAdbExposeJSON(infos []adb_expose.PidInfo) {
	// Debug: check if infos is nil or empty
	if infos == nil {
		fmt.Println("[]")
		return
	}

	type AdbExposeInfo struct {
		PID        int    `json:"pid"`
		BoxID      string `json:"boxId"`
		LocalPorts []int  `json:"localPorts"`
		Status     string `json:"status"`
		StartedAt  string `json:"startedAt"`
	}

	var jsonData []AdbExposeInfo
	for _, info := range infos {
		status := "Dead"
		if adb_expose.IsProcessAlive(info.Pid) {
			status = "Alive"
		}

		jsonInfo := AdbExposeInfo{
			PID:        info.Pid,
			BoxID:      info.BoxID,
			LocalPorts: info.LocalPorts,
			Status:     status,
			StartedAt:  info.StartedAt.Format("2006-01-02T15:04:05Z"),
		}
		jsonData = append(jsonData, jsonInfo)
	}

	// Ensure we always output a valid JSON array, even if empty
	jsonBytes, err := json.MarshalIndent(jsonData, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
		// Fallback to empty array if marshaling fails
		fmt.Println("[]")
		return
	}

	fmt.Println(string(jsonBytes))
}
