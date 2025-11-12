package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/babelcloud/gbox/packages/cli/internal/daemon"
	"github.com/spf13/cobra"
)

type DeviceConnectRegisterOptions struct {
	DeviceID string
}

func NewDeviceConnectRegisterCommand() *cobra.Command {
	opts := &DeviceConnectRegisterOptions{}

	cmd := &cobra.Command{
		Use:     "register [device_id] [flags]",
		Aliases: []string{"reg"},
		Short:   "Register a device for remote access",
		Long:    "Register a device for remote access. Use 'local' to register this machine as desktop, or provide a device ID to register an Android device.",
		Example: `  # Register an Android device by ID
  gbox device-connect register abc123xyz456

  # Register and connect this machine as desktop
  gbox device-connect register local`,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  false,
		SilenceErrors: true, // Don't show errors twice (we handle them in RunE)
		RunE: func(cmd *cobra.Command, args []string) error {
			// No interactive mode - require device ID
			if len(args) == 0 && opts.DeviceID == "" {
				return fmt.Errorf("device ID is required. Use 'gbox device-connect' for interactive selection")
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

	// Determine device type based on deviceID
	// If "local", register as desktop with auto-detected OS
	// Otherwise, register as mobile (Android) device
	if strings.EqualFold(deviceID, "local") {
		// For local registration, use empty deviceID and register as desktop
		// OS type will be auto-detected by registerDevice
		return registerDevice("", "")
	}

	// For Android device, ensure ADB is installed
	if !checkAdbInstalled() {
		printAdbInstallationHint()
		return fmt.Errorf("adb is not installed or not in your PATH; install adb and try again")
	}

	// Register as mobile (Android) device
	return registerDevice(deviceID, "android")
}

// registerDevice is defined in device_connect.go (same package)

type DeviceConnectLinuxConnectOptions struct {
	DeviceID string
}

// ExecuteDeviceConnectLinuxConnect is deprecated: use registerDevice with type="linux" instead
// This function is kept for backward compatibility but now calls registerDevice
func ExecuteDeviceConnectLinuxConnect(cmd *cobra.Command, opts *DeviceConnectLinuxConnectOptions, args []string) error {
	// Force restart local server on each execution of this command
	_ = daemon.DefaultManager.StopServer()
	if err := daemon.DefaultManager.StartServer(); err != nil {
		return fmt.Errorf("failed to restart local server: %v", err)
	}

	// Use registerDevice which handles both registration and connection for desktop devices
	deviceID := ""
	if opts.DeviceID != "" && !strings.EqualFold(opts.DeviceID, "local") {
		deviceID = opts.DeviceID
	}
	// Empty deviceID and deviceType will register as desktop with auto-detected OS
	return registerDevice(deviceID, "")
}

// getLocalRegIdPath returns the file path for storing the reg_id on this machine.
func getLocalRegIdPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".gbox")
	return filepath.Join(dir, "reg_id"), nil
}

// readLocalRegId reads reg_id from ~/.gbox/reg_id if exists.
func readLocalRegId() (string, error) {
	path, err := getLocalRegIdPath()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	// Trim trailing spaces/newlines
	s := strings.TrimSpace(string(data))
	return s, nil
}

// writeLocalRegId writes reg_id into ~/.gbox/reg_id, creating directory if needed.
func writeLocalRegId(regId string) error {
	path, err := getLocalRegIdPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(regId+"\n"), 0o600)
}
