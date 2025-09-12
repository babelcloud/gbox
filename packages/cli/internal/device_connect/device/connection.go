package device

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Note: assets will be embedded at build time using a different approach

// ScrcpyConnection handles the actual scrcpy server connection
type ScrcpyConnection struct {
	deviceSerial string
	scid         uint32
	adbPath      string
	serverPath   string
	conn         net.Conn
	Listener     net.Listener // Made public to match scrcpy-proxy
	serverCmd    *exec.Cmd
}

// NewScrcpyConnection creates a new scrcpy connection handler
func NewScrcpyConnection(deviceSerial string, scid uint32) *ScrcpyConnection {
	// Find adb path
	adbPath, err := exec.LookPath("adb")
	if err != nil {
		adbPath = "adb" // Fallback to PATH
	}

	// Find scrcpy-server.jar
	serverPath := findScrcpyServerJar()
	if serverPath == "" {
		log.Printf("Warning: scrcpy-server.jar not found, will try default location")
		serverPath = "/data/local/tmp/scrcpy-server.jar"
	}

	return &ScrcpyConnection{
		deviceSerial: deviceSerial,
		scid:         scid,
		adbPath:      adbPath,
		serverPath:   serverPath,
	}
}

// Connect establishes connection to scrcpy server on device
func (sc *ScrcpyConnection) Connect() (net.Conn, error) {
	log.Printf("Starting scrcpy connection for device %s on port %d", sc.deviceSerial, sc.scid)

	// 1. Push server file to device
	if err := sc.pushServerFile(); err != nil {
		return nil, fmt.Errorf("failed to push server file: %w", err)
	}

	// 2. Setup reverse port forwarding
	if err := sc.setupReversePortForward(); err != nil {
		return nil, fmt.Errorf("failed to setup reverse port forward: %w", err)
	}

	// 3. Start listener for scrcpy server connection
	listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", sc.scid))
	if err != nil {
		return nil, fmt.Errorf("failed to start listener on port %d: %w", sc.scid, err)
	}
	sc.Listener = listener

	// 4. Start scrcpy server on device
	if err := sc.startScrcpyServer(); err != nil {
		listener.Close()
		return nil, fmt.Errorf("failed to start scrcpy server: %w", err)
	}

	// 5. Accept connection from scrcpy server
	log.Printf("Waiting for scrcpy server to connect...")

	// Set deadline for accept
	if err := listener.(*net.TCPListener).SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		listener.Close()
		return nil, fmt.Errorf("failed to set deadline: %w", err)
	}

	conn, err := listener.Accept()
	if err != nil {
		listener.Close()
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			sc.killScrcpyServer()
			return nil, fmt.Errorf("timeout waiting for scrcpy server")
		}
		return nil, fmt.Errorf("failed to accept connection: %w", err)
	}

	// Clear deadline for future accepts
	listener.(*net.TCPListener).SetDeadline(time.Time{})

	log.Printf("Scrcpy server connected successfully")
	sc.conn = conn
	return conn, nil
}

// pushServerFile pushes scrcpy-server.jar to device
func (sc *ScrcpyConnection) pushServerFile() error {
	// Check if local server file exists
	if sc.serverPath != "" && sc.serverPath != "/data/local/tmp/scrcpy-server.jar" {
		// Check if file exists locally
		if _, err := os.Stat(sc.serverPath); err == nil {
			log.Printf("Pushing scrcpy-server.jar to device...")
			cmd := exec.Command(sc.adbPath, "-s", sc.deviceSerial, "push", sc.serverPath, "/data/local/tmp/scrcpy-server.jar")
			if output, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("failed to push server: %s", output)
			}
			log.Printf("Server file pushed successfully")
		}
	}

	// Verify server exists on device
	cmd := exec.Command(sc.adbPath, "-s", sc.deviceSerial, "shell", "ls", "/data/local/tmp/scrcpy-server.jar")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("scrcpy-server.jar not found on device")
	}

	return nil
}

// setupReversePortForward sets up adb reverse port forwarding
func (sc *ScrcpyConnection) setupReversePortForward() error {
	// Clean up any existing reverse forward
	cleanCmd := exec.Command(sc.adbPath, "-s", sc.deviceSerial, "reverse", "--remove", fmt.Sprintf("localabstract:scrcpy_%08x", sc.scid))
	cleanCmd.Run() // Ignore error if doesn't exist

	// Setup new reverse forward
	log.Printf("Setting up reverse port forward: scrcpy_%08x -> tcp:%d", sc.scid, sc.scid)
	cmd := exec.Command(sc.adbPath, "-s", sc.deviceSerial, "reverse",
		fmt.Sprintf("localabstract:scrcpy_%08x", sc.scid),
		fmt.Sprintf("tcp:%d", sc.scid))

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to setup reverse forward: %s", output)
	}

	return nil
}

// startScrcpyServer starts the scrcpy server process on device
func (sc *ScrcpyConnection) startScrcpyServer() error {
	// Kill any existing scrcpy server
	sc.killScrcpyServer()
	time.Sleep(200 * time.Millisecond)

	// Build scrcpy server command
	scidHex := fmt.Sprintf("%08x", sc.scid)

	cmd := exec.Command(sc.adbPath, "-s", sc.deviceSerial, "shell",
		"CLASSPATH=/data/local/tmp/scrcpy-server.jar",
		"app_process", "/", "com.genymobile.scrcpy.Server",
		"3.3.1", // Server version - must match the downloaded jar
		fmt.Sprintf("scid=%s", scidHex),
		"video=true",
		"audio=true",
		"control=true",
		"cleanup=true",
		"log_level=verbose",                      // Enable verbose logging to debug scroll issues
		"video_codec_options=i-frame-interval=1", // Force keyframe every 1 second to prevent video freezing
		"video_bit_rate=12000000",                // 12 Mbps for better quality
		"max_fps=60",                             // 60 FPS for smoother video
		"video_encoder=OMX.qcom.video.encoder.avc", // Use Qualcomm hardware encoder (more compatible)
		"audio_codec=opus",                         // Use Opus for better audio quality
		"audio_bit_rate=128000",                    // 128 kbps audio
	)

	log.Printf("Starting scrcpy server with command: %s", cmd.String())

	// Start the command
	sc.serverCmd = cmd

	// Capture output for debugging
	cmd.Stdout = &logWriter{prefix: "[scrcpy-out]"}
	cmd.Stderr = &logWriter{prefix: "[scrcpy-err]"}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start scrcpy server: %w", err)
	}

	// Give server time to start
	time.Sleep(500 * time.Millisecond)

	return nil
}

// killScrcpyServer kills any running scrcpy server on device
func (sc *ScrcpyConnection) killScrcpyServer() {
	// Kill by process name
	cmd := exec.Command(sc.adbPath, "-s", sc.deviceSerial, "shell", "pkill", "-f", "scrcpy.Server")
	cmd.Run()

	// Also kill our tracked process if exists
	if sc.serverCmd != nil && sc.serverCmd.Process != nil {
		sc.serverCmd.Process.Kill()
		sc.serverCmd = nil
	}
}

// Close closes the scrcpy connection
func (sc *ScrcpyConnection) Close() error {
	log.Printf("Closing scrcpy connection for device %s", sc.deviceSerial)

	// Close connection
	if sc.conn != nil {
		sc.conn.Close()
	}

	// Close listener
	if sc.Listener != nil {
		sc.Listener.Close()
	}

	// Kill server process
	sc.killScrcpyServer()

	// Clean up reverse forward
	cmd := exec.Command(sc.adbPath, "-s", sc.deviceSerial, "reverse", "--remove", fmt.Sprintf("localabstract:scrcpy_%08x", sc.scid))
	cmd.Run()

	return nil
}

// findScrcpyServerJar finds the scrcpy-server.jar file
func findScrcpyServerJar() string {
	// Note: embedded assets will be handled differently

	// Fallback to external files
	locations := []string{
		// In project assets directory (primary location)
		"./assets/scrcpy-server.jar",
		"../cli/assets/scrcpy-server.jar",
		"../../packages/cli/assets/scrcpy-server.jar",
		// In home directory
		filepath.Join(os.Getenv("HOME"), ".gbox", "scrcpy-server.jar"),
		// In scrcpy installation
		"/usr/local/share/scrcpy/scrcpy-server",
		"/opt/homebrew/share/scrcpy/scrcpy-server",
		"/usr/share/scrcpy/scrcpy-server",
	}

	for _, path := range locations {
		if _, err := os.Stat(path); err == nil {
			absPath, _ := filepath.Abs(path)
			return absPath
		}
	}

	return ""
}

// Note: embedded server extraction removed - using external files only

// logWriter implements io.Writer for logging
type logWriter struct {
	prefix string
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	log.Printf("%s %s", w.prefix, string(p))
	return len(p), nil
}
