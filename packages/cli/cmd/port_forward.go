package cmd

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"encoding/json"
	"os"
	"os/signal"
	"syscall"

	"github.com/babelcloud/gbox/packages/cli/config"
	port_forward "github.com/babelcloud/gbox/packages/cli/internal/port-forward"
	"github.com/spf13/cobra"
)

// PortForwardOptions holds options for the port-forward command
// You can expand this struct as needed
//
type PortForwardOptions struct {
	BoxID      string
	PortMap    string // e.g. 5555:5555 or :5555 or 5555
	Foreground bool
}

type PortForwardKillOptions struct {
	PID int
}

type PortForwardListOptions struct {
	// Add fields if needed for filtering, etc.
}

// NewPortForwardCommand creates the port-forward command
func NewPortForwardCommand() *cobra.Command {
	opts := &PortForwardOptions{}

	cmd := &cobra.Command{
		Use:   "port-forward <box_id> [port]",
		Short: "Port forward remote android adb to local",
		Long:  `Port forward remote android box's adb port (default 5555) to local for adb connect`,
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return ExecutePortForward(cmd, opts, args)
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return completeBoxIDs(cmd, args, toComplete)
		},
	}

	cmd.Flags().StringVarP(&opts.BoxID, "box-id", "b", "", "Box ID (optional, can also be first arg)")
	cmd.Flags().StringVarP(&opts.PortMap, "port", "p", "5555:5555", "Port mapping in the form [remote_port:local_port], [:local_port], or [remote_port]")
	cmd.Flags().BoolVarP(&opts.Foreground, "foreground", "f", false, "Run in foreground (default is background/daemon mode)")

	// Add subcommands: kill, list
	cmd.AddCommand(NewPortForwardKillCommand())
	cmd.AddCommand(NewPortForwardListCommand())

	return cmd
}

func NewPortForwardKillCommand() *cobra.Command {
	opts := &PortForwardKillOptions{}
	cmd := &cobra.Command{
		Use:   "kill <boxid> <localport> | --pid <pid>",
		Short: "Kill a running port-forward process",
		Args:  cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			return ExecutePortForwardKill(cmd, opts, args)
		},
	}
	cmd.Flags().Int("pid", 0, "PID of the port-forward process to kill")
	return cmd
}

func NewPortForwardListCommand() *cobra.Command {
	opts := &PortForwardListOptions{}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all running port-forward processes",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ExecutePortForwardList(cmd, opts)
		},
	}
	return cmd
}

// ExecutePortForward runs the port-forward logic
func ExecutePortForward(cmd *cobra.Command, opts *PortForwardOptions, args []string) error {
	if opts.BoxID == "" && len(args) > 0 {
		opts.BoxID = args[0]
	}
	if opts.BoxID == "" {
		return fmt.Errorf("Box ID is required, check --help for how to add it.")
	}

	// support port map as second argument
	portMap := opts.PortMap
	if len(args) > 1 && args[1] != "" {
		portMap = args[1]
	}

	// try to get API_KEY, if not set, return error
	pm := NewProfileManager()
	if err := pm.Load(); err != nil {
		return fmt.Errorf("Failed to load profile: %v", err)
	}
	current := pm.GetCurrent()
	if current == nil || current.APIKey == "" {
		return fmt.Errorf("No current profile or API key found. Please run 'gbox profile add' and 'gbox profile use'.")
	} else if current.OrganizationName == "local" {
		return fmt.Errorf("Local profile is not supported for port-forward.")
	}

	// extract remote port and local port from port map
	remotePort, localPort, err := parsePortMap(portMap)
	if err != nil {
		return fmt.Errorf("Invalid port map: %v", err)
	}

	logPath := fmt.Sprintf("%s/gbox-portforward-%s-%d.log", port_forward.GboxHomeDir(), opts.BoxID, localPort)
	if shouldReturn, err := port_forward.DaemonizeIfNeeded(opts.Foreground, logPath); shouldReturn {
		return err
	}
	// write pid file
	err = port_forward.WritePidFile(opts.BoxID, localPort, remotePort)
	if err != nil {
		return fmt.Errorf("Failed to write pid file: %v", err)
	}
	// clean up pid file and log file when exit
	defer func() {
		port_forward.RemovePidFile(opts.BoxID, localPort)
		port_forward.RemoveLogFile(logPath)
	}()
	// signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		port_forward.RemovePidFile(opts.BoxID, localPort)
		port_forward.RemoveLogFile(logPath)
		os.Exit(0)
	}()

	// prepare port forward config
	portForwardConfig := port_forward.Config{
		APIKey:      current.APIKey,
		BoxID:       opts.BoxID,
		GboxURL:     config.GetCloudAPIURL(),
		LocalAddr:   fmt.Sprintf("127.0.0.1:%d", localPort),
		TargetPorts: []int{remotePort},
	}

	retryInterval := 3 * time.Second
	log.Printf("Starting port-forward: local 127.0.0.1:%d <-> remote %d (auto-reconnect enabled)", localPort, remotePort)

	for {
		// 1. Listen on local port (fail if already in use)
		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", localPort))
		if err != nil {
			return fmt.Errorf("Failed to listen on port %d: %v", localPort, err)
		}
		log.Printf("Listening on 127.0.0.1:%d", localPort)

		// 2. Connect to websocket
	client := port_forward.ConnectWebSocket(portForwardConfig)
	if client == nil {
			l.Close()
			log.Printf("Failed to connect to WebSocket, retrying in %v...", retryInterval)
			time.Sleep(retryInterval)
			continue
	}

		// --- Concurrency & Reconnection Control Logic ---
	reconnectCh := make(chan struct{})
	stopAcceptCh := make(chan struct{})

		// Start the main loop for the WebSocket client.
	go func() {
		if err := client.Run(); err != nil {
			fmt.Printf("client run error: %v", err)
		}
		close(reconnectCh)
	}()

	acceptDone := make(chan struct{})
		// Start the local port listener goroutine.
	go func() {
		defer close(acceptDone)
		for {
			select {
			case <-stopAcceptCh:
				return
			default:
				localConn, err := l.Accept()
				if err != nil {
					log.Printf("accept error: %v", err)
					time.Sleep(time.Second)
					continue
				}
				log.Printf("new local tcp connection from %v", localConn.RemoteAddr())
				go port_forward.HandleLocalConnWithClient(localConn, client, remotePort)
			}
		}
	}()

		// Main flow waits for:
	select {
	case <-reconnectCh:
			log.Println("websocket disconnected, will attempt to reconnect...")
		close(stopAcceptCh)
			l.Close() // force accept goroutine to exit
		<-acceptDone
		client.Close()
			log.Printf("Reconnecting in %v...", retryInterval)
			time.Sleep(retryInterval)
			continue // retry loop
	case <-acceptDone:
		log.Println("accept loop ended")
			l.Close()
			client.Close()
		return nil
		}
	}
}

func ExecutePortForwardKill(cmd *cobra.Command, opts *PortForwardKillOptions, args []string) error {
	// support gbox port-forward kill <boxid> <localport> or --pid <pid>
	pidFlag, _ := cmd.Flags().GetInt("pid")
	if pidFlag > 0 {
		// kill by pid
		proc, err := os.FindProcess(pidFlag)
		if err != nil {
			return fmt.Errorf("find process: %v", err)
		}
		err = proc.Signal(syscall.SIGTERM)
		if err != nil {
			return fmt.Errorf("kill process: %v", err)
		}
		fmt.Printf("Killed port-forward process %d\n", pidFlag)
		return nil
	}
	if len(args) < 2 {
		return fmt.Errorf("Usage: gbox port-forward kill <boxid> <localport> | --pid <pid>")
	}
	boxid := args[0]
	localport, err := strconv.Atoi(args[1])
	if err != nil {
		return fmt.Errorf("invalid localport: %v", err)
	}
	path, err := port_forward.FindPidFile(boxid, localport)
	if err != nil {
		return fmt.Errorf("pid file not found: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open pid file: %v", err)
	}
	var info port_forward.PidInfo
	err = json.NewDecoder(f).Decode(&info)
	f.Close()
	if err != nil {
		return fmt.Errorf("decode pid file: %v", err)
	}
	proc, err := os.FindProcess(info.Pid)
	if err != nil {
		return fmt.Errorf("find process: %v", err)
	}
	err = proc.Signal(syscall.SIGTERM)
	if err != nil {
		return fmt.Errorf("kill process: %v", err)
	}
	port_forward.RemovePidFile(boxid, localport)
	fmt.Printf("Killed port-forward process %d (boxid=%s, port=%d)\n", info.Pid, boxid, localport)
	return nil
}

func ExecutePortForwardList(cmd *cobra.Command, opts *PortForwardListOptions) error {
	infos, err := port_forward.ListPidFiles()
	if err != nil {
		return err
	}
	fmt.Printf("| %-8s | %-36s | %-10s | %-10s | %-8s | %-20s |\n", "PID", "BoxID", "LocalPort", "RemotePort", "Status", "StartedAt")
	fmt.Println("|----------|--------------------------------------|------------|------------|----------|----------------------|")
	for _, info := range infos {
		status := "Dead"
		if port_forward.IsProcessAlive(info.Pid) {
			status = "Alive"
		}
		fmt.Printf("| %-8d | %-36s | %-10d | %-10d | %-8s | %-20s |\n", info.Pid, info.BoxID, info.LocalPort, info.RemotePort, status, info.StartedAt.Format("2006-01-02 15:04:05"))
	}
	return nil
}

// parsePortMap parses the port mapping string and returns remotePort, localPort
// Acceptable formats: "5555:6666", ":6666", "5555" (default localPort = 5555)
func parsePortMap(portMap string) (int, int, error) {
	var remotePortStr, localPortStr string
	parts := strings.Split(portMap, ":")
	if len(parts) == 2 {
		if parts[0] == "" {
			// ":6666" => remotePort = 5555, localPort = 6666
			remotePortStr = "5555"
			localPortStr = parts[1]
		} else {
			// "5555:6666" => remotePort = 5555, localPort = 6666
			remotePortStr = parts[0]
			localPortStr = parts[1]
		}
	} else if len(parts) == 1 {
		// "5555" => remotePort = 5555, localPort = 5555
		remotePortStr = parts[0]
		localPortStr = parts[0]
	} else {
		return 0, 0, fmt.Errorf("Invalid port map format")
	}

	remotePort, err := strconv.Atoi(remotePortStr)
	if err != nil || remotePort < 1 || remotePort > 65535 {
		return 0, 0, fmt.Errorf("Invalid remote port: %s", remotePortStr)
	}
	localPort, err := strconv.Atoi(localPortStr)
	if err != nil || localPort < 1 || localPort > 65535 {
		return 0, 0, fmt.Errorf("Invalid local port: %s", localPortStr)
	}

	return remotePort, localPort, nil
}