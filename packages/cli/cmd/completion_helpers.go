package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	client "github.com/babelcloud/gbox/packages/cli/internal/client"
	"github.com/spf13/cobra"
)

// completeBoxIDs provides completion for box IDs by fetching them from the API.
func completeBoxIDs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	debug := os.Getenv("DEBUG") == "true"

	// create SDK client
	sdkClient, err := client.NewClientFromProfile()
	if err != nil {
		if debug {
			fmt.Fprintf(os.Stderr, "DEBUG: [completion] Failed to initialize gbox client: %v\n", err)
		}
		return nil, cobra.ShellCompDirectiveError
	}

	if debug {
		fmt.Fprintf(os.Stderr, "DEBUG: [completion] Fetching box IDs using client abstraction\n")
	}

	// call client abstraction to get box list
	resp, err := client.ListBoxes(sdkClient, []string{})
	if err != nil {
		if debug {
			fmt.Fprintf(os.Stderr, "DEBUG: [completion] Failed to get box list: %v\n", err)
		}
		return nil, cobra.ShellCompDirectiveError
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

	var ids []string
	for _, m := range data {
		if id, ok := m["id"].(string); ok {
			ids = append(ids, id)
		}
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

	// create SDK client
	sdkClient, err := client.NewClientFromProfile()
	if err != nil {
		if debug {
			fmt.Fprintf(os.Stderr, "DEBUG: [ResolveBoxIDPrefix] Failed to initialize gbox client: %v\n", err)
		}
		return "", nil, fmt.Errorf("failed to initialize gbox client: %w", err)
	}

	if debug {
		fmt.Fprintf(os.Stderr, "DEBUG: [ResolveBoxIDPrefix] Fetching box IDs using client abstraction for prefix '%s'\n", prefix)
	}

	// call client abstraction to get box list
	resp, err := client.ListBoxes(sdkClient, []string{})
	if err != nil {
		if debug {
			fmt.Fprintf(os.Stderr, "DEBUG: [ResolveBoxIDPrefix] Failed to get box list: %v\n", err)
		}
		return "", nil, fmt.Errorf("failed to get box list: %w", err)
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

	if debug {
		var allIDs []string
		for _, m := range data {
			if id, ok := m["id"].(string); ok {
				allIDs = append(allIDs, id)
			}
		}
		fmt.Fprintf(os.Stderr, "DEBUG: [ResolveBoxIDPrefix] All fetched IDs: %v\n", allIDs)
	}

	// perform prefix matching
	for _, m := range data {
		if id, ok := m["id"].(string); ok {
			if strings.HasPrefix(id, prefix) {
				matchedIDs = append(matchedIDs, id)
			}
		}
	}

	if debug {
		fmt.Fprintf(os.Stderr, "DEBUG: [ResolveBoxIDPrefix] Matched IDs for prefix '%s': %v\n", prefix, matchedIDs)
	}

	// handle matching results
	if len(matchedIDs) == 0 {
		return "", nil, fmt.Errorf("no box found with ID prefix: %s", prefix)
	}
	if len(matchedIDs) == 1 {
		return matchedIDs[0], matchedIDs, nil // unique match
	}
	// multiple matches
	return "", matchedIDs, fmt.Errorf("multiple boxes found with ID prefix '%s'. Please be more specific. Matches:\n  %s", prefix, strings.Join(matchedIDs, "\n  "))
}
