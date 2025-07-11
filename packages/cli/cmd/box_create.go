package cmd

import (
	"fmt"
	"strings"

	model "github.com/babelcloud/gbox/packages/api-server/pkg/box"
	"github.com/spf13/cobra"
)

func parseKeyValuePairs(pairs []string, pairType string) (map[string]string, error) {
	if len(pairs) == 0 {
		return nil, nil
	}

	result := make(map[string]string)
	for _, pair := range pairs {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		} else {
			return nil, fmt.Errorf("invalid %s format: %s (must be KEY=VALUE)", pairType, pair)
		}
	}
	return result, nil
}

// parseVolumes parses volume mount strings in the format "source:target[:ro][:propagation]"
func parseVolumes(volumes []string) ([]model.VolumeMount, error) {
	if len(volumes) == 0 {
		return nil, nil
	}

	result := make([]model.VolumeMount, 0, len(volumes))
	for _, volume := range volumes {
		parts := strings.Split(volume, ":")
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid volume format: %s (must be source:target[:ro][:propagation])", volume)
		}

		mount := model.VolumeMount{
			Source: parts[0],
			Target: parts[1],
		}

		// Parse optional flags
		for i := 2; i < len(parts); i++ {
			switch parts[i] {
			case "ro":
				mount.ReadOnly = true
			case "private", "rprivate", "shared", "rshared", "slave", "rslave":
				mount.Propagation = parts[i]
			default:
				return nil, fmt.Errorf("invalid volume option: %s", parts[i])
			}
		}

		result = append(result, mount)
	}

	return result, nil
}

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
