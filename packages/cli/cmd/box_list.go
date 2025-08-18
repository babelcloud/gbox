package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

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
	// 如果显式指定了 API_ENDPOINT，则直接通过 HTTP 调用以保持原始字段（如 image）
	if base := os.Getenv("API_ENDPOINT"); base != "" {
		boxes, err := fetchBoxesDirect(base, opts.Filters)
		if err != nil {
			return fmt.Errorf("API call failed: %v", err)
		}
		return outputBoxes(boxes, opts.OutputFormat)
	}

	// 创建 SDK 客户端
	sdkClient, err := client.NewClientFromProfile()
	if err != nil {
		return fmt.Errorf("failed to initialize gbox client: %v", err)
	}

	// 调用 API using client abstraction
	resp, err := client.ListBoxes(sdkClient, opts.Filters)
	if err != nil {
		return fmt.Errorf("API call failed: %v", err)
	}

	// 输出结果
	return printResponse(resp, opts.OutputFormat)
}

// fetchBoxesDirect calls the boxes API directly and returns the raw data slice
func fetchBoxesDirect(base string, filters []string) ([]map[string]interface{}, error) {
	u, err := url.Parse(strings.TrimSuffix(base, "/"))
	if err != nil {
		return nil, err
	}
	u.Path = "/api/v1/boxes"

	// build query
	q := u.Query()
	for _, f := range filters {
		if strings.HasPrefix(f, "label=") || strings.HasPrefix(f, "labels=") {
			q.Add("labels", strings.TrimPrefix(strings.TrimPrefix(f, "label="), "labels="))
		}
		// other filters can be added similarly when needed
	}
	u.RawQuery = q.Encode()

	resp, err := http.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected HTTP status: %s", resp.Status)
	}

	var raw struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	return raw.Data, nil
}

// outputBoxes prints boxes according to output format using raw maps
func outputBoxes(data []map[string]interface{}, format string) error {
	if format == "json" {
		out := map[string]interface{}{"data": data}
		bytes, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(bytes))
		return nil
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

	renderTable(columns, data)
	return nil
}

// printResponse handles output based on the selected format
func printResponse(resp interface{}, outputFormat string) error {
	if resp == nil {
		return fmt.Errorf("empty response")
	}

	if outputFormat == "json" {
		// 构造测试所期望的精简字段
		type simpleBox struct {
			ID     string `json:"id"`
			Image  string `json:"image"`
			Status string `json:"status"`
			Type   string `json:"type"`
		}
		var out struct {
			Data []simpleBox `json:"data"`
		}
		// 将 SDK 响应转为通用结构以便提取期望字段
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

	renderTable(columns, data)
	return nil
}
