package device_connect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	DefaultPort = 19925
	DefaultURL  = "http://localhost:19925"
)

// DeviceInfo represents a device from the API
type DeviceInfo struct {
	Id                  string `json:"id"`
	Udid                string `json:"udid"`
	State               string `json:"state"`
	Interfaces          []struct {
		Name string `json:"name"`
		Ipv4 string `json:"ipv4"`
	} `json:"interfaces"`
	Pid                 int    `json:"pid"`
	BuildVersionRelease string `json:"ro.build.version.release"`
	BuildVersionSdk     string `json:"ro.build.version.sdk"`
	ProductManufacturer string `json:"ro.product.manufacturer"`
	ProductModel        string `json:"ro.product.model"`
	ProductCpuAbi       string `json:"ro.product.cpu.abi"`
	SerialNo            string `json:"ro.serialno"`
	LastUpdateTimestamp int64  `json:"last.update.timestamp"`
	ConnectionType      string `json:"connectionType"`
	IsRegistrable       bool   `json:"isRegistrable"`
}

// DeviceListResponse represents the response from GET /api/devices
type DeviceListResponse struct {
	Success           bool         `json:"success"`
	Devices           []DeviceInfo `json:"devices"`
	OnDemandEnabled   bool         `json:"onDemandEnabled"`
	Error             string       `json:"error,omitempty"`
}

// DeviceActionResponse represents the response from POST /api/devices/register and /api/devices/unregister
type DeviceActionResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message,omitempty"`
	DeviceID  string `json:"device_id,omitempty"`
	Error     string `json:"error,omitempty"`
}

// Client represents a device proxy API client
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new device proxy API client
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// IsServiceRunning checks if the device proxy service is running and returns onDemandEnabled status
func (c *Client) IsServiceRunning() (bool, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/api/devices", c.baseURL), nil)
	if err != nil {
		return false, false, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, false, fmt.Errorf("service returned status code: %d", resp.StatusCode)
	}

	// Parse response to check onDemandEnabled status
	var deviceListResp DeviceListResponse
	if err := json.NewDecoder(resp.Body).Decode(&deviceListResp); err != nil {
		return false, false, fmt.Errorf("failed to parse response: %v", err)
	}

	if !deviceListResp.Success {
		return false, false, fmt.Errorf("API returned success: false")
	}

	return true, deviceListResp.OnDemandEnabled, nil
}

// GetDevices retrieves all available devices from the API
func (c *Client) GetDevices() ([]DeviceInfo, error) {
	resp, err := c.httpClient.Get(fmt.Sprintf("%s/api/devices", c.baseURL))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var deviceListResp DeviceListResponse
	if err := json.NewDecoder(resp.Body).Decode(&deviceListResp); err != nil {
		return nil, err
	}

	if !deviceListResp.Success {
		return nil, fmt.Errorf("API error: %s", deviceListResp.Error)
	}

	return deviceListResp.Devices, nil
}

// GetDeviceInfo retrieves information about a specific device
func (c *Client) GetDeviceInfo(deviceID string) (*DeviceInfo, error) {
	devices, err := c.GetDevices()
	if err != nil {
		return nil, err
	}

	for _, device := range devices {
		if device.Id == deviceID {
			return &device, nil
		}
	}

	return nil, fmt.Errorf("device not found: %s", deviceID)
}

// RegisterDevice registers a device for remote access
func (c *Client) RegisterDevice(deviceID string) error {
	data := map[string]string{"deviceId": deviceID}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Post(fmt.Sprintf("%s/api/devices/register", c.baseURL), "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var deviceActionResp DeviceActionResponse
	if err := json.NewDecoder(resp.Body).Decode(&deviceActionResp); err != nil {
		return err
	}

	if !deviceActionResp.Success {
		return fmt.Errorf("failed to register device: %s", deviceActionResp.Error)
	}

	return nil
}

// UnregisterDevice unregisters a device
func (c *Client) UnregisterDevice(deviceID string) error {
	data := map[string]string{"deviceId": deviceID}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Post(fmt.Sprintf("%s/api/devices/unregister", c.baseURL), "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var deviceActionResp DeviceActionResponse
	if err := json.NewDecoder(resp.Body).Decode(&deviceActionResp); err != nil {
		return err
	}

	if !deviceActionResp.Success {
		return fmt.Errorf("failed to unregister device: %s", deviceActionResp.Error)
	}

	return nil
} 