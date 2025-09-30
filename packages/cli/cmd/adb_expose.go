package cmd

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/babelcloud/gbox/packages/cli/internal/adb_expose"
	client "github.com/babelcloud/gbox/packages/cli/internal/client"
	"github.com/spf13/cobra"
)

// AdbExposeOptions holds options for the adb-expose command
type AdbExposeOptions struct {
	BoxID      string
	LocalPort  int // Optional local port to bind to
	Foreground bool
}

type AdbExposeStopOptions struct {
	BoxID string
}

type AdbExposeListOptions struct {
	OutputFormat string
}

// NewAdbExposeCommand creates the adb-expose command
func NewAdbExposeCommand() *cobra.Command {
	opts := &AdbExposeOptions{}

	cmd := &cobra.Command{
		Use:   "adb-expose",
		Short: "Manage ADB port exposure for remote android boxes",
		Long: `Manage ADB port exposure for remote android boxes.

Examples:
  # Start ADB port exposure (interactive mode)
  gbox adb-expose

  # Start ADB port exposure for a specific box
  gbox adb-expose start <box_id>

  # Start with specific options
  gbox adb-expose start <box_id> --port 6666 --foreground

  # Stop ADB port exposure
  gbox adb-expose stop <box_id>

  # List running exposures
  gbox adb-expose list

  # List in JSON format
  gbox adb-expose list --json
`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ExecuteAdbExposeInteractive(cmd, opts)
		},
	}

	// Add subcommands: start, stop, list
	cmd.AddCommand(NewAdbExposeStartCommand())
	cmd.AddCommand(NewAdbExposeStopCommand())
	cmd.AddCommand(NewAdbExposeListCommand())

	return cmd
}

func NewAdbExposeStartCommand() *cobra.Command {
	opts := &AdbExposeOptions{}
	cmd := &cobra.Command{
		Use:   "start <box_id>",
		Short: "Start ADB port exposure for a specific box",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.BoxID = args[0]
			return ExecuteAdbExpose(cmd, opts, args)
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return completeBoxIDs(cmd, args, toComplete)
		},
		SilenceUsage:  true, // Don't show usage on error
		SilenceErrors: true, // Don't show errors (we handle them ourselves)
	}

	cmd.Flags().IntVarP(&opts.LocalPort, "port", "p", 0, "Local port to bind to (default: auto-find available port starting from 5555)")
	cmd.Flags().BoolVarP(&opts.Foreground, "foreground", "f", false, "Run in foreground (default is background/daemon mode)")

	return cmd
}

func NewAdbExposeStopCommand() *cobra.Command {
	opts := &AdbExposeStopOptions{}
	cmd := &cobra.Command{
		Use:   "stop <box_id>",
		Short: "Stop ADB port exposure for a specific box",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return ExecuteAdbExposeStop(cmd, opts, args)
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return completeBoxIDs(cmd, args, toComplete)
		},
		SilenceUsage:  true, // Don't show usage on error
		SilenceErrors: true, // Don't show errors (we handle them ourselves)
	}
	return cmd
}

func NewAdbExposeListCommand() *cobra.Command {
	opts := &AdbExposeListOptions{}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all exposed ADB ports",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ExecuteAdbExposeList(cmd, opts)
		},
		SilenceUsage:  true, // Don't show usage on error
		SilenceErrors: true, // Don't show errors (we handle them ourselves)
	}

	cmd.Flags().StringVarP(&opts.OutputFormat, "output", "o", "table", "Output format (table|json)")

	return cmd
}

// findAvailablePort finds an available port starting from startPort
func findAvailablePort(startPort int) (int, error) {
	for port := startPort; port <= 65535; port++ {
		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			listener.Close()
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available ports found starting from %d", startPort)
}

// ExecuteAdbExposeInteractive runs the interactive mode for adb-expose
func ExecuteAdbExposeInteractive(cmd *cobra.Command, opts *AdbExposeOptions) error {
	// Skip interactive mode if running in daemon mode
	if os.Getenv("GBOX_ADB_EXPOSE_DAEMON") != "" {
		return fmt.Errorf("interactive mode not available in daemon process")
	}

	// Use the new client-server architecture to list current exposures
	fmt.Println("Current ADB port exposures:")
	fmt.Println("============================")
	if err := adb_expose.ListCommand(""); err != nil {
		// If server is not running, just show a message
		fmt.Println("ADB Expose server is not running")
	}
	fmt.Println()

	// Get available boxes
	sdkClient, err := client.NewClientFromProfile()
	if err != nil {
		return fmt.Errorf("failed to create client: %v", err)
	}

	boxes, err := client.ListBoxesData(sdkClient, []string{})
	if err != nil {
		return fmt.Errorf("failed to list boxes: %v", err)
	}

	if len(boxes) == 0 {
		fmt.Println("No boxes available. Please create a box first using 'gbox box create'")
		return nil
	}

	// Filter running Android boxes
	var availableBoxes []client.BoxInfo
	for _, box := range boxes {
		if box.Status == "running" && strings.HasPrefix(box.Type, "android") {
			availableBoxes = append(availableBoxes, box)
		}
	}

	if len(availableBoxes) == 0 {
		// Check if there are any running Android boxes at all
		var runningAndroidBoxes []client.BoxInfo
		for _, box := range boxes {
			if box.Status == "running" && strings.HasPrefix(box.Type, "android") {
				runningAndroidBoxes = append(runningAndroidBoxes, box)
			}
		}

		if len(runningAndroidBoxes) == 0 {
			fmt.Println("No running Android boxes found. Please start an Android box first using 'gbox box create'")
		} else {
			fmt.Println("No available Android boxes to expose. All running Android boxes are already exposed.")
		}
		return nil
	}

	fmt.Printf("Select an Android box to expose ADB port (%d available):\n", len(availableBoxes))
	fmt.Println()

	for i, box := range availableBoxes {
		// Display the full type if available, otherwise show "unknown"
		displayType := box.Type
		if displayType == "" {
			displayType = "unknown"
		}

		// Display deviceType in parentheses if available
		displayInfo := ""
		if box.DeviceType != "" {
			displayInfo = fmt.Sprintf("(%s)", box.DeviceType)
		}

		fmt.Printf("%d. %s %s\n", i+1, box.ID, displayInfo)
	}
	fmt.Println()

	// Get user selection
	var selection int
	if len(availableBoxes) == 1 {
		fmt.Print("Enter selection [1]: ")
		var input string
		fmt.Scanln(&input)
		selection = 1 // Default to the only available option
	} else {
		fmt.Print("Enter selection (1-", len(availableBoxes), "): ")
		fmt.Scanf("%d", &selection)
	}

	if selection < 1 || selection > len(availableBoxes) {
		return fmt.Errorf("invalid selection")
	}

	selectedBox := availableBoxes[selection-1]
	opts.BoxID = selectedBox.ID

	// Find an available port starting from 5555
	defaultPort, err := findAvailablePort(5555)
	if err != nil {
		defaultPort = 5555 // Fallback to 5555 if no port found
	}

	// Ask for port preference with available default value
	fmt.Printf("Enter port number to bind to [%d]: ", defaultPort)
	var portInput string
	fmt.Scanln(&portInput)

	if portInput != "" {
		if port, err := strconv.Atoi(portInput); err == nil {
			opts.LocalPort = port
		}
	} else {
		// Set default port if user just pressed Enter
		opts.LocalPort = defaultPort
	}

	// Ask for foreground preference
	fmt.Print("Run in foreground? (y/N): ")
	var foregroundInput string
	fmt.Scanln(&foregroundInput)
	opts.Foreground = strings.ToLower(foregroundInput) == "y" || strings.ToLower(foregroundInput) == "yes"

	// Execute the actual port exposure
	return ExecuteAdbExpose(cmd, opts, []string{})
}

// ExecuteAdbExpose runs the adb-expose logic
// This function is now implemented in adb_expose_start.go

// ExecuteAdbExposeStop stops adb-expose processes for a specific box
// This function is now implemented in adb_expose_stop.go

// ExecuteAdbExposeList lists all exposed ADB ports
// This function is now implemented in adb_expose_list.go

func boxValid(boxID string) bool {
	// Skip box validation in daemon processes to avoid TLS issues
	if os.Getenv("GBOX_ADB_EXPOSE_DAEMON") != "" {
		return true
	}

	sdkClient, err := client.NewClientFromProfile()
	if err != nil {
		return false
	}
	box, err := client.GetBox(sdkClient, boxID)
	if err != nil {
		return false
	}
	return box.Status == "running"
}

// getPortUsageInfo attempts to find what process is using a specific port
// Returns a descriptive string about the port usage, or empty string if unable to determine
func getPortUsageInfo(port int) string {
	// Try to use lsof to find what's using the port (works on macOS and Linux)
	cmd := exec.Command("lsof", "-i", fmt.Sprintf(":%d", port))
	output, err := cmd.Output()
	if err != nil {
		// If lsof fails, try netstat as fallback
		cmd = exec.Command("netstat", "-an", "-p", "tcp")
		output, err = cmd.Output()
		if err != nil {
			return "unknown process"
		}

		// Parse netstat output to find the port
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, fmt.Sprintf(":%d", port)) && strings.Contains(line, "LISTEN") {
				// Extract process info if available
				fields := strings.Fields(line)
				if len(fields) >= 4 {
					return fmt.Sprintf("process (netstat shows: %s)", fields[len(fields)-1])
				}
				return "listening process"
			}
		}
		return "unknown process"
	}

	// Parse lsof output
	lines := strings.Split(string(output), "\n")
	if len(lines) > 1 { // Skip header line
		fields := strings.Fields(lines[1])
		if len(fields) >= 2 {
			pid := fields[1]
			processName := fields[0]
			return fmt.Sprintf("%s (PID: %s)", processName, pid)
		}
	}

	return "unknown process"
}

// printAdbExposeTable and printAdbExposeJSON are now implemented in adb_expose_list.go
