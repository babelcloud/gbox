package cmd

import (
	"fmt"
	"strings"

	"github.com/babelcloud/gbox/packages/cli/internal/daemon"
	"github.com/spf13/cobra"
)

type DeviceConnectUnregisterOptions struct {
	All bool
}

func NewDeviceConnectUnregisterCommand() *cobra.Command {
	opts := &DeviceConnectUnregisterOptions{}

	cmd := &cobra.Command{
		Use:     "unregister [serial_or_transport_id|local] [flags]",
		Aliases: []string{"unreg"},
		Short:   "Unregister one or all active gbox device connections",
		Long:    "Unregister one or all active gbox device connections. Use 'local' to unregister this machine.",
		Example: `  # Unregister device with Serial No or Transport ID:
  gbox device-connect unregister A4RYVB3A20008848
  gbox device-connect unregister adb-A4RYVB3A20008848._adb._tcp

  # Unregister local machine:
  gbox device-connect unregister local

  # Unregister all active device connections:
  gbox device-connect unregister --all`,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true, // Don't show usage on errors (e.g., device not found)
		SilenceErrors: true, // Don't show errors twice (we handle them in RunE)
		RunE: func(cmd *cobra.Command, args []string) error {
			err := ExecuteDeviceConnectUnregister(cmd, opts, args)
			if err != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), err)
			}
			return nil // Return nil to prevent Cobra from printing again
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&opts.All, "all", "a", false, "Disconnect all active device connections")

	return cmd
}

func ExecuteDeviceConnectUnregister(cmd *cobra.Command, opts *DeviceConnectUnregisterOptions, args []string) error {
	if opts.All {
		return unregisterAllDevices()
	}

	if len(args) == 0 {
		return runInteractiveUnregisterSelection()
	}

	deviceKey := args[0]

	// For "local" device, we don't need ADB check
	if strings.EqualFold(deviceKey, "local") {
		return unregisterLocalDevice()
	}

	// For Android devices, check ADB
	if !checkAdbInstalled() {
		printAdbInstallationHint()
		return fmt.Errorf("ADB is not installed or not in your PATH, please install ADB and try again")
	}

	return unregisterDevice(deviceKey)
}

// DeviceDTO is defined in device_connect_list.go (same package)

func unregisterAllDevices() error {
	// Get devices from daemon manager
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

	unregisteredCount := 0
	for _, device := range response.Devices {
		if device.IsRegistered {
			// Prefer transport id for API
			deviceKey := device.TransportID
			if deviceKey == "" {
				deviceKey = device.Serialno
			}
			// Get device-specific fields from metadata
			var name, connectionType string
			if device.Metadata != nil {
				if m, ok := device.Metadata["model"].(string); ok {
					name = m
				}
				if ct, ok := device.Metadata["connectionType"].(string); ok {
					connectionType = ct
				}
			}
			if name == "" {
				name = "Unknown"
			}
			if connectionType == "" {
				connectionType = "unknown"
			}

			fmt.Printf("Unregistering %s (%s, %s)...\n", deviceKey, name, connectionType)
			req := map[string]string{"deviceId": deviceKey}
			if err := daemon.DefaultManager.CallAPI("POST", "/api/devices/unregister", req, nil); err != nil {
				fmt.Printf("Failed to unregister %s: %v\n", deviceKey, err)
				continue
			}
			fmt.Printf("Device %s unregistered successfully.\n", deviceKey)
			unregisteredCount++
		}
	}

	if unregisteredCount == 0 {
		fmt.Println("No active connections found to unregister.")
	} else {
		fmt.Printf("Unregistered %d device(s).\n", unregisteredCount)
	}

	return nil
}

// unregisterLocalDevice unregisters the local desktop device using saved regId
func unregisterLocalDevice() error {
	// Read regId from local file
	regId, err := readLocalRegId()
	if err != nil || regId == "" {
		return fmt.Errorf("local device not registered or regId not found, use 'gbox device-connect register local' to register first")
	}

	fmt.Printf("Unregistering local device (regId: %s)...\n", regId)

	// Use regId as deviceId - the server will resolve it to actual device ID
	req := map[string]string{"deviceId": regId}
	if err := daemon.DefaultManager.CallAPI("POST", "/api/devices/unregister", req, nil); err != nil {
		return fmt.Errorf("failed to unregister local device: %v", err)
	}

	// Clear the saved regId after successful unregistration
	_ = writeLocalRegId("")

	fmt.Printf("Local device unregistered successfully.\n")
	return nil
}

func unregisterDevice(deviceKey string) error {
	// Get device info first to show details
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

	// Find the device to get its details
	var target *DeviceDTO
	for i := range response.Devices {
		d := &response.Devices[i]
		if deviceKey == d.TransportID || deviceKey == d.Serialno {
			target = d
			break
		}
	}

	if target == nil {
		return fmt.Errorf("device not found: %s", deviceKey)
	}

	// Get device-specific fields from metadata
	var model, connectionType string
	if target.Metadata != nil {
		if m, ok := target.Metadata["model"].(string); ok {
			model = m
		}
		if ct, ok := target.Metadata["connectionType"].(string); ok {
			connectionType = ct
		}
	}
	if model == "" {
		model = "Unknown"
	}
	if connectionType == "" {
		connectionType = "unknown"
	}

	fmt.Printf("Unregistering %s (%s, %s)...\n", deviceKey, model, connectionType)

	req := map[string]string{"deviceId": deviceKey}
	if err := daemon.DefaultManager.CallAPI("POST", "/api/devices/unregister", req, nil); err != nil {
		return fmt.Errorf("failed to unregister device: %v", err)
	}

	fmt.Printf("Device %s unregistered successfully.\n", deviceKey)

	return nil
}

func runInteractiveUnregisterSelection() error {
	// Get devices from daemon manager
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

	// Filter only registered devices
	var registeredDevices []DeviceDTO
	for _, device := range response.Devices {
		if device.IsRegistered {
			registeredDevices = append(registeredDevices, device)
		}
	}

	if len(registeredDevices) == 0 {
		fmt.Println("No active device connections found.")
		return nil
	}

	fmt.Println("Select a device to unregister:")
	fmt.Println()

	for i, device := range registeredDevices {
		// Get connectionType from metadata
		var connectionType string
		if device.Metadata != nil {
			if ct, ok := device.Metadata["connectionType"].(string); ok {
				connectionType = ct
			}
		}

		// Show Transport ID primarily for non-USB, else Serial No
		deviceKey := device.TransportID
		if strings.EqualFold(connectionType, "usb") && device.Serialno != "" {
			deviceKey = device.Serialno
		}

		// Get model from metadata
		var model string
		if device.Metadata != nil {
			if m, ok := device.Metadata["model"].(string); ok {
				model = m
			}
		}
		if model == "" {
			model = "Unknown"
		}
		if connectionType == "" {
			connectionType = "unknown"
		}

		fmt.Printf("%d. %s (%s, %s)\n",
			i+1,
			deviceKey,
			model,
			connectionType)
	}

	fmt.Println()
	fmt.Print("Enter a number: ")

	var choice int
	fmt.Scanf("%d", &choice)

	if choice < 1 || choice > len(registeredDevices) {
		return fmt.Errorf("invalid selection: %d", choice)
	}

	selectedDevice := registeredDevices[choice-1]
	// Prefer transport id for API
	deviceKey := selectedDevice.TransportID
	if deviceKey == "" {
		deviceKey = selectedDevice.Serialno
	}
	return unregisterDevice(deviceKey)
}
