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

type BoxStartOptions struct {
	OutputFormat string
}

type BoxStartResponse struct {
	Message string `json:"message"`
}

func NewBoxStartCommand() *cobra.Command {
	opts := &BoxStartOptions{}

	cmd := &cobra.Command{
		Use:   "start [box-id]",
		Short: "Start a stopped box",
		Long:  "Start a stopped box by its ID",
		Example: `  gbox box start 550e8400-e29b-41d4-a716-446655440000
  gbox box start 550e8400-e29b-41d4-a716-446655440000 --output json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStart(args[0], opts)
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

func runStart(boxIDPrefix string, opts *BoxStartOptions) error {
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
	startParams := sdk.V1BoxStartParams{}

	// 调试输出
	if os.Getenv("DEBUG") == "true" {
		fmt.Fprintf(os.Stderr, "Starting box: %s\n", resolvedBoxID)
	}

	// 调用 SDK
	ctx := context.Background()
	box, err := client.V1.Boxes.Start(ctx, resolvedBoxID, startParams)
	if err != nil {
		return fmt.Errorf("failed to start box: %v", err)
	}

	// 输出结果
	if opts.OutputFormat == "json" {
		boxJSON, _ := json.MarshalIndent(box, "", "  ")
		fmt.Println(string(boxJSON))
	} else {
		fmt.Printf("Box started successfully: %s\n", resolvedBoxID)
	}

	return nil
}
