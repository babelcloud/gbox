package cmd

import (
	"fmt"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect"
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
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return ExecuteDeviceConnectUnregister(cmd, opts, args)
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

	deviceID := args[0]
	return unregisterDevice(deviceID)
}

func unregisterAllDevices() error {
	client := getDeviceClient()

	devices, err := client.GetDevices()
	if err != nil {
		return fmt.Errorf("failed to get available devices: %v", err)
	}

	unregisteredCount := 0
	for _, device := range devices {
		if device.IsRegistrable {
			fmt.Printf("Unregistering %s (%s, %s)...\n",
				device.Udid, device.ProductModel, device.ConnectionType)

			if err := client.UnregisterDevice(device.Udid); err != nil {
				fmt.Printf("Failed to unregister %s: %v\n", device.Udid, err)
				continue
			}

			fmt.Printf("Device %s unregistered successfully.\n", device.Udid)
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
	client := getDeviceClient()

	device, err := client.GetDeviceInfo(deviceID)
	if err != nil {
		return fmt.Errorf("device not found: %s", deviceID)
	}

	fmt.Printf("Unregistering %s (%s, %s)...\n",
		deviceID, device.ProductModel, device.ConnectionType)

	if err := client.UnregisterDevice(deviceID); err != nil {
		return fmt.Errorf("failed to unregister device: %v", err)
	}

	fmt.Printf("Device %s unregistered successfully.\n", deviceID)

	return nil
}

func runInteractiveUnregisterSelection() error {
	client := getDeviceClient()

	devices, err := client.GetDevices()
	if err != nil {
		return fmt.Errorf("failed to get available devices: %v", err)
	}

	// Filter only registered devices
	var registeredDevices []device_connect.DeviceInfo
	for _, device := range devices {
		if device.IsRegistrable {
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
		fmt.Printf("%d. %s (%s, %s) - %s\n",
			i+1,
			device.Udid,
			device.ProductModel,
			device.ConnectionType,
			device.ProductManufacturer)
	}

	fmt.Println()
	fmt.Print("Enter a number: ")

	var choice int
	fmt.Scanf("%d", &choice)

	if choice < 1 || choice > len(registeredDevices) {
		return fmt.Errorf("invalid selection: %d", choice)
	}

	selectedDevice := registeredDevices[choice-1]
	return unregisterDevice(selectedDevice.Udid)
}
