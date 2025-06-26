package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	// 内部 SDK 客户端
	sdk "github.com/babelcloud/gbox-sdk-go"
	gboxclient "github.com/babelcloud/gbox/packages/cli/internal/gboxsdk"
	"github.com/spf13/cobra"
)

type BoxTerminateOptions struct {
	OutputFormat string
	TerminateAll bool
	Force        bool
}

func NewBoxTerminateCommand() *cobra.Command {
	opts := &BoxTerminateOptions{}

	cmd := &cobra.Command{
		Use:   "terminate [box-id]",
		Short: "Terminate a box by its ID",
		Long:  "Terminate a box by its ID or terminate all boxes",
		Example: `  gbox box terminate 550e8400-e29b-41d4-a716-446655440000
  gbox box terminate --all --force
  gbox box terminate --all
  gbox box terminate 550e8400-e29b-41d4-a716-446655440000 --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTerminate(opts, args)
		},
		ValidArgsFunction: completeBoxIDs,
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.OutputFormat, "output", "o", "text", "Output format (json or text)")
	flags.BoolVarP(&opts.TerminateAll, "all", "a", false, "Terminate all boxes")
	flags.BoolVarP(&opts.Force, "force", "f", false, "Force termination without confirmation")

	cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"json", "text"}, cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

func runTerminate(opts *BoxTerminateOptions, args []string) error {
	if !opts.TerminateAll && len(args) == 0 {
		return fmt.Errorf("must specify either --all or a box ID")
	}

	if opts.TerminateAll && len(args) > 0 {
		return fmt.Errorf("cannot specify both --all and a box ID")
	}

	if opts.TerminateAll {
		return terminateAllBoxes(opts)
	}

	return terminateBox(args[0], opts)
}

func terminateAllBoxes(opts *BoxTerminateOptions) error {
	// 创建 SDK 客户端
	client, err := gboxclient.NewClientFromProfile()
	if err != nil {
		return fmt.Errorf("failed to initialize gbox client: %v", err)
	}

	// 获取所有 boxes
	ctx := context.Background()
	listParams := sdk.V1BoxListParams{}
	resp, err := client.V1.Boxes.List(ctx, listParams)
	if err != nil {
		return fmt.Errorf("failed to get box list: %v", err)
	}

	if len(resp.Data) == 0 {
		if opts.OutputFormat == "json" {
			fmt.Println(`{"status":"success","message":"No boxes to terminate"}`)
		} else {
			fmt.Println("No boxes to terminate")
		}
		return nil
	}

	fmt.Println("The following boxes will be terminated:")
	for _, box := range resp.Data {
		fmt.Printf("  - %s\n", box.ID)
	}
	fmt.Println()

	if !opts.Force {
		fmt.Print("Are you sure you want to terminate all boxes? [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		reply, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %v", err)
		}

		reply = strings.TrimSpace(strings.ToLower(reply))
		if reply != "y" && reply != "yes" {
			if opts.OutputFormat == "json" {
				fmt.Println(`{"status":"cancelled","message":"Operation cancelled by user"}`)
			} else {
				fmt.Println("Operation cancelled")
			}
			return nil
		}
	}

	success := true
	for _, box := range resp.Data {
		if err := performBoxTermination(client, box.ID); err != nil {
			fmt.Printf("Error: Failed to terminate box %s: %v\n", box.ID, err)
			success = false
		}
	}

	if success {
		if opts.OutputFormat == "json" {
			fmt.Println(`{"status":"success","message":"All boxes terminated successfully"}`)
		} else {
			fmt.Println("All boxes terminated successfully")
		}
	} else {
		if opts.OutputFormat == "json" {
			fmt.Println(`{"status":"error","message":"Some boxes failed to terminate"}`)
		} else {
			fmt.Println("Some boxes failed to terminate")
		}
		return fmt.Errorf("some boxes failed to terminate")
	}
	return nil
}

func terminateBox(boxIDPrefix string, opts *BoxTerminateOptions) error {
	resolvedBoxID, _, err := ResolveBoxIDPrefix(boxIDPrefix)
	if err != nil {
		return fmt.Errorf("failed to resolve box ID: %w", err)
	}

	// 创建 SDK 客户端
	client, err := gboxclient.NewClientFromProfile()
	if err != nil {
		return fmt.Errorf("failed to initialize gbox client: %v", err)
	}

	if err := performBoxTermination(client, resolvedBoxID); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		return nil
	}

	if opts.OutputFormat == "json" {
		fmt.Println(`{"status":"success","message":"Box terminated successfully"}`)
	} else {
		fmt.Printf("Box %s terminated successfully\n", resolvedBoxID)
	}
	return nil
}

func performBoxTermination(client *sdk.Client, boxID string) error {
	// 构建 SDK 参数
	terminateParams := sdk.V1BoxTerminateParams{}

	// 调试输出
	if os.Getenv("DEBUG") == "true" {
		fmt.Fprintf(os.Stderr, "Terminating box: %s\n", boxID)
	}

	// 调用 SDK
	ctx := context.Background()
	err := client.V1.Boxes.Terminate(ctx, boxID, terminateParams)
	if err != nil {
		return fmt.Errorf("failed to terminate box: %v", err)
	}

	return nil
}
