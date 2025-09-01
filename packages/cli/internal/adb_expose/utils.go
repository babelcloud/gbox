package adb_expose

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/babelcloud/gbox/packages/cli/config"
	"github.com/gorilla/websocket"
)

// PidInfo holds info for a running port-forward process
// Support multiple ports in a single process
type PidInfo struct {
	Pid         int       `json:"pid"`
	BoxID       string    `json:"boxid"`
	LocalPorts  []int     `json:"localports"`
	RemotePorts []int     `json:"remoteports"`
	StartedAt   time.Time `json:"started_at"`
}

func ensureGboxDir() error {
	dir := config.GetGboxHome()
	return os.MkdirAll(dir, 0700)
}

const pidFileNamePrefix = "gbox-adb-expose-"
const pidFileNameSuffix = ".pid"
const logFileNameSuffix = ".log"

func pidFilePath(boxId string, localPort int) string {
	return config.GetGboxHome() + "/" + pidFileNamePrefix + boxId + "-" + strconv.Itoa(localPort) + pidFileNameSuffix
}

func logFilePath(boxId string, localPort int) string {
	return config.GetGboxHome() + "/" + pidFileNamePrefix + boxId + "-" + strconv.Itoa(localPort) + logFileNameSuffix
}

const pidFilePattern = "gbox-adb-expose-*.pid"

// WritePidFile writes a pid file for multiple ports (first local port is used for file name)
func WritePidFile(boxId string, localPorts, remotePorts []int) error {
	if err := ensureGboxDir(); err != nil {
		return err
	}
	// Use the first local port for the pid file name
	path := pidFilePath(boxId, localPorts[0])
	// check if pid file exists
	if _, err := os.Stat(path); err == nil {
		f, err := os.Open(path)
		if err == nil {
			var info PidInfo
			decodeErr := json.NewDecoder(f).Decode(&info)
			f.Close()
			if decodeErr == nil && IsProcessAlive(info.Pid) {
				return fmt.Errorf("adb-expose already running for boxId=%s, localPort=%d (pid=%d)", boxId, localPorts[0], info.Pid)
			}
		}
	}
	info := PidInfo{
		Pid:         os.Getpid(),
		BoxID:       boxId,
		LocalPorts:  localPorts,
		RemotePorts: remotePorts,
		StartedAt:   time.Now(),
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	return enc.Encode(&info)
}

// RemovePidFile removes the pid file for a given local port
func RemovePidFile(boxId string, localPort int) error {
	return os.Remove(pidFilePath(boxId, localPort))
}

func RemoveLogFile(boxId string, localPort int) error {
	return os.Remove(logFilePath(boxId, localPort))
}

func ListPidFiles() ([]PidInfo, error) {
	dir := config.GetGboxHome()
	files, err := filepath.Glob(dir + "/" + pidFilePattern)
	if err != nil {
		return nil, err
	}
	var infos []PidInfo
	for _, f := range files {
		file, err := os.Open(f)
		if err != nil {
			continue
		}
		var info PidInfo
		err = json.NewDecoder(file).Decode(&info)
		file.Close()
		if err == nil {
			infos = append(infos, info)
		}
	}
	return infos, nil
}

func IsProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 does not kill the process, just checks existence
	return proc.Signal(syscall.Signal(0)) == nil
}

func FindPidFile(boxId string, localPort int) (string, error) {
	path := pidFilePath(boxId, localPort)
	_, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	return path, nil
}

func getPortForwardURL(config Config) (string, error) {
	url := fmt.Sprintf("%s/api/v1/boxes/%s/port-forward-url", config.GboxURL, config.BoxID)

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

// PrintStartupMessage prints the startup message for adb-expose
func PrintStartupMessage(pid int, logPath string, boxID string) {
	fmt.Printf("[gbox] Adb-expose started in background for box %s (pid=%d). Logs: %s\n\nUse 'gbox adb-expose list' to view, 'gbox adb-expose stop %s' to stop.\n", boxID, pid, logPath, boxID)
}

func parseMessage(data []byte) (msgType byte, streamID uint32, payload []byte, err error) {
	if len(data) < 5 {
		return 0, 0, nil, fmt.Errorf("message too short")
	}

	msgType = data[0]
	streamID = binary.BigEndian.Uint32(data[1:5])
	payload = data[5:]

	return msgType, streamID, payload, nil
}

// PrepareGBOXEnvironment prepares environment variables for daemon process
// ensuring important GBOX environment variables are preserved
func PrepareGBOXEnvironment() []string {
	env := os.Environ()

	// Ensure important GBOX environment variables are passed to child process
	// This ensures the child has the same configuration context as parent
	for _, envVar := range []string{"GBOX_BASE_URL", "GBOX_API_KEY", "GBOX_HOME"} {
		if value := os.Getenv(envVar); value != "" {
			// Check if already in environment, if not add it
			found := false
			prefix := envVar + "="
			for i, existing := range env {
				if strings.HasPrefix(existing, prefix) {
					env[i] = envVar + "=" + value
					found = true
					break
				}
			}
			if !found {
				env = append(env, envVar+"="+value)
			}
		}
	}

	return env
}
