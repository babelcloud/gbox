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
	deviceProxyHome := config.GetDeviceProxyHome()

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

	// Prune device proxy cache
	if err := pruneDeviceProxyCache(deviceProxyHome, &cleanedItems, &errors); err != nil {
		errors = append(errors, fmt.Errorf("failed to prune device proxy cache: %v", err))
	}

	// Prune other cache files in gbox home
	if err := pruneGboxCache(gboxHome, opts.All, &cleanedItems, &errors); err != nil {
		errors = append(errors, fmt.Errorf("failed to prune gbox cache: %v", err))
	}

	// Report results
	if len(cleanedItems) > 0 {
		fmt.Println("\nCleaned cache items:")
		for _, item := range cleanedItems {
			fmt.Printf("  - %s\n", item)
		}
	} else {
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

// pruneDeviceProxyCache prunes device proxy related cache files
func pruneDeviceProxyCache(deviceProxyHome string, cleanedItems *[]string, errors *[]error) error {
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

// pruneGboxCache prunes other gbox cache files
func pruneGboxCache(gboxHome string, cleanCredentials bool, cleanedItems *[]string, errors *[]error) error {
	if _, err := os.Stat(gboxHome); os.IsNotExist(err) {
		return nil // Directory doesn't exist, nothing to clean
	}

	// Clean credentials file only if --all is specified
	if cleanCredentials {
		credentialsPath := filepath.Join(gboxHome, "credentials.json")
		if err := os.Remove(credentialsPath); err == nil {
			*cleanedItems = append(*cleanedItems, credentialsPath)
		} else if !os.IsNotExist(err) {
			*errors = append(*errors, fmt.Errorf("failed to remove credentials file: %v", err))
		}

		// Clean profiles file only if --all is specified
		profilesPath := filepath.Join(gboxHome, "profiles.toml")
		if err := os.Remove(profilesPath); err == nil {
			*cleanedItems = append(*cleanedItems, profilesPath)
		} else if !os.IsNotExist(err) {
			*errors = append(*errors, fmt.Errorf("failed to remove profiles file: %v", err))
		}
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
