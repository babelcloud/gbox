package gboxsdk

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	sdk "github.com/babelcloud/gbox-sdk-go"
	"github.com/babelcloud/gbox-sdk-go/option"
	"github.com/babelcloud/gbox/packages/cli/config"
)

// profile represents a single entry in the profile file.
// We keep the structure in sync with the CLI `profile` command.
// Only the fields we care about are defined.
type profile struct {
	APIKey           string `json:"api_key"`
	Name             string `json:"name"`
	OrganizationName string `json:"organization_name"`
	Current          bool   `json:"current"`
}

// NewClientFromProfile reads the profile file, selects the profile with
// `current` set to true and constructs a gbox-sdk-go Client.
//
// If the active profile's organization is "local" then the client will be
// created without an API key.
func NewClientFromProfile() (*sdk.Client, error) {
	// Environment variable takes precedence: if API_ENDPOINT is set, use it directly
	if endpoint := os.Getenv("API_ENDPOINT"); endpoint != "" {
		base := strings.TrimSuffix(endpoint, "/") + "/api/v1"
		client := sdk.NewClient(option.WithBaseURL(base))
		return &client, nil
	}

	// Get profile file path from config
	profilePath := config.GetProfilePath()

	data, err := os.ReadFile(profilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read profile file (%s): %w", profilePath, err)
	}

	var profiles []profile
	if err := json.Unmarshal(data, &profiles); err != nil {
		return nil, fmt.Errorf("failed to parse profile file: %w", err)
	}

	var current *profile
	for i, p := range profiles {
		if p.Current {
			current = &profiles[i]
			break
		}
	}

	if current == nil {
		return nil, fmt.Errorf("no current profile found in %s", profilePath)
	}

	if current.APIKey == "" {
		return nil, fmt.Errorf("current profile does not hold an api_key")
	}

	base := strings.TrimSuffix(config.GetCloudAPIURL(), "/") + "/api/v1"
	client := sdk.NewClient(
		option.WithAPIKey(current.APIKey),
		option.WithBaseURL(base),
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
func CreateAndroidBox(client *sdk.Client, deviceType string, env []string, labels []string, expiresIn string) (interface{}, error) {
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
func CreateLinuxBox(client *sdk.Client, env []string, labels []string) (interface{}, error) {
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
func ListBoxes(client *sdk.Client, filters []string) (interface{}, error) {
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
	// 构建 SDK 参数
	terminateParams := sdk.V1BoxTerminateParams{}

	// 调试输出
	if os.Getenv("DEBUG") == "true" {
		fmt.Fprintf(os.Stderr, "Terminating box: %s\n", boxID)
	}

	// 调用 SDK
	ctx := context.Background()
	err := client.V1.Boxes.Terminate(ctx, boxID, terminateParams)
	if err != nil {
		return fmt.Errorf("failed to terminate box: %v", err)
	}

	return nil
}

// GetBox gets a box by ID using the SDK
func GetBox(client *sdk.Client, boxID string) (interface{}, error) {
	// call SDK
	ctx := context.Background()
	box, err := client.V1.Boxes.Get(ctx, boxID)
	if err != nil {
		return nil, fmt.Errorf("failed to get box: %v", err)
	}

	return box, nil
}
