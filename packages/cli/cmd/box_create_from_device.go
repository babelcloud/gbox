package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/babelcloud/gbox/packages/cli/internal/cloud"
	"github.com/spf13/cobra"
)

type BoxCreateFromDeviceOptions struct {
	DeviceID     string
	Force        bool
	OutputFormat string
}

func NewBoxCreateFromDeviceCommand() *cobra.Command {
	opts := &BoxCreateFromDeviceOptions{}

	cmd := &cobra.Command{
		Use:   "from-device [device-id] [flags]",
		Short: "Create a box from a device",
		Long: `Create a box from an existing device by device ID.
This command allows you to create a box (Linux or Android) from a registered device.

If force is true, any existing box using the device will be terminated.`,
		Example: `  # Create a box from a device
  gbox box create from-device <device-id>

  # Create a box and force terminate any existing box
  gbox box create from-device <device-id> --force

  # Output in JSON format
  gbox box create from-device <device-id> --output json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.DeviceID = args[0]
			}
			return ExecuteBoxCreateFromDevice(cmd, opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.DeviceID, "device-id", "d", "", "Device ID to create box from")
	flags.BoolVarP(&opts.Force, "force", "f", true, "Force create box even if device is occupied")
	flags.StringVarP(&opts.OutputFormat, "output", "o", "text", "Output format (json or text)")

	cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"json", "text"}, cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

func ExecuteBoxCreateFromDevice(cmd *cobra.Command, opts *BoxCreateFromDeviceOptions) error {
	if opts.DeviceID == "" {
		return fmt.Errorf("device ID is required. Use --device-id or provide as argument")
	}

	deviceAPI := cloud.NewDeviceAPI()
	box, err := deviceAPI.DeviceToBox(opts.DeviceID, opts.Force)
	if err != nil {
		return fmt.Errorf("failed to create box from device: %v", err)
	}

	// Output result
	if opts.OutputFormat == "json" {
		boxJSON, _ := json.MarshalIndent(box, "", "  ")
		fmt.Println(string(boxJSON))
	} else {
		fmt.Printf("Box created successfully from device %s\n", opts.DeviceID)
		fmt.Printf("Box ID: %s\n", box.Id)
		if box.Type != "" {
			fmt.Printf("Box Type: %s\n", box.Type)
		}
		if box.Status != "" {
			fmt.Printf("Box Status: %s\n", box.Status)
		}
	}

	return nil
}
