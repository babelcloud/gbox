package cmd

import (
	"fmt"

	"github.com/babelcloud/gbox/packages/cli/internal/adb_expose"
	"github.com/spf13/cobra"
)

// ExecuteAdbExpose runs the adb-expose logic using the new client-server architecture
func ExecuteAdbExpose(cmd *cobra.Command, opts *AdbExposeOptions, args []string) error {
	if opts.BoxID == "" && len(args) > 0 {
		opts.BoxID = args[0]
	}
	if opts.BoxID == "" {
		return fmt.Errorf("box ID is required. Usage: gbox adb-expose start <box_id>")
	}

	// Determine local port to use
	localPort := opts.LocalPort
	if localPort == 0 {
		localPort = 5555 // Default port
	}

	// ADB always uses port 5555 on the remote side
	remotePort := 5555

	// Use the new client-server architecture
	return adb_expose.StartCommand(opts.BoxID, []int{localPort}, []int{remotePort}, opts.Foreground)
}