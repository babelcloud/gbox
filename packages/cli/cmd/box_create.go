package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	model "github.com/babelcloud/gbox/packages/api-server/pkg/box"
	"github.com/babelcloud/gbox/packages/cli/config"
	"github.com/spf13/cobra"
)

type BoxCreateOptions struct {
	OutputFormat    string
	Image           string
	Env             []string
	Labels          []string
	WorkingDir      string
	Command         []string
	ImagePullSecret string
	Volumes         []string
}

type BoxCreateResponse struct {
	ID string `json:"id"`
}

func parseKeyValuePairs(pairs []string, pairType string) (map[string]string, error) {
	if len(pairs) == 0 {
		return nil, nil
	}

	result := make(map[string]string)
	for _, pair := range pairs {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		} else {
			return nil, fmt.Errorf("invalid %s format: %s (must be KEY=VALUE)", pairType, pair)
		}
	}
	return result, nil
}

// parseVolumes parses volume mount strings in the format "source:target[:ro][:propagation]"
func parseVolumes(volumes []string) ([]model.VolumeMount, error) {
	if len(volumes) == 0 {
		return nil, nil
	}

	result := make([]model.VolumeMount, 0, len(volumes))
	for _, volume := range volumes {
		parts := strings.Split(volume, ":")
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid volume format: %s (must be source:target[:ro][:propagation])", volume)
		}

		mount := model.VolumeMount{
			Source: parts[0],
			Target: parts[1],
		}

		// Parse optional flags
		for i := 2; i < len(parts); i++ {
			switch parts[i] {
			case "ro":
				mount.ReadOnly = true
			case "private", "rprivate", "shared", "rshared", "slave", "rslave":
				mount.Propagation = parts[i]
			default:
				return nil, fmt.Errorf("invalid volume option: %s", parts[i])
			}
		}

		result = append(result, mount)
	}

	return result, nil
}

func NewBoxCreateCommand() *cobra.Command {
	opts := &BoxCreateOptions{}

	cmd := &cobra.Command{
		Use:   "create [flags] -- [command] [args...]",
		Short: "Create a new box",
		Long: `Create a new box with various options for image, environment, and commands.

You can specify box configurations through various flags, including which container image to use,
setting environment variables, adding labels, and specifying a working directory.

Command arguments can be specified directly in the command line or added after the '--' separator.`,
		Example: `  gbox box create --image python:3.9 -- python3 -c 'print("Hello")'
  gbox box create --env PATH=/usr/local/bin:/usr/bin:/bin -w /app -- node server.js
  gbox box create --label project=myapp --label env=prod -- python3 server.py
  gbox box create --volumes /host/path:/container/path:ro:rprivate --image python:3.9`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreate(opts, args)
		},
		DisableFlagsInUseLine: true,
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.OutputFormat, "output", "text", "Output format (json or text)")
	flags.StringVar(&opts.Image, "image", "", "Container image to use")
	flags.StringArrayVar(&opts.Env, "env", []string{}, "Environment variables in KEY=VALUE format")
	flags.StringArrayVarP(&opts.Labels, "label", "l", []string{}, "Custom labels in KEY=VALUE format")
	flags.StringVarP(&opts.WorkingDir, "work-dir", "w", "", "Working directory")
	flags.StringArrayVarP(&opts.Volumes, "volume", "v", nil, "Bind mount a volume (source:target[:ro][:propagation])")

	cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"json", "text"}, cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

func runCreate(opts *BoxCreateOptions, args []string) error {
	request := model.BoxCreateParams{}

	request.Image = opts.Image
	request.ImagePullSecret = opts.ImagePullSecret

	envMap, err := parseKeyValuePairs(opts.Env, "environment variable")
	if err != nil {
		return err
	}
	request.Env = envMap

	labelMap, err := parseKeyValuePairs(opts.Labels, "label")
	if err != nil {
		return err
	}
	request.ExtraLabels = labelMap

	if opts.WorkingDir != "" {
		request.WorkingDir = opts.WorkingDir
	}

	// Parse volume mounts
	volumes, err := parseVolumes(opts.Volumes)
	if err != nil {
		return err
	}
	request.Volumes = volumes

	if len(opts.Command) > 0 {
		request.Cmd = opts.Command[0]
		if len(opts.Command) > 1 {
			request.Args = opts.Command[1:]
		}
	} else if len(args) > 0 {
		request.Cmd = args[0]
		if len(args) > 1 {
			request.Args = args[1:]
		}
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("unable to serialize request: %v", err)
	}

	if os.Getenv("DEBUG") == "true" {
		fmt.Fprintf(os.Stderr, "Request body:\n")
		var prettyJSON bytes.Buffer
		json.Indent(&prettyJSON, requestBody, "", "  ")
		fmt.Fprintln(os.Stderr, prettyJSON.String())
	}

	apiBase := config.GetLocalAPIURL()
	apiURL := fmt.Sprintf("%s/api/v1/boxes", strings.TrimSuffix(apiBase, "/"))

	// Create a new HTTP request
	httpRequest, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return fmt.Errorf("unable to create request: %v", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "application/json-stream") // Explicitly request streaming response

	// Send the request using the default client
	client := &http.Client{}
	resp, err := client.Do(httpRequest)
	if err != nil {
		return fmt.Errorf("unable to connect to API server: %v", err)
	}
	defer resp.Body.Close()

	// Check if response is a streaming response
	contentType := resp.Header.Get("Content-Type")
	if contentType == "application/json-stream" {
		return handleBoxCreateStreamingResponse(resp.Body, opts.OutputFormat)
	}

	// Handle standard (non-streaming) response for backward compatibility
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}

	if resp.StatusCode != 201 {
		errMsg := fmt.Sprintf("API server returned HTTP %d", resp.StatusCode)
		if len(responseBody) > 0 {
			errMsg += fmt.Sprintf("\nResponse: %s", string(responseBody))
		}
		return fmt.Errorf("%s", errMsg)
	}

	if opts.OutputFormat == "json" {
		fmt.Println(string(responseBody))
	} else {
		var response BoxCreateResponse
		if err := json.Unmarshal(responseBody, &response); err != nil {
			return fmt.Errorf("failed to parse response: %v", err)
		}
		fmt.Printf("Box created with ID \"%s\"\n", response.ID)
	}
	return nil
}

// handleBoxCreateStreamingResponse processes a streaming response with progress updates for box creation
func handleBoxCreateStreamingResponse(body io.Reader, outputFormat string) error {
	fmt.Println("Creating box...")

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
	var finalResponse []byte

	// Buffer to store the complete response for final parsing
	responseBuffer := new(bytes.Buffer)
	teeReader := io.TeeReader(body, responseBuffer)

	decoder := json.NewDecoder(teeReader)
	var box *model.Box

	for {
		var rawMessage json.RawMessage
		if err := decoder.Decode(&rawMessage); err != nil {
			if err == io.EOF {
				// At EOF, the responseBuffer contains the full response
				finalResponse = responseBuffer.Bytes()
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
				isNowCompleteEvent := (pullProgress.Status == "Download complete" || pullProgress.Status == "Pull complete")

				if isNowCompleteEvent {
					if !layer.Complete { // If it was not already marked as complete
						layer.Complete = true // Mark it as complete now
						completedLayers++     // Increment the count of completed layers
					} else {
						layer.Complete = true
					}
				}

				layers[pullProgress.ID] = layer

				// Update total count whenever we discover new layers
				if len(layers) > totalLayers {
					totalLayers = len(layers)
				}
			}

			// Update the display (but rate limit to avoid flicker)
			if time.Since(lastOutput) > 100*time.Millisecond || pullProgress.ID == "" {
				lastOutput = time.Now()

				// Only redraw if there's any layer information
				if len(layers) > 0 {
					displayImagePullProgress(layers, completedLayers, totalLayers)
				} else if pullProgress.Status != "" {
					fmt.Printf("\r\033[K%s", pullProgress.Status)
					if pullProgress.Progress != "" {
						fmt.Printf(" %s", pullProgress.Progress)
					}
				}
			}

			continue
		}

		// Try to parse as a custom status message
		var statusMessage struct {
			Status  string     `json:"status"`
			Message string     `json:"message"`
			Error   string     `json:"error"`
			Box     *model.Box `json:"box"`
		}

		if err := json.Unmarshal(rawMessage, &statusMessage); err == nil {
			if statusMessage.Error != "" {
				return fmt.Errorf("error from server: %s", statusMessage.Error)
			}

			if statusMessage.Status == "complete" && statusMessage.Box != nil {
				// Found the box info in the stream
				box = statusMessage.Box
				fmt.Printf("\r\033[KBox created successfully\n")
			} else if statusMessage.Status == "prepare" {
				fmt.Printf("\r\033[KPreparing: %s", statusMessage.Message)
			} else if statusMessage.Message != "" {
				fmt.Printf("\r\033[K%s: %s", statusMessage.Status, statusMessage.Message)
			}
		}
	}

	// Ensure we have a blank line after all the progress output
	fmt.Print("\n")

	// If box is still nil, try to parse the final response for box information
	if box == nil && len(finalResponse) > 0 {
		// Try to parse final response as a standard response first
		var createResponse BoxCreateResponse
		if err := json.Unmarshal(finalResponse, &createResponse); err == nil && createResponse.ID != "" {
			// Simple ID-only response
			if outputFormat == "json" {
				fmt.Println(string(finalResponse))
			} else {
				fmt.Printf("Box created with ID \"%s\"\n", createResponse.ID)
			}
			return nil
		}

		// Try parsing as complete box object
		var boxResponse struct {
			Box *model.Box `json:"box"`
		}
		if err := json.Unmarshal(finalResponse, &boxResponse); err == nil && boxResponse.Box != nil {
			box = boxResponse.Box
		} else {
			// Try parsing directly as a Box
			var directBox model.Box
			if err := json.Unmarshal(finalResponse, &directBox); err == nil && directBox.ID != "" {
				box = &directBox
			}
		}
	}

	// Output the final result
	if box != nil {
		if outputFormat == "json" {
			boxJSON, _ := json.MarshalIndent(box, "", "  ")
			fmt.Println(string(boxJSON))
		} else {
			fmt.Printf("Box created with ID \"%s\"\n", box.ID)
		}
	} else {
		fmt.Println("Box created successfully, but no details received")
	}

	return nil
}

// displayImagePullProgress prints the current download progress for all layers
func displayImagePullProgress(layers map[string]struct {
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
				shortenImageLayerID(layer.ID),
				layer.Status)
		} else {
			fmt.Printf("\r\033[K[↓] %s: %s %s\n",
				shortenImageLayerID(layer.ID),
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

// shortenImageLayerID shortens a layer ID for display
func shortenImageLayerID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:12]
}
