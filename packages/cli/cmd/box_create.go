package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewBoxCreateCommand creates the parent command for box creation
func NewBoxCreateCommand() *cobra.Command {
	opts := &BoxCreateFromDeviceOptions{}

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new box",
		Long: `Create a new box with various options for image, environment, and commands.

Available box types:
  linux        - Create a Linux container box
  android      - Create an Android device box

Use '-d/--device-id' to create a box from an existing device by device ID.
Use 'gbox box create <type> --help' for more information about each type.`,
		Example: `  gbox box create linux --image python:3.9 -- python3 -c 'print("Hello")'
  gbox box create android --device-type virtual
  gbox box create -d <device-id>
  gbox box create linux --env PATH=/usr/local/bin:/usr/bin:/bin -w /app -- node server.js`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// If device-id is provided, create box from device directly
			if opts.DeviceID != "" {
				return ExecuteBoxCreateFromDevice(cmd, opts)
			}
			return fmt.Errorf("please specify a box type: linux or android, or use -d/--device-id to create from device\nUse 'gbox box create --help' for more information")
		},
	}

	// Root-level flags to create from device directly
	flags := cmd.Flags()
	flags.StringVarP(&opts.DeviceID, "device-id", "d", "", "Device ID to create box from")
	flags.BoolVarP(&opts.Force, "force", "f", true, "Force create box even if device is occupied")
	flags.StringVarP(&opts.OutputFormat, "output", "o", "text", "Output format (json or text)")

	cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"json", "text"}, cobra.ShellCompDirectiveNoFileComp
	})

	// Add subcommands
	cmd.AddCommand(
		NewBoxCreateLinuxCommand(),
		NewBoxCreateAndroidCommand(),
	)
	return cmd
}
