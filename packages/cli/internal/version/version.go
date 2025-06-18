package version

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"time"

	"github.com/babelcloud/gbox/packages/cli/config"
)

// These variables will be set at build time via -ldflags
var (
	// Version represents the application version (from git tags)
	Version = "dev"
	// BuildTime is the time when the binary was built
	BuildTime = "unknown"
	// CommitID is the git commit hash
	CommitID = "unknown"
)

// formatBuildTime returns a nicely formatted build time
func formatBuildTime() string {
	if BuildTime == "unknown" {
		return BuildTime
	}

	t, err := time.Parse(time.RFC3339, BuildTime)
	if err != nil {
		return BuildTime
	}

	return t.Format("Mon Jan 2 15:04:05 2006")
}

// ClientInfo returns structured client version information
func ClientInfo() map[string]string {
	return map[string]string{
		"Version":       Version,
		"APIVersion":    "v1",
		"GoVersion":     runtime.Version(),
		"GitCommit":     CommitID,
		"BuildTime":     BuildTime,
		"FormattedTime": formatBuildTime(),
		"OS":            runtime.GOOS,
		"Arch":          runtime.GOARCH,
	}
}

// serverInfoResponse defines the structure expected from the API server's version endpoint
type serverInfoResponse struct {
	Version       string `json:"version"`
	APIVersion    string `json:"apiVersion"`
	GoVersion     string `json:"goVersion"`
	GitCommit     string `json:"gitCommit"`
	BuildTime     string `json:"buildTime"`
	FormattedTime string `json:"formattedTime"`
	OS            string `json:"os"`
	Arch          string `json:"arch"`
}

// GetServerInfo retrieves version information from the API server
func GetServerInfo() (map[string]string, error) {
	apiURL := config.GetLocalAPIURL()
	url := fmt.Sprintf("%s/api/v1/version", apiURL)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to API server: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Try to read body for more details even on error
		bodyBytes, _ := io.ReadAll(resp.Body)
		errorMsg := fmt.Sprintf("API server returned status: %s", resp.Status)
		if len(bodyBytes) > 0 {
			errorMsg = fmt.Sprintf("%s, body: %s", errorMsg, string(bodyBytes))
		}
		return nil, fmt.Errorf("%s", errorMsg)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response from API server: %v", err)
	}

	var respData serverInfoResponse
	if err := json.Unmarshal(body, &respData); err != nil {
		return nil, fmt.Errorf("failed to parse server version information (body: %s): %v", string(body), err)
	}

	// Convert struct to map with PascalCase keys for consistency
	serverInfoMap := map[string]string{
		"Version":       respData.Version,
		"APIVersion":    respData.APIVersion,
		"GoVersion":     respData.GoVersion,
		"GitCommit":     respData.GitCommit,
		"BuildTime":     respData.BuildTime,
		"FormattedTime": respData.FormattedTime,
		"OS":            respData.OS,
		"Arch":          respData.Arch,
	}

	return serverInfoMap, nil
}
