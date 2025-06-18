package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"
)

const (
	profileFile = "profile.json"
)

var (
	profilePath = filepath.Join(configDir, profileFile)
)

// Profile represents a configuration item
type Profile struct {
	APIKey           string `json:"api_key"`
	APIKeyName       string `json:"api_key_name"`
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
		path:     profilePath,
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
		fmt.Printf("%d. %s - %s%s\n", i+1, profile.APIKeyName, profile.OrganizationName, current)
	}
}

// Add adds a new profile
func (pm *ProfileManager) Add(apiKey, apiKeyName, organizationName string) error {
	// Check if the same profile already exists
	for _, profile := range pm.profiles {
		if profile.APIKey == apiKey && profile.APIKeyName == apiKeyName && profile.OrganizationName == organizationName {
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
		APIKeyName:       apiKeyName,
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
			fmt.Printf("%d. %s - %s%s\n", i+1, profile.APIKeyName, profile.OrganizationName, current)
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
	Short: "Manage gbox configuration information",
	Long:  `Manage configuration information in ~/.gbox/profile.json file, including API key, organization name, etc.`,
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

// importManually manually input profile information
func importManually(pm *ProfileManager) error {
	var apiKey, apiKeyName, orgName string

	fmt.Print("Please enter API Key: ")
	fmt.Scanln(&apiKey)
	if apiKey == "" {
		return fmt.Errorf("API Key cannot be empty")
	}

	fmt.Print("Please enter API Key name: ")
	fmt.Scanln(&apiKeyName)
	if apiKeyName == "" {
		return fmt.Errorf("API Key name cannot be empty")
	}

	fmt.Print("Please enter organization name (optional, default is 'default'): ")
	fmt.Scanln(&orgName)
	if orgName == "" {
		orgName = "default"
	}

	// Check if the same profile already exists
	for _, profile := range pm.profiles {
		if profile.APIKey == apiKey && profile.APIKeyName == apiKeyName && profile.OrganizationName == orgName {
			return fmt.Errorf("same profile already exists")
		}
	}

	// Add new profile
	if err := pm.Add(apiKey, apiKeyName, orgName); err != nil {
		return err
	}

	fmt.Println("Profile imported successfully")
	return nil
}

var profileImportCmd = &cobra.Command{
	Use:   "import [--api-key KEY] [--api-key-name NAME] [--org-name ORG]",
	Short: "Import profile, supports importing from credentials.json or manual input",
	Long: `Import profile supports two methods:
1. Import from existing credentials.json file (default behavior)
2. Manually input API key, API key name, and organization name

Examples:
  gbox profile import                                    # Interactive selection of import method
  gbox profile import --api-key xxx --api-key-name test  # Direct manual input (org-name optional)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		pm := NewProfileManager()
		if err := pm.Load(); err != nil {
			return err
		}

		// Get command line arguments
		apiKey, _ := cmd.Flags().GetString("api-key")
		apiKeyName, _ := cmd.Flags().GetString("api-key-name")
		orgName, _ := cmd.Flags().GetString("org-name")

		// If API key parameter is provided, use manual input mode
		if apiKey != "" {
			if apiKeyName == "" {
				return fmt.Errorf("--api-key-name must be provided when using --api-key")
			}

			// If organization name is empty, set to default value
			if orgName == "" {
				orgName = "default"
			}

			// Check if the same profile already exists
			for _, profile := range pm.profiles {
				if profile.APIKey == apiKey && profile.APIKeyName == apiKeyName && profile.OrganizationName == orgName {
					return fmt.Errorf("same profile already exists")
				}
			}

			// Add new profile
			if err := pm.Add(apiKey, apiKeyName, orgName); err != nil {
				return err
			}

			fmt.Println("Profile imported successfully")
			return nil
		}

		// Interactive selection of import method
		fmt.Println("Please select import method:")
		fmt.Println("1. Import from credentials.json file")
		fmt.Println("2. Manual input")
		fmt.Print("Please select (1-2): ")

		var choice string
		fmt.Scanln(&choice)

		switch choice {
		case "1":
			// TODO Let user select which org to import, then automatically create new api key?
			return nil
		case "2":
			return importManually(pm)
		default:
			return fmt.Errorf("invalid choice: %s", choice)
		}
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
		fmt.Printf("  API Key Name: %s\n", current.APIKeyName)
		fmt.Printf("  Organization Name: %s\n", current.OrganizationName)
		fmt.Printf("  API Key: %s\n", current.APIKey)
		return nil
	},
}

func init() {
	// Add command line arguments for profileImportCmd
	profileImportCmd.Flags().String("api-key", "", "API key")
	profileImportCmd.Flags().String("api-key-name", "", "API key name")
	profileImportCmd.Flags().String("org-name", "", "Organization name (optional, default is 'default')")

	profileCmd.AddCommand(profileListCmd)
	profileCmd.AddCommand(profileImportCmd)
	profileCmd.AddCommand(profileUseCmd)
	profileCmd.AddCommand(profileDeleteCmd)
	profileCmd.AddCommand(profileCurrentCmd)
	rootCmd.AddCommand(profileCmd)
}
