package adb_expose

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)


// getPortForwardURL gets the WebSocket URL for port forwarding
func getPortForwardURL(config Config) (string, error) {
	url := fmt.Sprintf("%s/boxes/%s/port-forward-url", config.GboxURL, config.BoxID)

	reqBody := PortForwardRequest{
		Ports: config.TargetPorts,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request body: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.APIKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var response PortForwardResponse
	decodeErr := json.NewDecoder(resp.Body).Decode(&response)
	if decodeErr != nil {
		// read body content length, avoid leaking sensitive information
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("decode response failed (status %d, body length %d): %v", resp.StatusCode, len(body), decodeErr)
	}

	if response.URL == "" {
		return "", fmt.Errorf("API response missing URL field")
	}

	return response.URL, nil
}

// ConnectWebSocket creates a WebSocket connection for port forwarding
func ConnectWebSocket(config Config) (*MultiplexClient, error) {
	wsURL, err := getPortForwardURL(config)
	if err != nil {
		return nil, fmt.Errorf("failed to get port forward URL: %v", err)
	}

	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to establish WebSocket connection: %v", err)
	}

	client := NewMultiplexClient(ws)
	return client, nil
}

// parseMessage parses a multiplexing protocol message
func parseMessage(data []byte) (msgType byte, streamID uint32, payload []byte, err error) {
	if len(data) < 5 {
		return 0, 0, nil, fmt.Errorf("message too short")
	}

	msgType = data[0]
	streamID = binary.BigEndian.Uint32(data[1:5])
	payload = data[5:]

	return msgType, streamID, payload, nil
}

// ParsePorts parses a comma-separated string of ports
func ParsePorts(portStr string) ([]int, error) {
	if portStr == "" {
		return nil, fmt.Errorf("no ports specified")
	}

	parts := strings.Split(portStr, ",")
	ports := make([]int, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		port, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid port '%s': %v", part, err)
		}

		if port <= 0 || port > 65535 {
			return nil, fmt.Errorf("port %d is out of range (1-65535)", port)
		}

		ports = append(ports, port)
	}

	if len(ports) == 0 {
		return nil, fmt.Errorf("no valid ports specified")
	}

	return ports, nil
}