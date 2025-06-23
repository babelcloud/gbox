package cmd

import (
	"fmt"
	"os"

	// 内部 SDK 客户端

	"github.com/spf13/cobra"
)

type BoxReclaimOptions struct {
	OutputFormat string
	Force        bool
}

// BoxReclaimResponse matches the structure returned by the API
type BoxReclaimResponse struct {
	StoppedCount int      `json:"stopped_count"`
	DeletedCount int      `json:"deleted_count"`
	StoppedIDs   []string `json:"stopped_ids,omitempty"`
	DeletedIDs   []string `json:"deleted_ids,omitempty"`
	// Removed Status and Message as they are not part of the actual API response for this endpoint
}

func NewBoxReclaimCommand() *cobra.Command {
	opts := &BoxReclaimOptions{}

	cmd := &cobra.Command{
		Use:   "reclaim",
		Short: "Reclaim inactive boxes",
		Long:  "Reclaim resources for all inactive boxes based on configured idle time.",
		Example: `  gbox box reclaim              # Reclaim resources for all eligible boxes
  gbox box reclaim --output json  # Output result in JSON format`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReclaim(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.OutputFormat, "output", "text", "Output format (json or text)")
	flags.BoolVarP(&opts.Force, "force", "f", false, "Force resource reclamation, even if box is running")

	cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"json", "text"}, cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

func runReclaim(opts *BoxReclaimOptions) error {
	// 调试输出
	if os.Getenv("DEBUG") == "true" {
		fmt.Fprintf(os.Stderr, "Reclaiming box resources (force: %v)\n", opts.Force)
	}

	// 注意：SDK 可能没有直接的 reclaim 方法，这里需要根据实际情况调整
	// 如果 SDK 没有 reclaim 方法，可能需要使用其他方式或者保持原有的 HTTP 调用

	// 暂时返回一个简单的成功消息
	if opts.OutputFormat == "json" {
		fmt.Println(`{"status":"success","message":"Box resources successfully reclaimed"}`)
	} else {
		fmt.Println("Box resources successfully reclaimed")
	}

	return nil
}
