package cmd

import (
	"encoding/json"
	"fmt"

	client "github.com/babelcloud/gbox/packages/cli/internal/client"
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
	flags.StringVarP(&opts.OutputFormat, "output", "o", "text", "Output format (json or text)")
	flags.StringArrayVarP(&opts.Filters, "filter", "f", []string{}, "Filter boxes (format: field=value)")

	cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"json", "text"}, cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

func runList(opts *BoxListOptions) error {
	// create SDK client
	sdkClient, err := client.NewClientFromProfile()
	if err != nil {
		return fmt.Errorf("failed to initialize gbox client: %v", err)
	}

	// call API using client abstraction
	resp, err := client.ListBoxes(sdkClient, opts.Filters)
	if err != nil {
		return fmt.Errorf("API call failed: %v", err)
	}

	// output result
	return printResponse(resp, opts.OutputFormat)
}

// printResponse handles output based on the selected format
func printResponse(resp interface{}, outputFormat string) error {
	if resp == nil {
		return fmt.Errorf("empty response")
	}

	if outputFormat == "json" {
		// construct simplified fields expected by tests
		type simpleBox struct {
			ID     string `json:"id"`
			Image  string `json:"image"`
			Status string `json:"status"`
			Type   string `json:"type"`
		}
		var out struct {
			Data []simpleBox `json:"data"`
		}
		// convert SDK response to generic structure to extract expected fields
		var raw struct {
			Data []map[string]interface{} `json:"data"`
		}
		if rawBytes, _ := json.Marshal(resp); rawBytes != nil {
			_ = json.Unmarshal(rawBytes, &raw)
		}
		for _, m := range raw.Data {
			sb := simpleBox{}
			if v, ok := m["id"].(string); ok {
				sb.ID = v
			}
			if v, ok := m["image"].(string); ok {
				sb.Image = v
			}
			if v, ok := m["status"].(string); ok {
				sb.Status = v
			}
			if v, ok := m["type"].(string); ok {
				sb.Type = v
			}
			out.Data = append(out.Data, sb)
		}

		jsonBytes, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal response: %v", err)
		}
		fmt.Println(string(jsonBytes))
		return nil
	}

	// For text output, we need to extract data from the response
	var data []map[string]interface{}
	if rawBytes, _ := json.Marshal(resp); rawBytes != nil {
		var raw struct {
			Data []map[string]interface{} `json:"data"`
		}
		_ = json.Unmarshal(rawBytes, &raw)
		data = raw.Data
	}

	if len(data) == 0 {
		fmt.Println("No boxes found")
		return nil
	}

	// Define table columns
	columns := []TableColumn{
		{Header: "ID", Key: "id"},
		{Header: "TYPE", Key: "type"},
		{Header: "STATUS", Key: "status"},
	}

	RenderTable(columns, data)
	return nil
}
