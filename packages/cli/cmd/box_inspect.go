package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	// 内部 SDK 客户端

	gboxclient "github.com/babelcloud/gbox/packages/cli/internal/gboxsdk"
	"github.com/spf13/cobra"
)

type BoxInspectOptions struct {
	OutputFormat string
}

func NewBoxInspectCommand() *cobra.Command {
	opts := &BoxInspectOptions{}

	cmd := &cobra.Command{
		Use:   "inspect [box-id]",
		Short: "Get detailed information about a box",
		Long:  "Get detailed information about a box by its ID",
		Example: `  gbox box inspect 550e8400-e29b-41d4-a716-446655440000              # Get box details
  gbox box inspect 550e8400-e29b-41d4-a716-446655440000 --output json  # Get box details in JSON format`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInspect(args[0], opts)
		},
		ValidArgsFunction: completeBoxIDs,
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.OutputFormat, "output", "o", "text", "Output format (json or text)")

	cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"json", "text"}, cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

func runInspect(boxIDPrefix string, opts *BoxInspectOptions) error {
	resolvedBoxID, _, err := ResolveBoxIDPrefix(boxIDPrefix) // Use the new helper
	if err != nil {
		return fmt.Errorf("failed to resolve box ID: %w", err) // Return error if resolution fails
	}

	// 创建 SDK 客户端
	client, err := gboxclient.NewClientFromProfile()
	if err != nil {
		return fmt.Errorf("failed to initialize gbox client: %v", err)
	}

	// 调试输出
	if os.Getenv("DEBUG") == "true" {
		fmt.Fprintf(os.Stderr, "Inspecting box: %s\n", resolvedBoxID)
	}

	// 调用 SDK
	ctx := context.Background()
	box, err := client.V1.Boxes.Get(ctx, resolvedBoxID)
	if err != nil {
		return fmt.Errorf("failed to get box details: %v", err)
	}

	// 输出结果
	if opts.OutputFormat == "json" {
		boxJSON, _ := json.MarshalIndent(box, "", "  ")
		fmt.Println(string(boxJSON))
	} else {
		// 输出文本格式
		fmt.Println("Box details:")
		fmt.Println("------------")

		// 将 box 转换为 map 以便格式化输出
		boxBytes, _ := json.Marshal(box)
		var data map[string]interface{}
		if err := json.Unmarshal(boxBytes, &data); err != nil {
			return fmt.Errorf("failed to parse box data: %v", err)
		}

		// 定义期望的键顺序
		orderedKeys := []string{"id", "image", "status", "createdAt", "extra_labels"}
		printedKeys := make(map[string]bool)

		// 按期望顺序打印键
		for _, key := range orderedKeys {
			if value, exists := data[key]; exists {
				printKeyValue(key, value, opts.OutputFormat)
				printedKeys[key] = true
			}
		}

		// 打印任何剩余的键
		for key, value := range data {
			if !printedKeys[key] {
				printKeyValue(key, value, opts.OutputFormat)
			}
		}
	}

	return nil
}

// Helper function to print key-value pairs with special formatting for extra_labels
func printKeyValue(key string, value interface{}, outputFormat string) {
	// Special handling for extra_labels when output is text
	if key == "extra_labels" && outputFormat == "text" {
		if labelsMap, ok := value.(map[string]interface{}); ok {
			fmt.Printf("%-15s:", key)
			if len(labelsMap) > 0 {
				// Need to sort the labels for consistent output within extra_labels as well
				labelKeys := make([]string, 0, len(labelsMap))
				for k := range labelsMap {
					labelKeys = append(labelKeys, k)
				}
				// sort.Strings(labelKeys) // Requires importing "sort"
				// Iterate and print labels with the new format
				for i, labelKey := range labelKeys { // Iterate using sorted keys if sort was imported
					labelValue := labelsMap[labelKey]
					if i == 0 {
						// Print first label on the same line
						fmt.Printf(" %s: %v\n", labelKey, labelValue)
					} else {
						// Print subsequent labels on new lines, aligned
						fmt.Printf("%-15s  %s: %v\n", "", labelKey, labelValue)
					}
				}
			} else {
				fmt.Println() // Still print a newline even if empty, for consistent spacing
			}
			return // Handled extra_labels, exit function for this key
		}
	}

	// Default handling for other keys
	var valueStr string
	switch v := value.(type) {
	case string, float64, bool, int, nil:
		valueStr = fmt.Sprintf("%v", v)
	default:
		// For complex types (maps, slices, etc.) other than handled extra_labels, use JSON format
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			valueStr = fmt.Sprintf("%v (error marshaling: %v)", v, err)
		} else {
			valueStr = string(jsonBytes)
		}
	}
	fmt.Printf("%-15s: %s\n", key, valueStr)
}
