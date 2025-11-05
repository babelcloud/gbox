package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/babelcloud/gbox/packages/cli/internal/daemon"
	"github.com/babelcloud/gbox/packages/cli/internal/util"
	"github.com/spf13/cobra"
)

const (
	statusRegistered    = "Registered"
	statusNotRegistered = "Not Registered"
)

type DeviceConnectListOptions struct {
	OutputFormat string
}

// DeviceDTO is the API response structure for devices
type DeviceDTO struct {
	ID           string                 `json:"id"`
	TransportID  string                 `json:"transportId"`
	Serialno     string                 `json:"serialno"`
	AndroidID    string                 `json:"androidId"`
	Platform     string                 `json:"platform"`   // mobile, desktop
	OS           string                 `json:"os"`         // android, linux, windows, macos
	DeviceType   string                 `json:"deviceType"` // physical, emulator, vm
	IsRegistered bool                   `json:"isRegistered"`
	RegId        string                 `json:"regId"`
	IsLocal      bool                   `json:"isLocal"`  // true if this is the local desktop device
	Metadata     map[string]interface{} `json:"metadata"` // Device-specific metadata
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
		Success bool        `json:"success"`
		Devices []DeviceDTO `json:"devices"`
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

func outputDevicesJSONFromAPI(devices []DeviceDTO) error {
	// Output full DeviceDTO with all fields
	jsonBytes, err := json.MarshalIndent(devices, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal devices to JSON: %v", err)
	}
	fmt.Println(string(jsonBytes))
	return nil
}

func outputDevicesTextFromAPI(devices []DeviceDTO) error {
	if len(devices) == 0 {
		fmt.Println("No devices found.")
		return nil
	}

	// Build rows for sorting
	type row struct {
		serial            string
		deviceID          string
		serialOrTransport string
		os                string
		deviceType        string
		status            string
	}
	rows := make([]row, 0, len(devices))
	for _, device := range devices {
		deviceID := device.ID
		serialNo := device.Serialno
		transportID := device.TransportID
		isRegistered := device.IsRegistered

		status := statusNotRegistered
		if isRegistered {
			// Green for Registered
			status = "\x1b[32m" + statusRegistered + "\x1b[0m"
		}

		// Get OS and DeviceType from device
		os := device.OS
		if os == "" {
			os = "-"
		}

		// Get osVersion from metadata and combine with OS
		osVersion := ""
		if device.Metadata != nil {
			if ov, ok := device.Metadata["osVersion"].(string); ok && ov != "" {
				osVersion = ov
			}
		}

		// Format OS display: capitalize first letter and handle macOS
		osDisplay := os
		if os != "-" && os != "" {
			osLower := strings.ToLower(os)
			switch osLower {
			case "android":
				osDisplay = "Android"
			case "macos":
				osDisplay = "MacOS"
			case "linux":
				osDisplay = "Linux"
			case "windows":
				osDisplay = "Windows"
			default:
				// Capitalize first letter
				if len(os) > 0 {
					osDisplay = strings.ToUpper(os[:1]) + strings.ToLower(os[1:])
				}
			}

			// Append version if available
			if osVersion != "" {
				osDisplay = fmt.Sprintf("%s %s", osDisplay, osVersion)
			}
		}

		deviceType := device.DeviceType
		if deviceType == "" {
			deviceType = "-"
		}

		// Get connectionType from metadata for Android devices
		connectionType := ""
		if device.Metadata != nil {
			if ct, ok := device.Metadata["connectionType"].(string); ok {
				connectionType = ct
			}
		}

		// Second column: for USB show Serial No; otherwise show Transport ID (full value)
		serialOrTransport := transportID
		if strings.EqualFold(connectionType, "usb") && strings.TrimSpace(serialNo) != "" {
			serialOrTransport = serialNo
		}

		// DEVICE ID should be the remote cloud device ID. Fallback to "-" when empty
		uniqueDeviceID := deviceID
		if strings.TrimSpace(uniqueDeviceID) == "" {
			uniqueDeviceID = "-"
		}

		rows = append(rows, row{
			serial:            device.Serialno,
			deviceID:          uniqueDeviceID,
			serialOrTransport: serialOrTransport,
			os:                osDisplay,
			deviceType:        deviceType,
			status:            status,
		})
	}

	// Sort: by real Serial No first, then by deviceID, then by serial/transport
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].serial != rows[j].serial {
			return rows[i].serial < rows[j].serial
		}
		if rows[i].deviceID != rows[j].deviceID {
			return rows[i].deviceID < rows[j].deviceID
		}
		return rows[i].serialOrTransport < rows[j].serialOrTransport
	})

	// Prepare data for RenderTable in sorted order
	tableData := make([]map[string]interface{}, len(rows))
	for i, r := range rows {
		tableData[i] = map[string]interface{}{
			"device_id":           r.deviceID,
			"serial_or_transport": r.serialOrTransport,
			"os":                  r.os,
			"device_type":         r.deviceType,
			"status":              r.status,
		}
	}

	// Define table columns
	columns := []util.TableColumn{
		{Header: "DEVICE ID", Key: "device_id"},
		{Header: "SERIAL NO/TRANSPORT ID", Key: "serial_or_transport"},
		{Header: "OS", Key: "os"},
		{Header: "DEVICE TYPE", Key: "device_type"},
		{Header: "STATUS", Key: "status"},
	}

	util.RenderTable(columns, tableData)
	return nil
}
