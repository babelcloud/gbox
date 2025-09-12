package cmd

import (
	"github.com/babelcloud/gbox/packages/cli/internal/adb_expose"
	"github.com/spf13/cobra"
)

// ExecuteAdbExposeList lists all exposed ADB ports using the new client-server architecture
func ExecuteAdbExposeList(cmd *cobra.Command, opts *AdbExposeListOptions) error {
	// Use the new client-server architecture
	return adb_expose.ListCommand(opts.OutputFormat)
}
