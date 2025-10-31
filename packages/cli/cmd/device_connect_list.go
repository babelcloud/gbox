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
	deviceTypeDevice    = "device"
	deviceTypeEmulator  = "emulator"
)

type DeviceConnectListOptions struct {
	OutputFormat string
}

// DeviceDTO is the API response structure for devices
type DeviceDTO struct {
	ID             string `json:"id"`
	TransportID    string `json:"transportId"`
	Serialno       string `json:"serialno"`
	AndroidID      string `json:"androidId"`
	Model          string `json:"model"`
	Manufacturer   string `json:"manufacturer"`
	ConnectionType string `json:"connectionType"`
	IsRegistered   bool   `json:"isRegistered"`
	RegId          string `json:"regId"`
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
	// Create a simplified JSON output for compatibility
	type SimpleDeviceInfo struct {
		RegId            string `json:"reg_id"`
		DeviceID         string `json:"device_id"`
		Name             string `json:"name"`
		Type             string `json:"type"`
		ConnectionStatus string `json:"connection_status"`
	}

	var simpleDevices []SimpleDeviceInfo
	for _, device := range devices {
		deviceID := device.ID
		name := device.Model
		serialNo := device.Serialno
		regId := device.RegId
		isRegistered := device.IsRegistered

		status := statusNotRegistered
		if isRegistered {
			// Color "Registered" in green for better visibility
			status = "\u001b[32m" + statusRegistered + "\u001b[0m"
		}

		deviceType := deviceTypeDevice
		// Check if it's an emulator based on serial number
		if strings.Contains(strings.ToUpper(serialNo), "EMULATOR") {
			deviceType = deviceTypeEmulator
		}

		simpleDevices = append(simpleDevices, SimpleDeviceInfo{
			RegId:            regId,
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

func outputDevicesTextFromAPI(devices []DeviceDTO) error {
	if len(devices) == 0 {
		fmt.Println("No Android devices found.")
		return nil
	}

	// Build rows for sorting
	type row struct {
		serial            string
		deviceID          string
		serialOrTransport string
		name              string
		deviceType        string
		status            string
	}
	rows := make([]row, 0, len(devices))
	for _, device := range devices {
		deviceID := device.ID
		name := device.Model
		serialNo := device.Serialno
		transportID := device.TransportID
		isRegistered := device.IsRegistered

		status := statusNotRegistered
		if isRegistered {
			// Green for Registered
			status = "\x1b[32m" + statusRegistered + "\x1b[0m"
		}

		deviceType := deviceTypeDevice
		// Check if it's an emulator based on serial number
		if strings.Contains(strings.ToUpper(serialNo), "EMULATOR") {
			deviceType = deviceTypeEmulator
		}

		// Display "-" for empty fields
		if name == "" {
			name = "-"
		}

		// Second column: for USB show Serial No; otherwise show Transport ID (full value)
		serialOrTransport := transportID
		if strings.EqualFold(device.ConnectionType, "usb") && strings.TrimSpace(serialNo) != "" {
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
			name:              name,
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
			"name":                r.name,
			"type":                r.deviceType,
			"status":              r.status,
		}
	}

	// Define table columns
	columns := []util.TableColumn{
		{Header: "DEVICE ID", Key: "device_id"},
		{Header: "SERIAL NO/TRANSPORT ID", Key: "serial_or_transport"},
		{Header: "MODEL", Key: "name"},
		{Header: "TYPE", Key: "type"},
		{Header: "STATUS", Key: "status"},
	}

	util.RenderTable(columns, tableData)
	return nil
}
