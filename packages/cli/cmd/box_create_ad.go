package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	client "github.com/babelcloud/gbox/packages/cli/internal/client"
	"github.com/spf13/cobra"
)

type AndroidBoxCreateOptions struct {
	OutputFormat string
	DeviceType   string
	Env          []string
	Labels       []string
	ExpiresIn    string
}

func NewBoxCreateAndroidCommand() *cobra.Command {
	opts := &AndroidBoxCreateOptions{}

	cmd := &cobra.Command{
		Use:   "android [flags]",
		Short: "Create a new Android box",
		Long: `Create a new Android box with various options for device type, environment, and labels.

You can specify Android box configurations through various flags, including device type (virtual or physical),
setting environment variables, adding labels, and setting expiration time.`,
		Example: `  gbox box create android --device-type virtual
  gbox box create android --device-type physical --expires-in 2h
  gbox box create android --env DEBUG=true --label project=myapp
  gbox box create android --device-type virtual --expires-in 30m --label env=test`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAndroidCreate(opts)
		},
		DisableFlagsInUseLine: true,
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.OutputFormat, "output", "o", "text", "Output format (json or text)")
	flags.StringVarP(&opts.DeviceType, "device-type", "d", "virtual", "Device type (virtual or physical)")
	flags.StringArrayVarP(&opts.Env, "env", "e", []string{}, "Environment variables in KEY=VALUE format")
	flags.StringArrayVarP(&opts.Labels, "label", "l", []string{}, "Custom labels in KEY=VALUE format")
	flags.StringVar(&opts.ExpiresIn, "expires-in", "60m", "Box expiration time (e.g., 30s, 5m, 1h)")

	cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"json", "text"}, cobra.ShellCompDirectiveNoFileComp
	})
	cmd.RegisterFlagCompletionFunc("device-type", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"virtual", "physical"}, cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

func runAndroidCreate(opts *AndroidBoxCreateOptions) error {
	// create SDK client
	sdkClient, err := client.NewClientFromProfile()
	if err != nil {
		return fmt.Errorf("failed to initialize gbox client: %v", err)
	}

	// validate device type
	if opts.DeviceType != "virtual" && opts.DeviceType != "physical" {
		return fmt.Errorf("invalid device type: %s (must be 'virtual' or 'physical')", opts.DeviceType)
	}

	// validate expires-in format
	if opts.ExpiresIn != "" {
		if _, err := time.ParseDuration(opts.ExpiresIn); err != nil {
			return fmt.Errorf("invalid expires-in format: %s (must be duration like '30s', '5m', '1h')", opts.ExpiresIn)
		}
	}

	// create Android box using client abstraction
	box, err := client.CreateAndroidBox(sdkClient, opts.DeviceType, opts.Env, opts.Labels, opts.ExpiresIn)
	if err != nil {
		return fmt.Errorf("failed to create Android box: %v", err)
	}

	// output result
	if opts.OutputFormat == "json" {
		boxJSON, _ := json.MarshalIndent(box, "", "  ")
		fmt.Println(string(boxJSON))
	} else {
		// Extract ID from the response
		if boxMap, ok := box.(map[string]interface{}); ok {
			if id, exists := boxMap["id"].(string); exists {
				fmt.Printf("Android box created with ID \"%s\"\n", id)
			} else {
				fmt.Println("Android box created successfully")
			}
		} else {
			fmt.Println("Android box created successfully")
		}
	}

	return nil
}
