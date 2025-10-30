package cmd

import (
	"fmt"

	"github.com/babelcloud/gbox/packages/cli/internal/daemon"
	"github.com/spf13/cobra"
)

type DeviceConnectUnregisterOptions struct {
	All bool
}

func NewDeviceConnectUnregisterCommand() *cobra.Command {
	opts := &DeviceConnectUnregisterOptions{}

	cmd := &cobra.Command{
		Use:     "unregister [device_id] [flags]",
		Aliases: []string{"unreg"},
		Short:   "Unregister one or all active gbox device connections",
		Long:    "Unregister one or all active gbox device connections.",
		Example: `  # Unregister device with specific device ID:
  gbox device-connect unregister abc789pqr012-ip

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
	if !checkAdbInstalled() {
		printAdbInstallationHint()
		return fmt.Errorf("ADB is not installed or not in your PATH. Please install ADB and try again.")
	}

	if opts.All {
		return unregisterAllDevices()
	}

	if len(args) == 0 {
		return runInteractiveUnregisterSelection()
	}

	deviceID := args[0]
	return unregisterDevice(deviceID)
}

func unregisterAllDevices() error {
	// Get devices from daemon manager
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

	unregisteredCount := 0
	for _, device := range response.Devices {
		deviceID, _ := device["id"].(string)
		name, _ := device["ro.product.model"].(string)
		connectionType, _ := device["connectionType"].(string)
		isRegistrable, _ := device["isRegistrable"].(bool)

		if isRegistrable {
			fmt.Printf("Unregistering %s (%s, %s)...\n", deviceID, name, connectionType)

			req := map[string]string{"deviceId": deviceID}
			if err := daemon.DefaultManager.CallAPI("POST", "/api/devices/unregister", req, nil); err != nil {
				fmt.Printf("Failed to unregister %s: %v\n", deviceID, err)
				continue
			}

			fmt.Printf("Device %s unregistered successfully.\n", deviceID)
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

func unregisterDevice(deviceID string) error {
	// Get device info first to show details
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

	// Find the device to get its details
	var targetDevice map[string]interface{}
	for _, device := range response.Devices {
		if deviceID == device["id"].(string) {
			targetDevice = device
			break
		}
	}

	if targetDevice == nil {
		return fmt.Errorf("device not found: %s", deviceID)
	}

	model := "Unknown"
	if m, ok := targetDevice["ro.product.model"].(string); ok {
		model = m
	}

	connectionType := "Unknown"
	if ct, ok := targetDevice["connectionType"].(string); ok {
		connectionType = ct
	}

	fmt.Printf("Unregistering %s (%s, %s)...\n", deviceID, model, connectionType)

	req := map[string]string{"deviceId": deviceID}
	if err := daemon.DefaultManager.CallAPI("POST", "/api/devices/unregister", req, nil); err != nil {
		return fmt.Errorf("failed to unregister device: %v", err)
	}

	fmt.Printf("Device %s unregistered successfully.\n", deviceID)

	return nil
}

func runInteractiveUnregisterSelection() error {
	// Get devices from daemon manager
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

	// Filter only registered devices
	var registeredDevices []map[string]interface{}
	for _, device := range response.Devices {
		if isRegistrable, ok := device["isRegistrable"].(bool); ok && isRegistrable {
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
		deviceID, _ := device["id"].(string)
		model := "Unknown"
		if m, ok := device["ro.product.model"].(string); ok {
			model = m
		}
		connectionType := "Unknown"
		if ct, ok := device["connectionType"].(string); ok {
			connectionType = ct
		}
		manufacturer := ""
		if mfr, ok := device["ro.product.manufacturer"].(string); ok {
			manufacturer = mfr
		}

		fmt.Printf("%d. %s (%s, %s) - %s\n",
			i+1,
			deviceID,
			model,
			connectionType,
			manufacturer)
	}

	fmt.Println()
	fmt.Print("Enter a number: ")

	var choice int
	fmt.Scanf("%d", &choice)

	if choice < 1 || choice > len(registeredDevices) {
		return fmt.Errorf("invalid selection: %d", choice)
	}

	selectedDevice := registeredDevices[choice-1]
	deviceID := selectedDevice["id"].(string)
	return unregisterDevice(deviceID)
}
