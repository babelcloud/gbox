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

type BoxStopOptions struct {
	OutputFormat string
}

func NewBoxStopCommand() *cobra.Command {
	opts := &BoxStopOptions{}

	cmd := &cobra.Command{
		Use:   "stop [box-id]",
		Short: "Stop a running box",
		Long:  "Stop a running box by its ID",
		Example: `  gbox box stop 550e8400-e29b-41d4-a716-446655440000
  gbox box stop 550e8400-e29b-41d4-a716-446655440000 --output json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStop(args[0], opts)
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

func runStop(boxIDPrefix string, opts *BoxStopOptions) error {
	resolvedBoxID, _, err := ResolveBoxIDPrefix(boxIDPrefix) // Use the new helper
	if err != nil {
		return fmt.Errorf("failed to resolve box ID: %w", err) // Return error if resolution fails
	}

	apiBase := config.GetLocalAPIURL()
	// Use resolvedBoxID for the API call
	apiURL := fmt.Sprintf("%s/api/v1/boxes/%s/stop", strings.TrimSuffix(apiBase, "/"), resolvedBoxID)

	if os.Getenv("DEBUG") == "true" {
		fmt.Fprintf(os.Stderr, "Request URL: %s\n", apiURL)
	}

	req, err := http.NewRequest("POST", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
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
	return handleStopResponse(resp.StatusCode, body, resolvedBoxID, opts.OutputFormat)
}

// Define a local struct to unmarshal the response
type stopResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func handleStopResponse(statusCode int, body []byte, boxID, outputFormat string) error {
	switch statusCode {
	case http.StatusOK: // Use http.StatusOK constant
		// Attempt to parse the response body
		var response stopResponse
		if err := json.Unmarshal(body, &response); err != nil {
			// Handle JSON parsing error - maybe print raw body or generic message?
			if outputFormat == "json" {
				// If JSON output requested but parsing failed, print the raw body anyway?
				fmt.Println(string(body))
			} else {
				fmt.Fprintf(os.Stderr, "Error parsing server response: %v\nFalling back to generic message.\n", err)
				fmt.Println("Box stop command successful (could not parse server message)")
			}
			return nil // Or return err?
		}

		if outputFormat == "json" {
			// Print the original JSON body for JSON output
			fmt.Println(string(body))
		} else {
			// Use the message from the parsed response for text output
			fmt.Println(response.Message)
		}
	case http.StatusNotFound: // Use http.StatusNotFound constant
		fmt.Printf("Box not found: %s\n", boxID)
	default:
		errorMsg := fmt.Sprintf("Error: Failed to stop box (HTTP %d)", statusCode)
		if os.Getenv("DEBUG") == "true" {
			errorMsg = fmt.Sprintf("%s\nResponse: %s", errorMsg, string(body))
		}
		fmt.Println(errorMsg)
	}

	return nil
}
