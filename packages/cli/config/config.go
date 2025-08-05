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

	// Set default gbox home directory
	v.SetDefault("gbox.home", filepath.Join(xdg.Home, ".gbox"))

	// Set default profile file path (based on gbox.home)
	// Note: We can't use GetGboxHome() here because it's not available during init
	// The profile path will be resolved dynamically when accessed
	v.SetDefault("profile.path", "")

	v.SetDefault("github.client_secret", "")

	// Environment variables
	v.AutomaticEnv()
	v.BindEnv("api.endpoint.local", "API_ENDPOINT_LOCAL", "API_ENDPOINT")
	v.BindEnv("api.endpoint.cloud", "API_ENDPOINT_CLOUD")
	v.BindEnv("project.root", "PROJECT_ROOT")
	v.BindEnv("mcp.server.url", "MCP_SERVER_URL") // Bind MCP server URL env var
	v.BindEnv("gbox.home", "GBOX_HOME")
	v.BindEnv("device_proxy.home", "DEVICE_PROXY_HOME")
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
	// If profile.path is explicitly set, use it
	if profilePath := v.GetString("profile.path"); profilePath != "" {
		return profilePath
	}

	// Otherwise, use gbox.home + "/profile.json"
	return filepath.Join(GetGboxHome(), "profile.json")
}

// GetGithubClientSecret returns the GitHub OAuth client secret from env or config
func GetGithubClientSecret() string {
	if githubClientSecret != "" {
		return githubClientSecret
	}
	return v.GetString("github.client_secret")
}

// GetGboxHome returns the gbox home directory
func GetGboxHome() string {
	return v.GetString("gbox.home")
}

// GetDeviceProxyHome returns the device proxy home directory
func GetDeviceProxyHome() string {
	// Check if device_proxy.home is explicitly set
	if deviceProxyHome := v.GetString("device_proxy.home"); deviceProxyHome != "" {
		return deviceProxyHome
	}

	// Otherwise, use gbox.home + "/device-proxy"
	return filepath.Join(GetGboxHome(), "device-proxy")
}
