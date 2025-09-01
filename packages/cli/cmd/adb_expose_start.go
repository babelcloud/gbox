package cmd

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/babelcloud/gbox/packages/cli/config"
	"github.com/babelcloud/gbox/packages/cli/internal/adb_expose"
	"github.com/babelcloud/gbox/packages/cli/internal/profile"
	"github.com/spf13/cobra"
)

// ExecuteAdbExpose runs the adb-expose logic
func ExecuteAdbExpose(cmd *cobra.Command, opts *AdbExposeOptions, args []string) error {
	if opts.BoxID == "" && len(args) > 0 {
		opts.BoxID = args[0]
	}
	if opts.BoxID == "" || !boxValid(opts.BoxID) {
		return fmt.Errorf("the box you specified is not valid, check --help for how to add it or using 'gbox box list' to check")
	}

	// Determine local port to use
	localPort := opts.LocalPort
	if localPort == 0 {
		// Auto-find available port starting from 5555
		var err error
		localPort, err = findAvailablePort(5555)
		if err != nil {
			return fmt.Errorf("failed to find available port: %v", err)
		}
		log.Printf("Auto-selected local port: %d", localPort)
	} else {
		// Check if specified port is available
		if localPort < 1 || localPort > 65535 {
			return fmt.Errorf("invalid local port %d: port must be between 1 and 65535", localPort)
		}

		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", localPort))
		if err != nil {
			portInfo := getPortUsageInfo(localPort)
			if portInfo != "" {
				return fmt.Errorf("the port %d is already in use by: %s", localPort, portInfo)
			}
			return fmt.Errorf("the port %d is not available: %v", localPort, err)
		}
		listener.Close()
	}

	// ADB always uses port 5555 on the remote side
	remotePort := 5555

	// Get API Key with priority: GBOX_API_KEY env var > profile
	apiKey, err := profile.GetEffectiveAPIKey()
	if err != nil {
		return fmt.Errorf("failed to get API key: %v", err)
	}
	decodedKey, err := pm.DecodeAPIKey(current.APIKey)
	if err != nil {
		return fmt.Errorf("failed to decode API key: %w", err)
	}

	logPath := fmt.Sprintf("%s/gbox-adb-expose-%s-%d.log", config.GetGboxHome(), opts.BoxID, localPort)
	if shouldReturn, err := adb_expose.DaemonizeIfNeeded(opts.Foreground, logPath, opts.BoxID, true); shouldReturn {
		return err
	}

	// Write pid file
	if err := adb_expose.WritePidFile(opts.BoxID, []int{localPort}, []int{remotePort}); err != nil {
		return fmt.Errorf("failed to write pid file: %v", err)
	}

	// Clean up pid and log files on exit
	defer func() {
		adb_expose.RemovePidFile(opts.BoxID, localPort)
		adb_expose.RemoveLogFile(opts.BoxID, localPort)
	}()

	// Signal handling for cleanup
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		adb_expose.RemovePidFile(opts.BoxID, localPort)
		adb_expose.RemoveLogFile(opts.BoxID, localPort)
		os.Exit(0)
	}()

	// Get effective base URL for connection
	effectiveBaseURL := profile.GetEffectiveBaseURL()

	// Connect to websocket
	portForwardConfig := adb_expose.Config{
		APIKey:      apiKey,
		BoxID:       opts.BoxID,
		GboxURL:     effectiveBaseURL,
		TargetPorts: []int{remotePort},
	}

	retryInterval := 3 * time.Second
	log.Printf("Starting adb-expose: local port %d <-> remote ADB port %d (auto-reconnect enabled)", localPort, remotePort)

	for {
		// Listen on local port
		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", localPort))
		if err != nil {
			return fmt.Errorf("failed to listen on port %d: %v", localPort, err)
		}

		// Connect to websocket with retry (max 3 attempts)
		var client *adb_expose.MultiplexClient
		var connectErr error
		for attempt := 1; attempt <= 3; attempt++ {
			client, connectErr = adb_expose.ConnectWebSocket(portForwardConfig)
			if connectErr == nil {
				break
			}
			if attempt < 3 {
				log.Printf("adb-expose connection attempt %d failed: %v, retrying...", attempt, connectErr)
				time.Sleep(5 * time.Second)
			}
		}
		if connectErr != nil {
			listener.Close()
			return fmt.Errorf("failed to connect to adb-expose after 3 attempts: %v", connectErr)
		}

		// Concurrency & Reconnection Control Logic
		reconnectCh := make(chan struct{})
		stopAcceptCh := make(chan struct{})

		// Start the main loop for the WebSocket client.
		go func() {
			if err := client.Run(); err != nil {
				log.Printf("client run error: %v", err)
			}
			close(reconnectCh)
		}()

		acceptDone := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(1)

		// Start the local port listener goroutine.
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stopAcceptCh:
					return
				default:
					localConn, err := listener.Accept()
					if err != nil {
						log.Printf("accept error: %v", err)
						time.Sleep(time.Second)
						continue
					}
					go adb_expose.HandleLocalConnWithClient(localConn, client, remotePort)
				}
			}
		}()

		// Wait for all accept goroutines to exit
		go func() {
			wg.Wait()
			close(acceptDone)
		}()

		log.Printf("adb port is exposed, you can connect to the device by `adb connect 127.0.0.1:%d`", localPort)

		// Main flow waits for:
		select {
		case <-reconnectCh:
			log.Println("websocket disconnected, will attempt to reconnect...")
			close(stopAcceptCh)
			listener.Close() // force accept goroutine to exit
			<-acceptDone
			client.Close()
			log.Printf("Reconnecting in %v...", retryInterval)
			time.Sleep(retryInterval)
			continue // retry loop
		case <-acceptDone:
			log.Println("accept loop ended")
			listener.Close()
			client.Close()
			return nil
		}
	}
}
