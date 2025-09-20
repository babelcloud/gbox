package cmd

import (
	"fmt"

	"github.com/babelcloud/gbox/packages/cli/internal/adb_expose"
	"github.com/spf13/cobra"
)

// ExecuteAdbExposeStop stops adb-expose processes for a specific box using the new client-server architecture
func ExecuteAdbExposeStop(cmd *cobra.Command, opts *AdbExposeStopOptions, args []string) error {
	boxID := args[0]
	if boxID == "" {
		return fmt.Errorf("box ID is required. Usage: gbox adb-expose stop <box_id>")
	}

	// Use the new client-server architecture
	return adb_expose.StopCommand(boxID)
}