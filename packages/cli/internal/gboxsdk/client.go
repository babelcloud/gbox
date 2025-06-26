package gboxsdk

import (
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

	// When organization is local, we don't need API key
	if current.OrganizationName == "local" {
		base := strings.TrimSuffix(config.GetLocalAPIURL(), "/") + "/api/v1"
		client := sdk.NewClient(
			option.WithBaseURL(base),
		)
		return &client, nil
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
