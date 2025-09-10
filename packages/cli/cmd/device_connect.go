package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/babelcloud/gbox/packages/cli/config"
	"github.com/babelcloud/gbox/packages/cli/internal/daemon"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect"
	"github.com/babelcloud/gbox/packages/cli/internal/profile"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// printDeveloperModeHint prints the developer mode hint with dim formatting
func printDeveloperModeHint() {
	color.New(color.Faint).Println("If you can not see your devices here, make sure you have turned on the developer mode on your Android device. For more details, see https://docs.gbox.ai/cli")
}

func checkAdbInstalled() bool {
	_, err := exec.LookPath("adb")
	return err == nil
}

func checkFrpcInstalled() bool {
	_, err := exec.LookPath("frpc")
	return err == nil
}

func printAdbInstallationHint() {
	const (
		ansiRed    = "\033[31m"
		ansiYellow = "\033[33m"
		ansiBold   = "\033[1m"
		ansiReset  = "\033[0m"
	)

	fmt.Println()
	fmt.Printf("%s%s⚠️  IMPORTANT: Android Debug Bridge (ADB) Required%s\n", ansiRed, ansiBold, ansiReset)
	fmt.Printf("%s%s================================================%s\n", ansiYellow, ansiBold, ansiReset)
	fmt.Printf("%sTo use the device-connect feature, you need to install ADB tools first:%s\n", ansiYellow, ansiReset)
	fmt.Println()
	fmt.Printf("%s📱 Installation Methods:%s\n", ansiBold, ansiReset)
	fmt.Printf("  • macOS: brew install android-platform-tools\n")
	fmt.Printf("  • Ubuntu/Debian: sudo apt-get install android-tools-adb\n")
	fmt.Printf("  • Windows: Download Android SDK Platform Tools\n")
	fmt.Println()
	fmt.Printf("%s🔗 After installation, ensure:%s\n", ansiBold, ansiReset)
	fmt.Printf("  1. Enable Developer Options and USB Debugging on your Android device\n")
	fmt.Printf("  2. Connect device via USB or start an emulator\n")
	fmt.Printf("  3. Run 'adb devices' to confirm device recognition\n")
	fmt.Println()
	fmt.Printf("%s%s================================================%s\n", ansiYellow, ansiBold, ansiReset)
	fmt.Println()
}

func printFrpcInstallationHint() {
	const (
		ansiRed    = "\033[31m"
		ansiYellow = "\033[33m"
		ansiBold   = "\033[1m"
		ansiReset  = "\033[0m"
	)

	fmt.Println()
	fmt.Printf("%s%s⚠️  IMPORTANT: FRP Client (frpc) Required%s\n", ansiRed, ansiBold, ansiReset)
	fmt.Printf("%s%s==============================================%s\n", ansiYellow, ansiBold, ansiReset)
	fmt.Printf("%sTo use the device-connect feature, you need to install frpc (FRP Client) first:%s\n", ansiYellow, ansiReset)
	fmt.Println()
	fmt.Printf("%s🌐 Installation Methods:%s\n", ansiBold, ansiReset)
	fmt.Printf("  • macOS: brew install frpc\n")
	fmt.Printf("  • Ubuntu/Debian: Download from https://github.com/fatedier/frp/releases\n")
	fmt.Printf("  • Windows: Download from https://github.com/fatedier/frp/releases\n")
	fmt.Println()
	fmt.Printf("%s📥 Manual Installation:%s\n", ansiBold, ansiReset)
	fmt.Printf("  1. Download frpc binary for your platform from GitHub releases\n")
	fmt.Printf("  2. Extract and place frpc in your PATH or current directory\n")
	fmt.Printf("  3. Ensure frpc is executable: chmod +x frpc\n")
	fmt.Println()
	fmt.Printf("%s🔗 After installation, ensure:%s\n", ansiBold, ansiReset)
	fmt.Printf("  1. frpc is in your PATH or current directory\n")
	fmt.Printf("  2. Run 'frpc version' to confirm installation\n")
	fmt.Println()
	fmt.Printf("%s%s==============================================%s\n", ansiYellow, ansiBold, ansiReset)
	fmt.Println()
}

type DeviceConnectOptions struct {
	DeviceID   string
	Background bool
	UseNative  bool // Use native Go implementation instead of external binary
}

// Global client instance
var deviceClient *device_connect.Client

// getDeviceClient returns the global device client, initializing it if needed
func getDeviceClient() *device_connect.Client {
	if deviceClient == nil {
		deviceClient = device_connect.NewClient(device_connect.DefaultURL)
	}
	return deviceClient
}

func NewDeviceConnectCommand() *cobra.Command {
	opts := &DeviceConnectOptions{}

	cmd := &cobra.Command{
		Use:   "device-connect [command] [flags]",
		Short: "Manage remote connections for local Android development devices",
		Long: `Manage remote connections for local Android development devices.
This command allows you to securely connect Android devices (emulators or physical devices)
to remote cloud services for remote access and debugging.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ExecuteDeviceConnect(cmd, opts, args)
		},
		Example: `  # Interactively select a device to connect
  gbox device-connect

  # Connect to specific device
  gbox device-connect --device abc123xyz456-usb

  # Connect device in background
  gbox device-connect --device abc789pqr012-ip --background

  # List all available devices
  gbox device-connect ls

  # List devices in JSON format
  gbox device-connect ls --format json

  # Unregister specific device
  gbox device-connect unregister abc789pqr012-ip

  # Stop the device proxy service
  gbox device-connect kill-server`,
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.DeviceID, "device", "d", "", "Specify the Android device ID to connect to")
	flags.BoolVarP(&opts.Background, "background", "b", false, "Run connection in background")
	flags.BoolVar(&opts.UseNative, "native", false, "Use native Go implementation (experimental)")

	cmd.AddCommand(
		NewDeviceConnectListCommand(),
		NewDeviceConnectUnregisterCommand(),
		NewDeviceConnectKillServerCommand(),
		NewDeviceConnectServerCommand(),
	)

	return cmd
}

func ExecuteDeviceConnect(cmd *cobra.Command, opts *DeviceConnectOptions, args []string) error {
	if !checkAdbInstalled() {
		printAdbInstallationHint()
		return fmt.Errorf("ADB is not installed or not in your PATH. Please install ADB and try again.")
	}

	// Always use the unified server (like adb start-server)
	// The server will be auto-started if not running
	
	// Note: Legacy mode with external binaries is being phased out
	// All functionality now goes through the unified gbox server
	
	// The actual device connection will happen via HTTP API calls
	// to the server, which will be started automatically by the daemon manager

	if opts.DeviceID == "" {
		return runInteractiveDeviceSelection(opts)
	}
	return connectToDevice(opts.DeviceID, opts)
}

func isServiceRunning() (bool, error) {
	// First check if PID file exists
	deviceProxyHome := config.GetDeviceProxyHome()
	pidFile := filepath.Join(deviceProxyHome, "device-proxy.pid")

	if _, err := os.Stat(pidFile); os.IsNotExist(err) {
		return false, nil
	}

	// Read PID from file
	pidBytes, err := os.ReadFile(pidFile)
	if err != nil {
		return false, nil
	}

	var pid int
	if _, err := fmt.Sscanf(string(pidBytes), "%d", &pid); err != nil {
		return false, nil
	}

	// Check if process is still running
	if err := exec.Command("kill", "-0", fmt.Sprintf("%d", pid)).Run(); err != nil {
		// Process is not running, remove PID file
		os.Remove(pidFile)
		return false, nil
	}

	// Try to check service status via API
	client := getDeviceClient()
	running, onDemandEnabled, err := client.IsServiceRunning()
	if err != nil {
		// If API check fails, assume service is running (we have a valid PID)
		return true, nil
	}

	// Check if onDemandEnabled is false and warn user
	if running && !onDemandEnabled {
		fmt.Println("Warning: Reusing existing device-proxy service that does not have on-demand registration enabled.")
		fmt.Println("All devices will be automatically registered for remote access.")
		fmt.Println("If you don't want this behavior, either:")
		fmt.Println("  - Stop the existing service and restart with ENABLE_DEVICE_REGISTER_ON_DEMAND=true")
		fmt.Println("  - Use 'gbox device-connect kill-server' to stop the current service")
		fmt.Println()
	}

	return running, nil
}

func runInteractiveDeviceSelection(opts *DeviceConnectOptions) error {
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
	return connectToDevice(deviceID, opts)
}

func connectToDevice(deviceID string, opts *DeviceConnectOptions) error {
	// Register device via daemon API
	req := map[string]string{"deviceId": deviceID}
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

// executeKillServer calls the existing kill-server functionality
func executeKillServer() error {
	opts := &DeviceConnectKillServerOptions{
		Force: false,
		All:   false,
	}
	// Create a dummy command for ExecuteDeviceConnectKillServer
	// We only need this for the function signature, the actual cmd parameter is not used in the implementation
	return ExecuteDeviceConnectKillServer(nil, opts)
}
