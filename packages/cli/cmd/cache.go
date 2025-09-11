package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/babelcloud/gbox/packages/cli/config"
	"github.com/spf13/cobra"
)

// CacheOptions holds command options
type CacheOptions struct {
	Force bool
}

// NewCacheCommand creates a new cache command
func NewCacheCommand() *cobra.Command {
	opts := &CacheOptions{}

	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage GBOX cache",
		Long:  `Manage GBOX cache files and directories`,
	}

	// Add clean subcommand
	cleanCmd := &cobra.Command{
		Use:   "clean",
		Short: "Clean GBOX cache",
		Long:  `Clean all GBOX cache files and directories. This will remove downloaded binaries, version cache, and other cached data.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCacheClean(opts)
		},
	}

	cleanCmd.Flags().BoolVarP(&opts.Force, "force", "f", false, "Force clean without confirmation")

	cmd.AddCommand(cleanCmd)

	return cmd
}

// runCacheClean executes the cache clean command logic
func runCacheClean(opts *CacheOptions) error {
	if !opts.Force {
		fmt.Println("Cache clean requires --force flag to proceed.")
		fmt.Println("This will remove all cached files including downloaded binaries and version information.")
		fmt.Println("Use: gbox cache clean --force")
		return nil
	}

	fmt.Println("Cleaning GBOX cache...")

	// Get cache directories
	gboxHome := config.GetGboxHome()
	deviceProxyHome := config.GetDeviceProxyHome()

	var cleanedItems []string
	var errors []error

	// Clean device proxy cache
	if err := cleanDeviceProxyCache(deviceProxyHome, &cleanedItems, &errors); err != nil {
		errors = append(errors, fmt.Errorf("failed to clean device proxy cache: %v", err))
	}

	// Clean other cache files in gbox home
	if err := cleanGboxCache(gboxHome, &cleanedItems, &errors); err != nil {
		errors = append(errors, fmt.Errorf("failed to clean gbox cache: %v", err))
	}

	// Report results
	if len(cleanedItems) > 0 {
		fmt.Println("Cleaned cache items:")
		for _, item := range cleanedItems {
			fmt.Printf("  - %s\n", item)
		}
	} else {
		fmt.Println("No cache items found to clean.")
	}

	if len(errors) > 0 {
		fmt.Println("\nErrors encountered:")
		for _, err := range errors {
			fmt.Printf("  - %v\n", err)
		}
		return fmt.Errorf("cache clean completed with errors")
	}

	fmt.Println("Cache clean completed successfully.")
	return nil
}

// cleanDeviceProxyCache cleans device proxy related cache files
func cleanDeviceProxyCache(deviceProxyHome string, cleanedItems *[]string, errors *[]error) error {
	if _, err := os.Stat(deviceProxyHome); os.IsNotExist(err) {
		return nil // Directory doesn't exist, nothing to clean
	}

	// Clean version cache file
	versionCachePath := filepath.Join(deviceProxyHome, "version.json")
	if err := os.Remove(versionCachePath); err == nil {
		*cleanedItems = append(*cleanedItems, versionCachePath)
	} else if !os.IsNotExist(err) {
		*errors = append(*errors, fmt.Errorf("failed to remove version cache: %v", err))
	}

	// Clean device proxy binary
	binaryName := "gbox-device-proxy"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(deviceProxyHome, binaryName)
	if err := os.Remove(binaryPath); err == nil {
		*cleanedItems = append(*cleanedItems, binaryPath)
	} else if !os.IsNotExist(err) {
		*errors = append(*errors, fmt.Errorf("failed to remove device proxy binary: %v", err))
	}

	// Clean any downloaded asset files (tar.gz, zip, etc.)
	entries, err := os.ReadDir(deviceProxyHome)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		
		fileName := entry.Name()
		// Check if it's a downloaded asset file
		if isAssetFile(fileName) {
			assetPath := filepath.Join(deviceProxyHome, fileName)
			if err := os.Remove(assetPath); err == nil {
				*cleanedItems = append(*cleanedItems, assetPath)
			} else if !os.IsNotExist(err) {
				*errors = append(*errors, fmt.Errorf("failed to remove asset file %s: %v", fileName, err))
			}
		}
	}

	return nil
}

// cleanGboxCache cleans other gbox cache files
func cleanGboxCache(gboxHome string, cleanedItems *[]string, errors *[]error) error {
	if _, err := os.Stat(gboxHome); os.IsNotExist(err) {
		return nil // Directory doesn't exist, nothing to clean
	}

	// Clean credentials file
	credentialsPath := filepath.Join(gboxHome, "credentials.json")
	if err := os.Remove(credentialsPath); err == nil {
		*cleanedItems = append(*cleanedItems, credentialsPath)
	} else if !os.IsNotExist(err) {
		*errors = append(*errors, fmt.Errorf("failed to remove credentials file: %v", err))
	}

	// Clean profiles file
	profilesPath := filepath.Join(gboxHome, "profiles.toml")
	if err := os.Remove(profilesPath); err == nil {
		*cleanedItems = append(*cleanedItems, profilesPath)
	} else if !os.IsNotExist(err) {
		*errors = append(*errors, fmt.Errorf("failed to remove profiles file: %v", err))
	}

	// Clean log files
	logPatterns := []string{"gbox-adb-expose-*.log", "*.log"}
	for _, pattern := range logPatterns {
		matches, err := filepath.Glob(filepath.Join(gboxHome, pattern))
		if err != nil {
			*errors = append(*errors, fmt.Errorf("failed to glob log files with pattern %s: %v", pattern, err))
			continue
		}
		
		for _, match := range matches {
			if err := os.Remove(match); err == nil {
				*cleanedItems = append(*cleanedItems, match)
			} else if !os.IsNotExist(err) {
				*errors = append(*errors, fmt.Errorf("failed to remove log file %s: %v", match, err))
			}
		}
	}

	// Clean PID files
	pidPatterns := []string{"gbox-adb-expose-*.pid", "*.pid"}
	for _, pattern := range pidPatterns {
		matches, err := filepath.Glob(filepath.Join(gboxHome, pattern))
		if err != nil {
			*errors = append(*errors, fmt.Errorf("failed to glob PID files with pattern %s: %v", pattern, err))
			continue
		}
		
		for _, match := range matches {
			if err := os.Remove(match); err == nil {
				*cleanedItems = append(*cleanedItems, match)
			} else if !os.IsNotExist(err) {
				*errors = append(*errors, fmt.Errorf("failed to remove PID file %s: %v", match, err))
			}
		}
	}

	return nil
}

// isAssetFile checks if a file is a downloaded asset file
func isAssetFile(fileName string) bool {
	assetExtensions := []string{".tar.gz", ".zip", ".exe", ".dmg", ".deb", ".rpm"}
	for _, ext := range assetExtensions {
		if len(fileName) > len(ext) && fileName[len(fileName)-len(ext):] == ext {
			return true
		}
	}
	return false
}
