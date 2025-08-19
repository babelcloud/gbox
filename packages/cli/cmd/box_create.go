package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewBoxCreateCommand creates the parent command for box creation
func NewBoxCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new box",
		Long: `Create a new box with various options for image, environment, and commands.

Available box types:
  linux    - Create a Linux container box
  android  - Create an Android device box

Use 'gbox box create <type> --help' for more information about each type.`,
		Example: `  gbox box create linux --image python:3.9 -- python3 -c 'print("Hello")'
  gbox box create android --device-type virtual
  gbox box create linux --env PATH=/usr/local/bin:/usr/bin:/bin -w /app -- node server.js`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("please specify a box type: linux or android\nUse 'gbox box create --help' for more information")
		},
	}

	// Add subcommands
	cmd.AddCommand(NewBoxCreateLinuxCommand(), NewBoxCreateAndroidCommand())
	return cmd
}
