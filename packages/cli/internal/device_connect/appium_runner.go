package device_connect

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/babelcloud/gbox/packages/cli/config"
)

const (
	defaultAppiumHost = "127.0.0.1"
	defaultAppiumPort = "4723"
)

var (
	appiumCmd     *exec.Cmd
	appiumCmdMu   sync.Mutex
	appiumLogFile *os.File
)

// IsAppiumListening returns true if something is already listening on the Appium host:port
// (e.g. another Appium instance or another service). Uses TCP connect; does not verify
// that it is actually Appium.
func IsAppiumListening(host, port string) bool {
	addr := net.JoinHostPort(host, port)
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// CheckAppiumStatus checks if an Appium server is responding at host:port (GET /status).
// Returns true only if the endpoint returns 200 and response looks like Appium.
func CheckAppiumStatus(host, port string) bool {
	url := fmt.Sprintf("http://%s:%s/status", host, port)
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// StartAppiumServer starts the Appium server in the background if not already running.
// It sets ANDROID_HOME (and APPIUM_HOME) in the process environment. If another
// process is already listening on the configured port, logs a warning and returns nil
// (no error). Caller should ensure Appium is installed (e.g. via InstallAppium) before
// calling this. Returns an error only when starting our own process fails.
func StartAppiumServer() error {
	host := os.Getenv("APPIUM_HOST")
	if host == "" {
		host = defaultAppiumHost
	}
	port := os.Getenv("APPIUM_PORT")
	if port == "" {
		port = defaultAppiumPort
	}

	if IsAppiumListening(host, port) {
		log.Printf("[Appium] WARNING: Another process is already listening on %s:%s (e.g. an existing Appium server). Skipping auto-start to avoid port conflict.", host, port)
		return nil
	}

	appiumBinary := GetAppiumPath()
	if appiumBinary == "" {
		log.Printf("[Appium] Appium binary not found; skip starting. Install with: gbox setup or gbox device-connect")
		return nil
	}

	deviceProxyHome := config.GetDeviceProxyHome()
	appiumHome := filepath.Join(deviceProxyHome, "appium")

	androidHome, hasAndroid := DetectAndroidHome()
	if !hasAndroid {
		log.Printf("[Appium] ANDROID_HOME not set and could not be detected; Android automation may not work. Consider setting ANDROID_HOME or installing Android Studio / Android SDK.")
	}

	env := append([]string{}, os.Environ()...)
	env = setEnv(env, "APPIUM_HOME", appiumHome)
	if androidHome != "" {
		env = setEnv(env, "ANDROID_HOME", androidHome)
	}

	// Optional: write Appium stdout/stderr to a log file under device-proxy
	logDir := filepath.Join(deviceProxyHome, "logs")
	_ = os.MkdirAll(logDir, 0755)
	logPath := filepath.Join(logDir, "appium.log")
	fd, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("[Appium] Failed to open log file %s: %v", logPath, err)
		fd = nil
	} else {
		appiumLogFile = fd
	}

	// Appium 2.x: just run "appium" (optionally --port N)
	args := []string{}
	if port != defaultAppiumPort {
		args = append(args, "--port", port)
	}
	cmd := exec.Command(appiumBinary, args...)
	cmd.Env = env
	cmd.Dir = appiumHome
	if appiumLogFile != nil {
		cmd.Stdout = appiumLogFile
		cmd.Stderr = appiumLogFile
	}

	appiumCmdMu.Lock()
	defer appiumCmdMu.Unlock()
	if appiumCmd != nil {
		// Already started by us; avoid double start
		return nil
	}

	if err := cmd.Start(); err != nil {
		if appiumLogFile != nil {
			_ = appiumLogFile.Close()
			appiumLogFile = nil
		}
		return fmt.Errorf("failed to start Appium server: %w", err)
	}
	appiumCmd = cmd
	log.Printf("[Appium] Started Appium server (PID %d), log: %s", cmd.Process.Pid, logPath)
	return nil
}

// StopAppiumServer stops the Appium server process that we started. No-op if we did not
// start one. Does not affect other Appium instances that were already running.
func StopAppiumServer() {
	appiumCmdMu.Lock()
	cmd := appiumCmd
	appiumCmd = nil
	if appiumLogFile != nil {
		_ = appiumLogFile.Close()
		appiumLogFile = nil
	}
	appiumCmdMu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return
	}
	if err := cmd.Process.Kill(); err != nil {
		log.Printf("[Appium] Failed to kill Appium process %d: %v", cmd.Process.Pid, err)
		return
	}
	_ = cmd.Wait()
	log.Printf("[Appium] Stopped Appium server (PID %d)", cmd.Process.Pid)
}

func setEnv(env []string, key, value string) []string {
	prefix := key + "="
	for i, e := range env {
		if e == key+"=" || strings.HasPrefix(e, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

// GetAppiumPort returns the Appium port number (from APPIUM_PORT env or default).
func GetAppiumPort() int {
	p := os.Getenv("APPIUM_PORT")
	if p == "" {
		p = defaultAppiumPort
	}
	port, _ := strconv.Atoi(p)
	if port <= 0 {
		port = 4723
	}
	return port
}
