package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/babelcloud/gbox/packages/cli/config"
	"github.com/spf13/cobra"
)

// Profile represents a configuration item
type Profile struct {
	APIKey           string `json:"api_key"`
	Name             string `json:"name"`
	OrganizationName string `json:"organization_name"`
	Current          bool   `json:"current"`
}

// ProfileManager manages profile files
type ProfileManager struct {
	profiles []Profile
	path     string
}

// NewProfileManager creates a new ProfileManager
func NewProfileManager() *ProfileManager {
	return &ProfileManager{
		profiles: []Profile{},
		path:     config.GetProfilePath(),
	}
}

// Load loads profiles from file
func (pm *ProfileManager) Load() error {
	if _, err := os.Stat(pm.path); os.IsNotExist(err) {
		// File doesn't exist, create empty file
		return pm.Save()
	}

	data, err := os.ReadFile(pm.path)
	if err != nil {
		return fmt.Errorf("failed to read profile file: %v", err)
	}

	if len(data) == 0 {
		pm.profiles = []Profile{}
		return nil
	}

	if err := json.Unmarshal(data, &pm.profiles); err != nil {
		return fmt.Errorf("failed to parse profile file: %v", err)
	}

	return nil
}

// Save saves profiles to file
func (pm *ProfileManager) Save() error {
	if err := os.MkdirAll(filepath.Dir(pm.path), 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	data, err := json.MarshalIndent(pm.profiles, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize profile data: %v", err)
	}

	if err := os.WriteFile(pm.path, data, 0o600); err != nil {
		return fmt.Errorf("failed to write profile file: %v", err)
	}

	return nil
}

// List lists all profiles
func (pm *ProfileManager) List() {
	if len(pm.profiles) == 0 {
		fmt.Println("No profiles found")
		return
	}

	fmt.Println("Profiles:")
	fmt.Println("--------")
	for i, profile := range pm.profiles {
		current := ""
		if profile.Current {
			current = " (*)"
		}
		fmt.Printf("%d. %s - %s%s\n", i+1, profile.Name, profile.OrganizationName, current)
	}
}

// Add adds a new profile
func (pm *ProfileManager) Add(apiKey, name, organizationName string) error {
	// Check if the same profile already exists
	for _, profile := range pm.profiles {
		if profile.APIKey == apiKey && profile.Name == name && profile.OrganizationName == organizationName {
			return fmt.Errorf("same profile already exists")
		}
	}

	// Clear current flag from all profiles
	for i := range pm.profiles {
		pm.profiles[i].Current = false
	}

	// Add new profile and set as current
	newProfile := Profile{
		APIKey:           apiKey,
		Name:             name,
		OrganizationName: organizationName,
		Current:          true,
	}

	pm.profiles = append(pm.profiles, newProfile)

	return pm.Save()
}

// Use sets the current profile
func (pm *ProfileManager) Use(index int) error {
	if len(pm.profiles) == 0 {
		return fmt.Errorf("no profiles available, please add a profile first")
	}

	// If index is 0, show selection menu
	if index == 0 {
		fmt.Println("Available Profiles:")
		fmt.Println("------------------")
		for i, profile := range pm.profiles {
			current := ""
			if profile.Current {
				current = " (*)"
			}
			fmt.Printf("%d. %s - %s%s\n", i+1, profile.Name, profile.OrganizationName, current)
		}
		fmt.Print("\nPlease select a profile (enter number): ")

		var input string
		fmt.Scanln(&input)

		var err error
		index, err = strconv.Atoi(input)
		if err != nil {
			return fmt.Errorf("invalid input: %s", input)
		}
	}

	if index < 1 || index > len(pm.profiles) {
		return fmt.Errorf("invalid profile index: %d", index)
	}

	// Clear current flag from all profiles
	for i := range pm.profiles {
		pm.profiles[i].Current = false
	}

	// Set specified profile as current
	pm.profiles[index-1].Current = true

	return pm.Save()
}

// Remove removes the specified profile
func (pm *ProfileManager) Remove(index int) error {
	if index < 1 || index > len(pm.profiles) {
		return fmt.Errorf("invalid profile index: %d", index)
	}

	// Check if trying to delete current profile
	if pm.profiles[index-1].Current && len(pm.profiles) > 1 {
		return fmt.Errorf("cannot delete the currently active profile, please switch to another profile first")
	}

	// Remove specified profile
	pm.profiles = append(pm.profiles[:index-1], pm.profiles[index:]...)

	// If there are still profiles after deletion and no current profile, set the first one as current
	if len(pm.profiles) > 0 {
		hasCurrent := false
		for _, profile := range pm.profiles {
			if profile.Current {
				hasCurrent = true
				break
			}
		}
		if !hasCurrent {
			pm.profiles[0].Current = true
		}
	}

	return pm.Save()
}

// GetCurrent gets the current profile
func (pm *ProfileManager) GetCurrent() *Profile {
	for _, profile := range pm.profiles {
		if profile.Current {
			return &profile
		}
	}
	return nil
}

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage configuration profiles",
	Long:  `Manage configuration information in profile file, including API key, organization name, etc.`,
}

var profileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		pm := NewProfileManager()
		if err := pm.Load(); err != nil {
			return err
		}
		pm.List()
		return nil
	},
}

// addManually manually input profile information
func addManually(pm *ProfileManager) error {
	var apiKey, name, orgName string

	fmt.Print("Please enter API Key: ")
	fmt.Scanln(&apiKey)
	if apiKey == "" {
		return fmt.Errorf("API Key cannot be empty")
	}

	fmt.Print("Please enter profile name: ")
	fmt.Scanln(&name)
	if name == "" {
		return fmt.Errorf("Profile name cannot be empty")
	}

	fmt.Print("Please enter organization name (optional, default is 'default'): ")
	fmt.Scanln(&orgName)
	if orgName == "" {
		orgName = "default"
	}

	// Check if the same profile already exists
	for _, profile := range pm.profiles {
		if profile.APIKey == apiKey && profile.Name == name && profile.OrganizationName == orgName {
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
  gbox profile add                                    # Interactive mode`,
	RunE: func(cmd *cobra.Command, args []string) error {
		pm := NewProfileManager()
		if err := pm.Load(); err != nil {
			return err
		}

		// Retrieve CLI arguments
		apiKey, _ := cmd.Flags().GetString("key")
		name, _ := cmd.Flags().GetString("name")
		orgName, _ := cmd.Flags().GetString("org-name")

		// Enter interactive mode when --key is not provided
		if apiKey == "" {
			return addManually(pm)
		}

		// --name is required when --key is provided
		if name == "" {
			return fmt.Errorf("--name must be provided when using --key")
		}

		// Default organization name when not provided
		if orgName == "" {
			orgName = "default"
		}

		// Check duplicate profiles
		for _, profile := range pm.profiles {
			if profile.APIKey == apiKey && profile.Name == name && profile.OrganizationName == orgName {
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
		pm := NewProfileManager()
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
		pm := NewProfileManager()
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
		pm := NewProfileManager()
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
