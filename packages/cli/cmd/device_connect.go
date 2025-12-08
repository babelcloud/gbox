package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/babelcloud/gbox/packages/cli/config"
	"github.com/babelcloud/gbox/packages/cli/internal/daemon"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect"
	"github.com/babelcloud/gbox/packages/cli/internal/profile"
)

// Note: Device client functionality has been moved to daemon.DefaultManager
// All device operations now go through the unified server API

type DeviceConnectOptions struct {
	DeviceID   string
	Background bool
}

func NewDeviceConnectCommand() *cobra.Command {
	opts := &DeviceConnectOptions{}

	cmd := &cobra.Command{
		Use:   "device-connect [device_id] [flags]",
		Short: "Manage remote connections for local Android/Linux development devices",
		Long: `Manage remote connections for local Android/Linux development devices.
This command allows you to securely connect Android devices (emulators or physical devices)
to remote cloud services for remote access and debugging.

If no device ID is provided, an interactive device selection will be shown.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ExecuteDeviceConnect(cmd, opts, args)
		},
		Example: `  # Interactively select a device to connect
  gbox device-connect

  # Connect to specific device
  gbox device-connect abc123xyz456-usb

  # Connect in background mode
  gbox device-connect --background

  # List all available devices
  gbox device-connect ls

  # Register and connect this Linux machine to AP
  gbox device-connect register local`,
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.DeviceID, "device", "d", "", "Specify the Android device ID to connect")
	flags.BoolVarP(&opts.Background, "background", "b", false, "Run in background mode")

	cmd.AddCommand(
		NewDeviceConnectRegisterCommand(),
		NewDeviceConnectListCommand(),
		NewDeviceConnectUnregisterCommand(),
	)

	return cmd
}
func ExecuteDeviceConnect(cmd *cobra.Command, opts *DeviceConnectOptions, args []string) error {
	debug := os.Getenv("DEBUG") == "true"

	// Check and auto-install ADB if missing
	if !checkAdbInstalled() {
		if !debug {
			fmt.Println("→ Missing ADB, installing automatically...")
		}

		sp := device_connect.NewUISpinner(debug, "Installing ADB...")
		if err := installADB(); err != nil {
			sp.Fail("Failed to install ADB")
			printAdbInstallationHint()
			return fmt.Errorf("failed to install ADB automatically: %v\nPlease install ADB manually and try again", err)
		}

		// Verify installation
		if !checkAdbInstalled() {
			sp.Fail("ADB installation failed")
			printAdbInstallationHint()
			return fmt.Errorf("ADB installation completed but adb command not found in PATH")
		}
		sp.Success("ADB installed")
	}

	// Check and auto-install frpc if missing
	if !checkFrpcInstalled() {
		if !debug {
			fmt.Println("→ Missing frpc, installing automatically...")
		}

		sp := device_connect.NewUISpinner(debug, "Installing frpc...")
		if err := installFrpc(); err != nil {
			sp.Fail("Failed to install frpc")
			printFrpcInstallationHint()
			return fmt.Errorf("failed to install frpc automatically: %v\nPlease install frpc manually and try again", err)
		}

		// Verify installation
		if !checkFrpcInstalled() {
			sp.Fail("frpc installation failed")
			printFrpcInstallationHint()
			return fmt.Errorf("frpc installation completed but frpc command not found in PATH")
		}
		sp.Success("frpc installed")
	}

	// Check and install prerequisites
	if err := checkAndInstallPrerequisites(); err != nil {
		fmt.Fprintf(os.Stderr, "\n╔═══════════════════════════════════════╗\n")
		fmt.Fprintf(os.Stderr, "║  ❌  Prerequisites Installation Failed ║\n")
		fmt.Fprintf(os.Stderr, "╚═══════════════════════════════════════╝\n\n")
		fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
		fmt.Fprintf(os.Stderr, "💡 Quick Fix:\n")
		fmt.Fprintf(os.Stderr, "   • Fix the errors above and retry\n")
		fmt.Fprintf(os.Stderr, "   • Or run 'gbox setup' to install all dependencies\n")
		fmt.Fprintf(os.Stderr, "   • Or disable Appium: export GBOX_INSTALL_APPIUM=false\n\n")
		return err
	}

	// Always use the unified server (like adb start-server)
	// The server will be auto-started if not running

	// Note: Legacy mode with external binaries is being phased out
	// All functionality now goes through the unified gbox server

	// The actual device connection will happen via HTTP API calls
	// to the server, which will be started automatically by the daemon manager

	var deviceID string
	if len(args) > 0 {
		deviceID = args[0]
	} else if opts.DeviceID != "" {
		deviceID = opts.DeviceID
	}

	if deviceID == "" {
		return runInteractiveDeviceSelection(opts)
	}
	return connectToDevice(deviceID, opts)
}

// checkAndInstallPrerequisites checks and installs Node.js, npm, Appium and related components
func checkAndInstallPrerequisites() error {
	debug := os.Getenv("DEBUG") == "true"

	// Get Appium configuration from environment
	appiumCfg := device_connect.GetAppiumConfig()

	if !appiumCfg.InstallAppium {
		if debug {
			fmt.Println("[DEBUG] Appium installation is disabled (GBOX_INSTALL_APPIUM=false)")
		}
		return nil
	}

	// Check Node.js and npm
	if err := device_connect.CheckNodeInstalled(); err != nil {
		return fmt.Errorf("node.js and npm are not installed: %v\n\n"+
			"╔═══════════════════════════════════════╗\n"+
			"║         📦  Install Node.js           ║\n"+
			"╚═══════════════════════════════════════╝\n\n"+
			"Platform-specific installation:\n"+
			"  🍎 macOS:         brew install node\n"+
			"  🐧 Ubuntu/Debian: sudo apt-get install nodejs npm\n"+
			"  🪟 Windows:       Download from https://nodejs.org/\n\n"+
			"Or use our quick install script:\n"+
			"  curl -fsSL https://raw.githubusercontent.com/babelcloud/gbox/main/install.sh | bash", err)
	}

	if debug {
		fmt.Println("[DEBUG] ✅ Node.js and npm are installed")
	}

	// Check if Appium is already installed
	deviceProxyHome := config.GetDeviceProxyHome()
	appiumHome := filepath.Join(deviceProxyHome, "appium")

	if device_connect.IsAppiumInstalled(appiumHome) {
		if debug {
			appiumPath := device_connect.GetAppiumPath()
			fmt.Printf("[DEBUG] ✅ Appium is already installed at: %s\n", appiumPath)

			// Print configured components
			if len(appiumCfg.Drivers) > 0 {
				fmt.Printf("[DEBUG] 🔧 Configured drivers: %v\n", appiumCfg.Drivers)
			}

			if len(appiumCfg.Plugins) > 0 {
				fmt.Printf("[DEBUG] 🔌 Configured plugins: %v\n", appiumCfg.Plugins)
			}
		}

		// Try to install/update components
		if err := device_connect.InstallAppium(appiumCfg); err != nil {
			return fmt.Errorf("failed to install Appium components: %v", err)
		}
		return nil
	}

	// Install Appium and components
	if debug {
		fmt.Println("[DEBUG] Installing Appium Automation ...")
	}

	if err := device_connect.InstallAppium(appiumCfg); err != nil {
		return fmt.Errorf("failed to install Appium: %v", err)
	}

	if debug {
		fmt.Println("[DEBUG]  ✅  Appium Installation Completed!")
	}

	return nil
}

func runInteractiveDeviceSelection(opts *DeviceConnectOptions) error {
	// Use daemon manager to call API
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

	devices := response.Devices
	if len(devices) == 0 {
		fmt.Println("No devices found.")
		fmt.Println()
		printDeveloperModeHint()
		return nil
	}

	fmt.Println()
	fmt.Println("Select a device to register for remote access:")
	fmt.Println()
	printDeveloperModeHint()
	fmt.Println()

	// Display all devices returned from API
	for i, device := range devices {
		formatDeviceOption(i+1, device)
	}
	fmt.Println()
	fmt.Print("Enter a number: ")
	var choice int

	// Trap Ctrl+C while waiting for input so we can cleanup proxy first
	intCh := make(chan os.Signal, 1)
	signal.Notify(intCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(intCh)

	inputDone := make(chan struct{})
	go func() {
		// Read user input
		fmt.Scanf("%d", &choice)
		close(inputDone)
	}()

	select {
	case <-intCh:
		// User pressed Ctrl+C during selection; exit gracefully
		return nil
	case <-inputDone:
		// proceed
	}
	totalOptions := len(devices)
	if choice < 1 || choice > totalOptions {
		return fmt.Errorf("invalid selection: %d", choice)
	}

	selectedDevice := devices[choice-1]

	// Handle local device registration
	if selectedDevice.IsLocal {
		// Use empty deviceID to register as desktop with auto-detected OS
		// Server will automatically connect desktop devices after registration
		return registerDevice("", "")
	}

	// For Android devices, use TransportID for API call, fallback to Serialno if empty
	deviceID := selectedDevice.TransportID
	if strings.TrimSpace(deviceID) == "" {
		deviceID = selectedDevice.Serialno
	}
	return connectToDevice(deviceID, opts)
}

// formatDeviceOption formats a device for display in the interactive selection
func formatDeviceOption(index int, device DeviceDTO) {
	status := "Not Registered"
	statusColor := color.New(color.Faint)

	// If IsLocal=true, replace serialNo with "local" for display
	displaySerialNo := device.Serialno
	if device.IsLocal {
		displaySerialNo = "local"
	}

	isRegistered := device.IsRegistered

	if isRegistered {
		status = "Registered"
		statusColor = color.New(color.FgGreen)
	}

	// Get device-specific fields from metadata
	var model, manufacturer, connectionType string
	if device.Metadata != nil {
		if m, ok := device.Metadata["model"].(string); ok {
			model = m
		}
		if m, ok := device.Metadata["manufacturer"].(string); ok {
			manufacturer = m
		}
		if ct, ok := device.Metadata["connectionType"].(string); ok {
			connectionType = ct
		}
	}

	if strings.TrimSpace(model) == "" {
		model = "Unknown"
	}
	if strings.TrimSpace(manufacturer) == "" {
		manufacturer = "Unknown"
	}

	// Map Platform and OS to display label
	var platformLabel string
	if device.Platform == "mobile" && device.OS == "android" {
		platformLabel = "Android"
	} else if device.Platform == "desktop" {
		// Map OS to display label for desktop
		switch device.OS {
		case "macos":
			platformLabel = "MacOS"
		case "linux":
			platformLabel = "Linux"
		case "windows":
			platformLabel = "Windows"
		default:
			platformLabel = device.OS
		}
	} else {
		platformLabel = device.Platform
	}

	// For local devices, get OS version and hostname for display from metadata
	if device.IsLocal {
		var osVersion string
		var hostname string
		if device.Metadata != nil {
			if ov, ok := device.Metadata["osVersion"].(string); ok {
				osVersion = ov
			}
			if hn, ok := device.Metadata["hostname"].(string); ok {
				hostname = hn
			}
		}
		if osVersion == "" {
			// Fallback to runtime detection
			switch runtime.GOOS {
			case "linux":
				if version, err := getLinuxVersion(); err == nil {
					osVersion = version
				} else {
					osVersion = "Unknown"
				}
			case "darwin":
				if version, err := getMacOSVersion(); err == nil {
					osVersion = version
				} else {
					osVersion = "Unknown"
				}
			case "windows":
				if version, err := getWindowsVersion(); err == nil {
					osVersion = version
				} else {
					osVersion = "Unknown"
				}
			default:
				osVersion = "Unknown"
			}
		}
		// Use hostname from metadata, fallback to manufacturer if not available
		if hostname == "" {
			hostname = manufacturer
		}
		fmt.Printf("%d. %s (%s, %s) - %s [%s]\n",
			index,
			color.New(color.FgCyan).Sprint(displaySerialNo),
			platformLabel,
			osVersion,
			hostname,
			statusColor.Sprint(status))
	} else {
		// Format: serialNo (Platform, model, connectionType) - manufacturer [status]
		// For Android devices, connectionType is in metadata
		if connectionType == "" {
			connectionType = "unknown"
		}
		fmt.Printf("%d. %s (%s, %s, %s) - %s [%s]\n",
			index,
			color.New(color.FgCyan).Sprint(displaySerialNo),
			platformLabel,
			model,
			connectionType,
			manufacturer,
			statusColor.Sprint(status))
	}
}

func connectToDevice(deviceID string, opts *DeviceConnectOptions) error {
	// Register device via daemon API
	// For Android devices
	req := map[string]string{
		"deviceId":   deviceID,
		"deviceType": "mobile",
		"osType":     "android",
	}
	var resp map[string]interface{}

	if err := daemon.DefaultManager.CallAPI("POST", "/api/devices/register", req, &resp); err != nil {
		return fmt.Errorf("failed to register device: %v", err)
	}

	if success, ok := resp["success"].(bool); !ok || !success {
		return fmt.Errorf("failed to register device: %v", resp["error"])
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

	if opts.Background {
		fmt.Println("(Running in background. Use 'gbox device-connect unregister' to stop.)")
		return nil
	}

	fmt.Printf("(Running in foreground. Press %s to disconnect.)\n", color.New(color.FgYellow, color.Bold).Sprint("Ctrl+C"))

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	fmt.Printf("Disconnecting device %s...\n", deviceID)

	// Unregister the device via daemon API
	req = map[string]string{"deviceId": deviceID}
	if err := daemon.DefaultManager.CallAPI("POST", "/api/devices/unregister", req, nil); err != nil {
		fmt.Printf("Warning: failed to unregister device: %v\n", err)
	}

	return nil
}

// registerDevice registers a device for remote access
// If deviceID is empty and deviceType is empty, register as desktop with auto-detected OS
// If deviceID is provided, register as mobile (Android) device
// If deviceType is provided (for backward compatibility), use it to determine type
func registerDevice(deviceID string, deviceType string) error {
	// Register device via daemon API
	req := make(map[string]string)
	isDesktop := false

	// Determine device type based on parameters
	if deviceID == "" && deviceType == "" {
		// Empty deviceID and deviceType means register local machine as desktop
		req["deviceType"] = "desktop"
		isDesktop = true
		// Auto-detect OS type
		switch runtime.GOOS {
		case "linux":
			req["osType"] = "linux"
		case "darwin":
			req["osType"] = "macos"
		case "windows":
			req["osType"] = "windows"
		default:
			req["osType"] = "linux" // Default fallback
		}
	} else if deviceType != "" {
		// Backward compatibility: use provided deviceType
		oldType := strings.ToLower(deviceType)
		if oldType == "android" {
			req["deviceType"] = "mobile"
			req["osType"] = "android"
		} else if oldType == "linux" {
			req["deviceType"] = "desktop"
			req["osType"] = "linux"
			isDesktop = true
		} else {
			// Default: treat as desktop and try to detect OS
			req["deviceType"] = "desktop"
			isDesktop = true
			switch runtime.GOOS {
			case "linux":
				req["osType"] = "linux"
			case "darwin":
				req["osType"] = "macos"
			case "windows":
				req["osType"] = "windows"
			default:
				req["osType"] = "linux" // Default fallback
			}
		}
	} else {
		// deviceID is provided, register as mobile (Android) device
		req["deviceType"] = "mobile"
		req["osType"] = "android"
	}

	// For desktop devices, try to reuse regId if deviceID is empty
	if isDesktop && deviceID == "" {
		if regId, _ := readLocalRegId(); regId != "" {
			req["regId"] = regId
		}
	}

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

	// Resolve actual device ID and regId from response
	actualID := deviceID
	regIdStr := ""
	if data, ok := resp["data"].(map[string]interface{}); ok {
		if id, ok2 := data["id"].(string); ok2 && id != "" {
			actualID = id
		}
		if rid, ok2 := data["regId"].(string); ok2 && rid != "" {
			regIdStr = rid
		} else if actualID != "" {
			regIdStr = actualID
		}
	}
	// Also check top-level fields
	if regIdStr == "" {
		if v, ok := resp["regId"]; ok {
			if rid, ok2 := v.(string); ok2 && rid != "" {
				regIdStr = rid
			}
		}
	}
	if actualID != "" && deviceID == "" {
		deviceID = actualID
	}

	// For desktop devices, persist regId for future reuse
	if isDesktop && regIdStr != "" {
		_ = writeLocalRegId(regIdStr)
	}

	// Display registration result
	if isDesktop {
		if actualID != "" && regIdStr != "" {
			fmt.Printf("Desktop device registered. Device ID: %s (regId: %s)\n", actualID, regIdStr)
		} else if actualID != "" {
			fmt.Printf("Desktop device registered. Device ID: %s\n", actualID)
		} else {
			fmt.Printf("Desktop device registered.\n")
		}
	} else {
		if actualID != "" {
			fmt.Printf("Device registered. Device ID: %s\n", actualID)
		}
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