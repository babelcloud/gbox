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
	apiURL := buildReclaimAPIURL(opts.Force)

	if os.Getenv("DEBUG") == "true" {
		fmt.Fprintf(os.Stderr, "Request URL: %s\n", apiURL)
	}

	req, err := http.NewRequest("POST", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

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

	return handleReclaimResponse(resp.StatusCode, body, opts.OutputFormat)
}

func buildReclaimAPIURL(force bool) string {
	apiBase := config.GetLocalAPIURL()
	apiURL := fmt.Sprintf("%s/api/v1/boxes/reclaim", strings.TrimSuffix(apiBase, "/"))

	if force {
		apiURL += "?force=true"
	}

	return apiURL
}

func handleReclaimResponse(statusCode int, body []byte, outputFormat string) error {
	switch statusCode {
	case 200:
		if outputFormat == "json" {
			fmt.Println(string(body))
		} else {
			var response BoxReclaimResponse
			if err := json.Unmarshal(body, &response); err != nil {
				fmt.Println("Box resources successfully reclaimed")
			} else {
				fmt.Println("Box resources successfully reclaimed")
				if response.StoppedCount > 0 {
					fmt.Printf("Stopped %d boxes\n", response.StoppedCount)
				}
				if response.DeletedCount > 0 {
					fmt.Printf("Deleted %d boxes\n", response.DeletedCount)
				}
			}
		}
	case 404:
		fmt.Println("No inactive boxes found to reclaim or API endpoint not found.")
	case 400:
		fmt.Printf("Error: Invalid request: %s\n", string(body))
	default:
		errorMsg := fmt.Sprintf("Error: Failed to reclaim box resources (HTTP %d)", statusCode)
		if os.Getenv("DEBUG") == "true" {
			errorMsg = fmt.Sprintf("%s\nResponse: %s", errorMsg, string(body))
		}
		fmt.Println(errorMsg)
	}

	return nil
}
