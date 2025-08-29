package cmd

import (
	"encoding/base64"
	"fmt"

	"github.com/babelcloud/gbox/packages/cli/internal/profile"
	"github.com/spf13/cobra"
)

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage configuration profiles",
	Long:  `Manage configuration information in profile file, including API key, organization name, etc.`,
}

var profileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		pm := profile.NewProfileManager()
		if err := pm.Load(); err != nil {
			return err
		}

		format, _ := cmd.Flags().GetString("format")
		pm.List(format)
		return nil
	},
}

func init() {
	profileListCmd.Flags().StringP("format", "f", "table", "Output format (table|json)")
}

// addManually manually input profile information
func addManually(pm *profile.ProfileManager, initialKey, initialOrg string) error {
	var key string

	// Use provided values or prompt for input
	if initialKey != "" {
		key = initialKey
		maskedKey := maskAPIKey(key)
		fmt.Printf("API Key: %s\n", maskedKey)
	} else {
		fmt.Print("Please enter API Key: ")
		fmt.Scanln(&key)
		if key == "" {
			return fmt.Errorf("API Key cannot be empty")
		}
	}

	// Add new profile (org will be auto-retrieved from API)
	if err := pm.Add("", "", key, ""); err != nil {
		return err
	}

	fmt.Println("Profile added successfully")
	return nil
}

var profileAddCmd = &cobra.Command{
	Use:   "add [--key|-k KEY]",
	Short: "Add profile via API key",
	Long: `Add a profile by providing an API key. Organization will be automatically retrieved from the API.

Examples:
  gbox profile add --key xxx  # Direct add with API key
  gbox profile add            # Interactive mode`,
	RunE: func(cmd *cobra.Command, args []string) error {
		pm := profile.NewProfileManager()
		if err := pm.Load(); err != nil {
			return err
		}

		// Retrieve CLI arguments
		key, _ := cmd.Flags().GetString("key")

		// Check if API key is provided
		if key == "" {
			// Enter interactive mode for missing parameters
			return addManually(pm, key, "")
		}

		// Add new profile
		if err := pm.Add("", "", key, ""); err != nil {
			return err
		}

		fmt.Println("Profile added successfully")
		return nil
	},
}

var profileUseCmd = &cobra.Command{
	Use:   "use [profile-id]",
	Short: "Set current profile",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pm := profile.NewProfileManager()
		if err := pm.Load(); err != nil {
			return err
		}

		var profileID string

		if len(args) == 0 {
			// No arguments, show selection menu
			profiles := pm.GetProfiles()
			if len(profiles) == 0 {
				return fmt.Errorf("no profiles available, please add a profile first")
			}

			fmt.Println("Available Profiles:")
			fmt.Println("------------------")
			currentProfile := pm.GetCurrent()
			for id, profile := range profiles {
				current := ""
				if currentProfile != nil && id == pm.GetCurrentProfileID() {
					current = " (*)"
				}
				fmt.Printf("%s. %s%s\n", id, profile.GetOrgName(), current)
			}
			fmt.Print("\nPlease select a profile (enter ID): ")

			fmt.Scanln(&profileID)
		} else {
			// Has arguments, use provided profile ID
			profileID = args[0]
		}

		if err := pm.Use(profileID); err != nil {
			return err
		}

		fmt.Printf("Switched to profile '%s'\n", profileID)
		return nil
	},
}

var profileDeleteCmd = &cobra.Command{
	Use:   "delete [profile-id]",
	Short: "Delete specified profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pm := profile.NewProfileManager()
		if err := pm.Load(); err != nil {
			return err
		}

		profileID := args[0]

		if err := pm.Remove(profileID); err != nil {
			return err
		}

		fmt.Printf("Profile '%s' deleted\n", profileID)
		return nil
	},
}

var profileCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show current profile information",
	RunE: func(cmd *cobra.Command, args []string) error {
		pm := profile.NewProfileManager()
		if err := pm.Load(); err != nil {
			return err
		}

		current := pm.GetCurrent()
		if current == nil {
			fmt.Println("No current profile set")
			return nil
		}

		fmt.Println("Current Profile:")
		fmt.Printf("  Profile ID: %s\n", pm.GetCurrentProfileID())
		fmt.Printf("  Organization: %s\n", current.GetOrgName())
		// Decode API key for display
		decodedBytes, err := base64.StdEncoding.DecodeString(current.APIKey)
		if err != nil {
			fmt.Printf("  API Key: [Error decoding: %v]\n", err)
		} else {
			key := string(decodedBytes)
			maskedKey := maskAPIKey(key)
			fmt.Printf("  API Key: %s\n", maskedKey)
		}
		// Only show Base URL if it's different from default
		if current.BaseURL != "" && current.BaseURL != pm.GetDefaultBaseURL() {
			fmt.Printf("  Base URL: %s\n", current.BaseURL)
		}
		return nil
	},
}

var profileGetCmd = &cobra.Command{
	Use:   "get [profile-id]",
	Short: "Show specific profile information",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pm := profile.NewProfileManager()
		if err := pm.Load(); err != nil {
			return err
		}

		profileID := args[0]
		profile := pm.GetProfile(profileID)
		if profile == nil {
			return fmt.Errorf("profile '%s' not found", profileID)
		}

		fmt.Printf("Profile '%s':\n", profileID)
		fmt.Printf("  Organization: %s\n", profile.GetOrgName())
		// Decode API key for display
		decodedBytes, err := base64.StdEncoding.DecodeString(profile.APIKey)
		if err != nil {
			fmt.Printf("  API Key: [Error decoding: %v]\n", err)
		} else {
			key := string(decodedBytes)
			maskedKey := maskAPIKey(key)
			fmt.Printf("  API Key: %s\n", maskedKey)
		}
		// Only show Base URL if it's different from default
		if profile.BaseURL != "" && profile.BaseURL != pm.GetDefaultBaseURL() {
			fmt.Printf("  Base URL: %s\n", profile.BaseURL)
		}
		return nil
	},
}

func init() {
	// Add command line arguments for profileAddCmd
	profileAddCmd.Flags().StringP("key", "k", "", "API key")

	profileCmd.AddCommand(profileListCmd)
	profileCmd.AddCommand(profileAddCmd)
	profileCmd.AddCommand(profileUseCmd)
	profileCmd.AddCommand(profileDeleteCmd)
	profileCmd.AddCommand(profileCurrentCmd)
	profileCmd.AddCommand(profileGetCmd)
	rootCmd.AddCommand(profileCmd)
}

// maskAPIKey masks an API key for display, showing only first and last few characters
func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}

	// Show first 4 and last 4 characters
	first := key[:4]
	last := key[len(key)-4:]
	return first + "****" + last
}
