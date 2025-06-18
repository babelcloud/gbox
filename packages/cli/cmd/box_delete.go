package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/babelcloud/gbox/packages/cli/config"
	"github.com/spf13/cobra"
)

type BoxDeleteOptions struct {
	OutputFormat string
	DeleteAll    bool
	Force        bool
}

type BoxListResponse struct {
	Boxes []struct {
		ID string `json:"id"`
	} `json:"boxes"`
}

func NewBoxDeleteCommand() *cobra.Command {
	opts := &BoxDeleteOptions{}

	cmd := &cobra.Command{
		Use:   "delete [box-id]",
		Short: "Delete a box by its ID",
		Long:  "Delete a box by its ID or delete all boxes",
		Example: `  gbox box delete 550e8400-e29b-41d4-a716-446655440000
  gbox box delete --all --force
  gbox box delete --all
  gbox box delete 550e8400-e29b-41d4-a716-446655440000 --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDelete(opts, args)
		},
		ValidArgsFunction: completeBoxIDs,
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.OutputFormat, "output", "text", "Output format (json or text)")
	flags.BoolVar(&opts.DeleteAll, "all", false, "Delete all boxes")
	flags.BoolVar(&opts.Force, "force", false, "Force deletion without confirmation")

	cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"json", "text"}, cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

func runDelete(opts *BoxDeleteOptions, args []string) error {
	if !opts.DeleteAll && len(args) == 0 {
		return fmt.Errorf("must specify either --all or a box ID")
	}

	if opts.DeleteAll && len(args) > 0 {
		return fmt.Errorf("cannot specify both --all and a box ID")
	}

	if opts.DeleteAll {
		return deleteAllBoxes(opts)
	}

	return deleteBox(args[0], opts)
}

func deleteAllBoxes(opts *BoxDeleteOptions) error {
	apiBase := config.GetLocalAPIURL()
	apiURL := fmt.Sprintf("%s/api/v1/boxes", strings.TrimSuffix(apiBase, "/"))

	resp, err := http.Get(apiURL)
	if err != nil {
		return fmt.Errorf("failed to get box list: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}

	if os.Getenv("DEBUG") == "true" {
		fmt.Fprintf(os.Stderr, "API response:\n")
		var prettyJSON bytes.Buffer
		if err := json.Indent(&prettyJSON, body, "", "  "); err == nil {
			fmt.Fprintln(os.Stderr, prettyJSON.String())
		} else {
			fmt.Fprintln(os.Stderr, string(body))
		}
	}

	var response BoxListResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("failed to parse JSON response: %v", err)
	}

	if len(response.Boxes) == 0 {
		if opts.OutputFormat == "json" {
			fmt.Println(`{"status":"success","message":"No boxes to delete"}`)
		} else {
			fmt.Println("No boxes to delete")
		}
		return nil
	}

	fmt.Println("The following boxes will be deleted:")
	for _, box := range response.Boxes {
		fmt.Printf("  - %s\n", box.ID)
	}
	fmt.Println()

	if !opts.Force {
		fmt.Print("Are you sure you want to delete all boxes? [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		reply, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %v", err)
		}

		reply = strings.TrimSpace(strings.ToLower(reply))
		if reply != "y" && reply != "yes" {
			if opts.OutputFormat == "json" {
				fmt.Println(`{"status":"cancelled","message":"Operation cancelled by user"}`)
			} else {
				fmt.Println("Operation cancelled")
			}
			return nil
		}
	}

	success := true
	for _, box := range response.Boxes {
		if err := performBoxDeletion(box.ID); err != nil {
			fmt.Printf("Error: Failed to delete box %s: %v\n", box.ID, err)
			success = false
		}
	}

	if success {
		if opts.OutputFormat == "json" {
			fmt.Println(`{"status":"success","message":"All boxes deleted successfully"}`)
		} else {
			fmt.Println("All boxes deleted successfully")
		}
	} else {
		if opts.OutputFormat == "json" {
			fmt.Println(`{"status":"error","message":"Some boxes failed to delete"}`)
		} else {
			fmt.Println("Some boxes failed to delete")
		}
		return fmt.Errorf("some boxes failed to delete")
	}
	return nil
}

func deleteBox(boxIDPrefix string, opts *BoxDeleteOptions) error {
	resolvedBoxID, _, err := ResolveBoxIDPrefix(boxIDPrefix)
	if err != nil {
		return fmt.Errorf("failed to resolve box ID: %w", err)
	}

	if err := performBoxDeletion(resolvedBoxID); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		return nil
	}

	if opts.OutputFormat == "json" {
		fmt.Println(`{"status":"success","message":"Box deleted successfully"}`)
	} else {
		fmt.Printf("Box %s deleted successfully\n", resolvedBoxID)
	}
	return nil
}

func performBoxDeletion(boxID string) error {
	apiBase := config.GetLocalAPIURL()
	apiURL := fmt.Sprintf("%s/api/v1/boxes/%s", strings.TrimSuffix(apiBase, "/"), boxID)

	req, err := http.NewRequest("DELETE", apiURL, strings.NewReader(`{"force":true}`))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Failed to delete box. Make sure the API server is running and the ID '%s' is correct", boxID)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		if resp.StatusCode == 404 {
			return fmt.Errorf("Failed to delete box. Make sure the API server is running and the ID '%s' is correct", boxID)
		}

		errorMsg := fmt.Sprintf("failed to delete box, HTTP status code: %d", resp.StatusCode)

		if os.Getenv("DEBUG") == "true" {
			body, _ := io.ReadAll(resp.Body)
			if len(body) > 0 {
				errorMsg = fmt.Sprintf("%s\nResponse: %s", errorMsg, string(body))
			}
		}
		return fmt.Errorf("%s", errorMsg)
	}

	return nil
}
