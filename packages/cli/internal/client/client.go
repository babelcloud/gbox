package gboxsdk

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	sdk "github.com/babelcloud/gbox-sdk-go"
	"github.com/babelcloud/gbox-sdk-go/option"
	"github.com/babelcloud/gbox/packages/cli/internal/profile"
)

// BoxInfo represents a simplified box information structure for CLI usage
type BoxInfo struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Status     string `json:"status"`
	Image      string `json:"image,omitempty"`
	DeviceType string `json:"deviceType,omitempty"`
	// Add other fields as needed
}

// NewClientFromProfile reads the profile file, selects the profile with
// `current` set to true and constructs a gbox-sdk-go Client.
//
// If the active profile's organization is "local" then the client will be
// created without an API key.
func NewClientFromProfile() (*sdk.Client, error) {
	// Get effective base URL and API key using profile manager
	baseURL := profile.Default.GetEffectiveBaseURL()
	apiKey, err := profile.Default.GetEffectiveAPIKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}

	// Debug output
	if os.Getenv("DEBUG") == "true" {
		fmt.Fprintf(os.Stderr, "Base URL: %s\n", baseURL)
		// Use profile manager to get masked API key for display
		pm := profile.NewProfileManager()
		if current := profile.Default.GetCurrent(); current != nil {
			maskedKey := pm.GetMaskedAPIKey(current.APIKey)
			fmt.Fprintf(os.Stderr, "API key: %s\n", maskedKey)
		}
	}

	client := sdk.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(baseURL),
	)
	return &client, nil
}

// parseKeyValuePairs parses a slice of strings in KEY=VALUE format into a map
func parseKeyValuePairs(pairs []string, pairType string) (map[string]interface{}, error) {
	if len(pairs) == 0 {
		return nil, nil
	}

	result := make(map[string]interface{})
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

// CreateAndroidBox creates a new Android box using the SDK
func CreateAndroidBox(client *sdk.Client, deviceType string, env []string, labels []string, expiresIn string) (*sdk.AndroidBox, error) {
	// parse environment variables
	envMap, err := parseKeyValuePairs(env, "environment variable")
	if err != nil {
		return nil, err
	}

	// parse labels
	labelMap, err := parseKeyValuePairs(labels, "label")
	if err != nil {
		return nil, err
	}
	if labelMap == nil {
		labelMap = make(map[string]interface{})
	}
	labelMap["device_type"] = deviceType

	// build SDK parameters
	createParams := sdk.V1BoxNewAndroidParams{
		CreateAndroidBox: sdk.CreateAndroidBoxParam{
			Wait: sdk.Bool(true), // wait for operation to complete
			Config: sdk.CreateBoxConfigParam{
				ExpiresIn:  sdk.String(expiresIn),
				Envs:       envMap,
				Labels:     labelMap,
				DeviceType: sdk.CreateBoxConfigDeviceType(deviceType),
			},
		},
	}

	// debug output
	if os.Getenv("DEBUG") == "true" {
		fmt.Fprintf(os.Stderr, "Request params:\n")
		requestJSON, _ := json.MarshalIndent(createParams, "", "  ")
		fmt.Fprintln(os.Stderr, string(requestJSON))
	}

	// call SDK
	ctx := context.Background()
	box, err := client.V1.Boxes.NewAndroid(ctx, createParams)
	if err != nil {
		return nil, fmt.Errorf("failed to create Android box: %v", err)
	}

	return box, nil
}

// CreateLinuxBox creates a new Linux box using the SDK
func CreateLinuxBox(client *sdk.Client, env []string, labels []string) (*sdk.LinuxBox, error) {
	// parse environment variables
	envMap, err := parseKeyValuePairs(env, "environment variable")
	if err != nil {
		return nil, err
	}

	// parse labels
	labelMap, err := parseKeyValuePairs(labels, "label")
	if err != nil {
		return nil, err
	}

	// build SDK parameters
	createParams := sdk.V1BoxNewLinuxParams{
		CreateLinuxBox: sdk.CreateLinuxBoxParam{
			Wait: sdk.Bool(true), // wait for operation to complete
			Config: sdk.CreateBoxConfigParam{
				Envs:   envMap,
				Labels: labelMap,
			},
		},
	}

	// debug output
	if os.Getenv("DEBUG") == "true" {
		fmt.Fprintf(os.Stderr, "Request params:\n")
		requestJSON, _ := json.MarshalIndent(createParams, "", "  ")
		fmt.Fprintln(os.Stderr, string(requestJSON))
	}

	// call SDK
	ctx := context.Background()
	box, err := client.V1.Boxes.NewLinux(ctx, createParams)
	if err != nil {
		return nil, fmt.Errorf("failed to create Linux box: %v", err)
	}

	return box, nil
}

// ListBoxes lists all boxes using the SDK
func ListBoxes(client *sdk.Client, filters []string) (*sdk.V1BoxListResponse, error) {
	// build list parameters
	params := buildListParams(filters)

	// call SDK
	ctx := context.Background()
	resp, err := client.V1.Boxes.List(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list boxes: %v", err)
	}

	return resp, nil
}

// ListBoxesData extracts box data from the SDK response and returns a slice of BoxInfo
// This method handles the data parsing logic that was previously done in cmd layer
func ListBoxesData(client *sdk.Client, filters []string) ([]BoxInfo, error) {
	resp, err := ListBoxes(client, filters)
	if err != nil {
		return nil, err
	}

	// Extract boxes data from response
	var boxesData []map[string]interface{}
	if rawBytes, _ := json.Marshal(resp); rawBytes != nil {
		var raw struct {
			Data []map[string]interface{} `json:"data"`
		}
		if err := json.Unmarshal(rawBytes, &raw); err != nil {
			return nil, fmt.Errorf("failed to parse response data: %v", err)
		}
		boxesData = raw.Data
	}

	// Convert to typed BoxInfo slice
	var boxes []BoxInfo
	for _, box := range boxesData {
		boxInfo := BoxInfo{}

		if id, ok := box["id"].(string); ok {
			boxInfo.ID = id
		}
		if boxType, ok := box["type"].(string); ok {
			boxInfo.Type = boxType
		}
		if status, ok := box["status"].(string); ok {
			boxInfo.Status = status
		}
		if image, ok := box["image"].(string); ok {
			boxInfo.Image = image
		}

		// Extract deviceType from config.deviceType path
		if config, ok := box["config"].(map[string]interface{}); ok {
			if deviceType, ok := config["deviceType"].(string); ok {
				boxInfo.DeviceType = deviceType
			}
		}

		boxes = append(boxes, boxInfo)
	}

	return boxes, nil
}

// ListBoxesRawData extracts raw box data as map[string]interface{} for backward compatibility
// This method provides the same data structure that was previously used in cmd layer
func ListBoxesRawData(client *sdk.Client, filters []string) ([]map[string]interface{}, error) {
	resp, err := ListBoxes(client, filters)
	if err != nil {
		return nil, err
	}

	// Extract boxes data from response
	var boxesData []map[string]interface{}
	if rawBytes, _ := json.Marshal(resp); rawBytes != nil {
		var raw struct {
			Data []map[string]interface{} `json:"data"`
		}
		if err := json.Unmarshal(rawBytes, &raw); err != nil {
			return nil, fmt.Errorf("failed to parse response data: %v", err)
		}
		boxesData = raw.Data
	}

	return boxesData, nil
}

// buildListParams parses CLI --filter flags into SDK query parameters
func buildListParams(filters []string) sdk.V1BoxListParams {
	var params sdk.V1BoxListParams
	for _, filter := range filters {
		parts := strings.SplitN(filter, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, value := parts[0], parts[1]
		switch strings.ToLower(key) {
		case "label", "labels":
			params.Labels = value
		case "status":
			params.Status = strings.Split(value, ",")
		case "type":
			params.Type = strings.Split(value, ",")
		}
	}
	return params
}

// TerminateBox terminates a box using the SDK
func TerminateBox(client *sdk.Client, boxID string) error {
	// build SDK parameters
	terminateParams := sdk.V1BoxTerminateParams{}

	// debug output
	if os.Getenv("DEBUG") == "true" {
		fmt.Fprintf(os.Stderr, "Terminating box: %s\n", boxID)
	}

	// call SDK
	ctx := context.Background()
	err := client.V1.Boxes.Terminate(ctx, boxID, terminateParams)
	if err != nil {
		return fmt.Errorf("failed to terminate box: %v", err)
	}

	return nil
}

// GetBox gets a box by ID using the SDK
func GetBox(client *sdk.Client, boxID string) (*sdk.V1BoxGetResponseUnion, error) {
	// call SDK
	ctx := context.Background()
	box, err := client.V1.Boxes.Get(ctx, boxID)
	if err != nil {
		return nil, fmt.Errorf("failed to get box: %v", err)
	}

	return box, nil
}
