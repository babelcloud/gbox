package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/spf13/viper"
)

var v *viper.Viper

var githubClientSecret string

func init() {
	v = viper.New()

	// Set default values
	v.SetDefault("api.endpoint.local", "http://localhost:28080")
	// v.SetDefault("api.endpoint.cloud", "http://gbox.localhost:2080")
	v.SetDefault("api.endpoint.cloud", "https://gbox.ai")

	v.SetDefault("project.root", "")
	v.SetDefault("mcp.server.url", "http://localhost:28090/sse") // Default MCP server URL

	// Set default profile file path
	v.SetDefault("profile.path", filepath.Join(xdg.Home, ".gbox", "profile.json"))

	v.SetDefault("github.client_secret", "")

	// Environment variables
	v.AutomaticEnv()
	v.BindEnv("api.endpoint.local", "API_ENDPOINT_LOCAL", "API_ENDPOINT")
	v.BindEnv("api.endpoint.cloud", "API_ENDPOINT_CLOUD")
	v.BindEnv("project.root", "PROJECT_ROOT")
	v.BindEnv("mcp.server.url", "MCP_SERVER_URL")  // Bind MCP server URL env var
	v.BindEnv("profile.path", "GBOX_PROFILE_PATH") // Bind profile path env var
	v.BindEnv("github.client_secret", "GBOX_GITHUB_CLIENT_SECRET")

	// Config file
	v.SetConfigName("config")
	v.SetConfigType("yaml")

	// Look for config in the following paths
	configPaths := []string{
		".",
		"$HOME/.gbox",
		"/etc/gbox",
	}

	for _, path := range configPaths {
		expandedPath := os.ExpandEnv(path)
		v.AddConfigPath(expandedPath)
	}

	// Read config file if it exists
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// Config file was found but another error was produced
			panic(fmt.Sprintf("Fatal error reading config file: %s", err))
		}
		// Config file not found; ignore error and use defaults
	}
}

// GetLocalAPIURL returns the local API server URL
func GetLocalAPIURL() string {
	return v.GetString("api.endpoint.local")
}

// GetCloudAPIURL returns the cloud API server URL
func GetCloudAPIURL() string {
	return v.GetString("api.endpoint.cloud")
}

// GetProjectRoot returns the project root directory
func GetProjectRoot() string {
	return v.GetString("project.root")
}

// GetMcpServerUrl returns the MCP server URL
func GetMcpServerUrl() string {
	return v.GetString("mcp.server.url")
}

// GetProfilePath returns the profile file path
func GetProfilePath() string {
	return v.GetString("profile.path")
}

// GetGithubClientSecret returns the GitHub OAuth client secret from env or config
func GetGithubClientSecret() string {
	if githubClientSecret != "" {
		return githubClientSecret
	}
	return v.GetString("github.client_secret")
}
