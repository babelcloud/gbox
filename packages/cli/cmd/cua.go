package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/babelcloud/gbox/packages/cli/config"
	"github.com/spf13/cobra"
)

type CuaAndroidOptions struct {
	OutputFormat string
}

type SSEMessage struct {
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

type CuaExecuteRequest struct {
	OpenAIAPIKey string `json:"openai_api_key"`
	Task         string `json:"task"`
}

// NewCuaCommand creates and returns the cua command
func NewCuaCommand() *cobra.Command {
	cuaCmd := &cobra.Command{
		Use:     "cua",
		Short:   "Manage CUA (Computer Use Agent) resources",
		Long:    `The cua command is used to manage Computer Use Agent resources and execute various automation tasks.`,
		Example: `  gbox cua android "search for something on Google"  # Execute an Android automation task`,
	}

	// Add all cua-related subcommands
	cuaCmd.AddCommand(
		NewCuaAndroidCommand(),
	)

	return cuaCmd
}

func NewCuaAndroidCommand() *cobra.Command {
	opts := &CuaAndroidOptions{}

	cmd := &cobra.Command{
		Use:     "android [task]",
		Short:   "Execute an Android automation task",
		Long:    "Execute an Android automation task using Computer Use Agent",
		Example: `  gbox cua android "search for gruai on Google"`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAndroidTask(args[0], opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.OutputFormat, "output", "text", "Output format (json or text)")

	cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"json", "text"}, cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

func runAndroidTask(task string, opts *CuaAndroidOptions) error {
	// Check for OpenAI API key in environment
	openaiAPIKey := os.Getenv("OPENAI_API_KEY")
	if openaiAPIKey == "" {
		return fmt.Errorf("OPENAI_API_KEY environment variable is not set. Please set it before running the command")
	}

	apiBase := config.GetLocalAPIURL()
	apiURL := fmt.Sprintf("%s/api/v1/cua/execute", strings.TrimSuffix(apiBase, "/"))

	if os.Getenv("DEBUG") == "true" {
		fmt.Fprintf(os.Stderr, "Request URL: %s\n", apiURL)
	}

	// Prepare request body
	requestData := CuaExecuteRequest{
		OpenAIAPIKey: openaiAPIKey,
		Task:         task,
	}

	requestBody, err := json.Marshal(requestData)
	if err != nil {
		return fmt.Errorf("failed to marshal request data: %v", err)
	}

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(string(requestBody)))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("API call failed: %v", err)
	}
	defer resp.Body.Close()

	if os.Getenv("DEBUG") == "true" {
		fmt.Fprintf(os.Stderr, "Response status code: %d\n", resp.StatusCode)
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("API returned error status: %d", resp.StatusCode)
	}

	// Handle SSE stream
	return handleSSEStream(resp, opts.OutputFormat)
}

func handleSSEStream(resp *http.Response, outputFormat string) error {
	scanner := bufio.NewScanner(resp.Body)

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines and SSE comments
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		// Parse SSE data line
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			// Skip [DONE] message
			if data == "[DONE]" {
				break
			}

			// Parse JSON message
			var message SSEMessage
			if err := json.Unmarshal([]byte(data), &message); err != nil {
				if os.Getenv("DEBUG") == "true" {
					fmt.Fprintf(os.Stderr, "Failed to parse SSE message: %v\n", err)
				}
				continue
			}

			// Output message based on format
			if outputFormat == "json" {
				fmt.Println(data)
			} else {
				fmt.Printf("[%s] %s\n", message.Timestamp, message.Message)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading SSE stream: %v", err)
	}

	return nil
}
