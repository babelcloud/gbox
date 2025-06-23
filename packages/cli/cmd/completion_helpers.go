package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	// 内部 SDK 客户端
	sdk "github.com/babelcloud/gbox-sdk-go"
	gboxclient "github.com/babelcloud/gbox/packages/cli/internal/gboxsdk"
	"github.com/spf13/cobra"
)

// completeBoxIDs provides completion for box IDs by fetching them from the API.
func completeBoxIDs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	debug := os.Getenv("DEBUG") == "true"

	// 创建 SDK 客户端
	client, err := gboxclient.NewClientFromProfile()
	if err != nil {
		if debug {
			fmt.Fprintf(os.Stderr, "DEBUG: [completion] Failed to initialize gbox client: %v\n", err)
		}
		return nil, cobra.ShellCompDirectiveError
	}

	if debug {
		fmt.Fprintf(os.Stderr, "DEBUG: [completion] Fetching box IDs using SDK\n")
	}

	// 调用 SDK 获取 box 列表
	ctx := context.Background()
	listParams := sdk.V1BoxListParams{}
	resp, err := client.V1.Boxes.List(ctx, listParams)
	if err != nil {
		if debug {
			fmt.Fprintf(os.Stderr, "DEBUG: [completion] Failed to get box list: %v\n", err)
		}
		return nil, cobra.ShellCompDirectiveError
	}

	var ids []string
	for _, box := range resp.Data {
		ids = append(ids, box.ID)
	}

	if debug {
		fmt.Fprintf(os.Stderr, "DEBUG: [completion] Found box IDs: %v (toComplete: '%s')\n", ids, toComplete)
	}
	return ids, cobra.ShellCompDirectiveNoFileComp
}

// ResolveBoxIDPrefix takes a prefix string and returns the unique full Box ID if found,
// or an error if not found or if multiple matches exist.
// It also returns the list of matched IDs in case of multiple matches.
func ResolveBoxIDPrefix(prefix string) (fullID string, matchedIDs []string, err error) {
	debug := os.Getenv("DEBUG") == "true"
	if prefix == "" {
		return "", nil, fmt.Errorf("box ID prefix cannot be empty")
	}

	// 创建 SDK 客户端
	client, err := gboxclient.NewClientFromProfile()
	if err != nil {
		if debug {
			fmt.Fprintf(os.Stderr, "DEBUG: [ResolveBoxIDPrefix] Failed to initialize gbox client: %v\n", err)
		}
		return "", nil, fmt.Errorf("failed to initialize gbox client: %w", err)
	}

	if debug {
		fmt.Fprintf(os.Stderr, "DEBUG: [ResolveBoxIDPrefix] Fetching box IDs using SDK for prefix '%s'\n", prefix)
	}

	// 调用 SDK 获取 box 列表
	ctx := context.Background()
	listParams := sdk.V1BoxListParams{}
	resp, err := client.V1.Boxes.List(ctx, listParams)
	if err != nil {
		if debug {
			fmt.Fprintf(os.Stderr, "DEBUG: [ResolveBoxIDPrefix] Failed to get box list: %v\n", err)
		}
		return "", nil, fmt.Errorf("failed to get box list: %w", err)
	}

	if debug {
		var allIDs []string
		for _, box := range resp.Data {
			allIDs = append(allIDs, box.ID)
		}
		fmt.Fprintf(os.Stderr, "DEBUG: [ResolveBoxIDPrefix] All fetched IDs: %v\n", allIDs)
	}
	fmt.Fprintf(os.Stderr, "DEBUG: [ResolveBoxIDPrefix] resp.Data: %v\n", resp.Data)
	// 执行前缀匹配
	for _, box := range resp.Data {
		if strings.HasPrefix(box.ID, prefix) {
			matchedIDs = append(matchedIDs, box.ID)
		}
	}

	if debug {
		fmt.Fprintf(os.Stderr, "DEBUG: [ResolveBoxIDPrefix] Matched IDs for prefix '%s': %v\n", prefix, matchedIDs)
	}

	// 处理匹配结果
	if len(matchedIDs) == 0 {
		return "", nil, fmt.Errorf("no box found with ID prefix: %s", prefix)
	}
	if len(matchedIDs) == 1 {
		return matchedIDs[0], matchedIDs, nil // 唯一匹配
	}
	// 多个匹配
	return "", matchedIDs, fmt.Errorf("multiple boxes found with ID prefix '%s'. Please be more specific. Matches:\n  %s", prefix, strings.Join(matchedIDs, "\n  "))
}
