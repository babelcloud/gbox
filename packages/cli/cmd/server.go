package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/babelcloud/gbox/packages/cli/internal/daemon"
	"github.com/babelcloud/gbox/packages/cli/internal/server"
	"github.com/spf13/cobra"
)

// NewServerCmd creates the server command with subcommands
func NewServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Manage the gbox server",
		Long:  `Manage the gbox server daemon for device operations.`,
	}

	cmd.AddCommand(newServerStartCmd())
	cmd.AddCommand(newServerStopCmd())
	cmd.AddCommand(newServerStatusCmd())
	cmd.AddCommand(newServerRestartCmd())

	return cmd
}

// newServerStartCmd creates the 'server start' subcommand
func newServerStartCmd() *cobra.Command {
	var (
		port       int
		foreground bool
		replyFd    int
	)

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the server",
		Long:  `Start the gbox server if it's not already running.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if foreground {
				// Run in foreground mode
				return runServerInForeground(port)
			}
			// Default: run in daemon mode with IPC communication
			return runServerInDaemon(port, replyFd)
		},
		Example: `  # Start server in background
  gbox server start

  # Start server in foreground (see logs)
  gbox server start --foreground
  gbox server start -f

  # Start server on specific port
  gbox server start -p 8080`,
	}

	flags := cmd.Flags()
	flags.IntVarP(&port, "port", "p", 29888, "Server port")
	flags.BoolVarP(&foreground, "foreground", "f", false, "Run server in foreground (show logs)")
	flags.IntVar(&replyFd, "reply-fd", 0, "File descriptor for IPC communication (internal use)")

	return cmd
}

// newServerStopCmd creates the 'server stop' subcommand
func newServerStopCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the server",
		Long:  `Stop the gbox server if it's running.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			dm := daemon.NewManager()

			if force {
				// Force stop all processes
				dm.CleanupOldServers()
				fmt.Println("Force stopped all server processes")
				return nil
			}

			// Normal stop
			if !dm.IsServerRunning() {
				fmt.Println("Server is not running")
				return nil
			}

			fmt.Println("Stopping gbox server...")
			if err := dm.StopServer(); err != nil {
				// Try force cleanup if normal stop fails
				dm.CleanupOldServers()
				fmt.Println("Server stopped (forced)")
				return nil
			}

			fmt.Println("Server stopped successfully")
			return nil
		},
		Example: `  # Stop the server
  gbox server stop

  # Force stop all server processes
  gbox server stop --force
  gbox server stop -f`,
	}

	flags := cmd.Flags()
	flags.BoolVarP(&force, "force", "f", false, "Force stop all server processes")

	return cmd
}

// newServerStatusCmd creates the 'server status' subcommand
func newServerStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check server status",
		Long:  `Check if the gbox server is running and display its status.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			dm := daemon.NewManager()

			if dm.IsServerRunning() {
				fmt.Println("✅ Server is running")
				fmt.Printf("   Web UI: http://localhost:29888\n")
				fmt.Printf("   API endpoint: http://localhost:29888/api/status\n")

				// Try to get more info from API
				client := &http.Client{Timeout: 2 * time.Second}
				if resp, err := client.Get("http://localhost:29888/api/status"); err == nil {
					defer resp.Body.Close()
					var status map[string]interface{}
					if json.NewDecoder(resp.Body).Decode(&status) == nil {
						if services, ok := status["services"].(map[string]interface{}); ok {
							fmt.Println("   Services:")
							for name, active := range services {
								if active.(bool) {
									fmt.Printf("     - %s: active\n", name)
								}
							}
						}
					}
				}
			} else {
				fmt.Println("❌ Server is not running")
				fmt.Println("   Use 'gbox server start' to start the server")
			}

			return nil
		},
	}

	return cmd
}

// newServerRestartCmd creates the 'server restart' subcommand
func newServerRestartCmd() *cobra.Command {
	var (
		port       int
		foreground bool
	)

	cmd := &cobra.Command{
		Use:   "restart",
		Short: "Restart the server",
		Long:  `Stop and then start the gbox server.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			dm := daemon.NewManager()

			// Stop server if it's running
			if dm.IsServerRunning() {
				fmt.Println("Stopping gbox server...")
				if err := dm.StopServer(); err != nil {
					// Try force cleanup if normal stop fails
					dm.CleanupOldServers()
					fmt.Println("Server stopped (forced)")
				} else {
					fmt.Println("Server stopped successfully")
				}

				// Wait a moment for cleanup
				time.Sleep(500 * time.Millisecond)
			}

			// Start server
			if foreground {
				// Run in foreground mode
				fmt.Println("Restarting server in foreground mode...")
				return runServerInForeground(port)
			}

			// Start in background mode
			fmt.Printf("Starting gbox server on port %d...\n", port)
			if err := dm.StartServer(); err != nil {
				return fmt.Errorf("failed to start server: %v", err)
			}

			fmt.Println("Server restarted successfully")
			fmt.Printf("Web UI available at: http://localhost:%d\n", port)
			return nil
		},
		Example: `  # Restart the server
  gbox server restart

  # Restart in foreground mode
  gbox server restart --foreground
  gbox server restart -f

  # Restart on specific port
  gbox server restart -p 8080`,
	}

	flags := cmd.Flags()
	flags.IntVarP(&port, "port", "p", 29888, "Server port")
	flags.BoolVarP(&foreground, "foreground", "f", false, "Run server in foreground after restart (show logs)")

	return cmd
}

// Helper functions

// runServerInDaemon runs the server in daemon mode with IPC communication
func runServerInDaemon(port int, replyFd int) error {
	// Start the server
	server := server.NewGBoxServer(port)

	// Start server in a goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- server.Start()
	}()

	// Wait a bit for server to start
	time.Sleep(2 * time.Second)

	// Check if server started successfully by trying to connect
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 1*time.Second)
	if err != nil {
		// Server failed to start
		if replyFd > 0 {
			replyFile := os.NewFile(uintptr(replyFd), "reply")
			if replyFile != nil {
				replyFile.WriteString(fmt.Sprintf("ERROR: server failed to start: %v", err))
				replyFile.Close()
			}
		}
		return fmt.Errorf("server failed to start: %v", err)
	}
	conn.Close()

	// Server started successfully
	if replyFd > 0 {
		replyFile := os.NewFile(uintptr(replyFd), "reply")
		if replyFile != nil {
			replyFile.WriteString("OK")
			replyFile.Close()
		}
	}

	// Keep the server running
	select {}
}

func startServerInBackground(port int) error {
	dm := daemon.NewManager()

	// Check if already running
	if dm.IsServerRunning() {
		fmt.Println("Server is already running")
		fmt.Printf("Web UI available at: http://localhost:%d\n", port)
		return nil
	}

	fmt.Printf("Starting gbox server on port %d...\n", port)
	if err := dm.StartServer(); err != nil {
		return fmt.Errorf("failed to start server: %v", err)
	}

	fmt.Println("Server started successfully")
	fmt.Printf("Web UI available at: http://localhost:%d\n", port)
	return nil
}

func runServerInForeground(port int) error {
	// Check if another instance is running
	dm := daemon.NewManager()
	if dm.IsServerRunning() {
		fmt.Println("Warning: Another server instance is already running")
		fmt.Println("Stop it first with 'gbox server stop' or use a different port")
		return fmt.Errorf("server already running")
	}

	// Starting server in foreground mode

	srv := server.NewGBoxServer(port)
	if err := srv.Start(); err != nil {
		return fmt.Errorf("failed to start server: %v", err)
	}

	// ANSI color codes
	const (
		ColorReset = "\033[0m"
		ColorGreen = "\033[32m"
		ColorBlue  = "\033[34m"
		ColorCyan  = "\033[36m"
	)

	fmt.Printf("%s🚀 GBOX Local Server%s %s➜ %shttp://localhost:%d%s\n", ColorGreen, ColorReset, ColorCyan, ColorBlue, port, ColorReset)
	fmt.Printf("%sPress Ctrl+C to stop...%s\n", ColorCyan, ColorReset)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down server...")
	if err := srv.Stop(); err != nil {
		log.Printf("Error stopping server: %v", err)
	}

	return nil
}
