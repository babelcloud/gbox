package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	// internal SDK client
	sdk "github.com/babelcloud/gbox-sdk-go"
	gboxclient "github.com/babelcloud/gbox/packages/cli/internal/gboxsdk"
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
	client, err := gboxclient.NewClientFromProfile()
	if err != nil {
		return fmt.Errorf("failed to initialize gbox client: %v", err)
	}

	// parse environment variables
	envMap, err := parseKeyValuePairs(opts.Env, "environment variable")
	if err != nil {
		return err
	}

	// parse labels
	labelMap, err := parseKeyValuePairs(opts.Labels, "label")
	if err != nil {
		return err
	}

	// build SDK parameters
	createParams := sdk.V1BoxNewLinuxParams{
		CreateLinuxBox: sdk.CreateLinuxBoxParam{
			Wait: sdk.Bool(true), // wait for operation to complete
			Config: sdk.CreateBoxConfigParam{
				Envs:   envMap,
				Labels: labelMap,
			},
		},
	}

	// debug output
	if os.Getenv("DEBUG") == "true" {
		fmt.Fprintf(os.Stderr, "Request params:\n")
		requestJSON, _ := json.MarshalIndent(createParams, "", "  ")
		fmt.Fprintln(os.Stderr, string(requestJSON))
	}

	// call SDK
	ctx := context.Background()
	box, err := client.V1.Boxes.NewLinux(ctx, createParams)
	if err != nil {
		return fmt.Errorf("failed to create box: %v", err)
	}

	// output result
	if opts.OutputFormat == "json" {
		boxJSON, _ := json.MarshalIndent(box, "", "  ")
		fmt.Println(string(boxJSON))
	} else {
		fmt.Printf("Box created with ID \"%s\"\n", box.ID)
	}

	return nil
}
