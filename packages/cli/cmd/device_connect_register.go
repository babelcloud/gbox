package cmd

import (
	"fmt"
	"strings"

	"github.com/babelcloud/gbox/packages/cli/internal/daemon"
	"github.com/babelcloud/gbox/packages/cli/internal/profile"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

type DeviceConnectRegisterOptions struct {
	DeviceID string
	Type     string
}

func NewDeviceConnectRegisterCommand() *cobra.Command {
	opts := &DeviceConnectRegisterOptions{}

	cmd := &cobra.Command{
		Use:     "register [device_id] [flags]",
		Aliases: []string{"reg"},
		Short:   "Register a device for remote access",
		Long:    "Register a device for remote access. Default type is 'android'. Use --type linux to register a linux device.",
		Example: `  # Interactively select a device to register
  gbox device-connect register

  # Register specific device
  gbox device-connect register abc123xyz456-usb

  # Register a linux device
  gbox device-connect register --type linux my-linux-device-id`,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true, // Don't show usage on errors
		SilenceErrors: true, // Don't show errors twice (we handle them in RunE)
		RunE: func(cmd *cobra.Command, args []string) error {
			err := ExecuteDeviceConnectRegister(cmd, opts, args)
			if err != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), err)
			}
			return nil // Return nil to prevent Cobra from printing again
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.DeviceID, "device", "d", "", "Specify the device ID to register")
	flags.StringVar(&opts.Type, "type", "android", "Device type to register: android|linux")

	return cmd
}

func ExecuteDeviceConnectRegister(cmd *cobra.Command, opts *DeviceConnectRegisterOptions, args []string) error {
	if strings.ToLower(opts.Type) != "linux" {
		if !checkAdbInstalled() {
			printAdbInstallationHint()
			return fmt.Errorf("ADB is not installed or not in your PATH. Please install ADB and try again.")
		}
	}

	var deviceID string
	if len(args) > 0 {
		deviceID = args[0]
	} else if opts.DeviceID != "" {
		deviceID = opts.DeviceID
	}

	if deviceID == "" {
		if strings.ToLower(opts.Type) != "linux" {
			return runInteractiveDeviceRegistration()
		}
	}

	return registerDevice(deviceID, opts.Type)
}

func runInteractiveDeviceRegistration() error {
	// Use daemon manager to call API
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

	devices := response.Devices
	if len(devices) == 0 {
		fmt.Println("No Android devices found.")
		fmt.Println()
		printDeveloperModeHint()
		return nil
	}

	fmt.Println()
	fmt.Println("Select a device to register for remote access:")
	fmt.Println()
	printDeveloperModeHint()
	fmt.Println()

	for i, device := range devices {
		status := "Not Registered"
		statusColor := color.New(color.Faint)

		// Extract device info from map
		serialNo := device["ro.serialno"].(string)
		connectionType := device["connectionType"].(string)
		isRegistered, _ := device["isRegistrable"].(bool)

		if isRegistered {
			status = "Registered"
			statusColor = color.New(color.FgGreen)
		}

		model := "Unknown"
		if m, ok := device["ro.product.model"].(string); ok {
			model = m
		}

		manufacturer := ""
		if mfr, ok := device["ro.product.manufacturer"].(string); ok {
			manufacturer = mfr
		}

		fmt.Printf("%d. %s (%s, %s) - %s [%s]\n",
			i+1,
			color.New(color.FgCyan).Sprint(serialNo+"-"+connectionType),
			model,
			connectionType,
			manufacturer,
			statusColor.Sprint(status))
	}
	fmt.Println()
	fmt.Print("Enter a number: ")
	var choice int
	fmt.Scanf("%d", &choice)
	if choice < 1 || choice > len(devices) {
		return fmt.Errorf("invalid selection: %d", choice)
	}

	selectedDevice := devices[choice-1]
	deviceID := selectedDevice["id"].(string)
	return registerDevice(deviceID, "android")
}

func registerDevice(deviceID string, deviceType string) error {
	// Register device via daemon API
	req := map[string]string{"type": strings.ToLower(deviceType)}
	if deviceID != "" {
		req["deviceId"] = deviceID
	}
	var resp map[string]interface{}

	if err := daemon.DefaultManager.CallAPI("POST", "/api/devices/register", req, &resp); err != nil {
		return fmt.Errorf("failed to register device: %v", err)
	}

	if success, ok := resp["success"].(bool); !ok || !success {
		return fmt.Errorf("failed to register device: %v", resp["error"])
	}

	// Resolve actual device ID from response data if available
	actualID := deviceID
	if data, ok := resp["data"].(map[string]interface{}); ok {
		if id, ok2 := data["id"].(string); ok2 && id != "" {
			actualID = id
		}
	}
	if actualID != "" && deviceID == "" {
		deviceID = actualID
	}
	if actualID != "" {
		fmt.Printf("Device registered. Device ID: %s\n", actualID)
	}

	fmt.Printf("Establishing remote connection for device %s...\n", deviceID)
	fmt.Printf("Connection established successfully!\n")

	// Display local Web UI URL
	fmt.Printf("\n📱 View and control your device at: %s\n", color.CyanString("http://localhost:29888"))
	fmt.Printf("   This is the local live-view interface for device control\n")

	// Get and display devices URL for the current profile
	pm := profile.NewProfileManager()
	if err := pm.Load(); err == nil {
		if devicesURL, err := pm.GetDevicesURL(); err == nil {
			fmt.Printf("\n☁️  Remote access available at: %s\n", color.CyanString(devicesURL))
		}
	}

	fmt.Printf("\n💡 Device registered successfully. Use 'gbox device-connect unregister %s' to disconnect when needed.\n", deviceID)

	return nil
}
