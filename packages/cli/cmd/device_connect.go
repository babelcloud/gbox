package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/babelcloud/gbox/packages/cli/config"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect"
	"github.com/spf13/cobra"
)

// printDeveloperModeHint prints the developer mode hint with dim formatting
func printDeveloperModeHint() {
	const (
		ansiDim   = "\033[2m"
		ansiReset = "\033[0m"
	)
	fmt.Printf("%sIf you can not see your devices here, make sure you have turned on the developer mode on your Android device. For more details, see https://docs.gbox.ai/cli%s\n", ansiDim, ansiReset)
}

type DeviceConnectOptions struct {
	DeviceID   string
	Background bool
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

	cmd.AddCommand(
		NewDeviceConnectListCommand(),
		NewDeviceConnectUnregisterCommand(),
		NewDeviceConnectKillServerCommand(),
	)

	return cmd
}

func ExecuteDeviceConnect(cmd *cobra.Command, opts *DeviceConnectOptions, args []string) error {
	// Ensure device proxy service is running
	if err := device_connect.EnsureDeviceProxyRunning(isServiceRunning); err != nil {
		return fmt.Errorf("failed to start device proxy service: %v", err)
	}

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
	client := getDeviceClient()
	devices, err := client.GetDevices()
	if err != nil {
		return fmt.Errorf("failed to get available devices: %v", err)
	}
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
		if device.IsRegistrable { // Assuming IsRegistrable should be IsRegistered
			status = "Registered"
		}
		fmt.Printf("%d. %s (%s, %s) - %s [%s]\n",
			i+1,
			device.Id,
			device.ProductModel,
			device.ConnectionType,
			device.ProductManufacturer,
			status)
	}
	fmt.Println()
	fmt.Print("Enter a number: ")
	var choice int
	fmt.Scanf("%d", &choice)
	if choice < 1 || choice > len(devices) {
		return fmt.Errorf("invalid selection: %d", choice)
	}

	selectedDevice := devices[choice-1]
	return connectToDevice(selectedDevice.Id, opts)
}

func connectToDevice(deviceID string, opts *DeviceConnectOptions) error {
	client := getDeviceClient()

	device, err := client.GetDeviceInfo(deviceID)
	if err != nil {
		return fmt.Errorf("failed to get device info: %v", err)
	}

	fmt.Printf("Establishing remote connection for %s (%s, %s)...\n",
		device.ProductModel, device.ConnectionType, device.ProductManufacturer)

	// Register the device
	if err := client.RegisterDevice(deviceID); err != nil {
		return fmt.Errorf("failed to register device: %v", err)
	}

	fmt.Printf("Connection established successfully!\n")

	if opts.Background {
		fmt.Println("(Running in background. Use 'gbox device-connect unregister' to stop.)")
		return nil
	}

	fmt.Println("(Running in foreground. Press Ctrl+C to disconnect.)")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	fmt.Printf("Disconnecting %s (%s, %s)...\n",
		device.ProductModel, device.ConnectionType, device.ProductManufacturer)
	return client.UnregisterDevice(deviceID)
}
