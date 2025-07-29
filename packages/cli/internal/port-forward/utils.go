package port_forward

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"os/user"

	"github.com/gorilla/websocket"
)

// PidInfo holds info for a running port-forward process
type PidInfo struct {
	Pid        int       `json:"pid"`
	BoxID      string    `json:"boxid"`
	LocalPort  int       `json:"localport"`
	RemotePort int       `json:"remoteport"`
	StartedAt  time.Time `json:"started_at"`
}

// GboxHomeDir returns the ~/.gbox directory path
func GboxHomeDir() string {
	usr, err := user.Current()
	if err != nil {
		return os.Getenv("HOME") + "/.gbox"
	}
	return usr.HomeDir + "/.gbox"
}

func ensureGboxDir() error {
	dir := GboxHomeDir()
	return os.MkdirAll(dir, 0700)
}

const pidFileNamePrefix = "gbox-portforward-"
const pidFileNameSuffix = ".pid"

func pidFilePath(boxid string, localport int) string {
	return GboxHomeDir() + "/" + pidFileNamePrefix + boxid + "-" + strconv.Itoa(localport) + pidFileNameSuffix
}

const pidFilePattern = "gbox-portforward-*.pid"

func WritePidFile(boxid string, localport, remoteport int) error {
	if err := ensureGboxDir(); err != nil {
		return err
	}
	path := pidFilePath(boxid, localport)
	// check if pid file exists
	if _, err := os.Stat(path); err == nil {
		// if exists, read pid and check process
		f, err := os.Open(path)
		if err == nil {
			var info PidInfo
			decodeErr := json.NewDecoder(f).Decode(&info)
			f.Close()
			if decodeErr == nil && IsProcessAlive(info.Pid) {
				return fmt.Errorf("port-forward already running for boxid=%s, localport=%d (pid=%d)", boxid, localport, info.Pid)
			}
		}
		// if pid file exists but process is not running, remove it
	}
	info := PidInfo{
		Pid:        os.Getpid(),
		BoxID:      boxid,
		LocalPort:  localport,
		RemotePort: remoteport,
		StartedAt:  time.Now(),
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	return enc.Encode(&info)
}

func RemovePidFile(boxid string, localport int) error {
	return os.Remove(pidFilePath(boxid, localport))
}

func RemoveLogFile(logPath string) error {
	return os.Remove(logPath)
}

func ListPidFiles() ([]PidInfo, error) {
	dir := GboxHomeDir()
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

func FindPidFile(boxid string, localport int) (string, error) {
	path := pidFilePath(boxid, localport)
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

func ConnectWebSocket(config Config) *MultiplexClient {
    wsURL, err := getPortForwardURL(config)
    if err != nil {
        log.Printf("get port forward URL error: %v", err)
        return nil
    }

    log.Printf("connecting to WebSocket: %s", wsURL)

    ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
    if err != nil {
        log.Printf("ws dial error: %v", err)
        return nil
    }
    log.Println("ws dial success")

    client := NewMultiplexClient(ws)
    return client
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

// DaemonizeIfNeeded forks to background if foreground==false and not already daemonized.
// logPath: if not empty, background process logs to this file.
// Returns (shouldReturn, err): if shouldReturn==true, caller should return immediately (parent process or error).
func DaemonizeIfNeeded(foreground bool, logPath string) (bool, error) {
	if foreground || os.Getenv("GBOX_PORTFORWARD_DAEMON") != "" {
		return false, nil
	}
	// open log file
	logFile := os.Stdout
	if logPath != "" {
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return true, fmt.Errorf("Failed to open log file: %v", err)
		}
		logFile = f
		defer f.Close()
	}
	attr := &os.ProcAttr{
		Dir:   "",
		Env:   append(os.Environ(), "GBOX_PORTFORWARD_DAEMON=1"),
		Files: []*os.File{os.Stdin, logFile, logFile},
		Sys:   &syscall.SysProcAttr{Setsid: true},
	}
	args := os.Args
	// Remove -f/--foreground from args if present
	newArgs := []string{}
	for i := 0; i < len(args); i++ {
		if args[i] == "-f" || args[i] == "--foreground" {
			continue
		}
		newArgs = append(newArgs, args[i])
	}
	proc, err := os.StartProcess(args[0], newArgs, attr)
	if err != nil {
		return true, fmt.Errorf("Failed to daemonize: %v", err)
	}
	fmt.Printf("[gbox] Port-forward started in background (pid=%d). Logs: %s\nUse 'gbox port-forward list' to view, 'gbox port-forward kill <pid>' to stop.\n", proc.Pid, logPath)
	return true, nil
}