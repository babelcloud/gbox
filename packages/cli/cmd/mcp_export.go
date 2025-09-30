package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/babelcloud/gbox/packages/cli/config"
	"github.com/spf13/cobra"
)

// Define the structure for the new MCP server entry using URL
type McpServerEntry struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
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
	var scope string

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export MCP configuration for Claude Desktop/Cursor (Android only)",
		Long: `Export MCP server configuration for Claude Desktop, Cursor, or Claude-Code (Android only).

Only Android MCP server is supported. The configuration will use npx @gbox.ai/mcp-server.
`,
		Example: `  # Export Android MCP server configuration (default)
  gbox mcp export --merge-to claude

  # Export to Cursor
  gbox mcp export --merge-to cursor

  # Generate claude mcp add command (claude-code, user scope, default)
  gbox mcp export --merge-to claude-code

  # Generate claude mcp add command (claude-code, specify scope)
  gbox mcp export --merge-to claude-code --scope project

  # Preview configuration only
  gbox mcp export --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return exportConfig(mergeTo, dryRun, scope)
		},
	}

	cmd.Flags().StringVarP(&mergeTo, "merge-to", "m", "", "Merge configuration into target config file (claude|cursor|claude-code)")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "Preview merge result without applying changes")
	cmd.Flags().StringVarP(&scope, "scope", "s", "user", "MCP server scope for claude-code (local|project|user)")

	cmd.RegisterFlagCompletionFunc("scope", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"local", "project", "user"}, cobra.ShellCompDirectiveNoFileComp
	})

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
			if dirExists(filepath.Join(packagesDir, "mcp-server")) || dirExists(filepath.Join(packagesDir, "mcp-android-server")) {
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

	if dirExists(filepath.Join(packagesDir, "mcp-server")) || dirExists(filepath.Join(packagesDir, "mcp-android-server")) {
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

func exportConfig(mergeTo string, dryRun bool, scope string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	claudeConfig := filepath.Join(homeDir, "Library", "Application Support", "Claude", "claude_desktop_config.json")
	cursorConfig := filepath.Join(homeDir, ".cursor", "mcp.json")

	configToExport := McpConfig{
		McpServers: map[string]McpServerEntry{
			"gbox-android": {
				Command: "npx",
				Args:    []string{"-y", "@gbox.ai/mcp-server@latest"},
			},
		},
	}

	if mergeTo != "" {
		if mergeTo != "claude" && mergeTo != "cursor" && mergeTo != "claude-code" {
			return fmt.Errorf("--merge-to target must be either 'claude', 'cursor', or 'claude-code'")
		}
		if mergeTo == "claude-code" {
			return outputClaudeCodeCommand(dryRun, scope)
		}
		targetConfig := claudeConfig
		if mergeTo == "cursor" {
			targetConfig = cursorConfig
		}
		if err := os.MkdirAll(filepath.Dir(targetConfig), 0755); err != nil {
			return fmt.Errorf("failed to create target directory: %w", err)
		}
		mergedJSON, err := mergeAndMarshalConfigs(targetConfig, configToExport)
		if err != nil {
			return fmt.Errorf("failed to merge configurations for '%s': %w", targetConfig, err)
		}
		if dryRun {
			var prettyJSON bytes.Buffer
			if err := json.Indent(&prettyJSON, mergedJSON, "", "  "); err != nil {
				fmt.Println(string(mergedJSON))
				fmt.Println("Warning: Could not pretty-print JSON.")
			} else {
				fmt.Println(prettyJSON.String())
			}
		} else {
			if err := os.WriteFile(targetConfig, mergedJSON, 0644); err != nil {
				return fmt.Errorf("failed to write configuration to '%s': %w", targetConfig, err)
			}
			fmt.Printf("Configuration merged into %s\n", targetConfig)
		}
	} else {
		output, _ := json.MarshalIndent(configToExport, "", "  ")
		fmt.Println(string(output))
		fmt.Println()
		fmt.Println("To merge this configuration, run:")
		fmt.Println("  gbox mcp export --merge-to claude      # For Claude Desktop")
		fmt.Println("  gbox mcp export --merge-to cursor      # For Cursor")
		fmt.Println("  gbox mcp export --merge-to claude-code # For Claude-Code (generates claude mcp add command)")
		fmt.Println()
		fmt.Println("Note: Android server will use the published npm package @gbox.ai/mcp-server@latest via npx.")
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

// outputClaudeCodeCommand outputs the claude mcp add command for claude-code integration
func outputClaudeCodeCommand(dryRun bool, scope string) error {
	serverName := "gbox-android"
	var envArgs []string
	envArgs = append(envArgs, "-e", "MODE=stdio")

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "mcp", "add", serverName)
	cmdArgs = append(cmdArgs, envArgs...)
	cmdArgs = append(cmdArgs, "-s", scope)
	cmdArgs = append(cmdArgs, "--", "npx", "-y", "@gbox.ai/mcp-server")

	if dryRun {
		fmt.Println("Copy and execute the following command in your target directory:")
		fmt.Println("----------------------------------------")
		fmt.Printf("claude %s\n", strings.Join(cmdArgs, " "))
		fmt.Println()
		fmt.Println("Note: Android mcp server will use the published npm package @gbox.ai/mcp-server@latest via npx.")
	} else {
		return executeClaudeCommand(cmdArgs, serverName)
	}
	return nil
}

// executeClaudeCommand executes the claude mcp add command
func executeClaudeCommand(cmdArgs []string, serverName string) error {
	// Check if claude command is available
	claudeCmd := exec.Command("claude", cmdArgs...)
	claudeCmd.Stdout = os.Stdout
	claudeCmd.Stderr = os.Stderr

	fmt.Printf("Executing: claude %s\n", strings.Join(cmdArgs, " "))

	if err := claudeCmd.Run(); err != nil {
		return fmt.Errorf("failed to execute claude mcp add command: %w", err)
	}

	fmt.Println("MCP server configuration completed successfully!")
	if serverName == "gbox-android" {
		fmt.Println("Note: Android mcp server will automatically use API key from current profile.")
	}

	return nil
}
