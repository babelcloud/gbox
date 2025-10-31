package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/babelcloud/gbox/packages/cli/internal/daemon"
	"github.com/spf13/cobra"
)

type DeviceConnectLinuxConnectOptions struct {
	DeviceID string
}

func ExecuteDeviceConnectLinuxConnect(cmd *cobra.Command, opts *DeviceConnectLinuxConnectOptions, args []string) error {
	// Force restart local server on each execution of this command
	_ = daemon.DefaultManager.StopServer()
	if err := daemon.DefaultManager.StartServer(); err != nil {
		return fmt.Errorf("failed to restart local server: %v", err)
	}

	req := map[string]string{}
	// Try to reuse regId stored on this machine if available
	if opts.DeviceID == "" {
		if regId, _ := readLocalRegId(); regId != "" {
			req["regId"] = regId
		}
	}
	if opts.DeviceID != "" {
		req["deviceId"] = opts.DeviceID
	}
	var resp map[string]interface{}
	if err := daemon.DefaultManager.CallAPI("POST", "/api/devices/linux/connect", req, &resp); err != nil {
		return fmt.Errorf("failed to connect linux device to access point: %v", err)
	}
	if success, ok := resp["success"].(bool); !ok || !success {
		return fmt.Errorf("failed to connect linux device: %v", resp["error"])
	}
	// Persist regId returned from server for future reuse
	deviceIdStr := ""
	if v, ok := resp["deviceId"]; ok {
		if id, ok2 := v.(string); ok2 {
			deviceIdStr = id
		}
	}
	regIdStr := ""
	if v, ok := resp["regId"]; ok {
		if regId, ok2 := v.(string); ok2 && regId != "" {
			regIdStr = regId
			_ = writeLocalRegId(regId)
		}
	}
	if deviceIdStr != "" && regIdStr != "" {
		fmt.Printf("Linux device connected. Device ID: %s (regId: %s)\n", deviceIdStr, regIdStr)
	} else if deviceIdStr != "" {
		fmt.Printf("Linux device connected. Device ID: %s\n", deviceIdStr)
	} else {
		fmt.Println("Linux device connected to access point.")
	}
	return nil
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
	s := string(data)
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r' || s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
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
