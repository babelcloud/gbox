package cmd

import (
	"fmt"
	"os"

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
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			pm := NewProfileManager()
			if err := pm.Load(); err != nil {
				// If we cannot load the profile, do not block execution – just inform the user.
				fmt.Fprintf(os.Stderr, "Warning: failed to load profile file: %v\n", err)
				return nil
			}

			current := pm.GetCurrent()
			if current == nil {
				fmt.Fprintln(os.Stderr, "No profile selected. Use 'gbox profile use' to select one.")
			} else {
				if current.OrganizationName == "" {
					fmt.Fprintf(os.Stderr, "Using profile: %s\n", current.Name)
				} else {
					fmt.Fprintf(os.Stderr, "Using profile: %s (organization: %s)\n", current.Name, current.OrganizationName)
				}
			}
			return nil
		},
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
