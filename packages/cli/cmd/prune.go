package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/babelcloud/gbox/packages/cli/config"
	"github.com/spf13/cobra"
)

// PruneOptions holds command options
type PruneOptions struct {
	All   bool
	Force bool
}

// NewPruneCommand creates a new prune command
func NewPruneCommand() *cobra.Command {
	opts := &PruneOptions{}

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Clean GBOX cache",
		Long:  `Clean GBOX cache files and directories. By default, this will clean all cache except login credentials and profiles. Use --all to also clean credentials.json and profiles.toml.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPrune(opts)
		},
	}

	cmd.Flags().BoolVar(&opts.All, "all", false, "Also clean login credentials and profiles (credentials.json and profiles.toml)")
	cmd.Flags().BoolVarP(&opts.Force, "force", "f", false, "Force clean without confirmation")

	return cmd
}

// runPrune executes the prune command logic
func runPrune(opts *PruneOptions) error {
	// Get cache directories
	gboxHome := config.GetGboxHome()
	cliCacheHome := config.GetCliCacheHome()

	// Ask for confirmation if not forced
	if !opts.Force {
		fmt.Print("\nAre you sure you want to continue? (y/N): ")
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read user input: %v", err)
		}
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Operation cancelled.")
			return nil
		}
	}

	fmt.Println("\nCleaning GBOX cache...")

	var cleanedItems []string
	var errors []error

	// Prune CLI cache directory (contains all cache files)
	if err := pruneCliCache(cliCacheHome, opts.All, &cleanedItems, &errors); err != nil {
		errors = append(errors, fmt.Errorf("failed to prune CLI cache: %v", err))
	}

	// Prune device-proxy directory (created by device proxy binary at runtime)
	deviceProxyHome := filepath.Join(gboxHome, "device-proxy")
	if err := pruneDeviceProxyHome(deviceProxyHome, &cleanedItems, &errors); err != nil {
		errors = append(errors, fmt.Errorf("failed to prune device-proxy directory: %v", err))
	}

	// Prune credentials and profiles from gbox home (only if --all is specified)
	if opts.All {
		if err := pruneCredentialsAndProfiles(gboxHome, &cleanedItems, &errors); err != nil {
			errors = append(errors, fmt.Errorf("failed to prune credentials and profiles: %v", err))
		}
	}

	// Report results
	if len(cleanedItems) <= 0 {
		fmt.Println("\nNo cache items found to clean.")
	} 

	if len(errors) > 0 {
		fmt.Println("\nErrors encountered:")
		for _, err := range errors {
			fmt.Printf("  - %v\n", err)
		}
		return fmt.Errorf("prune completed with errors")
	}

	fmt.Println("\nPrune completed successfully.")
	return nil
}

// pruneCliCache prunes all CLI cache files from the unified cache directory
func pruneCliCache(cliCacheHome string, cleanCredentials bool, cleanedItems *[]string, errors *[]error) error {
	if _, err := os.Stat(cliCacheHome); os.IsNotExist(err) {
		return nil // Directory doesn't exist, nothing to clean
	}

	// Clean version cache file
	versionCachePath := filepath.Join(cliCacheHome, "version.json")
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
	binaryPath := filepath.Join(cliCacheHome, binaryName)
	if err := os.Remove(binaryPath); err == nil {
		*cleanedItems = append(*cleanedItems, binaryPath)
	} else if !os.IsNotExist(err) {
		*errors = append(*errors, fmt.Errorf("failed to remove device proxy binary: %v", err))
	}

	// Clean any downloaded asset files (tar.gz, zip, etc.)
	entries, err := os.ReadDir(cliCacheHome)
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
			assetPath := filepath.Join(cliCacheHome, fileName)
			if err := os.Remove(assetPath); err == nil {
				*cleanedItems = append(*cleanedItems, assetPath)
			} else if !os.IsNotExist(err) {
				*errors = append(*errors, fmt.Errorf("failed to remove asset file %s: %v", fileName, err))
			}
		}
	}

	// Clean log files
	logPatterns := []string{"gbox-adb-expose-*.log", "device-proxy.log", "*.log"}
	for _, pattern := range logPatterns {
		matches, err := filepath.Glob(filepath.Join(cliCacheHome, pattern))
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
	pidPatterns := []string{"gbox-adb-expose-*.pid", "device-proxy.pid", "*.pid"}
	for _, pattern := range pidPatterns {
		matches, err := filepath.Glob(filepath.Join(cliCacheHome, pattern))
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

// pruneDeviceProxyHome prunes the device-proxy directory created by device proxy binary at runtime
func pruneDeviceProxyHome(deviceProxyHome string, cleanedItems *[]string, errors *[]error) error {
	if _, err := os.Stat(deviceProxyHome); os.IsNotExist(err) {
		return nil // Directory doesn't exist, nothing to clean
	}

	// Remove the entire device-proxy directory and all its contents
	if err := os.RemoveAll(deviceProxyHome); err != nil {
		*errors = append(*errors, fmt.Errorf("failed to remove device-proxy directory: %v", err))
		return err
	}

	*cleanedItems = append(*cleanedItems, deviceProxyHome)
	return nil
}

// pruneCredentialsAndProfiles prunes credentials and profiles from gbox home
func pruneCredentialsAndProfiles(gboxHome string, cleanedItems *[]string, errors *[]error) error {
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
