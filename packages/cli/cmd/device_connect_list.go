package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/babelcloud/gbox/packages/cli/internal/daemon"
	"github.com/spf13/cobra"
)

const (
	statusRegistered    = "Registered"
	statusNotRegistered = "Not Registered"
	deviceTypeDevice    = "device"
	deviceTypeEmulator  = "emulator"
)

type DeviceConnectListOptions struct {
	OutputFormat string
}

func NewDeviceConnectListCommand() *cobra.Command {
	opts := &DeviceConnectListOptions{}

	cmd := &cobra.Command{
		Use:     "ls [flags]",
		Aliases: []string{"list"},
		Short:   "List all detectable local Android devices and their registration status",
		Long:    "List all detectable local Android devices and their registration status.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return ExecuteDeviceConnectList(cmd, opts)
		},
		Example: `  # List all local Android devices and their registration status:
  gbox device-connect ls

  # List devices in JSON format for scripting:
  gbox device-connect ls --format json`,
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.OutputFormat, "format", "", "text", "Output format: text (default) or json")

	cmd.RegisterFlagCompletionFunc("format", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"text", "json"}, cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

func ExecuteDeviceConnectList(cmd *cobra.Command, opts *DeviceConnectListOptions) error {
	if !checkAdbInstalled() {
		printAdbInstallationHint()
		return fmt.Errorf("ADB is not installed or not in your PATH; please install ADB and try again")
	}

	if !checkFrpcInstalled() {
		printFrpcInstallationHint()
		return fmt.Errorf("frpc is not installed or not in your PATH; please install frpc and try again")
	}

	// Use daemon manager to call unified server API
	var response struct {
		Success bool                     `json:"success"`
		Devices []map[string]interface{} `json:"devices"`
	}

	if err := daemon.DefaultManager.CallAPI("GET", "/api/devices", nil, &response); err != nil {
		return fmt.Errorf("failed to get available devices: %v", err)
	}

	if !response.Success {
		return fmt.Errorf("failed to get devices from server")
	}

	if opts.OutputFormat == "json" {
		return outputDevicesJSONFromAPI(response.Devices)
	}

	return outputDevicesTextFromAPI(response.Devices)
}

func outputDevicesJSONFromAPI(devices []map[string]interface{}) error {
	// Create a simplified JSON output for compatibility
	type SimpleDeviceInfo struct {
		DeviceID         string `json:"device_id"`
		Name             string `json:"name"`
		Type             string `json:"type"`
		ConnectionStatus string `json:"connection_status"`
	}

	var simpleDevices []SimpleDeviceInfo
	for _, device := range devices {
		deviceID, _ := device["id"].(string)
		name, _ := device["ro.product.model"].(string)
		serialNo, _ := device["ro.serialno"].(string)
		isRegistrable, _ := device["isRegistrable"].(bool)

		status := statusNotRegistered
		if isRegistrable {
			status = statusRegistered
		}

		deviceType := deviceTypeDevice
		// Check if it's an emulator based on serial number
		if strings.Contains(strings.ToUpper(serialNo), "EMULATOR") {
			deviceType = deviceTypeEmulator
		}

		simpleDevices = append(simpleDevices, SimpleDeviceInfo{
			DeviceID:         deviceID,
			Name:             name,
			Type:             deviceType,
			ConnectionStatus: status,
		})
	}

	jsonBytes, err := json.MarshalIndent(simpleDevices, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal devices to JSON: %v", err)
	}
	fmt.Println(string(jsonBytes))
	return nil
}

func outputDevicesTextFromAPI(devices []map[string]interface{}) error {
	if len(devices) == 0 {
		fmt.Println("No Android devices found.")
		return nil
	}

	// Calculate column widths based on content
	deviceIDWidth := len("DEVICE ID")
	nameWidth := len("NAME")
	typeWidth := len("TYPE")
	statusWidth := len("STATUS")

	// Find maximum widths for each column
	for _, device := range devices {
		deviceID, _ := device["id"].(string)
		name, _ := device["ro.product.model"].(string)

		if len(deviceID) > deviceIDWidth {
			deviceIDWidth = len(deviceID)
		}
		if len(name) > nameWidth {
			nameWidth = len(name)
		}
		if len(deviceTypeEmulator) > typeWidth {
			typeWidth = len(deviceTypeEmulator)
		}
		if len(statusNotRegistered) > statusWidth {
			statusWidth = len(statusNotRegistered)
		}
	}

	// Add some padding
	deviceIDWidth += 2
	nameWidth += 2
	typeWidth += 2
	statusWidth += 2

	// Print header
	fmt.Printf("%-*s %-*s %-*s %-*s\n",
		deviceIDWidth, "DEVICE ID",
		nameWidth, "NAME",
		typeWidth, "TYPE",
		statusWidth, "STATUS")

	// Print data rows
	for _, device := range devices {
		deviceID, _ := device["id"].(string)
		name, _ := device["ro.product.model"].(string)
		serialNo, _ := device["ro.serialno"].(string)
		isRegistrable, _ := device["isRegistrable"].(bool)

		status := statusNotRegistered
		if isRegistrable {
			status = statusRegistered
		}

		deviceType := deviceTypeDevice
		// Check if it's an emulator based on serial number
		if strings.Contains(strings.ToUpper(serialNo), "EMULATOR") {
			deviceType = deviceTypeEmulator
		}

		fmt.Printf("%-*s %-*s %-*s %-*s\n",
			deviceIDWidth, deviceID,
			nameWidth, name,
			typeWidth, deviceType,
			statusWidth, status)
	}

	return nil
}
