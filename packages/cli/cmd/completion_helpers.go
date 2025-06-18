package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/babelcloud/gbox/packages/cli/config"
	"github.com/spf13/cobra"
)

// BoxListCompletionResponse is used to parse the list of boxes for completion.
type BoxListCompletionResponse struct {
	Boxes []struct {
		ID string `json:"id"`
	} `json:"boxes"`
}

// completeBoxIDs provides completion for box IDs by fetching them from the API.
func completeBoxIDs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	debug := os.Getenv("DEBUG") == "true"
	apiBase := config.GetLocalAPIURL()
	// Ensure apiBase is not empty before trying to use it
	if apiBase == "" {
		if debug {
			fmt.Fprintln(os.Stderr, "DEBUG: [completion] API URL is not configured.")
		}
		return nil, cobra.ShellCompDirectiveError
	}
	apiURL := fmt.Sprintf("%s/api/v1/boxes", strings.TrimSuffix(apiBase, "/"))

	if debug {
		fmt.Fprintf(os.Stderr, "DEBUG: [completion] Fetching box IDs from %s\n", apiURL)
	}

	client := &http.Client{}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		if debug {
			fmt.Fprintf(os.Stderr, "DEBUG: [completion] Failed to create request for box list: %v\n", err)
		}
		return nil, cobra.ShellCompDirectiveError
	}

	resp, err := client.Do(req)
	if err != nil {
		if debug {
			fmt.Fprintf(os.Stderr, "DEBUG: [completion] Failed to get box list: %v\n", err)
		}
		return nil, cobra.ShellCompDirectiveError
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if debug {
			bodyBytes, _ := io.ReadAll(resp.Body) // Read body for debugging
			fmt.Fprintf(os.Stderr, "DEBUG: [completion] API for box list returned non-200 status: %d, body: %s\n", resp.StatusCode, string(bodyBytes))
		}
		return nil, cobra.ShellCompDirectiveError
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		if debug {
			fmt.Fprintf(os.Stderr, "DEBUG: [completion] Failed to read response body: %v\n", err)
		}
		return nil, cobra.ShellCompDirectiveError
	}

	var response BoxListCompletionResponse
	if err := json.Unmarshal(body, &response); err != nil {
		if debug {
			fmt.Fprintf(os.Stderr, "DEBUG: [completion] Failed to parse JSON for box list: %v\nBody: %s\n", err, string(body)) // Log the body on JSON error
		}
		return nil, cobra.ShellCompDirectiveError
	}

	var ids []string
	for _, box := range response.Boxes {
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

	// 1. Fetch all box IDs (similar to completeBoxIDs)
	apiBase := config.GetLocalAPIURL()
	if apiBase == "" {
		if debug {
			fmt.Fprintln(os.Stderr, "DEBUG: [ResolveBoxIDPrefix] API URL is not configured.")
		}
		return "", nil, fmt.Errorf("API URL is not configured")
	}
	apiURL := fmt.Sprintf("%s/api/v1/boxes", strings.TrimSuffix(apiBase, "/"))

	if debug {
		fmt.Fprintf(os.Stderr, "DEBUG: [ResolveBoxIDPrefix] Fetching box IDs from %s for prefix '%s'\n", apiURL, prefix)
	}

	// It's generally a good idea to use a client with a timeout.
	// For simplicity here, we use a default client.
	client := &http.Client{}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create request for box list: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get box list: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body) // Read body for debugging, ignore error here
		return "", nil, fmt.Errorf("API for box list returned non-200 status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read response body for box list: %w", err)
	}

	// Assuming BoxListCompletionResponse is defined as in completeBoxIDs
	// which should be in the same file:
	// type BoxListCompletionResponse struct {
	// 	Boxes []struct {
	// 		ID string `json:"id"`
	// 	} `json:"boxes"`
	// }
	var response BoxListCompletionResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", nil, fmt.Errorf("failed to parse JSON for box list: %w\nBody: %s", err, string(body))
	}

	if debug {
		var allIDs []string
		for _, box := range response.Boxes {
			allIDs = append(allIDs, box.ID)
		}
		fmt.Fprintf(os.Stderr, "DEBUG: [ResolveBoxIDPrefix] All fetched IDs: %v\n", allIDs)
	}

	// 2. Perform prefix matching
	for _, box := range response.Boxes {
		if strings.HasPrefix(box.ID, prefix) {
			matchedIDs = append(matchedIDs, box.ID)
		}
	}

	if debug {
		fmt.Fprintf(os.Stderr, "DEBUG: [ResolveBoxIDPrefix] Matched IDs for prefix '%s': %v\n", prefix, matchedIDs)
	}

	// 3. Handle matching results
	if len(matchedIDs) == 0 {
		return "", nil, fmt.Errorf("no box found with ID prefix: %s", prefix)
	}
	if len(matchedIDs) == 1 {
		return matchedIDs[0], matchedIDs, nil // Unique match
	}
	// Multiple matches
	return "", matchedIDs, fmt.Errorf("multiple boxes found with ID prefix '%s'. Please be more specific. Matches:\n  %s", prefix, strings.Join(matchedIDs, "\n  "))
}
