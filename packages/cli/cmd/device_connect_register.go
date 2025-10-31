package cmd

import (
	"fmt"
	"runtime"
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
		Long:    "Register a device for remote access. Default type is 'android'. Use 'local' to register this Linux machine and connect it to Access Point (AP).",
		Example: `  # Register an Android device by ID
  gbox device-connect register abc123xyz456

  # Register and connect this Linux machine
  gbox device-connect register local`,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  false,
		SilenceErrors: true, // Don't show errors twice (we handle them in RunE)
		RunE: func(cmd *cobra.Command, args []string) error {
			// When no args, enter interactive registration
			if len(args) == 0 && opts.DeviceID == "" {
				return runInteractiveDeviceRegistration()
			}
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
	// Resolve device ID from args/flags first
	var deviceID string
	if len(args) > 0 {
		deviceID = args[0]
	} else if opts.DeviceID != "" {
		deviceID = opts.DeviceID
	}

	// Special token: "local" means register this Linux machine and connect it to AP
	if strings.EqualFold(deviceID, "local") {
		return registerAndConnectLocalLinux(opts)
	}

	// For Android type, ensure ADB is installed
	if strings.ToLower(opts.Type) != "linux" {
		if !checkAdbInstalled() {
			printAdbInstallationHint()
			return fmt.Errorf("adb is not installed or not in your PATH; install adb and try again")
		}
	}

	// No interactive mode here; interactive is handled above when args are empty
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
	// Always offer a local option
	offerLocal := true
	if len(devices) == 0 {
		fmt.Println("No Android devices found.")
		fmt.Println()
		printDeveloperModeHint()
		// Continue to offer local option below
	}

	fmt.Println()
	if offerLocal {
		fmt.Println("Select a device to register for remote access, or choose this machine:")
	} else {
		fmt.Println("Select a device to register for remote access:")
	}
	fmt.Println()
	printDeveloperModeHint()
	fmt.Println()

	indexBase := 1
	if offerLocal {
		label := "This machine"
		switch runtime.GOOS {
		case "linux":
			label = "This machine (Linux)"
		case "darwin":
			label = "This machine (macOS)"
		case "windows":
			label = "This machine (Windows)"
		}
		fmt.Printf("%d. %s\n", indexBase, color.New(color.FgCyan).Sprint(label))
		indexBase++
	}

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
			indexBase+i,
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

	totalOptions := len(devices)
	if offerLocal {
		totalOptions++
		if choice == 1 {
			return registerAndConnectLocalLinux(nil)
		}
		choice = choice - 1
	}

	if choice < 1 || choice > totalOptions {
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

// registerAndConnectLocalLinux registers this Linux machine and immediately connects it to AP.
func registerAndConnectLocalLinux(opts *DeviceConnectRegisterOptions) error {
	req := map[string]string{"type": "linux"}
	var resp map[string]interface{}

	if err := daemon.DefaultManager.CallAPI("POST", "/api/devices/register", req, &resp); err != nil {
		return fmt.Errorf("failed to register linux device: %v", err)
	}

	if success, ok := resp["success"].(bool); !ok || !success {
		return fmt.Errorf("failed to register linux device: %v", resp["error"])
	}

	var deviceID string
	var regId string
	if data, ok := resp["data"].(map[string]interface{}); ok {
		if id, ok2 := data["id"].(string); ok2 {
			deviceID = id
		}
		if rid, ok2 := data["regId"].(string); ok2 && rid != "" {
			regId = rid
		}
	}
	if regId == "" {
		regId = deviceID
	}

	if regId != "" {
		_ = writeLocalRegId(regId)
	}

	if deviceID != "" {
		fmt.Printf("Linux device registered. Device ID: %s\n", deviceID)
	} else {
		fmt.Printf("Linux device registered.\n")
	}

	linuxOpts := &DeviceConnectLinuxConnectOptions{}
	// Reuse provided -d flag if present and not the literal "local"
	if opts != nil && opts.DeviceID != "" && !strings.EqualFold(opts.DeviceID, "local") {
		linuxOpts.DeviceID = opts.DeviceID
	}
	return ExecuteDeviceConnectLinuxConnect(nil, linuxOpts, nil)
}

// registerLocalLinux registers this Linux machine explicitly (no implicit connect)
func registerLocalLinux() error {
	req := map[string]string{"type": "linux"}
	var resp map[string]interface{}

	if err := daemon.DefaultManager.CallAPI("POST", "/api/devices/register", req, &resp); err != nil {
		return fmt.Errorf("failed to register linux device: %v", err)
	}

	if success, ok := resp["success"].(bool); !ok || !success {
		return fmt.Errorf("failed to register linux device: %v", resp["error"])
	}

	var deviceID string
	var regId string
	if data, ok := resp["data"].(map[string]interface{}); ok {
		if id, ok2 := data["id"].(string); ok2 {
			deviceID = id
		}
		if rid, ok2 := data["regId"].(string); ok2 && rid != "" {
			regId = rid
		}
	}
	if regId == "" {
		regId = deviceID
	}

	if regId != "" {
		_ = writeLocalRegId(regId)
	}

	if deviceID != "" {
		fmt.Printf("Linux device registered. Device ID: %s\n", deviceID)
	} else {
		fmt.Printf("Linux device registered.\n")
	}

	// Show cloud console URL
	pm := profile.NewProfileManager()
	if err := pm.Load(); err == nil {
		if devicesURL, err := pm.GetDevicesURL(); err == nil {
			fmt.Printf("\n☁️  Manage your devices at: %s\n", color.CyanString(devicesURL))
		}
	}

	fmt.Printf("\nNext: run %s to connect this machine to Access Point.\n", color.CyanString("gbox device-connect register local"))
	return nil
}
