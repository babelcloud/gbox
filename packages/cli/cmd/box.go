package cmd

import (
	"github.com/spf13/cobra"
)

// NewBoxCommand creates and returns the box command
func NewBoxCommand() *cobra.Command {
	boxCmd := &cobra.Command{
		Use:   "box",
		Short: "Manage box resources",
		Long:  `The box command is used to manage box resources, including creating, terminating, listing, and executing commands.`,
		Example: `  gbox box list                                           # List all boxes
  gbox box create                                                      # Create a new box
  gbox box terminate 550e8400-e29b-41d4-a716-446655440000              # Terminate a specific box
  gbox box exec 550e8400-e29b-41d4-a716-446655440000 -- ls             # Execute a command in a box
  gbox box cp ./local_file 550e8400-e29b-41d4-a716-446655440000:/work  # Copy a local file to a box`,
	}

	// Add all box-related subcommands
	boxCmd.AddCommand(
		NewBoxCreateCommand(),
		NewBoxTerminateCommand(),
		NewBoxListCommand(),
		NewBoxExecCommand(),
		NewBoxInspectCommand(),
		NewBoxCpCommand(),
	)

	return boxCmd
}
