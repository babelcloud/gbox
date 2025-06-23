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

type BoxDeleteOptions struct {
	OutputFormat string
	DeleteAll    bool
	Force        bool
}

func NewBoxDeleteCommand() *cobra.Command {
	opts := &BoxDeleteOptions{}

	cmd := &cobra.Command{
		Use:   "delete [box-id]",
		Short: "Delete a box by its ID",
		Long:  "Delete a box by its ID or delete all boxes",
		Example: `  gbox box delete 550e8400-e29b-41d4-a716-446655440000
  gbox box delete --all --force
  gbox box delete --all
  gbox box delete 550e8400-e29b-41d4-a716-446655440000 --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDelete(opts, args)
		},
		ValidArgsFunction: completeBoxIDs,
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.OutputFormat, "output", "text", "Output format (json or text)")
	flags.BoolVar(&opts.DeleteAll, "all", false, "Delete all boxes")
	flags.BoolVar(&opts.Force, "force", false, "Force deletion without confirmation")

	cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"json", "text"}, cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

func runDelete(opts *BoxDeleteOptions, args []string) error {
	if !opts.DeleteAll && len(args) == 0 {
		return fmt.Errorf("must specify either --all or a box ID")
	}

	if opts.DeleteAll && len(args) > 0 {
		return fmt.Errorf("cannot specify both --all and a box ID")
	}

	if opts.DeleteAll {
		return deleteAllBoxes(opts)
	}

	return deleteBox(args[0], opts)
}

func deleteAllBoxes(opts *BoxDeleteOptions) error {
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
			fmt.Println(`{"status":"success","message":"No boxes to delete"}`)
		} else {
			fmt.Println("No boxes to delete")
		}
		return nil
	}

	fmt.Println("The following boxes will be deleted:")
	for _, box := range resp.Data {
		fmt.Printf("  - %s\n", box.ID)
	}
	fmt.Println()

	if !opts.Force {
		fmt.Print("Are you sure you want to delete all boxes? [y/N] ")
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
		if err := performBoxDeletion(client, box.ID); err != nil {
			fmt.Printf("Error: Failed to delete box %s: %v\n", box.ID, err)
			success = false
		}
	}

	if success {
		if opts.OutputFormat == "json" {
			fmt.Println(`{"status":"success","message":"All boxes deleted successfully"}`)
		} else {
			fmt.Println("All boxes deleted successfully")
		}
	} else {
		if opts.OutputFormat == "json" {
			fmt.Println(`{"status":"error","message":"Some boxes failed to delete"}`)
		} else {
			fmt.Println("Some boxes failed to delete")
		}
		return fmt.Errorf("some boxes failed to delete")
	}
	return nil
}

func deleteBox(boxIDPrefix string, opts *BoxDeleteOptions) error {
	resolvedBoxID, _, err := ResolveBoxIDPrefix(boxIDPrefix)
	if err != nil {
		return fmt.Errorf("failed to resolve box ID: %w", err)
	}

	// 创建 SDK 客户端
	client, err := gboxclient.NewClientFromProfile()
	if err != nil {
		return fmt.Errorf("failed to initialize gbox client: %v", err)
	}

	if err := performBoxDeletion(client, resolvedBoxID); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		return nil
	}

	if opts.OutputFormat == "json" {
		fmt.Println(`{"status":"success","message":"Box deleted successfully"}`)
	} else {
		fmt.Printf("Box %s deleted successfully\n", resolvedBoxID)
	}
	return nil
}

func performBoxDeletion(client *sdk.Client, boxID string) error {
	// 构建 SDK 参数
	terminateParams := sdk.V1BoxTerminateParams{}

	// 调试输出
	if os.Getenv("DEBUG") == "true" {
		fmt.Fprintf(os.Stderr, "Deleting box: %s\n", boxID)
	}

	// 调用 SDK
	ctx := context.Background()
	err := client.V1.Boxes.Terminate(ctx, boxID, terminateParams)
	if err != nil {
		return fmt.Errorf("failed to delete box: %v", err)
	}

	return nil
}
