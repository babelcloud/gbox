package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	// 内部 SDK 客户端
	sdk "github.com/babelcloud/gbox-sdk-go"
	gboxclient "github.com/babelcloud/gbox/packages/cli/internal/gboxsdk"
	"github.com/spf13/cobra"
)

type BoxStopOptions struct {
	OutputFormat string
}

func NewBoxStopCommand() *cobra.Command {
	opts := &BoxStopOptions{}

	cmd := &cobra.Command{
		Use:   "stop [box-id]",
		Short: "Stop a running box",
		Long:  "Stop a running box by its ID",
		Example: `  gbox box stop 550e8400-e29b-41d4-a716-446655440000
  gbox box stop 550e8400-e29b-41d4-a716-446655440000 --output json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStop(args[0], opts)
		},
		ValidArgsFunction: completeBoxIDs,
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.OutputFormat, "output", "text", "Output format (json or text)")

	cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"json", "text"}, cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

func runStop(boxIDPrefix string, opts *BoxStopOptions) error {
	resolvedBoxID, _, err := ResolveBoxIDPrefix(boxIDPrefix) // Use the new helper
	if err != nil {
		return fmt.Errorf("failed to resolve box ID: %w", err) // Return error if resolution fails
	}

	// 创建 SDK 客户端
	client, err := gboxclient.NewClientFromProfile()
	if err != nil {
		return fmt.Errorf("failed to initialize gbox client: %v", err)
	}

	// 构建 SDK 参数
	stopParams := sdk.V1BoxStopParams{}

	// 调试输出
	if os.Getenv("DEBUG") == "true" {
		fmt.Fprintf(os.Stderr, "Stopping box: %s\n", resolvedBoxID)
	}

	// 调用 SDK
	ctx := context.Background()
	box, err := client.V1.Boxes.Stop(ctx, resolvedBoxID, stopParams)
	if err != nil {
		return fmt.Errorf("failed to stop box: %v", err)
	}

	// 输出结果
	if opts.OutputFormat == "json" {
		boxJSON, _ := json.MarshalIndent(box, "", "  ")
		fmt.Println(string(boxJSON))
	} else {
		fmt.Printf("Box stopped successfully: %s\n", resolvedBoxID)
	}

	return nil
}
