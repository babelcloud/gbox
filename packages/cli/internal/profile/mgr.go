package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/babelcloud/gbox/packages/cli/config"
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

// GetProfiles returns all profiles
func (pm *ProfileManager) GetProfiles() []Profile {
	return pm.profiles
}

// GetCurrentAPIKey gets the API key from the current profile
func GetCurrentAPIKey() (string, error) {
	pm := NewProfileManager()
	if err := pm.Load(); err != nil {
		return "", fmt.Errorf("failed to load profiles: %v", err)
	}

	current := pm.GetCurrent()
	if current == nil {
		return "", fmt.Errorf("no current profile set. Please run 'gbox profile use' to set a current profile first")
	}

	if current.APIKey == "" {
		return "", fmt.Errorf("current profile has no API key. Please run 'gbox profile add' to configure a profile first")
	}

	return current.APIKey, nil
}
