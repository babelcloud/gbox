package cmd

import (
	"encoding/json"
	"fmt"

	client "github.com/babelcloud/gbox/packages/cli/internal/client"
	"github.com/spf13/cobra"
)

type LinuxBoxCreateOptions struct {
	OutputFormat string
	Env          []string
	Labels       []string
}

func NewBoxCreateLinuxCommand() *cobra.Command {
	opts := &LinuxBoxCreateOptions{}

	cmd := &cobra.Command{
		Use:   "linux [flags] -- [command] [args...]",
		Short: "Create a new Linux box",
		Long: `Create a new Linux box with various options for image, environment, and commands.

You can specify box configurations through various flags, including which container image to use,
setting environment variables, adding labels, and specifying a working directory.

Command arguments can be specified directly in the command line or added after the '--' separator.`,
		Example: `  gbox box create linux --env PATH=/usr/local/bin:/usr/bin:/bin -- python3 -c 'print("Hello")'
  gbox box create linux --label project=myapp --label env=prod`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLinuxCreate(opts)
		},
		DisableFlagsInUseLine: true,
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.OutputFormat, "output", "o", "text", "Output format (json or text)")
	flags.StringArrayVarP(&opts.Env, "env", "e", []string{}, "Environment variables in KEY=VALUE format")
	flags.StringArrayVarP(&opts.Labels, "label", "l", []string{}, "Custom labels in KEY=VALUE format")

	cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"json", "text"}, cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

func runLinuxCreate(opts *LinuxBoxCreateOptions) error {
	// create SDK client
	sdkClient, err := client.NewClientFromProfile()
	if err != nil {
		return fmt.Errorf("failed to initialize gbox client: %v", err)
	}

	// create Linux box using client abstraction
	box, err := client.CreateLinuxBox(sdkClient, opts.Env, opts.Labels)
	if err != nil {
		return fmt.Errorf("failed to create Linux box: %v", err)
	}

	// output result
	if opts.OutputFormat == "json" {
		boxJSON, _ := json.MarshalIndent(box, "", "  ")
		fmt.Println(string(boxJSON))
	} else {
		// Extract ID from the response
		if box.ID != "" {
			fmt.Printf("Linux box created with ID \"%s\"\n", box.ID)
		} else {
			fmt.Println("Linux box created successfully")
		}
	}

	return nil
}
