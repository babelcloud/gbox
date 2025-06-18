package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	model "github.com/babelcloud/gbox/packages/api-server/pkg/box"
	"github.com/babelcloud/gbox/packages/cli/config"
	"github.com/spf13/cobra"
)

// BoxImageUpdateOptions holds flags for the image update command
type BoxImageUpdateOptions struct {
	ImageReference string
	DryRun         bool
	OutputFormat   string
	Force          bool
}

// NewBoxImageCommand returns the parent command for all box image operations
func NewBoxImageCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "image",
		Short: "Manage box container images",
		Long:  `Operations related to Docker images used by boxes, such as updating, listing, or pruning images.`,
	}

	cmd.AddCommand(NewBoxImageUpdateCommand())
	return cmd
}

// NewBoxImageUpdateCommand returns the command for updating box container images
func NewBoxImageUpdateCommand() *cobra.Command {
	opts := &BoxImageUpdateOptions{}

	cmd := &cobra.Command{
		Use:   "update [flags]",
		Short: "Update box container images",
		Long: `Update Docker images used by boxes.

This command will check if the specified image (or default box image if none provided) 
is up-to-date locally, pulls the new version if needed, and removes outdated versions 
to free up disk space. 

Use the --dry-run flag to see which operations would be performed without actually 
executing them.
Use the --force flag to forcefully remove old images, even if they are used by stopped containers.`,
		Example: `  # Update the default box image
  gbox box image update

  # Update a specific image
  gbox box image update --name node:18

  # Preview operations without executing them
  gbox box image update --dry-run

  # Force update and removal of old images
  gbox box image update --force

  # Update a specific image in dry run mode with JSON output
  gbox box image update --name python:3.10 --dry-run --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runImageUpdate(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.ImageReference, "image", "", "Image name to update (default: babelcloud/gbox-playwright)")
	flags.BoolVar(&opts.DryRun, "dry-run", false, "Only print what would be done, without executing operations")
	flags.StringVar(&opts.OutputFormat, "output", "text", "Output format (json or text)")
	flags.BoolVar(&opts.Force, "force", false, "Force removal of old images")

	cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"json", "text"}, cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

// runImageUpdate executes the image update operation
func runImageUpdate(opts *BoxImageUpdateOptions) error {
	// Build API URL with query parameters
	apiBase := config.GetLocalAPIURL()
	apiURL := fmt.Sprintf("%s/api/v1/boxes/images/update", strings.TrimSuffix(apiBase, "/"))

	// Add query parameters if provided
	queryParams := url.Values{}
	if opts.ImageReference != "" {
		queryParams.Add("image", opts.ImageReference)
	}
	if opts.DryRun {
		queryParams.Add("dryRun", "true")
	}
	if opts.Force {
		queryParams.Add("force", "true")
	}

	// Always request stream response
	queryParams.Add("stream", "true")

	// Append query parameters to URL if any
	if len(queryParams) > 0 {
		apiURL = fmt.Sprintf("%s?%s", apiURL, queryParams.Encode())
	}

	// Create request
	req, err := http.NewRequest("POST", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	// Set Accept header to request json-stream response
	req.Header.Set("Accept", "application/json-stream")
	req.Header.Set("Content-Type", "application/json")

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("unable to connect to API server: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		responseBody, _ := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("API server returned HTTP %d", resp.StatusCode)
		if len(responseBody) > 0 {
			errMsg += fmt.Sprintf("\nResponse: %s", string(responseBody))
		}
		return fmt.Errorf("%s", errMsg)
	}

	// Process response using streaming handler
	return handleStreamingResponse(resp.Body, opts)
}

// handleStreamingResponse processes a streaming response with progress updates
func handleStreamingResponse(body io.Reader, opts *BoxImageUpdateOptions) error {
	// Store the layers being downloaded and their progress
	layers := make(map[string]struct {
		ID       string
		Status   string
		Progress string
		Complete bool
	})

	var lastOutput time.Time
	var totalLayers int
	var completedLayers int
	decoder := json.NewDecoder(body)
	var finalResponse *model.ImageUpdateResponse

	for {
		var rawMessage json.RawMessage
		if err := decoder.Decode(&rawMessage); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("error reading response stream: %v", err)
		}

		// Try to parse as a Docker pull progress message
		var pullProgress struct {
			ID             string          `json:"id"`
			Status         string          `json:"status"`
			Progress       string          `json:"progress"`
			ProgressDetail json.RawMessage `json:"progressDetail"`
			Error          string          `json:"error"`
		}

		if err := json.Unmarshal(rawMessage, &pullProgress); err == nil {
			if pullProgress.Error != "" {
				return fmt.Errorf("error from server: %s", pullProgress.Error)
			}

			// Update progress for this layer
			if pullProgress.ID != "" {
				// Update or add this layer
				layer := layers[pullProgress.ID]
				layer.ID = pullProgress.ID
				layer.Status = pullProgress.Status

				if pullProgress.Progress != "" {
					layer.Progress = pullProgress.Progress
				}

				// Check if layer is complete
				if pullProgress.Status == "Download complete" || pullProgress.Status == "Pull complete" {
					layer.Complete = true
					completedLayers++
				}

				layers[pullProgress.ID] = layer

				// Update total count for first time
				if totalLayers == 0 && len(layers) > 0 {
					totalLayers = len(layers)
				}
			}

			// Update the display (but rate limit to avoid flicker)
			if time.Since(lastOutput) > 100*time.Millisecond || pullProgress.ID == "" {
				lastOutput = time.Now()

				// Only redraw if there's any layer information
				if len(layers) > 0 {
					printProgress(layers, completedLayers, totalLayers)
				} else if pullProgress.ID == "" && pullProgress.Status != "" &&
					!(strings.EqualFold(pullProgress.Status, "prepare") ||
						strings.EqualFold(pullProgress.Status, "preparing") ||
						strings.EqualFold(pullProgress.Status, "waiting")) {
					// Only print general status if it's not a layer update and not one of the ignored preliminary statuses.
					fmt.Printf("\r%s", pullProgress.Status)
					if pullProgress.Progress != "" {
						fmt.Printf(" %s", pullProgress.Progress)
					}
					fmt.Println()
				}
			}

			continue
		}

		// Try to parse as a final response
		var response model.ImageUpdateResponse
		if err := json.Unmarshal(rawMessage, &response); err == nil {
			// Store the final response to display after the stream closes
			finalResponse = &response
			continue
		}

		// Try to parse as a custom status message
		var statusMessage struct {
			Status  string `json:"status"`
			Message string `json:"message"`
			Error   string `json:"error"`
		}

		if err := json.Unmarshal(rawMessage, &statusMessage); err == nil {
			if statusMessage.Error != "" {
				return fmt.Errorf("error from server: %s", statusMessage.Error)
			}

			if statusMessage.Status == "complete" {
				fmt.Printf("\rCompleted: %s\n", statusMessage.Message)
			} else if statusMessage.Status == "prepare" {
				// Do nothing for "prepare" status to avoid confusing output
			} else {
				fmt.Printf("\r%s: %s\n", statusMessage.Status, statusMessage.Message)
			}
		}
	}

	// Ensure we have a blank line after all the progress output
	fmt.Print("\n")

	// If we got a final response, display it
	if finalResponse != nil {
		outputResults(finalResponse, opts.OutputFormat)
	} else {
		// If we didn't get a final response in the stream, fetch the results separately
		finalResults, err := fetchFinalResults(opts)
		if err != nil {
			fmt.Printf("Warning: Could not fetch final results: %v\n", err)
		} else if finalResults != nil {
			outputResults(finalResults, opts.OutputFormat)
		}
	}

	return nil
}

// fetchFinalResults makes a separate API call to get the current image status
func fetchFinalResults(opts *BoxImageUpdateOptions) (*model.ImageUpdateResponse, error) {
	// Build API URL with query parameters but without streaming
	apiBase := config.GetLocalAPIURL()
	apiURL := fmt.Sprintf("%s/api/v1/boxes/images/update", strings.TrimSuffix(apiBase, "/"))

	// Add query parameters if provided
	queryParams := url.Values{}
	if opts.ImageReference != "" {
		queryParams.Add("image", opts.ImageReference)
	}
	queryParams.Add("dryRun", "true") // Use dry run to just get status without changes

	// Append query parameters to URL
	apiURL = fmt.Sprintf("%s?%s", apiURL, queryParams.Encode())

	// Make the request
	resp, err := http.Post(apiURL, "application/json", nil)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to API server: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API server returned HTTP %d", resp.StatusCode)
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	// Parse the response
	var response model.ImageUpdateResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	return &response, nil
}

// printProgress prints the current download progress for all layers
func printProgress(layers map[string]struct {
	ID       string
	Status   string
	Progress string
	Complete bool
}, completed, total int) {
	// Clear line and move to beginning
	fmt.Print("\r\033[K")

	// Show overall progress
	if total > 0 {
		percentage := (float64(completed) / float64(total)) * 100
		fmt.Printf("Overall progress: [%d/%d] %.1f%%\n", completed, total, percentage)
	}

	// Limit how many layers we display to avoid cluttering the screen
	const maxDisplayLayers = 5

	// Convert map to slice for better display control
	layerSlice := make([]struct {
		ID       string
		Status   string
		Progress string
		Complete bool
	}, 0, len(layers))

	for _, layer := range layers {
		layerSlice = append(layerSlice, layer)
	}

	// Prioritize incomplete layers
	sort.Slice(layerSlice, func(i, j int) bool {
		if layerSlice[i].Complete != layerSlice[j].Complete {
			return !layerSlice[i].Complete // Incomplete layers first
		}
		return layerSlice[i].ID < layerSlice[j].ID // Then sort by ID
	})

	// Display up to maxDisplayLayers layers
	displayCount := len(layerSlice)
	if displayCount > maxDisplayLayers {
		displayCount = maxDisplayLayers
	}

	for i := 0; i < displayCount; i++ {
		layer := layerSlice[i]
		if layer.Complete {
			fmt.Printf("\r\033[K[✓] %s: %s\n",
				shortenLayerID(layer.ID),
				layer.Status)
		} else {
			fmt.Printf("\r\033[K[↓] %s: %s %s\n",
				shortenLayerID(layer.ID),
				layer.Status,
				layer.Progress)
		}
	}

	// If we're not showing all layers, indicate how many more there are
	if len(layerSlice) > maxDisplayLayers {
		remaining := len(layerSlice) - maxDisplayLayers
		fmt.Printf("\r\033[K... and %d more layers ...\n", remaining)
	}

	// Move cursor back up to overwrite on next update
	moveCursorUp := displayCount + 1 // +1 for the overall progress line
	if len(layerSlice) > maxDisplayLayers {
		moveCursorUp++ // +1 for the "and X more" line
	}

	for i := 0; i < moveCursorUp; i++ {
		fmt.Print("\033[1A")
	}
}

// shortenLayerID shortens a layer ID for display
func shortenLayerID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:12]
}

// outputResults displays the results of the image update operation
func outputResults(response *model.ImageUpdateResponse, outputFormat string) {
	if outputFormat == "json" {
		// Pretty print JSON
		jsonData, _ := json.MarshalIndent(response, "", "  ")
		fmt.Println(string(jsonData))
	} else {
		// Text format
		if response.ErrorMessage != "" {
			fmt.Printf("Error: %s\n\n", response.ErrorMessage)
		}

		if len(response.Images) == 0 {
			if response.ErrorMessage == "" {
				fmt.Println("No images found or processed.")
			}
			return
		}

		// Print table header
		fmt.Printf("%-50s | %-15s | %-10s\n", "Image", "Status", "Action")
		fmt.Printf("%-50s-+-%-15s-+-%-10s\n", strings.Repeat("-", 50), strings.Repeat("-", 15), strings.Repeat("-", 10))

		// Print table rows
		for _, img := range response.Images {
			imageName := fmt.Sprintf("%s:%s", img.Repository, img.Tag)
			var status string

			switch img.Status {
			case model.ImageStatusUpToDate:
				status = "Up-to-date"
			case model.ImageStatusMissing:
				status = "Missing"
			case model.ImageStatusOutdated:
				status = "Outdated"
			default:
				status = string(img.Status)
			}

			fmt.Printf("%-50s | %-15s | %-10s\n", imageName, status, img.Action)
		}
	}
}
