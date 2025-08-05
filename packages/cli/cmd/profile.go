package cmd

import (
	"fmt"
	"strconv"

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
		pm.List()
		return nil
	},
}

// addManually manually input profile information
func addManually(pm *profile.ProfileManager, initialAPIKey, initialName, initialOrgName string) error {
	var apiKey, name, orgName string

	// Use provided values or prompt for input
	if initialAPIKey != "" {
		apiKey = initialAPIKey
		fmt.Printf("API Key: %s\n", apiKey)
	} else {
		fmt.Print("Please enter API Key: ")
		fmt.Scanln(&apiKey)
		if apiKey == "" {
			return fmt.Errorf("API Key cannot be empty")
		}
	}

	if initialName != "" {
		name = initialName
		fmt.Printf("Profile name: %s\n", name)
	} else {
		fmt.Print("Please enter profile name: ")
		fmt.Scanln(&name)
		if name == "" {
			return fmt.Errorf("Profile name cannot be empty")
		}
	}

	if initialOrgName != "" {
		orgName = initialOrgName
		fmt.Printf("Organization name: %s\n", orgName)
	} else {
		fmt.Print("Please enter organization name (optional, default is 'default'): ")
		fmt.Scanln(&orgName)
		if orgName == "" {
			orgName = "default"
		}
	}

	// Check if the same profile already exists
	for _, p := range pm.GetProfiles() {
		if p.APIKey == apiKey && p.Name == name && p.OrganizationName == orgName {
			return fmt.Errorf("same profile already exists")
		}
	}

	// Add new profile
	if err := pm.Add(apiKey, name, orgName); err != nil {
		return err
	}

	fmt.Println("Profile added successfully")
	return nil
}

var profileAddCmd = &cobra.Command{
	Use:   "add [--key|-k KEY] [--name|-n NAME] [--org-name|-o ORG]",
	Short: "Add profile via API key",
	Long: `Add a profile by providing an API key, profile name and (optionally) an organization name. You can either pass them through command-line flags or enter them interactively.

Examples:
  gbox profile add --key xxx --name test          # Direct add (org-name optional)
  gbox profile add --key xxx                      # Interactive mode for missing name/org
  gbox profile add --name test                    # Interactive mode for missing key/org
  gbox profile add                                # Fully interactive mode`,
	RunE: func(cmd *cobra.Command, args []string) error {
		pm := profile.NewProfileManager()
		if err := pm.Load(); err != nil {
			return err
		}

		// Retrieve CLI arguments
		apiKey, _ := cmd.Flags().GetString("key")
		name, _ := cmd.Flags().GetString("name")
		orgName, _ := cmd.Flags().GetString("org-name")

		// Check if all required parameters are provided
		allProvided := apiKey != "" && name != ""

		if !allProvided {
			// Enter interactive mode for missing parameters
			return addManually(pm, apiKey, name, orgName)
		}

		// Default organization name when not provided
		if orgName == "" {
			orgName = "default"
		}

		// Check duplicate profiles
		for _, p := range pm.GetProfiles() {
			if p.APIKey == apiKey && p.Name == name && p.OrganizationName == orgName {
				return fmt.Errorf("same profile already exists")
			}
		}

		// Add new profile
		if err := pm.Add(apiKey, name, orgName); err != nil {
			return err
		}

		fmt.Println("Profile added successfully")
		return nil
	},
}

var profileUseCmd = &cobra.Command{
	Use:   "use [index]",
	Short: "Set current profile",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pm := profile.NewProfileManager()
		if err := pm.Load(); err != nil {
			return err
		}

		var index int
		var err error

		if len(args) == 0 {
			// No arguments, use interactive selection
			index = 0
		} else {
			// Has arguments, parse index
			index, err = strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid index: %s", args[0])
			}
		}

		if err := pm.Use(index); err != nil {
			return err
		}

		if index == 0 {
			fmt.Println("Profile switched successfully")
		} else {
			fmt.Printf("Switched to profile %d\n", index)
		}
		return nil
	},
}

var profileDeleteCmd = &cobra.Command{
	Use:   "delete [index]",
	Short: "Delete specified profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pm := profile.NewProfileManager()
		if err := pm.Load(); err != nil {
			return err
		}

		index, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid index: %s", args[0])
		}

		if err := pm.Remove(index); err != nil {
			return err
		}

		fmt.Printf("Profile %d deleted\n", index)
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
		fmt.Printf("  Profile Name: %s\n", current.Name)
		fmt.Printf("  Organization Name: %s\n", current.OrganizationName)
		fmt.Printf("  API Key: %s\n", current.APIKey)
		return nil
	},
}

func init() {
	// Add command line arguments for profileAddCmd
	profileAddCmd.Flags().StringP("key", "k", "", "API key")
	profileAddCmd.Flags().StringP("name", "n", "", "Profile name")
	profileAddCmd.Flags().StringP("org-name", "o", "", "Organization name (optional, default is 'default')")

	profileCmd.AddCommand(profileListCmd)
	profileCmd.AddCommand(profileAddCmd)
	profileCmd.AddCommand(profileUseCmd)
	profileCmd.AddCommand(profileDeleteCmd)
	profileCmd.AddCommand(profileCurrentCmd)
	rootCmd.AddCommand(profileCmd)
}
