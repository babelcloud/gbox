package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	client "github.com/babelcloud/gbox/packages/cli/internal/client"
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
	sdkClient, err := client.NewClientFromProfile()
	if err != nil {
		return fmt.Errorf("failed to initialize gbox client: %v", err)
	}

	// 获取所有 boxes using client abstraction
	resp, err := client.ListBoxes(sdkClient, []string{})
	if err != nil {
		return fmt.Errorf("failed to get box list: %v", err)
	}

	// Extract data from response
	var data []map[string]interface{}
	if rawBytes, _ := json.Marshal(resp); rawBytes != nil {
		var raw struct {
			Data []map[string]interface{} `json:"data"`
		}
		_ = json.Unmarshal(rawBytes, &raw)
		data = raw.Data
	}

	if len(data) == 0 {
		if opts.OutputFormat == "json" {
			fmt.Println(`{"status":"success","message":"No boxes to terminate"}`)
		} else {
			fmt.Println("No boxes to terminate")
		}
		return nil
	}

	fmt.Println("The following boxes will be terminated:")
	for _, m := range data {
		if id, ok := m["id"].(string); ok {
			fmt.Printf("  - %s\n", id)
		}
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
	for _, m := range data {
		if id, ok := m["id"].(string); ok {
			if err := client.TerminateBox(sdkClient, id); err != nil {
				fmt.Printf("Error: Failed to terminate box %s: %v\n", id, err)
				success = false
			}
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
	sdkClient, err := client.NewClientFromProfile()
	if err != nil {
		return fmt.Errorf("failed to initialize gbox client: %v", err)
	}

	if err := client.TerminateBox(sdkClient, resolvedBoxID); err != nil {
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
