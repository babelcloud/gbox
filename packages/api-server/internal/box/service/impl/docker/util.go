package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/babelcloud/gbox/packages/api-server/config"
	"github.com/babelcloud/gbox/packages/api-server/internal/box/service"
	model "github.com/babelcloud/gbox/packages/api-server/pkg/box"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
)

const (
	// Label keys
	labelPrefix    = "gbox"
	labelID        = labelPrefix + ".id"
	labelName      = labelPrefix + ".name"
	labelInstance  = labelPrefix + ".instance"
	labelNamespace = labelPrefix + ".namespace"
	labelVersion   = labelPrefix + ".version"
	labelComponent = labelPrefix + ".component"
	labelManagedBy = labelPrefix + ".managed-by"

	DefaultImage = "ubuntu:latest"
)

// containerName returns the docker container name for a given box ID.
func containerName(id string) string {
	// This needs to match the naming convention used in Create
	return fmt.Sprintf("gbox-%s", id)
}

// getContainerByID gets a container by box ID
func (s *Service) getContainerByID(ctx context.Context, id string) (*types.Container, error) {
	if id == "" {
		return nil, fmt.Errorf("box ID is required")
	}

	// Build filter for the specific box ID
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", fmt.Sprintf("%s=%s", labelID, id))

	boxes, err := s.client.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filterArgs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	if len(boxes) == 0 {
		return nil, service.ErrBoxNotFound
	}

	return &boxes[0], nil
}

// inspectContainerByID gets detailed container info by box ID using ContainerInspect
func (s *Service) inspectContainerByID(ctx context.Context, id string) (types.ContainerJSON, error) {
	if id == "" {
		return types.ContainerJSON{}, fmt.Errorf("box ID is required")
	}
	containerJSON, err := s.client.ContainerInspect(ctx, containerName(id))
	if err != nil {
		// Reuse the same error handling logic
		return types.ContainerJSON{}, handleContainerError(err, id)
	}
	return containerJSON, nil
}

// handleContainerError converts Docker client errors to service-level errors.
// Defined here temporarily for linting, should be in util.go ideally
func handleContainerError(err error, id string) error {
	if err == nil {
		return nil
	}
	// Example: Check for "not found" type errors
	if strings.Contains(strings.ToLower(err.Error()), "no such container") {
		return fmt.Errorf("box %s not found: %w", id, service.ErrBoxNotFound)
	}
	// Return a generic error for other cases
	return fmt.Errorf("docker error for box %s: %w", id, err)
}

// containerToBox converts a Docker container to a Box
func containerToBox(c interface{}) *model.Box {
	var id, status, image string
	var labels map[string]string
	var env []string
	var createdAt time.Time

	switch c := c.(type) {
	case types.ContainerJSON:
		id = c.Config.Labels[labelID]
		status = mapContainerState(c.State.Status)
		image = c.Config.Image
		labels = c.Config.Labels
		env = c.Config.Env
		if t, err := time.Parse(time.RFC3339, c.Created); err == nil {
			createdAt = t
		}
	case types.Container:
		id = c.Labels[labelID]
		status = mapContainerState(c.State)
		image = c.Image
		labels = c.Labels
		createdAt = time.Unix(c.Created, 0)
	case *types.Container:
		id = c.Labels[labelID]
		status = mapContainerState(c.State)
		image = c.Image
		labels = c.Labels
		createdAt = time.Unix(c.Created, 0)
	}

	// --- Restored Original logic ---
	// Extract extra labels (exclude internal labels and strip prefix)
	extraLabels := make(map[string]string)
	// Define the prefix to strip. Ensure this matches the prefix used in PrepareLabels.
	const prefixToStrip = labelPrefix + ".extra." // Should evaluate to "gbox.extra."

	for k, v := range labels { // 'labels' comes from the container info
		// Exclude specific internal labels used for identification/management
		if k == labelID || k == labelName || k == labelInstance || k == labelManagedBy || k == labelComponent || k == labelNamespace || k == labelVersion {
			continue
		}

		// Check if the key has the extra label prefix and remove it
		if strings.HasPrefix(k, prefixToStrip) {
			originalKey := strings.TrimPrefix(k, prefixToStrip)
			// Prevent adding empty keys if the original label was just the prefix
			if originalKey != "" {
				extraLabels[originalKey] = v
			}
		} else {
			// Do nothing for labels that don't have the prefix and are not internal gbox labels.
			// This effectively filters out labels like desktop.docker.io/*
		}
	}
	// --- End Restored Original logic ---

	// Parse environment variables to map
	envMap := make(map[string]string)
	for _, envVar := range env {
		if parts := strings.SplitN(envVar, "=", 2); len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	// Extract working directory from labels if available
	workingDir := ""
	if wd, exists := labels["gbox.working_dir"]; exists {
		workingDir = wd
	}

	// Parse expires_in from labels to set ExpiresAt
	var expiresAt time.Time
	if expiresIn, exists := labels["gbox.expires_in"]; exists && expiresIn != "" {
		// Try to parse expires_in as duration and add to CreatedAt
		if duration, err := time.ParseDuration(expiresIn); err == nil {
			expiresAt = createdAt.Add(duration)
		}
	}

	// TODO: better way to determine box type
	boxType := "linux" // default
	if t, exists := labels["gbox.type"]; exists {
		boxType = t
	} else if strings.Contains(image, "android") {
		boxType = "android"
	}

	// Use current time as UpdatedAt (could be enhanced to track actual updates)
	updatedAt := time.Now()

	return &model.Box{
		ID:          id,
		Status:      status,
		Image:       image,
		CreatedAt:   createdAt,
		ExpiresAt:   expiresAt,
		Type:        boxType,
		UpdatedAt:   updatedAt,
		ExtraLabels: extraLabels,
		Config: model.LinuxAndroidBoxConfig{
			Envs:       envMap,
			Labels:     extraLabels, // Use the cleaned extra labels
			WorkingDir: workingDir,
			// TODO: extract from container limits if available
			CPU:     0.0,
			Memory:  0.0,
			Storage: 0.0,
			Browser: model.LinuxAndroidBoxConfigBrowser{
				Type:    "",
				Version: "",
			},
			Os: model.LinuxAndroidBoxConfigOs{
				Version: "", // Could be detected from image
			},
			Resolution: model.LinuxAndroidBoxConfigResolution{
				Width:  1920, // Default resolution
				Height: 1080,
			},
		},
	}
}

// mapContainerState maps Docker container states to Box states
func mapContainerState(state string) string {
	switch state {
	case "running":
		return "running"
	case "created":
		return "created"
	case "restarting":
		return "restarting"
	case "removing":
		return "removing"
	case "paused":
		return "paused"
	case "exited":
		return "stopped"
	case "dead":
		return "dead"
	default:
		return "unknown"
	}
}

func PrepareLabels(boxID string, p *model.BoxCreateParams) map[string]string {
	labels := map[string]string{
		labelID:        boxID,
		labelName:      "gbox",
		labelInstance:  fmt.Sprintf("gbox-%s", boxID),
		labelNamespace: config.GetInstance().Cluster.Namespace,
		labelVersion:   "v1",
		labelComponent: "sandbox",
		labelManagedBy: "gru-api-server",

		// Add standard Docker Compose labels for UI grouping
		"com.docker.compose.project": config.GetInstance().Cluster.Namespace,
		"com.docker.compose.service": boxID,
		"com.docker.compose.oneoff":  "False",
	}

	// Add command configuration to labels if provided
	if p.Cmd != "" {
		labels[labelPrefix+".cmd"] = p.Cmd
	}
	if len(p.Args) > 0 {
		labels[labelPrefix+".args"] = JoinArgs(p.Args)
	}
	if p.WorkingDir != "" {
		labels[labelPrefix+".working-dir"] = p.WorkingDir
	}

	// Add custom labels with prefix
	if p.ExtraLabels != nil {
		for k, v := range p.ExtraLabels {
			labels[labelPrefix+".extra."+k] = v
		}
	}

	return labels
}

// JoinArgs converts a string array to a JSON string
func JoinArgs(args []string) string {
	if len(args) == 0 {
		return ""
	}
	// Convert args array to JSON string to preserve spaces and special characters
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return ""
	}
	return string(argsJSON)
}

// GetCommand returns the command to run, falling back to default if none specified
func GetCommand(cmd string, args []string) []string {
	if cmd == "" {
		return []string{"sleep", "infinity"}
	}
	if len(args) == 0 {
		// If no args provided, use shell to parse the command string
		return []string{"/bin/sh", "-c", cmd}
	}
	// If args are provided, use direct command array
	return append([]string{cmd}, args...)
}

// GetEnvVars converts environment variables map to string slice
func GetEnvVars(env map[string]string) []string {
	if env == nil {
		return nil
	}

	vars := make([]string, 0, len(env))
	for k, v := range env {
		vars = append(vars, fmt.Sprintf("%s=%s", k, v))
	}
	return vars
}

// EnsureImageTag ensures an image name has a tag, using various fallback strategies
func EnsureImageTag(image string) string {
	// Handle empty input - return empty to let GetImage handle defaults
	if image == "" {
		return ""
	}

	// If already has tag, return as-is
	if strings.Contains(image, ":") {
		return image
	}

	// Try to get tag from config/env vars
	if resolvedWithTag := config.CheckImageTag(image); strings.Contains(resolvedWithTag, ":") {
		return resolvedWithTag
	}

	// Fallback to latest
	return image + ":latest"
}

func GetImage(image string) string {
	// Handle empty input: use default image with configured tag
	if image == "" {
		defaultImageTag := config.GetDefaultImageTag()
		if defaultImageTag == "" {
			defaultImageTag = "latest"
		}
		return "babelcloud/gbox-playwright:" + defaultImageTag
	}

	// Use unified tag handling for non-empty inputs
	ensuredImage := EnsureImageTag(image)
	if ensuredImage == "" {
		// If EnsureImageTag returned empty, apply default logic
		defaultImageTag := config.GetDefaultImageTag()
		if defaultImageTag == "" {
			defaultImageTag = "latest"
		}
		return "babelcloud/gbox-playwright:" + defaultImageTag
	}
	return ensuredImage
}

// MapToEnv converts a map of environment variables to a slice of "key=value" strings
func MapToEnv(env map[string]string) []string {
	if env == nil {
		return nil
	}
	result := make([]string, 0, len(env))
	for k, v := range env {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}
	return result
}

// WaitForResponse reads from a reader until EOF and returns any error encountered
func WaitForResponse(reader io.Reader) ([]byte, error) {
	var buf []byte
	decoder := json.NewDecoder(reader)
	for {
		var response struct {
			Status   string `json:"status"`
			Error    string `json:"error"`
			Progress string `json:"progress"`
		}
		if err := decoder.Decode(&response); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if response.Error != "" {
			return nil, fmt.Errorf("%s", response.Error)
		}
		buf = append(buf, []byte(response.Status+"\n")...)
	}
	return buf, nil
}

// ProcessPullProgress reads Docker pull progress from reader and writes to the writer
// Returns error if encountered
func ProcessPullProgress(reader io.Reader, writer io.Writer) error {
	decoder := json.NewDecoder(reader)
	encoder := json.NewEncoder(writer)

	for {
		var response struct {
			Status         string          `json:"status"`
			ProgressDetail json.RawMessage `json:"progressDetail"`
			ID             string          `json:"id,omitempty"`
			Error          string          `json:"error,omitempty"`
			Progress       string          `json:"progress,omitempty"`
		}

		if err := decoder.Decode(&response); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		if response.Error != "" {
			return fmt.Errorf("%s", response.Error)
		}

		// Send progress to client
		if err := encoder.Encode(response); err != nil {
			return err
		}

		// Flush the writer if it's a flusher
		if f, ok := writer.(http.Flusher); ok {
			f.Flush()
		}
	}

	return nil
}
