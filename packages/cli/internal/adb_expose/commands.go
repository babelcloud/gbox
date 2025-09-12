package adb_expose

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	sdk "github.com/babelcloud/gbox-sdk-go"
	gboxsdk "github.com/babelcloud/gbox/packages/cli/internal/client"
)

// StartCommand starts port forwarding using the main GBOX server API
func StartCommand(boxID string, localPorts, remotePorts []int, foreground bool) error {
	// First check if the box exists
	if err := checkBoxExists(boxID); err != nil {
		return err
	}

	// Ensure main GBOX server is running
	if err := ensureServerRunning(); err != nil {
		return fmt.Errorf("failed to start server: %v", err)
	}

	// Create request payload
	reqBody := map[string]interface{}{
		"box_id":       boxID,
		"local_ports":  localPorts,
		"remote_ports": remotePorts,
	}

	// Convert to JSON
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %v", err)
	}

	// Send HTTP request to main server
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Post("http://127.0.0.1:29888/api/adb-expose/start",
		"application/json",
		bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send request to server: %v", err)
	}
	defer resp.Body.Close()

	// Parse response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %v", err)
	}

	// Check if request was successful
	if resp.StatusCode == http.StatusConflict {
		// Handle 409 Conflict - already running
		fmt.Printf("ADB port is already exposed for box %s\n", boxID)
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		errorMsg, _ := result["error"].(string)
		// Check for specific error types and provide user-friendly messages
		if strings.Contains(errorMsg, "box is not running") {
			return fmt.Errorf("box %s is not running or does not exist", boxID)
		}
		return fmt.Errorf("server error: %s", errorMsg)
	}

	success, _ := result["success"].(bool)
	if !success {
		errorMsg, _ := result["error"].(string)
		// Check for specific error types and provide user-friendly messages
		if strings.Contains(errorMsg, "box is not running") {
			return fmt.Errorf("box %s is not running or does not exist", boxID)
		}
		return fmt.Errorf("failed to start ADB port expose: %s", errorMsg)
	}

	// Print success message
	fmt.Printf("✅ ADB port exposed for box %s on port %v\n", boxID, localPorts[0])

	if !foreground {
		fmt.Printf("\n💡 Use 'gbox adb-expose list' to view all exposed ports\n")
		fmt.Printf("   Use 'gbox adb-expose stop %s' to stop\n", boxID)
	}

	return nil
}

// StopCommand stops port forwarding using the main GBOX server API
func StopCommand(boxID string) error {
	// First check if the box exists
	if err := checkBoxExists(boxID); err != nil {
		return err
	}

	// Ensure main GBOX server is running
	if err := ensureServerRunning(); err != nil {
		return fmt.Errorf("failed to start server: %v", err)
	}

	// Create request payload
	reqBody := map[string]interface{}{
		"box_id": boxID,
	}

	// Convert to JSON
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %v", err)
	}

	// Send HTTP request to main server
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Post("http://127.0.0.1:29888/api/adb-expose/stop",
		"application/json",
		bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send request to server: %v", err)
	}
	defer resp.Body.Close()

	// Parse response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %v", err)
	}

	// Check if request was successful
	if resp.StatusCode == http.StatusNotFound {
		// Box exists but ADB port expose is not active
		return fmt.Errorf("ADB port expose is not active for box %s", boxID)
	}

	if resp.StatusCode != http.StatusOK {
		errorMsg, _ := result["error"].(string)
		return fmt.Errorf("server error: %s", errorMsg)
	}

	success, _ := result["success"].(bool)
	if !success {
		errorMsg, _ := result["error"].(string)
		return fmt.Errorf("failed to stop ADB port expose: %s", errorMsg)
	}

	// Print success message
	fmt.Printf("✅ ADB port expose stopped for box %s\n", boxID)

	return nil
}

// ListCommand lists all running port forwards using the new client-server architecture
func ListCommand(outputFormat string) error {
	// Ensure server is running before making requests
	if err := ensureServerRunning(); err != nil {
		return fmt.Errorf("failed to start server: %v", err)
	}

	// Check if main server is running by trying to connect to it
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	// Try to get ADB Expose list from the main server
	resp, err := client.Get("http://127.0.0.1:29888/api/adb-expose/list")
	if err != nil {
		fmt.Println("ADB Expose server is not running")
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Println("ADB Expose server is not responding properly")
		return nil
	}

	// Parse response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %v", err)
	}

	// Display results
	forwards, ok := result["forwards"].([]interface{})
	if !ok || len(forwards) == 0 {
		fmt.Println("No ADB ports are currently exposed")
		return nil
	}

	// Convert to table data format
	var tableData []map[string]interface{}
	for _, forward := range forwards {
		f, ok := forward.(map[string]interface{})
		if !ok {
			continue
		}

		boxID, _ := f["box_id"].(string)
		localPorts, _ := f["local_ports"].([]interface{})
		startedAt, _ := f["started_at"].(string)

		localPortStr := formatPortsFromInterface(localPorts)

		// Don't truncate box ID - show full ID

		tableData = append(tableData, map[string]interface{}{
			"box_id":     boxID,
			"port":       localPortStr,
			"started_at": startedAt,
		})
	}

	// Output based on format
	if outputFormat == "json" {
		// Output JSON format
		jsonData, err := json.MarshalIndent(map[string]interface{}{
			"forwards": tableData,
		}, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %v", err)
		}
		fmt.Println(string(jsonData))
	} else {
		// Render table
		renderTable(tableData)
	}

	return nil
}

// formatPortsFromInterface formats a slice of ports from interface{} as a string
func formatPortsFromInterface(ports []interface{}) string {
	if len(ports) == 0 {
		return "none"
	}

	portStrs := make([]string, len(ports))
	for i, port := range ports {
		if portFloat, ok := port.(float64); ok {
			portStrs[i] = strconv.Itoa(int(portFloat))
		} else if portStr, ok := port.(string); ok {
			portStrs[i] = portStr
		} else {
			portStrs[i] = "unknown"
		}
	}
	return strings.Join(portStrs, ",")
}

// renderTable renders the ADB Expose list table
func renderTable(data []map[string]interface{}) {
	if len(data) == 0 {
		fmt.Println("No ADB ports are currently exposed")
		return
	}

	// Print table header
	fmt.Printf("%-40s %-12s %-25s\n", "Box ID", "Port", "Started At")
	fmt.Println(strings.Repeat("-", 77))

	// Print data rows
	for _, row := range data {
		boxID, _ := row["box_id"].(string)
		port, _ := row["port"].(string)
		startedAt, _ := row["started_at"].(string)

		fmt.Printf("%-40s %-12s %-25s\n", boxID, port, startedAt)
	}
}

// ensureServerRunning ensures the GBOX server is running, starting it if necessary
func ensureServerRunning() error {
	// Check if server is already running
	if isServerRunning() {
		// Check if server version matches current build
		if !isServerVersionCompatible() {
			fmt.Println("🔄 Server version mismatch, restarting server...")
			// Kill existing server and start new one
			if err := killExistingServer(); err != nil {
				fmt.Printf("⚠️  Warning: failed to kill existing server: %v\n", err)
			}
			return startServerInBackground()
		}
		return nil
	}

	// Start server in background
	return startServerInBackground()
}

// isServerRunning checks if the server is already running
func isServerRunning() bool {
	conn, err := http.Get("http://127.0.0.1:29888/health")
	if err != nil {
		return false
	}
	defer conn.Body.Close()
	return conn.StatusCode == http.StatusOK
}

// isServerVersionCompatible checks if the running server version matches current build
func isServerVersionCompatible() bool {
	// Get server build ID
	serverBuildID, err := getServerBuildID()
	if err != nil {
		// If we can't get server build ID, assume incompatible
		return false
	}

	// Get current build ID
	currentBuildID := getCurrentBuildID()

	// For development, we'll use a more lenient approach:
	// If both build IDs contain "unknown" (development mode), do exact comparison
	// This ensures that recompiled binaries trigger server restart
	if strings.Contains(serverBuildID, "unknown") && strings.Contains(currentBuildID, "unknown") {
		return currentBuildID == serverBuildID
	}

	// For production builds, do exact comparison
	return currentBuildID == serverBuildID
}

// getCurrentBuildID returns the current build ID
func getCurrentBuildID() string {
	// For development, use a simple approach that changes when binary is recompiled
	// In production, this would be set by build scripts
	execPath, err := os.Executable()
	if err != nil {
		return "unknown"
	}

	info, err := os.Stat(execPath)
	if err != nil {
		return "unknown"
	}

	// Use modification time + file size to detect binary changes
	// This will change when the binary is recompiled
	buildTime := info.ModTime().Format("2006-01-02T15:04:05") // No timezone, more stable
	gitCommit := "unknown"
	fileSize := info.Size()

	return fmt.Sprintf("%s-%s-%d", buildTime, gitCommit, fileSize)
}

// getServerBuildID gets the build ID from the running server
func getServerBuildID() (string, error) {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://127.0.0.1:29888/api/server/info")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	var info map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "", err
	}

	buildID, ok := info["build_id"].(string)
	if !ok {
		return "", fmt.Errorf("build_id not found in server response")
	}

	return buildID, nil
}

// killExistingServer kills the existing server process
func killExistingServer() error {
	// Read PID from PID file
	pidFile := filepath.Join(os.Getenv("HOME"), ".gbox", "cli", "gbox-server.pid")
	pidData, err := os.ReadFile(pidFile)
	if err != nil {
		// PID file doesn't exist, try to find process by port
		return killServerByPort()
	}

	pid := strings.TrimSpace(string(pidData))
	if pid == "" {
		// Empty PID file, try to find process by port
		return killServerByPort()
	}

	// Convert PID to int
	pidInt, err := strconv.Atoi(pid)
	if err != nil {
		// Invalid PID, try to find process by port
		return killServerByPort()
	}

	// Try to kill the process by PID first
	if err := killProcessByPID(pidInt); err != nil {
		// If PID-based kill fails, try port-based kill
		return killServerByPort()
	}

	// Remove PID file
	os.Remove(pidFile)

	return nil
}

// killProcessByPID kills a process by its PID
func killProcessByPID(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process %d: %v", pid, err)
	}

	// Send SIGTERM first
	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM to process %d: %v", pid, err)
	}

	// Wait for graceful shutdown
	for i := 0; i < 10; i++ {
		time.Sleep(500 * time.Millisecond)
		// Check if process is still running
		if err := process.Signal(syscall.Signal(0)); err != nil {
			// Process is dead
			return nil
		}
	}

	// Process still running, force kill
	if err := process.Signal(syscall.SIGKILL); err != nil {
		return fmt.Errorf("failed to send SIGKILL to process %d: %v", pid, err)
	}

	// Wait a bit more for SIGKILL to take effect
	time.Sleep(1 * time.Second)

	return nil
}

// killServerByPort kills the server process by finding it via port
func killServerByPort() error {
	// Use lsof to find the process using port 29888
	cmd := exec.Command("lsof", "-ti:29888")
	output, err := cmd.Output()
	if err != nil {
		// No process found on port, that's fine
		return nil
	}

	pids := strings.Fields(string(output))
	for _, pidStr := range pids {
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}

		// Kill the process
		if err := killProcessByPID(pid); err != nil {
			fmt.Printf("Warning: failed to kill process %d: %v\n", pid, err)
		}
	}

	return nil
}

// startServerInBackground starts the server in background mode with IPC communication
func startServerInBackground() error {
	// Create a pipe for IPC communication
	reader, writer, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("failed to create pipe: %v", err)
	}
	defer reader.Close()
	defer writer.Close()

	// Get the current executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}

	// Create command to start server in background with reply fd
	cmd := exec.Command(execPath, "server", "start", "--reply-fd", "3")

	// Set up process attributes for daemon mode
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // Create new process group
	}

	// Pass the write end of the pipe as file descriptor 3
	cmd.ExtraFiles = []*os.File{writer}

	// Redirect output to log file
	homeDir, _ := os.UserHomeDir()
	gboxDir := filepath.Join(homeDir, ".gbox", "cli")
	logFile := filepath.Join(gboxDir, "server.log")

	logFileHandle, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %v", err)
	}
	defer logFileHandle.Close()

	cmd.Stdout = logFileHandle
	cmd.Stderr = logFileHandle

	// Start the process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start server: %v", err)
	}

	// Close the write end in parent process
	writer.Close()

	// Read the reply from the child process
	replyChan := make(chan error, 1)
	go func() {
		buffer := make([]byte, 1024)
		n, err := reader.Read(buffer)
		if err != nil {
			replyChan <- fmt.Errorf("failed to read reply: %v", err)
			return
		}

		reply := string(buffer[:n])
		if reply == "OK" {
			replyChan <- nil
		} else {
			replyChan <- fmt.Errorf("server startup failed: %s", reply)
		}
	}()

	// Wait for reply with timeout
	select {
	case err := <-replyChan:
		if err != nil {
			// Server failed to start, clean up the process
			cmd.Process.Kill()
			return err
		}
	case <-time.After(10 * time.Second):
		// Timeout waiting for reply
		cmd.Process.Kill()
		return fmt.Errorf("timeout waiting for server startup reply")
	}

	// Write PID to file
	pidFile := filepath.Join(gboxDir, "gbox-server.pid")
	pidFileHandle, err := os.OpenFile(pidFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create PID file: %v", err)
	}
	defer pidFileHandle.Close()

	if _, err := pidFileHandle.WriteString(strconv.Itoa(cmd.Process.Pid)); err != nil {
		return fmt.Errorf("failed to write PID file: %v", err)
	}

	return nil
}

// createGBOXClient creates a GBOX client for API calls
func createGBOXClient() (*sdk.Client, error) {
	return gboxsdk.NewClientFromProfile()
}

// checkBoxExists checks if a box exists using the GBOX API
func checkBoxExists(boxID string) error {
	// Create a client to check if the box exists
	client, err := createGBOXClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %v", err)
	}

	// Check if box exists
	box, err := gboxsdk.GetBox(client, boxID)
	if err != nil {
		// If we can't get the box, it might not exist
		return fmt.Errorf("box %s does not exist or is not accessible", boxID)
	}

	// Check if box is running
	if box.Status != "running" {
		return fmt.Errorf("box %s is not running (status: %s)", boxID, box.Status)
	}

	return nil
}