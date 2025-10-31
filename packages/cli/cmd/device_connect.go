package cmd

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
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

// Note: Device client functionality has been moved to daemon.DefaultManager
// All device operations now go through the unified server API

type DeviceConnectOptions struct {
	DeviceID   string
	Background bool
	Linux      bool
}

func NewDeviceConnectCommand() *cobra.Command {
	opts := &DeviceConnectOptions{}

	cmd := &cobra.Command{
		Use:   "device-connect [device_id] [flags]",
		Short: "Manage remote connections for local devices",
		Long: `Manage remote connections for local devices.
This command allows you to securely connect Android devices (emulators or physical devices)
or connect this Linux machine to remote cloud services for remote access and debugging.

If no device ID is provided, an interactive device selection will be shown for Android.
Use --linux to connect this Linux machine to Access Point (AP).`,
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

  # Register a device for remote access
  gbox device-connect register

  # Unregister specific device
  gbox device-connect unregister abc789pqr012-ip

  # Connect this Linux machine to AP (optionally reuse/provide device id)
  gbox device-connect --linux
  gbox device-connect --linux -d <device-id>`,
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.DeviceID, "device", "d", "", "Specify the Android device ID to connect")
	flags.BoolVarP(&opts.Background, "background", "b", false, "Run in background mode")
	flags.BoolVar(&opts.Linux, "linux", false, "Connect this Linux machine to Access Point (AP)")

	cmd.AddCommand(
		NewDeviceConnectRegisterCommand(),
		NewDeviceConnectListCommand(),
		NewDeviceConnectUnregisterCommand(),
	)

	return cmd
}
func ExecuteDeviceConnect(cmd *cobra.Command, opts *DeviceConnectOptions, args []string) error {
	debug := os.Getenv("DEBUG") == "true"

	// Linux path: directly reuse linux-connect flow and return
	if opts.Linux {
		linuxOpts := &DeviceConnectLinuxConnectOptions{DeviceID: opts.DeviceID}
		return ExecuteDeviceConnectLinuxConnect(cmd, linuxOpts, args)
	}

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

// runAsRoot executes a command with root privileges if needed
func runAsRoot(name string, args ...string) error {
	// Check if already running as root (Unix-like systems)
	if runtime.GOOS != "windows" {
		cmd := exec.Command("id", "-u")
		output, err := cmd.Output()
		if err == nil && strings.TrimSpace(string(output)) == "0" {
			// Already root, run directly
			cmd := exec.Command(name, args...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()
		}
	}

	// Check if sudo is available
	if _, err := exec.LookPath("sudo"); err == nil {
		// Use sudo
		fullArgs := append([]string{name}, args...)
		cmd := exec.Command("sudo", fullArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	// No sudo available, try running directly
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// installADB attempts to install ADB using the system package manager
func installADB() error {
	if _, err := exec.LookPath("brew"); err == nil {
		// macOS with Homebrew
		cmd := exec.Command("brew", "install", "android-platform-tools")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	if _, err := exec.LookPath("apt-get"); err == nil {
		// Debian/Ubuntu
		return runAsRoot("apt-get", "install", "-y", "android-tools-adb")
	}

	if _, err := exec.LookPath("yum"); err == nil {
		// RHEL/CentOS
		return runAsRoot("yum", "install", "-y", "android-tools")
	}

	return fmt.Errorf("unable to detect package manager")
}

// installFrpc attempts to install frpc using the system package manager or GitHub releases
func installFrpc() error {
	// Try Homebrew first on macOS
	if runtime.GOOS == "darwin" {
		if _, err := exec.LookPath("brew"); err == nil {
			cmd := exec.Command("brew", "install", "frpc")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err == nil {
				return nil
			}
			// If brew fails, fall through to GitHub installation
		}
	}

	// Download from GitHub releases for all platforms
	return installFrpcFromGitHub()
}

// installFrpcFromGitHub downloads and installs frpc from GitHub releases
func installFrpcFromGitHub() error {
	// Get latest frpc version from GitHub API
	resp, err := http.Get("https://api.github.com/repos/fatedier/frp/releases/latest")
	if err != nil {
		return fmt.Errorf("failed to fetch frpc version: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch frpc version: HTTP %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("failed to parse release info: %v", err)
	}

	// Remove 'v' prefix from version
	frpcVersion := strings.TrimPrefix(release.TagName, "v")
	if frpcVersion == "" {
		return fmt.Errorf("invalid version tag: %s", release.TagName)
	}

	// Detect OS and architecture
	osType := runtime.GOOS
	archType := runtime.GOARCH

	// Map architecture names to frp naming convention
	switch archType {
	case "amd64":
		// Keep as is
	case "arm64":
		// Keep as is
	case "arm":
		// Keep as is
	default:
		return fmt.Errorf("unsupported architecture: %s", archType)
	}

	// Construct download URL
	downloadURL := fmt.Sprintf(
		"https://github.com/fatedier/frp/releases/download/v%s/frp_%s_%s_%s.tar.gz",
		frpcVersion, frpcVersion, osType, archType,
	)

	// Create temporary directory for frpc binary only
	tempDir, err := os.MkdirTemp("", "frpc-install-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Download and stream-extract in one pass
	resp, err = http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download frpc: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download frpc: HTTP %d", resp.StatusCode)
	}

	// Create gzip reader directly from HTTP response body (no intermediate file)
	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %v", err)
	}
	defer gzr.Close()

	// Create tar reader from gzip stream
	tr := tar.NewReader(gzr)

	// Extract frpc binary directly from stream
	var frpcBinaryPath string
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar entry: %v", err)
		}

		// Look for frpc binary
		if filepath.Base(header.Name) == "frpc" && header.Typeflag == tar.TypeReg {
			frpcBinaryPath = filepath.Join(tempDir, "frpc")

			// Create file with proper permissions
			outFile, err := os.OpenFile(frpcBinaryPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return fmt.Errorf("failed to create frpc binary: %v", err)
			}

			// Stream-copy from tar to file
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return fmt.Errorf("failed to extract frpc binary: %v", err)
			}

			if err := outFile.Close(); err != nil {
				return fmt.Errorf("failed to close frpc binary: %v", err)
			}

			// Found and extracted, stop processing archive
			break
		}
	}

	if frpcBinaryPath == "" {
		return fmt.Errorf("frpc binary not found in archive")
	}

	// Install to system location
	installPath := "/usr/local/bin/frpc"
	if err := installBinaryWithSudo(frpcBinaryPath, installPath); err != nil {
		return fmt.Errorf("failed to install frpc to %s: %v", installPath, err)
	}

	return nil
}

// installBinaryWithSudo installs a binary to the system location, using sudo if necessary
func installBinaryWithSudo(src, dst string) error {
	// Try direct copy first (works if we have write permission)
	if err := copyBinaryFile(src, dst); err == nil {
		return nil
	}

	// If direct copy fails, use install command with runAsRoot (Unix-like systems)
	if runtime.GOOS != "windows" {
		if err := runAsRoot("install", "-m", "755", src, dst); err != nil {
			return fmt.Errorf("install with elevated privileges failed: %v", err)
		}
		return nil
	}

	return fmt.Errorf("permission denied and elevated privileges not available")
}

// copyBinaryFile copies a binary file with executable permissions
func copyBinaryFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source: %v", err)
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("failed to create destination: %v", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy: %v", err)
	}

	return nil
}
