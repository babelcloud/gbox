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
	flags.StringVar(&opts.OutputFormat, "output", "text", "Output format (json or text)")

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

	apiBase := config.GetLocalAPIURL()
	// Use resolvedBoxID for the API call
	apiURL := fmt.Sprintf("%s/api/v1/boxes/%s", strings.TrimSuffix(apiBase, "/"), resolvedBoxID)

	if os.Getenv("DEBUG") == "true" {
		fmt.Fprintf(os.Stderr, "Request URL: %s\n", apiURL)
	}

	resp, err := http.Get(apiURL) // GET request for inspect
	if err != nil {
		return fmt.Errorf("API call failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}

	if os.Getenv("DEBUG") == "true" {
		fmt.Fprintf(os.Stderr, "Response status code: %d\n", resp.StatusCode)
		fmt.Fprintf(os.Stderr, "Response content: %s\n", string(body))
	}

	// Pass resolvedBoxID to the handler
	return handleInspectResponse(resp.StatusCode, body, resolvedBoxID, opts.OutputFormat)
}

func handleInspectResponse(statusCode int, body []byte, boxID, outputFormat string) error {
	switch statusCode {
	case 200:
		if outputFormat == "json" {
			// Output JSON directly
			fmt.Printf("%s\n", string(body))
		} else {
			// Output in text format
			fmt.Println("Box details:")
			fmt.Println("------------")

			// Parse JSON and format output
			var data map[string]interface{}
			if err := json.Unmarshal(body, &data); err != nil {
				return fmt.Errorf("failed to parse JSON response: %v", err)
			}

			// Define the desired order of keys
			orderedKeys := []string{"id", "image", "status", "created_at", "extra_labels"}
			printedKeys := make(map[string]bool)

			// Print keys in the desired order
			for _, key := range orderedKeys {
				if value, exists := data[key]; exists {
					printKeyValue(key, value, outputFormat) // Use a helper function for printing
					printedKeys[key] = true
				}
			}

			// Print any remaining keys (that were not in the ordered list)
			for key, value := range data {
				if !printedKeys[key] {
					printKeyValue(key, value, outputFormat) // Use the same helper function
				}
			}
		}
	case 404:
		fmt.Printf("Box not found: %s\n", boxID)
	default:
		errorMsg := fmt.Sprintf("Error: Failed to get box details (HTTP %d)", statusCode)
		if os.Getenv("DEBUG") == "true" {
			errorMsg = fmt.Sprintf("%s\nResponse: %s", errorMsg, string(body))
		}
		fmt.Println(errorMsg)
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
