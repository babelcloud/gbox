package adb_expose

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ForwardInfo represents information about a port forward
type ForwardInfo struct {
	BoxID       string    `json:"box_id"`
	LocalPorts  []int     `json:"local_ports"`
	RemotePorts []int     `json:"remote_ports"`
	Status      string    `json:"status"`
	StartedAt   time.Time `json:"started_at"`
	Error       string    `json:"error,omitempty"`
}

// Client represents an ADB expose client
type Client struct {
	serverURL string
	client    *http.Client
}

// NewClient creates a new ADB expose client
func NewClient(serverURL string) *Client {
	return &Client{
		serverURL: serverURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Start starts ADB port exposure for a box
func (c *Client) Start(boxID string, localPorts, remotePorts []int) error {
	reqBody := map[string]interface{}{
		"box_id":       boxID,
		"local_ports":  localPorts,
		"remote_ports": remotePorts,
	}

	return c.makeRequest("POST", "/api/adb-expose/start", reqBody)
}

// Stop stops ADB port exposure for a box
func (c *Client) Stop(boxID string) error {
	reqBody := map[string]interface{}{
		"box_id": boxID,
	}

	return c.makeRequest("POST", "/api/adb-expose/stop", reqBody)
}

// List returns all active ADB port exposures
func (c *Client) List() ([]ForwardInfo, error) {
	resp, err := c.client.Get(c.serverURL + "/api/adb-expose/list")
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	var result struct {
		Forwards []ForwardInfo `json:"forwards"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	return result.Forwards, nil
}

// makeRequest makes an HTTP request to the server
func (c *Client) makeRequest(method, endpoint string, reqBody interface{}) error {
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %v", err)
	}

	resp, err := c.client.Post(c.serverURL+endpoint, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %v", err)
	}

	if resp.StatusCode == http.StatusConflict {
		fmt.Printf("ADB port is already exposed for box %s\n", reqBody.(map[string]interface{})["box_id"])
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		errorMsg, _ := result["error"].(string)
		return fmt.Errorf("server error: %s", errorMsg)
	}

	success, _ := result["success"].(bool)
	if !success {
		errorMsg, _ := result["error"].(string)
		return fmt.Errorf("operation failed: %s", errorMsg)
	}

	return nil
}