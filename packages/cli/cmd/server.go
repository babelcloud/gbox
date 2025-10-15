package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/babelcloud/gbox/packages/cli/internal/daemon"
	procgroup "github.com/babelcloud/gbox/packages/cli/internal/proc_group"
	"github.com/babelcloud/gbox/packages/cli/internal/server"
	"github.com/google/uuid"
	"github.com/pkg/errors"
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
		port                   int
		foreground             bool
		internalDaemon         bool
		daemonStartLogFilename string
	)

	cmd := &cobra.Command{
		Use:           "start",
		Short:         "Start the server",
		Long:          `Start the gbox server if it's not already running.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if foreground {
				// Run in foreground mode
				return runServerInForeground(port)
			}
			if internalDaemon {
				return runServerInBackground(port, daemonStartLogFilename)
			}
			// Default: run in daemon mode with IPC communication
			return runServerInDaemon(port)
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

	// Flag --internal-daemon is hidden in help message for internal use.
	flags.BoolVarP(&internalDaemon, "internal-daemon", "", false, "")
	flags.Lookup("internal-daemon").Hidden = true
	flags.StringVarP(&daemonStartLogFilename, "daemon-start-log-filename", "", "", "")
	flags.Lookup("daemon-start-log-filename").Hidden = true

	return cmd
}

// newServerStopCmd creates the 'server stop' subcommand
func newServerStopCmd() *cobra.Command {
	var (
		port  int
		force bool
	)

	cmd := &cobra.Command{
		Use:           "stop",
		Short:         "Stop the server",
		Long:          `Stop the gbox server if it's running.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return stopServer(port, force)
		},
		Example: `  # Stop the server
  gbox server stop

  # Stop server running on specified port
  gbox server stop --port 29888
  gbox server stop -p 29888`,
	}

	flags := cmd.Flags()
	flags.IntVarP(&port, "port", "p", 29888, "Server port")
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
		Use:           "restart",
		Short:         "Restart the server",
		Long:          `Stop and then start the gbox server.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := stopServer(port, true); err != nil {
				return err
			}
			if foreground {
				// Run in foreground mode
				return runServerInForeground(port)
			}
			return runServerInDaemon(port)
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
func runServerInDaemon(port int) error {
	if err := checkServerStatus(port); err != nil {
		if err == ServerMismatchedError {
			return errors.Wrapf(err, "port %d is already been used", port)
		}
	} else {
		fmt.Printf("server has been already started on port %d\n", port)
		return nil
	}

	executable, err := os.Executable()
	if err != nil {
		return errors.Wrap(err, "failed to get exectuable")
	}

	runId := uuid.New()
	daemonStartLogFilename := filepath.Join(os.TempDir(), "gbox-server-"+runId.String())
	defer os.RemoveAll(daemonStartLogFilename)

	cmd := exec.Command(executable, "server", "start", "--port", strconv.Itoa(port), "--internal-daemon", "--daemon-start-log-filename", daemonStartLogFilename)
	procgroup.SetProcGrp(cmd)
	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to start server daemon")
	}

	for range 3 {
		time.Sleep(time.Second)
		if err := checkServerStatus(port); err != nil {
			startLog, err := os.ReadFile(daemonStartLogFilename)
			if err != nil {
				continue
			}
			return errors.Errorf("fail to start server on port %d: %s", port, string(startLog))
		}
	}

	fmt.Printf("server has been started on port %d\n", port)
	return nil
}

func runServerInBackground(port int, startLogFilename string) error {
	userHome, err := os.UserHomeDir()
	if err != nil {
		err := errors.Wrapf(err, "failed to get user home directory")
		os.WriteFile(startLogFilename, []byte(err.Error()), 0600)
		return err
	}

	logFile := filepath.Join(userHome, ".gbox/cli/server.log")
	logFd, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		err := errors.Wrapf(err, "failed to create log file: %s", logFile)
		os.WriteFile(startLogFilename, []byte(err.Error()), 0600)
		return err
	}
	defer logFd.Close()

	log.SetOutput(logFd)

	server := server.NewGBoxServer(port)
	if err := server.Start(); err != nil && err != http.ErrServerClosed {
		err := errors.Wrapf(err, "failed to start server")
		os.WriteFile(startLogFilename, []byte(err.Error()), 0600)
		return err
	}
	return nil
}

func checkServerStatus(port int) error {
	url := fmt.Sprintf("http://localhost:%d/api/health", port)
	resp, err := http.Get(url)
	if err != nil {
		return ServerPortUnavailableError
	}
	defer resp.Body.Close()
	decoder := json.NewDecoder(resp.Body)
	var body struct {
		Status  string `json:"status"`
		Service string `json:"service"`
	}
	if err := decoder.Decode(&body); err != nil {
		return ServerMismatchedError
	}
	if body.Service != "gbox-server" {
		return ServerMismatchedError
	}
	return nil
}

func stopServer(port int, force bool) error {
	if err := checkServerStatus(port); err != nil && !force {
		if err == ServerPortUnavailableError {
			return errors.Errorf("server is not running")
		}
		if err == ServerMismatchedError {
			return errors.Wrapf(err, "port %d is already been used by other process", port)
		}
	}

	url := fmt.Sprintf("http://localhost:%d/api/server/shutdown", port)
	resp, err := http.Post(url, "application/json", nil)
	if err != nil {
		if force {
			return nil
		}
		return ServerPortUnavailableError
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)
	time.Sleep(1 * time.Second)
	return nil
}

func runServerInForeground(port int) error {
	if err := checkServerStatus(port); err != nil {
		if err == ServerMismatchedError {
			return errors.Wrapf(err, "port %d is already been used", port)
		}
	} else {
		fmt.Printf("server has been already started on port %d\n", port)
		return nil
	}

	server := server.NewGBoxServer(port)
	errChan := make(chan error)
	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	for range 3 {
		time.Sleep(time.Second)
		if err := checkServerStatus(port); err != nil {
			select {
			case startErr := <-errChan:
				return errors.Wrapf(startErr, "fail to start server on port %d", port)
			default:
				continue
			}
		}
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
	if err := server.Stop(); err != nil {
		log.Printf("Error stopping server: %v", err)
	}

	return nil
}

var ServerPortUnavailableError = &serverPortUnavailableError{}

type serverPortUnavailableError struct{}

func (e *serverPortUnavailableError) Error() string {
	return "server port unavailable"
}

var ServerMismatchedError = &serverMismatchedError{}

type serverMismatchedError struct{}

func (e *serverMismatchedError) Error() string {
	return "server mismatched"
}
