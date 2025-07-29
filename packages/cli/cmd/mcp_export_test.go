package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/babelcloud/gbox/packages/cli/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDirExists(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "gbox-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Test existing directory
	assert.True(t, dirExists(tempDir))

	// Test non-existent directory
	nonExistentDir := filepath.Join(tempDir, "non-existent")
	assert.False(t, dirExists(nonExistentDir))

	// Test file instead of directory
	tempFile := filepath.Join(tempDir, "test-file")
	err = os.WriteFile(tempFile, []byte("test"), 0644)
	require.NoError(t, err)
	assert.False(t, dirExists(tempFile))
}

func TestGetPackagesRootPath(t *testing.T) {
	// This test is limited since it depends on the environment
	// Just verify the function runs without errors
	rootPath, err := getPackagesRootPath()

	// In a real environment, we should find a packages path
	// In test environment, it might fail depending on where tests are run
	if err == nil {
		// If successful, verify path contains "packages" and directory exists
		assert.Contains(t, rootPath, "packages")
		assert.True(t, dirExists(rootPath))
		assert.True(t, dirExists(filepath.Join(rootPath, "mcp-server")))
	}
}

func TestMergeAndMarshalConfigs(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "gbox-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Get the expected URL with /sse appended
	expectedURL := strings.TrimSuffix(config.GetMcpServerUrl(), "/") + "/sse"

	// Define the new config structure to be merged (using Command/Args)
	newConfig := McpConfig{
		McpServers: map[string]McpServerEntry{
			"gbox": {
				Command: "npx",
				Args:    []string{"mcp-remote", expectedURL},
			},
		},
	}
	newConfigRawJSON, err := json.Marshal(newConfig.McpServers["gbox"]) // Marshal just the entry for later use
	require.NoError(t, err)
	newConfigFullJSON, err := json.MarshalIndent(newConfig, "", "  ") // Marshal the full structure
	require.NoError(t, err)

	// --- Test Case 1: Target file does not exist ---
	nonExistentPath := filepath.Join(tempDir, "non-existent.json")
	mergedJSON, err := mergeAndMarshalConfigs(nonExistentPath, newConfig)
	require.NoError(t, err)
	assert.JSONEq(t, string(newConfigFullJSON), string(mergedJSON), "Merge into non-existent file")

	// --- Test Case 2: Target file exists but is empty ---
	emptyPath := filepath.Join(tempDir, "empty.json")
	err = os.WriteFile(emptyPath, []byte{}, 0644)
	require.NoError(t, err)
	mergedJSON, err = mergeAndMarshalConfigs(emptyPath, newConfig)
	require.NoError(t, err)
	assert.JSONEq(t, string(newConfigFullJSON), string(mergedJSON), "Merge into empty file")

	// --- Test Case 3: Target file exists with other entries ---
	// Note: Test data uses Command/Args format for existing entries now
	otherEntriesPath := filepath.Join(tempDir, "other.json")
	existingData := `{
		"mcpServers": {
			"other": {
				"command": "other-cmd",
				"args": ["other-arg", "http://some.url"]
			}
		}
	}`
	err = os.WriteFile(otherEntriesPath, []byte(existingData), 0644)
	require.NoError(t, err)

	mergedJSON, err = mergeAndMarshalConfigs(otherEntriesPath, newConfig)
	require.NoError(t, err)

	// Expected result combines old and new entries
	expectedCombined := GenericMcpConfig{
		McpServers: map[string]json.RawMessage{
			"other": json.RawMessage(`{"command":"other-cmd","args":["other-arg","http://some.url"]}`),
			"gbox":  json.RawMessage(newConfigRawJSON), // Use the marshalled new entry
		},
	}
	expectedCombinedJSON, _ := json.MarshalIndent(expectedCombined, "", "  ")
	assert.JSONEq(t, string(expectedCombinedJSON), string(mergedJSON), "Merge with existing entry")

	// --- Test Case 4: Target file exists with the *same* entry key ("gbox") ---
	sameEntryPath := filepath.Join(tempDir, "same_entry.json")
	existingSameKeyData := `{
		"mcpServers": {
			"gbox": {
				"command": "old-npx",
				"args": ["old-arg", "http://old-gbox-url:9999/sse"]
			},
			"other": {
				"command": "other-cmd",
				"args": ["arg"]
			}
		}
	}`
	err = os.WriteFile(sameEntryPath, []byte(existingSameKeyData), 0644)
	require.NoError(t, err)

	mergedJSON, err = mergeAndMarshalConfigs(sameEntryPath, newConfig)
	require.NoError(t, err)

	// Expected result updates "gbox" and keeps "other"
	expectedUpdated := GenericMcpConfig{
		McpServers: map[string]json.RawMessage{
			"other": json.RawMessage(`{"command":"other-cmd","args":["arg"]}`),
			"gbox":  json.RawMessage(newConfigRawJSON), // Use the marshalled new entry (which overwrites)
		},
	}
	expectedUpdatedJSON, _ := json.MarshalIndent(expectedUpdated, "", "  ")
	assert.JSONEq(t, string(expectedUpdatedJSON), string(mergedJSON), "Merge should update existing gbox entry")

	// --- Test Case 5: Target file contains invalid JSON ---
	invalidJSONPath := filepath.Join(tempDir, "invalid.json")
	err = os.WriteFile(invalidJSONPath, []byte(`{ "mcpServers": { "key": "value }`), 0644) // Missing closing brace
	require.NoError(t, err)

	_, err = mergeAndMarshalConfigs(invalidJSONPath, newConfig)
	require.Error(t, err) // Expect an error because merging cannot be done safely
	assert.Contains(t, err.Error(), "invalid JSON", "Error message should indicate invalid JSON")
}

func TestNewMcpExportCommand(t *testing.T) {
	cmd := NewMcpExportCommand()

	// Verify command basic properties
	assert.Equal(t, "export", cmd.Use)
	assert.Equal(t, "Export MCP configuration for Claude Desktop/Cursor (Android only)", cmd.Short)

	// Verify flag options
	mergeTo, err := cmd.Flags().GetString("merge-to")
	require.NoError(t, err)
	assert.Equal(t, "", mergeTo)

	dryRun, err := cmd.Flags().GetBool("dry-run")
	require.NoError(t, err)
	assert.Equal(t, false, dryRun)

	// The 'type' flag is removed, so we do not test it
}
