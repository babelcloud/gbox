package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/babelcloud/gbox/packages/cli/config"
	"github.com/spf13/cobra"
)

type BoxListOptions struct {
	OutputFormat string
	Filters      []string
}

type BoxResponse struct {
	Boxes []struct {
		ID     string `json:"id"`
		Image  string `json:"image"`
		Status string `json:"status"`
	} `json:"boxes"`
}

func NewBoxListCommand() *cobra.Command {
	opts := &BoxListOptions{}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all available boxes",
		Long:  "List all available boxes with various filtering options",
		Example: `  gbox box list
  gbox box list --output json
  gbox box list --filter 'label=project=myapp'
  gbox box list --filter 'ancestor=ubuntu:latest'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.OutputFormat, "output", "text", "Output format (json or text)")
	flags.StringArrayVarP(&opts.Filters, "filter", "f", []string{}, "Filter boxes (format: field=value)")

	cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"json", "text"}, cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

func runList(opts *BoxListOptions) error {
	queryParams := buildQueryParams(opts.Filters)

	apiBase := config.GetLocalAPIURL()
	apiURL := fmt.Sprintf("%s/api/v1/boxes%s", strings.TrimSuffix(apiBase, "/"), queryParams)

	if os.Getenv("DEBUG") == "true" {
		fmt.Fprintf(os.Stderr, "Request URL: %s\n", apiURL)
	}

	resp, err := http.Get(apiURL)
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

	return handleResponse(resp.StatusCode, body, opts.OutputFormat)
}

func buildQueryParams(filters []string) string {
	if len(filters) == 0 {
		return ""
	}

	var params []string
	for _, filter := range filters {
		parts := strings.SplitN(filter, "=", 2)
		if len(parts) == 2 {
			field := parts[0]
			value := url.QueryEscape(parts[1])
			params = append(params, fmt.Sprintf("filter=%s=%s", field, value))
		}
	}

	return "?" + strings.Join(params, "&")
}

func handleResponse(statusCode int, body []byte, outputFormat string) error {
	switch statusCode {
	case 200:
		var response BoxResponse
		if err := json.Unmarshal(body, &response); err != nil {
			return fmt.Errorf("failed to parse JSON response: %v", err)
		}

		if outputFormat == "json" {
			fmt.Println(string(body))
		} else {
			printTextFormat(response)
		}
	case 404:
		fmt.Println("No boxes found")
	default:
		errorMsg := fmt.Sprintf("failed to get box list (HTTP %d)", statusCode)
		if os.Getenv("DEBUG") == "true" {
			errorMsg = fmt.Sprintf("%s\nResponse: %s", errorMsg, string(body))
		}
		return fmt.Errorf("%s", errorMsg)
	}

	return nil
}

func printTextFormat(response BoxResponse) {
	if len(response.Boxes) == 0 {
		fmt.Println("No boxes found")
		return
	}

	fmt.Println("ID                                      IMAGE               STATUS")
	fmt.Println("---------------------------------------- ------------------- ---------------")

	for _, box := range response.Boxes {
		image := box.Image
		if strings.HasPrefix(image, "sha256:") {
			image = strings.TrimPrefix(image, "sha256:")
			if len(image) > 12 {
				image = image[:12]
			}
		}
		fmt.Printf("%-40s %-20s %s\n", box.ID, image, box.Status)
	}
}
