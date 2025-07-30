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
	"sync"
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
	PortMaps   []string // Support multiple port mappings
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
		Use:   "port-forward BOX_ID [options] [LOCAL_PORT:]REMOTE_PORT [...[LOCAL_PORT_N:]REMOTE_PORT_N]",
		Short: "Forward one or more ports from a remote box to your local machine (multi-port, kubectl style)",
		Long: `Forward one or more ports from a remote android box to your local machine.

Examples:
  # Run in foreground
  gbox port-forward box123 8888:5555 --foreground

  # Use with --port/-p flag
  gbox port-forward box123 -p 8888:5555 -p 9999:6666

  # Forward a range of ports
  gbox port-forward box123 8000:8000 8001:8001 8002:8003
`,
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return ExecutePortForward(cmd, opts, args)
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return completeBoxIDs(cmd, args, toComplete)
		},
	}

	cmd.Flags().StringVarP(&opts.BoxID, "box-id", "b", "", "Box ID (optional, can also be first arg)")
	cmd.Flags().StringSliceVarP(&opts.PortMaps, "port", "p", []string{"5555:5555"}, "Port mapping(s) in the form [local_port:remote_port], [:remote_port], or [local_port]. Can be specified multiple times or space-separated.")
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

	// Collect all port mapping arguments
	var portMaps []string
	if len(opts.PortMaps) > 0 {
		portMaps = opts.PortMaps
	}
	if len(args) > 1 {
		// args[1:] may also be port mappings
		portMaps = args[1:]
	}
	if len(portMaps) == 0 {
		portMaps = []string{"5555:5555"}
	}

	// Parse all port mappings
	pairs, err := parsePortMaps(portMaps)
	if err != nil {
		return fmt.Errorf("Invalid port map(s): %v", err)
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

	logPath := fmt.Sprintf("%s/gbox-portforward-%s-%d.log", port_forward.GboxHomeDir(), opts.BoxID, pairs[0].Local) // Use the first local port for log path
	if shouldReturn, err := port_forward.DaemonizeIfNeeded(opts.Foreground, logPath); shouldReturn {
		return err
	}
	// Write pid file for all ports
	err = port_forward.WritePidFile(opts.BoxID,
		func() []int { arr := make([]int, len(pairs)); for i, p := range pairs { arr[i] = p.Local }; return arr }(),
		func() []int { arr := make([]int, len(pairs)); for i, p := range pairs { arr[i] = p.Remote }; return arr }(),
	)
	if err != nil {
		return fmt.Errorf("Failed to write pid file: %v", err)
	}
	// Clean up pid and log files on exit
	defer func() {
		for _, pair := range pairs {
			port_forward.RemovePidFile(opts.BoxID, pair.Local)
			port_forward.RemoveLogFile(fmt.Sprintf("%s/gbox-portforward-%s-%d.log", port_forward.GboxHomeDir(), opts.BoxID, pair.Local))
		}
	}()
	// Signal handling for cleanup
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		for _, pair := range pairs {
			port_forward.RemovePidFile(opts.BoxID, pair.Local)
			port_forward.RemoveLogFile(fmt.Sprintf("%s/gbox-portforward-%s-%d.log", port_forward.GboxHomeDir(), opts.BoxID, pair.Local))
		}
		os.Exit(0)
	}()

	// Connect to websocket only once
	portForwardConfig := port_forward.Config{
		APIKey:      current.APIKey,
		BoxID:       opts.BoxID,
		GboxURL:     config.GetCloudAPIURL(),
		TargetPorts: make([]int, 0, len(pairs)),
	}
	for _, pair := range pairs {
		portForwardConfig.TargetPorts = append(portForwardConfig.TargetPorts, pair.Remote)
	}

	retryInterval := 3 * time.Second
	log.Printf("Starting port-forward: local <-> remote (auto-reconnect enabled)")

	for {
		listeners := make([]net.Listener, 0, len(pairs))
		// Listen on all local ports
		for _, pair := range pairs {
			l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", pair.Local))
			if err != nil {
				// Close all opened listeners
				for _, ll := range listeners {
					ll.Close()
				}
				return fmt.Errorf("Failed to listen on port %d: %v", pair.Local, err)
			}
			log.Printf("Listening on 127.0.0.1:%d", pair.Local)
			listeners = append(listeners, l)
		}

		// Connect to websocket
		client := port_forward.ConnectWebSocket(portForwardConfig)
		if client == nil {
			for _, l := range listeners {
				l.Close()
			}
			log.Printf("Failed to connect to WebSocket, retrying in %v...", retryInterval)
			time.Sleep(retryInterval)
			continue
		}

		// Concurrency & Reconnection Control Logic
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
		var wg sync.WaitGroup
		wg.Add(len(listeners))
		// Start the local port listener goroutines.
		for idx, l := range listeners {
			pair := pairs[idx]
			go func(l net.Listener, remotePort int) {
				defer wg.Done()
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
						log.Printf("new local tcp connection from %v (local port %d)", localConn.RemoteAddr(), pair.Local)
						go port_forward.HandleLocalConnWithClient(localConn, client, remotePort)
					}
				}
			}(l, pair.Remote)
		}
		// Wait for all accept goroutines to exit
		go func() {
			wg.Wait()
			close(acceptDone)
		}()

		// Main flow waits for:
		select {
		case <-reconnectCh:
			log.Println("websocket disconnected, will attempt to reconnect...")
			close(stopAcceptCh)
			for _, l := range listeners {
				l.Close() // force accept goroutine to exit
			}
			<-acceptDone
			client.Close()
			log.Printf("Reconnecting in %v...", retryInterval)
			time.Sleep(retryInterval)
			continue // retry loop
		case <-acceptDone:
			log.Println("accept loop ended")
			for _, l := range listeners {
				l.Close()
			}
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
	// 删除所有相关 pid 文件
	for _, lp := range info.LocalPorts {
		port_forward.RemovePidFile(boxid, lp)
	}
	fmt.Printf("Killed port-forward process %d (boxid=%s, ports=%v)\n", info.Pid, boxid, info.LocalPorts)
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
		for i := 0; i < len(info.LocalPorts) && i < len(info.RemotePorts); i++ {
			fmt.Printf("| %-8d | %-36s | %-10d | %-10d | %-8s | %-20s |\n", info.Pid, info.BoxID, info.LocalPorts[i], info.RemotePorts[i], status, info.StartedAt.Format("2006-01-02 15:04:05"))
		}
	}
	return nil
}

// parsePortMap parses the port mapping string and returns localPort, remotePort
// Acceptable formats: "6666:5555" (local:remote), ":5555", "6666" (default remotePort = 5555)
func parsePortMap(portMap string) (int, int, error) {
	var localPortStr, remotePortStr string
	parts := strings.Split(portMap, ":")
	if len(parts) == 2 {
		if parts[0] == "" {
			// ":5555" => localPort = 5555, remotePort = 5555
			localPortStr = parts[1]
			remotePortStr = parts[1]
		} else {
			// "6666:5555" => localPort = 6666, remotePort = 5555
			localPortStr = parts[0]
			remotePortStr = parts[1]
		}
	} else if len(parts) == 1 {
		// "5555" => localPort = 5555, remotePort = 5555
		localPortStr = parts[0]
		remotePortStr = parts[0]
	} else {
		return 0, 0, fmt.Errorf("Invalid port map format")
	}

	localPort, err := strconv.Atoi(localPortStr)
	if err != nil || localPort < 1 || localPort > 65535 {
		return 0, 0, fmt.Errorf("Invalid local port: %s", localPortStr)
	}
	remotePort, err := strconv.Atoi(remotePortStr)
	if err != nil || remotePort < 1 || remotePort > 65535 {
		return 0, 0, fmt.Errorf("Invalid remote port: %s", remotePortStr)
	}

	return localPort, remotePort, nil
}

// parsePortMaps parses multiple port mapping strings
// Acceptable: ["5555:6666", ":7777", "8888"] (local:remote)
type PortPair struct {
	Local  int
	Remote int
}

func parsePortMaps(portMaps []string) ([]PortPair, error) {
	var pairs []PortPair
	for _, pm := range portMaps {
		local, remote, err := parsePortMap(pm)
		if err != nil {
			return nil, err
		}
		pairs = append(pairs, PortPair{Local: local, Remote: remote})
	}
	return pairs, nil
}