package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/babelcloud/gbox/packages/cli/config"
	"github.com/spf13/cobra"
)

// Define the structure for the new MCP server entry using URL
type McpServerEntry struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

// Keep McpConfig using the specific new entry type for generation
type McpConfig struct {
	McpServers map[string]McpServerEntry `json:"mcpServers"`
}

// Define a generic structure to read potentially mixed-format existing config
type GenericMcpConfig struct {
	McpServers map[string]json.RawMessage `json:"mcpServers"`
}

func NewMcpExportCommand() *cobra.Command {
	var mergeTo string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export MCP configuration for Claude Desktop/Cursor",
		RunE: func(cmd *cobra.Command, args []string) error {
			return exportConfig(mergeTo, dryRun)
		},
	}

	cmd.Flags().StringVar(&mergeTo, "merge-to", "", "Merge configuration into target config file (claude|cursor)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview merge result without applying changes")

	return cmd
}

func getPackagesRootPath() (string, error) {
	projectRoot := config.GetProjectRoot()
	if projectRoot != "" {
		packagesDir := filepath.Join(projectRoot, "packages")
		if dirExists(packagesDir) {
			return packagesDir, nil
		}
	}

	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}
	realExecPath, err := filepath.EvalSymlinks(execPath)
	if err != nil {
		return "", fmt.Errorf("failed to get real executable path: %w", err)
	}

	standardSubPath := filepath.Join("packages", "cli")

	if !strings.Contains(realExecPath, standardSubPath) {
		// Try alternative structure if not in standard subpath (e.g., when run via 'go run')
		cwd, err := os.Getwd()
		if err == nil {
			packagesDir := filepath.Join(cwd, "packages")
			if dirExists(filepath.Join(packagesDir, "mcp-server")) {
				return packagesDir, nil
			}
		}
		return "", fmt.Errorf("unexpected binary location: %s; expected to contain %q or run from project root", realExecPath, standardSubPath)
	}

	packagesIndex := strings.Index(realExecPath, standardSubPath)
	if packagesIndex == -1 {
		return "", fmt.Errorf("could not find %q in path: %s", standardSubPath, realExecPath)
	}

	// Calculate packages directory based on the binary location
	packagesDir := realExecPath[:packagesIndex]
	packagesDir = filepath.Clean(filepath.Join(packagesDir, "packages"))

	if dirExists(filepath.Join(packagesDir, "mcp-server")) {
		return packagesDir, nil
	}

	return "", fmt.Errorf("could not determine packages root directory from binary location: %s", realExecPath)
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func exportConfig(mergeTo string, dryRun bool) error {
	packagesRoot, err := getPackagesRootPath()
	if err != nil {
		return fmt.Errorf("failed to find project root: %w", err)
	}

	mcpServerDir := filepath.Join(packagesRoot, "mcp-server")
	serverScript := filepath.Join(mcpServerDir, "dist", "index.js")

	if _, err := os.Stat(serverScript); os.IsNotExist(err) {
		return fmt.Errorf("server script not found at %s\nPlease build the MCP server first by running:\n  cd %s && pnpm build", serverScript, mcpServerDir)
	}

	serverScriptAbs, err := filepath.Abs(serverScript)

	if err != nil {
		return fmt.Errorf("failed to get absolute path for server script: %w", err)
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	claudeConfig := filepath.Join(homeDir, "Library", "Application Support", "Claude", "claude_desktop_config.json")
	cursorConfig := filepath.Join(homeDir, ".cursor", "mcp.json")

	var configToExport McpConfig

	if os.Getenv("DEBUG") == "true" {
		configToExport = McpConfig{
			McpServers: map[string]McpServerEntry{
				"gbox": {
					Command: "bash",
					Args:    []string{"-c", fmt.Sprintf("cd %s && pnpm --silent dev", mcpServerDir)},
				},
			},
		}

	} else if os.Getenv("SSE_MODE") == "true" {
		configToExport = McpConfig{
			McpServers: map[string]McpServerEntry{
				"gbox": {
					Command: "npx",
					Args:    []string{"mcp-remote", config.GetMcpServerUrl()}, // Use formatted URL
				},
			},
		}
	} else {
		configToExport = McpConfig{
			McpServers: map[string]McpServerEntry{
				"gbox": {
					Command: "node",
					Args:    []string{serverScriptAbs},
				},
			},
		}
	}

	if mergeTo != "" {
		if mergeTo != "claude" && mergeTo != "cursor" {
			return fmt.Errorf("--merge-to target must be either 'claude' or 'cursor'")
		}

		targetConfig := claudeConfig
		if mergeTo == "cursor" {
			targetConfig = cursorConfig
		}

		if err := os.MkdirAll(filepath.Dir(targetConfig), 0755); err != nil {
			return fmt.Errorf("failed to create target directory: %w", err)
		}

		// Use the updated mergeAndMarshalConfigs function which returns final JSON bytes
		mergedJSON, err := mergeAndMarshalConfigs(targetConfig, configToExport)
		if err != nil {
			return fmt.Errorf("failed to merge configurations for '%s': %w", targetConfig, err)
		}

		if dryRun {
			fmt.Println("Preview of merged configuration:")
			fmt.Println("----------------------------------------")
			// Pretty print the final JSON bytes
			var prettyJSON bytes.Buffer
			// Indent the final JSON bytes for preview
			if err := json.Indent(&prettyJSON, mergedJSON, "", "  "); err != nil {
				// Fallback to non-indented if indent fails (shouldn't happen with valid JSON)
				fmt.Println(string(mergedJSON))
				fmt.Println("Warning: Could not pretty-print JSON.")
			} else {
				fmt.Println(prettyJSON.String())
			}
		} else {
			// Write the merged JSON bytes directly
			if err := os.WriteFile(targetConfig, mergedJSON, 0644); err != nil {
				return fmt.Errorf("failed to write configuration to '%s': %w", targetConfig, err)
			}
			fmt.Printf("Configuration merged into %s\n", targetConfig)
		}
	} else {
		// Output only the new config if not merging
		output, _ := json.MarshalIndent(configToExport, "", "  ")
		fmt.Println(string(output))
		fmt.Println()
		fmt.Println("To merge this configuration, run:")
		fmt.Println("  gbox mcp export --merge-to claude   # For Claude Desktop")
		fmt.Println("  gbox mcp export --merge-to cursor   # For Cursor")
	}

	return nil
}

// New function to handle merging generically and return final JSON bytes
// This replaces the previous mergeConfigs function.
func mergeAndMarshalConfigs(targetPath string, newConfig McpConfig) ([]byte, error) {
	// Read existing content
	content, err := os.ReadFile(targetPath)
	if err != nil && !os.IsNotExist(err) {
		// Return error if reading fails for reasons other than file not existing
		return nil, fmt.Errorf("failed to read target config '%s': %w", targetPath, err)
	}

	// Prepare the structure to hold the final merged data using generic RawMessage
	finalConfigData := GenericMcpConfig{
		McpServers: make(map[string]json.RawMessage),
	}

	// If existing config exists and is not empty, unmarshal it generically
	if err == nil && len(content) > 0 {
		if err := json.Unmarshal(content, &finalConfigData); err != nil {
			// If existing JSON is invalid, return error instead of overwriting potentially important data
			// This prevents destroying a config file that might have other valid entries
			return nil, fmt.Errorf("invalid JSON in target config '%s', cannot merge safely: %w", targetPath, err)
		}
		// Ensure McpServers map is initialized if it was null or missing in the JSON
		if finalConfigData.McpServers == nil {
			finalConfigData.McpServers = make(map[string]json.RawMessage)
		}
	}

	// Iterate through the *new* config entries we want to add/update (currently only "gbox")
	for key, newEntryValue := range newConfig.McpServers {
		// Marshal the specific new entry (McpServerEntry) into json.RawMessage
		newEntryJSON, err := json.Marshal(newEntryValue)
		if err != nil {
			// This should ideally not happen with our defined struct, but check anyway
			return nil, fmt.Errorf("internal error: failed to marshal new entry for key '%s': %w", key, err)
		}
		// Add or replace the entry in the final map using the raw JSON
		finalConfigData.McpServers[key] = json.RawMessage(newEntryJSON)
	}

	// Marshal the final combined structure back into JSON bytes for writing/preview
	// Use MarshalIndent for a readable output file format
	mergedJSON, err := json.MarshalIndent(finalConfigData, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("internal error: failed to marshal final merged config: %w", err)
	}

	return mergedJSON, nil
}
