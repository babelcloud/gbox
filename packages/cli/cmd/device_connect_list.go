package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect"
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
		Example: `  # List all local Android devices and their registration status (default text format):
  gbox device-connect ls

  # List all local Android devices and their registration status in JSON format:
  gbox device-connect ls --format json`,
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.OutputFormat, "format", "", "text", "Specify output format. Options are \"text\" (default) or \"json\".")

	cmd.RegisterFlagCompletionFunc("format", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"text", "json"}, cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

func ExecuteDeviceConnectList(cmd *cobra.Command, opts *DeviceConnectListOptions) error {
	// Ensure device proxy service is running
	if err := device_connect.EnsureDeviceProxyRunning(isServiceRunning); err != nil {
		return fmt.Errorf("failed to start device proxy service: %v", err)
	}

	client := getDeviceClient()

	devices, err := client.GetDevices()
	if err != nil {
		return fmt.Errorf("failed to get available devices: %v", err)
	}

	if opts.OutputFormat == "json" {
		return outputDevicesJSON(devices)
	}

	return outputDevicesText(devices)
}

func outputDevicesJSON(devices []device_connect.DeviceInfo) error {
	// Create a simplified JSON output for compatibility
	type SimpleDeviceInfo struct {
		DeviceID         string `json:"device_id"`
		Name             string `json:"name"`
		Type             string `json:"type"`
		ConnectionStatus string `json:"connection_status"`
	}

	var simpleDevices []SimpleDeviceInfo
	for _, device := range devices {
		status := statusNotRegistered
		if device.IsRegistrable {
			status = statusRegistered
		}

		deviceType := deviceTypeDevice
		// Check if it's an emulator based on serial number
		if strings.Contains(strings.ToUpper(device.SerialNo), "EMULATOR") {
			deviceType = deviceTypeEmulator
		}

		simpleDevices = append(simpleDevices, SimpleDeviceInfo{
			DeviceID:         device.Udid,
			Name:             device.ProductModel,
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

func outputDevicesText(devices []device_connect.DeviceInfo) error {
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
		if len(device.Udid) > deviceIDWidth {
			deviceIDWidth = len(device.Udid)
		}
		if len(device.ProductModel) > nameWidth {
			nameWidth = len(device.ProductModel)
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
		status := statusNotRegistered
		if device.IsRegistrable {
			status = statusRegistered
		}

		deviceType := deviceTypeDevice
		// Check if it's an emulator based on serial number
		if strings.Contains(strings.ToUpper(device.SerialNo), "EMULATOR") {
			deviceType = deviceTypeEmulator
		}

		fmt.Printf("%-*s %-*s %-*s %-*s\n",
			deviceIDWidth, device.Id,
			nameWidth, device.ProductModel,
			typeWidth, deviceType,
			statusWidth, status)
	}

	return nil
}
