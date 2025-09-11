package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/babelcloud/gbox/packages/cli/config"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect"
	"github.com/spf13/cobra"
)

type DeviceConnectKillServerOptions struct {
	Force bool
	All   bool
}

func NewDeviceConnectKillServerCommand() *cobra.Command {
	opts := &DeviceConnectKillServerOptions{}

	cmd := &cobra.Command{
		Use:     "kill-server [flags]",
		Aliases: []string{"kill"},
		Short:   "Stop the device proxy service",
		Long:    "Stop the device proxy service running on port 19925.",
		Example: `  # Stop the device proxy service gracefully (PID file only)
  gbox device-connect kill-server

  # Force kill the device proxy service (PID file only)
  gbox device-connect kill-server --force

  # Kill all device proxy processes (port and name detection)
  gbox device-connect kill-server --all

  # Force kill all device proxy processes
  gbox device-connect kill-server --all --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ExecuteDeviceConnectKillServer(cmd, opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&opts.Force, "force", "f", false, "Force kill the service process")
	flags.BoolVarP(&opts.All, "all", "a", false, "Kill all device proxy processes (not just PID file)")

	return cmd
}

func ExecuteDeviceConnectKillServer(cmd *cobra.Command, opts *DeviceConnectKillServerOptions) error {
	// Check if PID file exists first
	cliCacheHome := config.GetCliCacheHome()
	pidFile := filepath.Join(cliCacheHome, "device-proxy.pid")

	pidFileExists := false
	var pidFromFile int
	if _, err := os.Stat(pidFile); err == nil {
		pidFileExists = true
		// Try to read PID from file
		if pidBytes, err := os.ReadFile(pidFile); err == nil {
			fmt.Sscanf(string(pidBytes), "%d", &pidFromFile)
		}
	}

	// Check if any device-proxy processes are currently running
	hasRunningProcesses := false
	if opts.All {
		portProcesses, _ := device_connect.FindProcessesOnPort(device_connect.DefaultPort)
		nameProcesses, _ := device_connect.FindGboxDeviceProxyProcesses()

		// Check if there are any actual device-proxy processes
		for _, pid := range portProcesses {
			if device_connect.IsDeviceProxyProcess(pid) {
				hasRunningProcesses = true
				break
			}
		}
		for range nameProcesses {
			hasRunningProcesses = true
			break
		}
	} else {
		// When not using --all, only check if the PID from file is still running
		if pidFileExists && pidFromFile > 0 {
			// Check if the process is still running
			if err := exec.Command("kill", "-0", fmt.Sprintf("%d", pidFromFile)).Run(); err == nil {
				hasRunningProcesses = true
			}
		}
	}

	// If no processes are running and no PID file exists, report that service is not running
	if !hasRunningProcesses && !pidFileExists {
		fmt.Println("Device proxy service is not running.")
		return nil
	}

	fmt.Println("Stopping device proxy service...")

	// Method 1: Always try to kill processes using PID file
	if pidFileExists {
		// PID file exists, try to kill the process
		pidBytes, err := os.ReadFile(pidFile)
		if err == nil {
			var pid int
			if _, err := fmt.Sscanf(string(pidBytes), "%d", &pid); err == nil {
				if err := device_connect.KillProcess(pid, opts.Force); err == nil {
					fmt.Printf("Killed process %d from PID file\n", pid)
				} else {
					fmt.Printf("Warning: failed to kill process %d from PID file: %v\n", pid, err)
				}
			}
		}
		// Remove PID file regardless of success
		os.Remove(pidFile)
	}

	// Only use port and name-based killing when --all flag is set
	if opts.All {
		// Method 2: Find and kill processes by port, but only if they are device-proxy processes
		portProcesses, err := device_connect.FindProcessesOnPort(device_connect.DefaultPort)
		if err == nil && len(portProcesses) > 0 {
			for _, pid := range portProcesses {
				// Check if this process is actually a device-proxy process
				if device_connect.IsDeviceProxyProcess(pid) {
					if err := device_connect.KillProcess(pid, opts.Force); err == nil {
						fmt.Printf("Killed process %d using port %d\n", pid, device_connect.DefaultPort)
					} else {
						fmt.Printf("Warning: failed to kill process %d using port %d: %v\n", pid, device_connect.DefaultPort, err)
					}
				}
			}
		}

		// Method 3: Find and kill processes by name
		nameProcesses, err := device_connect.FindGboxDeviceProxyProcesses()
		if err == nil && len(nameProcesses) > 0 {
			for _, pid := range nameProcesses {
				if err := device_connect.KillProcess(pid, opts.Force); err == nil {
					fmt.Printf("Killed process %d by name\n", pid)
				} else {
					fmt.Printf("Warning: failed to kill process %d by name: %v\n", pid, err)
				}
			}
		}
	}

	// Check if any device-proxy processes are still running (only when --all is used)
	if opts.All {
		remainingPortProcesses, _ := device_connect.FindProcessesOnPort(device_connect.DefaultPort)
		remainingNameProcesses, _ := device_connect.FindGboxDeviceProxyProcesses()

		// Filter out non-device-proxy processes from port processes
		var deviceProxyPortProcesses []int
		for _, pid := range remainingPortProcesses {
			if device_connect.IsDeviceProxyProcess(pid) {
				deviceProxyPortProcesses = append(deviceProxyPortProcesses, pid)
			}
		}

		if len(deviceProxyPortProcesses) == 0 && len(remainingNameProcesses) == 0 {
			fmt.Println("Device proxy service stopped successfully.")
			return nil
		} else {
			fmt.Println("Warning: Some device proxy processes may still be running:")

			// Show device-proxy processes found by port
			if len(deviceProxyPortProcesses) > 0 {
				fmt.Printf("  Device proxy processes using port %d:\n", device_connect.DefaultPort)
				for _, pid := range deviceProxyPortProcesses {
					if cmd, err := device_connect.GetProcessCommand(pid); err == nil {
						fmt.Printf("    PID %d: %s\n", pid, cmd)
					} else {
						fmt.Printf("    PID %d: <command not available>\n", pid)
					}
				}
			}

			// Show processes found by name
			if len(remainingNameProcesses) > 0 {
				fmt.Println("  Device proxy processes found by name:")
				for _, pid := range remainingNameProcesses {
					if cmd, err := device_connect.GetProcessCommand(pid); err == nil {
						fmt.Printf("    PID %d: %s\n", pid, cmd)
					} else {
						fmt.Printf("    PID %d: <command not available>\n", pid)
					}
				}
			}

			fmt.Println("Use 'gbox device-connect kill-server --all --force' to force kill all remaining processes.")
			return nil
		}
	} else {
		// When not using --all, just report success if PID file was handled
		fmt.Println("Device proxy service stopped successfully.")
		return nil
	}
}
