package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/babelcloud/gbox/packages/cli/config"
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
	if !checkAdbInstalled() {
		printAdbInstallationHint()
		return fmt.Errorf("ADB is not installed or not in your PATH; please install ADB and try again")
	}

	if !checkFrpcInstalled() {
		printFrpcInstallationHint()
		return fmt.Errorf("frpc is not installed or not in your PATH; please install frpc and try again")
	}

	// Check and install prerequisites
	if err := checkAndInstallPrerequisites(); err != nil {
		fmt.Fprintf(os.Stderr, "\n╔═══════════════════════════════════════╗\n")
		fmt.Fprintf(os.Stderr, "║  ❌  Prerequisites Installation Failed ║\n")
		fmt.Fprintf(os.Stderr, "╚═══════════════════════════════════════╝\n\n")
		fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
		fmt.Fprintf(os.Stderr, "💡 Quick Fix:\n")
		fmt.Fprintf(os.Stderr, "   • Fix the errors above and retry\n")
		fmt.Fprintf(os.Stderr, "   • Or disable Appium: export GBOX_INSTALL_APPIUM=false\n\n")
		return err
	}

	// Ensure device proxy service is running
	if err := device_connect.EnsureDeviceProxyRunning(isServiceRunning); err != nil {
		return fmt.Errorf("failed to start device proxy service: %v", err)
	}

	if opts.DeviceID == "" {
		return runInteractiveDeviceSelection(opts)
	}
	return connectToDevice(opts.DeviceID, opts)
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
		return fmt.Errorf("Node.js and npm are not installed: %v\n\n"+
			"╔═══════════════════════════════════════╗\n"+
			"║  📦  Install Node.js                  ║\n"+
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
	devices, err := getDevicesWithValidation(client, 60*time.Second)
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
		statusColor := color.New(color.Faint) // 使用淡色（灰色）
		if device.IsRegistrable {             // Assuming IsRegistrable should be IsRegistered
			status = "Registered"
			statusColor = color.New(color.FgGreen)
		}
		fmt.Printf("%d. %s (%s, %s) - %s [%s]\n",
			i+1,
			color.New(color.FgCyan).Sprint(device.SerialNo+"-"+device.ConnectionType),
			device.ProductModel,
			device.ConnectionType,
			device.ProductManufacturer,
			statusColor.Sprint(status))
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
		// User pressed Ctrl+C during selection; stop proxy first then exit gracefully
		_ = executeKillServer()
		return nil
	case <-inputDone:
		// proceed
	}
	if choice < 1 || choice > len(devices) {
		return fmt.Errorf("invalid selection: %d", choice)
	}

	selectedDevice := devices[choice-1]
	return connectToDevice(selectedDevice.Id, opts)
}

func connectToDevice(deviceID string, opts *DeviceConnectOptions) error {
	client := getDeviceClient()

	device, err := getDeviceInfoWithRetry(client, deviceID, 60*time.Second)
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

	// Get and display devices URL for the current profile
	pm := profile.NewProfileManager()
	if err := pm.Load(); err == nil {
		if devicesURL, err := pm.GetDevicesURL(); err == nil {
			fmt.Printf("You can view your devices at: %s\n", color.CyanString(devicesURL))
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
	fmt.Printf("Disconnecting %s (%s, %s)...\n",
		device.ProductModel, device.ConnectionType, device.ProductManufacturer)

	// First unregister the device
	if err := client.UnregisterDevice(deviceID); err != nil {
		fmt.Printf("Warning: failed to unregister device: %v\n", err)
	}

	// Then stop the device proxy service using existing kill-server logic
	if err := executeKillServer(); err != nil {
		fmt.Printf("Warning: failed to stop device proxy service: %v\n", err)
	}

	return nil
}

// executeKillServer calls the existing kill-server functionality
func executeKillServer() error {
	opts := &DeviceConnectKillServerOptions{
		Force: true,
		All:   false,
	}
	// Create a dummy command for ExecuteDeviceConnectKillServer
	// We only need this for the function signature, the actual cmd parameter is not used in the implementation
	return ExecuteDeviceConnectKillServer(nil, opts)
}

// getDeviceInfoWithRetry waits for the API to be ready and then looks up the device
func getDeviceInfoWithRetry(client *device_connect.Client, deviceID string, timeout time.Duration) (*device_connect.DeviceInfo, error) {
	devices, err := getDevicesWithValidation(client, timeout)
	if err != nil {
		return nil, err
	}
	for _, d := range devices {
		if d.Id == deviceID {
			return &d, nil
		}
	}
	return nil, fmt.Errorf("device not found: %s", deviceID)
}

// getDevicesWithValidation retries fetching devices until at least one device has a non-empty serial number,
// or until the timeout is reached. This avoids the abnormal "-usb" entries when the upstream is not ready.
func getDevicesWithValidation(client *device_connect.Client, timeout time.Duration) ([]device_connect.DeviceInfo, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	var lastDevices []device_connect.DeviceInfo
	for {
		devices, err := client.GetDevices()
		if err == nil {
			lastDevices = devices
			if !hasAnyEmptySerial(devices) {
				return devices, nil
			}
			lastErr = fmt.Errorf("some devices missing serial numbers; retrying")
		} else {
			lastErr = err
		}

		if time.Now().After(deadline) {
			if len(lastDevices) > 0 {
				valid := filterValidDevices(lastDevices)
				if len(valid) > 0 {
					return valid, nil
				}
			}
			return nil, lastErr
		}
		time.Sleep(1 * time.Second)
	}
}

// allSerialMissing returns true when every device has an empty SerialNo
func allSerialMissing(devices []device_connect.DeviceInfo) bool {
	if len(devices) == 0 {
		return false
	}
	for _, d := range devices {
		if d.SerialNo != "" {
			return false
		}
	}
	return true
}

// hasAnyEmptySerial returns true if any device has an empty SerialNo
func hasAnyEmptySerial(devices []device_connect.DeviceInfo) bool {
	for _, d := range devices {
		if d.SerialNo == "" {
			return true
		}
	}
	return false
}

// filterValidDevices returns only devices with a non-empty SerialNo
func filterValidDevices(devices []device_connect.DeviceInfo) []device_connect.DeviceInfo {
	var result []device_connect.DeviceInfo
	for _, d := range devices {
		if d.SerialNo != "" {
			result = append(result, d)
		}
	}
	return result
}
