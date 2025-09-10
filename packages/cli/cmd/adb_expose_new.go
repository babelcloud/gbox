package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/babelcloud/gbox/packages/cli/internal/daemon"
	"github.com/spf13/cobra"
)

// NewAdbExposeCommand creates the adb-expose command that uses the unified server
func NewAdbExposeCommandNew() *cobra.Command {
	var (
		localPort  int
		remotePort int
		device     string
		protocol   string
		list       bool
		remove     bool
	)

	cmd := &cobra.Command{
		Use:   "adb-expose",
		Short: "Expose ADB ports (similar to adb forward)",
		Long: `Expose ADB ports from Android device to local machine.
This is similar to 'adb forward' but managed by the gbox server.

The gbox server will be automatically started if not running.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// List all forwards
			if list {
				return listForwards()
			}

			// Remove forward
			if remove {
				if localPort == 0 {
					return fmt.Errorf("--local-port required for --remove")
				}
				return removeForward(device, localPort, remotePort)
			}

			// Add forward
			if localPort == 0 || remotePort == 0 {
				return fmt.Errorf("both --local-port and --remote-port are required")
			}

			return addForward(device, localPort, remotePort, protocol)
		},
		Example: `  # Forward port 8080 from device to local port 8080
  gbox adb-expose -l 8080 -r 8080

  # Forward with specific device
  gbox adb-expose -d emulator-5554 -l 8080 -r 8080

  # List all forwards
  gbox adb-expose --list

  # Remove a forward
  gbox adb-expose --remove -l 8080`,
	}

	flags := cmd.Flags()
	flags.IntVarP(&localPort, "local-port", "l", 0, "Local port to forward to")
	flags.IntVarP(&remotePort, "remote-port", "r", 0, "Remote port on device")
	flags.StringVarP(&device, "device", "d", "", "Target device serial")
	flags.StringVarP(&protocol, "protocol", "p", "tcp", "Protocol (tcp or unix)")
	flags.BoolVar(&list, "list", false, "List all port forwards")
	flags.BoolVar(&remove, "remove", false, "Remove a port forward")

	return cmd
}

func addForward(device string, localPort, remotePort int, protocol string) error {
	req := map[string]interface{}{
		"device_serial": device,
		"local_port":    localPort,
		"remote_port":   remotePort,
		"protocol":      protocol,
	}

	var resp map[string]interface{}
	if err := daemon.DefaultManager.CallAPI("POST", "/api/adb-expose/start", req, &resp); err != nil {
		return fmt.Errorf("failed to add forward: %v", err)
	}

	if success, ok := resp["success"].(bool); ok && success {
		fmt.Printf("Port forward added: %d -> %d\n", localPort, remotePort)
	} else {
		return fmt.Errorf("failed to add forward: %v", resp["error"])
	}

	return nil
}

func removeForward(device string, localPort, remotePort int) error {
	req := map[string]interface{}{
		"device_serial": device,
		"local_port":    localPort,
		"remote_port":   remotePort,
	}

	var resp map[string]interface{}
	if err := daemon.DefaultManager.CallAPI("POST", "/api/adb-expose/stop", req, &resp); err != nil {
		return fmt.Errorf("failed to remove forward: %v", err)
	}

	if success, ok := resp["success"].(bool); ok && success {
		fmt.Println("Port forward removed")
	} else {
		return fmt.Errorf("failed to remove forward: %v", resp["error"])
	}

	return nil
}

func listForwards() error {
	var resp map[string]interface{}
	if err := daemon.DefaultManager.CallAPI("GET", "/api/adb-expose/list", nil, &resp); err != nil {
		return fmt.Errorf("failed to list forwards: %v", err)
	}

	// Display all forwards
	if forwards, ok := resp["forwards"].([]interface{}); ok && len(forwards) > 0 {
		fmt.Println("Active port forwards:")
		for _, f := range forwards {
			if forward, ok := f.(map[string]interface{}); ok {
				device := forward["device_serial"].(string)
				local := forward["local"].(string)
				remote := forward["remote"].(string)
				
				// Parse ports if available
				localPort := ""
				remotePort := ""
				
				if lp, ok := forward["local_port"].(float64); ok {
					localPort = strconv.Itoa(int(lp))
				} else {
					// Extract from string like "tcp:8080"
					parts := strings.Split(local, ":")
					if len(parts) > 1 {
						localPort = parts[1]
					}
				}
				
				if rp, ok := forward["remote_port"].(float64); ok {
					remotePort = strconv.Itoa(int(rp))
				} else {
					// Extract from string like "tcp:8080"
					parts := strings.Split(remote, ":")
					if len(parts) > 1 {
						remotePort = parts[1]
					}
				}
				
				fmt.Printf("  %s: %s -> %s\n", device, localPort, remotePort)
			}
		}
	} else {
		fmt.Println("No active port forwards")
	}

	// Display managed forwards
	if managed, ok := resp["managed"].([]interface{}); ok && len(managed) > 0 {
		fmt.Println("\nManaged by gbox server:")
		for _, m := range managed {
			if forward, ok := m.(map[string]interface{}); ok {
				device := forward["device_serial"].(string)
				localPort := int(forward["local_port"].(float64))
				remotePort := int(forward["remote_port"].(float64))
				protocol := forward["protocol"].(string)
				
				fmt.Printf("  %s: %s:%d -> %s:%d\n", 
					device, "tcp", localPort, protocol, remotePort)
			}
		}
	}

	return nil
}