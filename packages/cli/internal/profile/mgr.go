package profile

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/babelcloud/gbox/packages/cli/config"
	"github.com/pelletier/go-toml/v2"
)

// Common error messages
const (
	ErrNoCurrentProfile    = "no current profile set. Please run 'gbox profile use' to set a current profile first"
	ErrNoAPIKey            = "current profile has no API key. Please run 'gbox profile add' to configure a profile first"
	ErrProfileNotFound     = "profile '%s' not found"
	ErrCannotDeleteCurrent = "cannot delete the currently active profile, please switch to another profile first"
	ErrInvalidAPIKey       = "invalid API key"
	ErrEmptyAPIKey         = "API key is empty"
)

// ProfileConfig represents the complete profile configuration
type ProfileConfig struct {
	Current  string             `toml:"current"`
	Profiles map[string]Profile `toml:"profiles"`
	Defaults ProfileDefaults    `toml:"defaults"`
}

// Profile represents a configuration profile
type Profile struct {
	OrgName string `toml:"org_name,omitempty"` // New field name
	Org     string `toml:"org,omitempty"`      // Legacy field for backward compatibility
	OrgSlug string `toml:"org_slug,omitempty"`
	APIKey  string `toml:"key"`
	BaseURL string `toml:"base_url,omitempty"`
}

// ProfileDefaults represents global defaults
type ProfileDefaults struct {
	BaseURL string `toml:"base_url,omitempty"`
}

// OrgInfo represents organization information returned from API
type OrgInfo struct {
	Name string
	Slug string
}

// ProfileManager manages profile files
type ProfileManager struct {
	config ProfileConfig
	path   string
}

// Default is the default ProfileManager instance for package-level operations
var Default = func() *ProfileManager {
	pm := NewProfileManager()
	if err := pm.Load(); err != nil {
		// Log warning but don't fail - will retry on next access
		fmt.Fprintf(os.Stderr, "Warning: failed to load default profile manager: %v\n", err)
	} else {
		// Check for potential configuration mismatch
		pm.checkConfigurationMismatch()
	}
	return pm
}()

// RefreshDefault refreshes the default ProfileManager instance
// This is useful when profile configuration changes and you want to reload from disk
func RefreshDefault() error {
	err := Default.Load()
	if err == nil {
		// Check for potential configuration mismatch after reload
		Default.checkConfigurationMismatch()
	}
	return err
}

// NewProfileManager creates a new ProfileManager
func NewProfileManager() *ProfileManager {
	return &ProfileManager{
		config: ProfileConfig{
			Profiles: make(map[string]Profile),
			Defaults: ProfileDefaults{
				BaseURL: config.GetBaseURL(),
			},
		},
		path: config.GetProfilePath(),
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
		pm.config = ProfileConfig{
			Profiles: make(map[string]Profile),
			Defaults: ProfileDefaults{
				BaseURL: config.GetBaseURL(),
			},
		}
		return nil
	}

	if err := toml.Unmarshal(data, &pm.config); err != nil {
		return fmt.Errorf("failed to parse profile file: %v", err)
	}

	// Initialize profiles map if it's nil
	if pm.config.Profiles == nil {
		pm.config.Profiles = make(map[string]Profile)
	}

	// Set default base URL if not set
	if pm.config.Defaults.BaseURL == "" {
		pm.config.Defaults.BaseURL = config.GetBaseURL()
	}

	// Check if migration is needed and perform it
	if pm.needsMigration() {
		pm.performMigration()
	}

	return nil
}

// Save saves profiles to file
func (pm *ProfileManager) Save() error {
	if err := os.MkdirAll(filepath.Dir(pm.path), 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	// Create a clean config for saving (omit base_url when it matches defaults)
	cleanConfig := pm.createCleanConfigForSaving()

	data, err := toml.Marshal(cleanConfig)
	if err != nil {
		return fmt.Errorf("failed to serialize profile data: %v", err)
	}

	if err := os.WriteFile(pm.path, data, 0o600); err != nil {
		return fmt.Errorf("failed to write profile file: %v", err)
	}

	return nil
}

// createCleanConfigForSaving creates a clean config structure for saving
// that omits base_url fields when they match the default value
func (pm *ProfileManager) createCleanConfigForSaving() ProfileConfig {
	cleanConfig := ProfileConfig{
		Current:  pm.config.Current,
		Profiles: make(map[string]Profile),
		Defaults: pm.config.Defaults,
	}

	// Copy profiles, omitting base_url when it matches defaults
	for id, profile := range pm.config.Profiles {
		cleanProfile := Profile{
			OrgName: profile.OrgName,
			Org:     profile.Org,
			OrgSlug: profile.OrgSlug,
			APIKey:  profile.APIKey,
		}

		// Only include base_url if it's different from defaults
		if profile.BaseURL != "" && profile.BaseURL != pm.config.Defaults.BaseURL {
			cleanProfile.BaseURL = profile.BaseURL
		}

		cleanConfig.Profiles[id] = cleanProfile
	}

	return cleanConfig
}

// List lists all profiles
func (pm *ProfileManager) List(format string) {
	if len(pm.config.Profiles) == 0 {
		if format == "json" {
			fmt.Println("[]")
		} else {
			fmt.Println("No profiles found")
		}
		return
	}

	// Handle JSON format
	if format == "json" {
		pm.listJSON()
		return
	}

	// Default to table format
	pm.listTable()
}

// listTable displays profiles in table format
func (pm *ProfileManager) listTable() {
	// Check if any profile has a non-default base URL
	showBaseURL := false
	for _, profile := range pm.config.Profiles {
		if profile.BaseURL != "" && profile.BaseURL != pm.config.Defaults.BaseURL {
			showBaseURL = true
			break
		}
	}

	// Calculate column widths based on content
	maxIDLen := 2      // "ID" header
	maxKeyLen := 3     // "Key" header
	maxOrgLen := 12    // "Organization" header
	maxBaseURLLen := 8 // "Base URL" header

	for id, profile := range pm.config.Profiles {
		// Add arrow to current profile ID for width calculation
		displayID := id
		if id == pm.config.Current {
			displayID = "→ " + id
		}
		if len(displayID) > maxIDLen {
			maxIDLen = len(displayID)
		}

		// Calculate masked key width
		maskedKey := pm.GetMaskedAPIKey(profile.APIKey)
		if len(maskedKey) > maxKeyLen {
			maxKeyLen = len(maskedKey)
		}

		if len(profile.GetOrgName()) > maxOrgLen {
			maxOrgLen = len(profile.GetOrgName())
		}
		if showBaseURL {
			baseURL := profile.BaseURL
			if baseURL == "" {
				baseURL = pm.config.Defaults.BaseURL + " (default)"
			}
			if len(baseURL) > maxBaseURLLen {
				maxBaseURLLen = len(baseURL)
			}
		}
	}

	// Add some padding
	maxIDLen += 2
	maxKeyLen += 2
	maxOrgLen += 2
	maxBaseURLLen += 2

	// Print header based on whether base URL column is needed
	if showBaseURL {
		fmt.Printf("  %-*s %-*s %-*s %-*s\n", maxIDLen-2, "ID", maxKeyLen, "Key", maxOrgLen, "Organization", maxBaseURLLen, "Base URL")
		fmt.Println("  " + strings.Repeat("-", maxIDLen+maxKeyLen+maxOrgLen+maxBaseURLLen))
	} else {
		fmt.Printf("  %-*s %-*s %-*s\n", maxIDLen-2, "ID", maxKeyLen, "Key", maxOrgLen, "Organization")
		fmt.Println("  " + strings.Repeat("-", maxIDLen+maxKeyLen+maxOrgLen-1))
	}

	// Print profiles
	for id, profile := range pm.config.Profiles {
		isCurrent := id == pm.config.Current

		// Get masked key
		maskedKey := pm.GetMaskedAPIKey(profile.APIKey)

		if showBaseURL {
			baseURL := profile.BaseURL
			if baseURL == "" {
				baseURL = pm.config.Defaults.BaseURL + " (default)"
			}
			if isCurrent {
				fmt.Print("\033[32m→ ") // Color the arrow and space
				fmt.Printf("\033[32m%-*s\033[0m %-*s %-*s %-*s\n", maxIDLen-2, id, maxKeyLen, maskedKey, maxOrgLen, profile.GetOrgName(), maxBaseURLLen, baseURL)
			} else {
				fmt.Printf("  %-*s %-*s %-*s %-*s\n", maxIDLen-2, id, maxKeyLen, maskedKey, maxOrgLen, profile.GetOrgName(), maxBaseURLLen, baseURL)
			}
		} else {
			if isCurrent {
				fmt.Print("\033[32m→ ") // Color the arrow and space
				fmt.Printf("\033[32m%-*s\033[0m %-*s %-*s\n", maxIDLen-2, id, maxKeyLen, maskedKey, maxOrgLen, profile.GetOrgName())
			} else {
				fmt.Printf("  %-*s %-*s %-*s\n", maxIDLen-2, id, maxKeyLen, maskedKey, maxOrgLen, profile.GetOrgName())
			}
		}
	}
}

// Add adds a new profile
func (pm *ProfileManager) Add(id, org, key, baseURL string) error {
	// Determine base URL with priority: provided baseURL > config default
	if baseURL == "" {
		baseURL = config.GetBaseURL()
	}

	// Store the effective base URL for this profile
	effectiveBaseURL := baseURL

	// Always get org info from API to ensure we have both name and slug
	var orgSlug string
	orgInfo, err := pm.getOrgInfoFromAPI(key, effectiveBaseURL)
	if err != nil {
		return fmt.Errorf("failed to validate API key and get organization info: %v", err)
	}

	// Use provided org name if available, otherwise use API response
	if org == "" {
		org = orgInfo.Name
	}
	orgSlug = orgInfo.Slug

	// Generate ID if not provided
	if id == "" {
		// Check if base URL indicates a specific environment
		if strings.Contains(effectiveBaseURL, "staging") {
			id = "staging"
		} else if strings.Contains(effectiveBaseURL, "localhost") || strings.Contains(effectiveBaseURL, "127.0.0.1") {
			id = "local"
		} else if effectiveBaseURL == config.DefaultBaseURL {
			id = "default"
		} else {
			// For other URLs, use the hostname
			u, err := url.Parse(effectiveBaseURL)
			if err == nil && u.Host != "" {
				id = normalizeID(u.Host)
			} else {
				id = normalizeID(effectiveBaseURL)
			}
		}
	} else {
		// Normalize the provided ID
		id = normalizeID(id)
	}

	// Ensure unique ID
	originalID := id
	counter := 1
	for {
		if _, exists := pm.config.Profiles[id]; !exists {
			break
		}
		id = fmt.Sprintf("%s_%d", originalID, counter)
		counter++
	}

	// Check if profile with same org and base URL already exists (for override)
	encodedKey := base64.StdEncoding.EncodeToString([]byte(key))
	var existingProfileID string
	var duplicateAPIKeyID string

	// First pass: check for org and base URL combination
	for existingID, existingProfile := range pm.config.Profiles {
		if existingProfile.GetOrgName() == org {
			existingBaseURL := existingProfile.BaseURL
			if existingBaseURL == "" {
				existingBaseURL = pm.config.Defaults.BaseURL
			}
			if existingBaseURL == baseURL {
				existingProfileID = existingID
				break
			}
		}
	}

	// Second pass: check for duplicate API key only if no org/base_url match found
	if existingProfileID == "" {
		for existingID, existingProfile := range pm.config.Profiles {
			if existingProfile.APIKey == encodedKey {
				duplicateAPIKeyID = existingID
				break
			}
		}
	}

	// If we found a profile with the same API key, use it for override (update org info)
	if existingProfileID == "" && duplicateAPIKeyID != "" {
		existingProfileID = duplicateAPIKeyID
	}

	// Create profile, always store the effective base URL
	profile := Profile{
		OrgName: org,
		OrgSlug: orgSlug, // Store orgSlug
		APIKey:  encodedKey,
	}

	// Always store the effective base URL in the profile
	profile.BaseURL = effectiveBaseURL

	// Override existing profile if same org and base URL combination exists
	if existingProfileID != "" {
		pm.config.Profiles[existingProfileID] = profile
		// Always set as current when overriding
		pm.config.Current = existingProfileID
	} else {
		// Add new profile
		pm.config.Profiles[id] = profile
		// Always set as current for new profile
		pm.config.Current = id
	}

	return pm.Save()
}

// Use sets the current profile
func (pm *ProfileManager) Use(id string) error {
	if len(pm.config.Profiles) == 0 {
		return fmt.Errorf("no profiles available, please add a profile first")
	}

	// Check if profile exists
	if _, exists := pm.config.Profiles[id]; !exists {
		return fmt.Errorf("profile '%s' not found", id)
	}

	pm.config.Current = id
	return pm.Save()
}

// Remove removes the specified profile
func (pm *ProfileManager) Remove(id string) error {
	if _, exists := pm.config.Profiles[id]; !exists {
		return fmt.Errorf(ErrProfileNotFound, id)
	}

	// Check if trying to delete current profile
	if id == pm.config.Current && len(pm.config.Profiles) > 1 {
		return errors.New(ErrCannotDeleteCurrent)
	}

	// Remove specified profile
	delete(pm.config.Profiles, id)

	// If there are still profiles after deletion and no current profile, set the first one as current
	if len(pm.config.Profiles) > 0 && pm.config.Current == "" {
		for existingID := range pm.config.Profiles {
			pm.config.Current = existingID
			break
		}
	}

	return pm.Save()
}

// GetCurrent gets the current profile with default values filled in
func (pm *ProfileManager) GetCurrent() *Profile {
	if pm.config.Current == "" {
		return nil
	}

	if profile, exists := pm.config.Profiles[pm.config.Current]; exists {
		// Create a copy to prevent external modification
		profileCopy := profile
		// Fill in default values if not set
		if profileCopy.BaseURL == "" {
			profileCopy.BaseURL = pm.config.Defaults.BaseURL
		}
		return &profileCopy
	}
	return nil
}

// GetProfile gets a specific profile by ID
func (pm *ProfileManager) GetProfile(id string) *Profile {
	if profile, exists := pm.config.Profiles[id]; exists {
		// Return a copy to prevent external modification
		profileCopy := profile
		return &profileCopy
	}
	return nil
}

// GetOrgName returns the organization name with backward compatibility
func (p *Profile) GetOrgName() string {
	// Prefer org_name, fallback to org for backward compatibility
	if p.OrgName != "" {
		return p.OrgName
	}
	return p.Org
}

// GetProfiles returns a copy of all profiles to prevent external modification
func (pm *ProfileManager) GetProfiles() map[string]Profile {
	profiles := make(map[string]Profile, len(pm.config.Profiles))
	for id, profile := range pm.config.Profiles {
		profiles[id] = profile
	}
	return profiles
}

// GetCurrentAPIKey gets the decoded API key from the current profile
func (pm *ProfileManager) GetCurrentAPIKey() (string, error) {
	current := pm.GetCurrent()
	if current == nil {
		return "", errors.New(ErrNoCurrentProfile)
	}

	if current.APIKey == "" {
		return "", errors.New(ErrNoAPIKey)
	}

	return pm.DecodeAPIKey(current.APIKey)
}

// DecodeAPIKey decodes a base64 encoded API key
func (pm *ProfileManager) DecodeAPIKey(encodedKey string) (string, error) {
	if encodedKey == "" {
		return "", errors.New(ErrEmptyAPIKey)
	}

	decodedBytes, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil {
		return "", fmt.Errorf("failed to decode API key: %v", err)
	}

	return string(decodedBytes), nil
}

// GetMaskedAPIKey gets the masked version of an API key
func (pm *ProfileManager) GetMaskedAPIKey(encodedKey string) string {
	if encodedKey == "" {
		return "***"
	}

	decodedKey, err := pm.DecodeAPIKey(encodedKey)
	if err != nil {
		return "***"
	}

	return maskAPIKey(decodedKey)
}

// GetCurrentProfileID gets the current profile ID
func (pm *ProfileManager) GetCurrentProfileID() string {
	return pm.config.Current
}

// HasCurrentProfile checks if a current profile is set
func (pm *ProfileManager) HasCurrentProfile() bool {
	return pm.config.Current != "" && pm.GetCurrent() != nil
}

// GetProfileCount returns the number of profiles
func (pm *ProfileManager) GetProfileCount() int {
	return len(pm.config.Profiles)
}

// GetProfileIDs returns a slice of profile IDs
func (pm *ProfileManager) GetProfileIDs() []string {
	ids := make([]string, 0, len(pm.config.Profiles))
	for id := range pm.config.Profiles {
		ids = append(ids, id)
	}
	return ids
}

// ProfileExists checks if a profile with the given ID exists
func (pm *ProfileManager) ProfileExists(id string) bool {
	_, exists := pm.config.Profiles[id]
	return exists
}

// checkConfigurationMismatch checks for potential configuration issues
// and warns if environment variables don't match profile configuration
func (pm *ProfileManager) checkConfigurationMismatch() {
	envBaseURL := os.Getenv("GBOX_BASE_URL")
	envAPIKey := os.Getenv("GBOX_API_KEY")

	// Skip check if no environment base URL is set
	if envBaseURL == "" {
		return
	}

	// Skip check if environment API key is set (user is overriding everything)
	if envAPIKey != "" {
		return
	}

	current := pm.GetCurrent()
	if current == nil {
		return
	}

	// Get the effective base URL from profile (with defaults)
	profileBaseURL := current.BaseURL
	if profileBaseURL == "" {
		profileBaseURL = pm.config.Defaults.BaseURL
	}

	// Normalize URLs for comparison (remove trailing slash)
	envBaseURL = strings.TrimSuffix(envBaseURL, "/")
	profileBaseURL = strings.TrimSuffix(profileBaseURL, "/")

	// Warn if they don't match
	if envBaseURL != profileBaseURL {
		fmt.Fprintf(os.Stderr, "Warning: GBOX_BASE_URL environment variable (%s) differs from profile base URL (%s). "+
			"This may cause connection issues. Consider setting GBOX_API_KEY or updating your profile.\n",
			envBaseURL, profileBaseURL)
	}
}

// GetDefaultBaseURL gets the default base URL
func (pm *ProfileManager) GetDefaultBaseURL() string {
	return pm.config.Defaults.BaseURL
}

// GetEffectiveBaseURL gets the effective base URL with priority: GBOX_BASE_URL > profile > config default
func (pm *ProfileManager) GetEffectiveBaseURL() string {
	var baseURL string

	// First priority: GBOX_BASE_URL environment variable
	if envURL := os.Getenv("GBOX_BASE_URL"); envURL != "" {
		baseURL = envURL
	} else {
		// Second priority: current profile's base URL
		// Get effective base URL from current profile (includes profile defaults)
		current := pm.GetCurrent()
		if current != nil {
			baseURL = current.BaseURL
		} else {
			// No current profile, use config default
			baseURL = config.GetBaseURL()
		}
	}

	// Trim trailing slash for consistency
	return strings.TrimSuffix(baseURL, "/")
}

// GetEffectiveAPIKey gets the effective API key with priority: GBOX_API_KEY > profile
func (pm *ProfileManager) GetEffectiveAPIKey() (string, error) {
	// First priority: GBOX_API_KEY environment variable
	if envAPIKey := os.Getenv("GBOX_API_KEY"); envAPIKey != "" {
		// Environment variable is already decoded (plain text)
		return envAPIKey, nil
	}

	// Second priority: current profile's API key
	current := pm.GetCurrent()
	if current == nil {
		return "", errors.New(ErrNoCurrentProfile)
	}

	if current.APIKey == "" {
		return "", errors.New(ErrNoAPIKey)
	}

	// Decode profile API key (it's base64-encoded at rest)
	return pm.DecodeAPIKey(current.APIKey)
}

// normalizeID normalizes an ID string
func normalizeID(id string) string {
	// Convert to lowercase and replace spaces/underscores with hyphens
	normalized := strings.ToLower(id)
	normalized = strings.ReplaceAll(normalized, " ", "-")
	normalized = strings.ReplaceAll(normalized, "_", "-")

	// Remove special characters
	var result strings.Builder
	for _, char := range normalized {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-' {
			result.WriteRune(char)
		}
	}

	normalized = result.String()

	// Ensure it's not empty
	if normalized == "" {
		normalized = "profile"
	}

	return normalized
}

// listJSON displays profiles in JSON format
func (pm *ProfileManager) listJSON() {
	// Create a slice to hold profile data
	profiles := make([]map[string]interface{}, 0, len(pm.config.Profiles))

	for id, profile := range pm.config.Profiles {
		profileData := map[string]interface{}{
			"id":      id,
			"org":     profile.GetOrgName(),
			"key":     pm.GetMaskedAPIKey(profile.APIKey),
			"current": id == pm.config.Current,
		}

		// Include org_slug if available
		if profile.OrgSlug != "" {
			profileData["org_slug"] = profile.OrgSlug
		}

		// Only include base_url if it's different from default
		if profile.BaseURL != "" && profile.BaseURL != pm.config.Defaults.BaseURL {
			profileData["base_url"] = profile.BaseURL
		}

		profiles = append(profiles, profileData)
	}

	// Output JSON
	fmt.Printf("%s\n", toJSON(profiles))
}

// maskAPIKey masks an API key for display
func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return "***"
	}
	return key[:4] + "****" + key[len(key)-4:]
}

// toJSON converts data to JSON string
func toJSON(data interface{}) string {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(jsonData)
}

// getOrgInfoFromAPI tries to get organization info from API using the provided API key
// Returns OrgInfo and error. If error is nil, the API key is valid.
func (pm *ProfileManager) getOrgInfoFromAPI(apiKey, baseURL string) (*OrgInfo, error) {
	// API key from command line is plain text, not base64 encoded
	// Make HTTP request to get organization info
	client := &http.Client{}
	// Ensure baseURL doesn't end with slash to avoid double slashes
	baseEndpoint := strings.TrimSuffix(baseURL, "/")
	// Remove /api/v1 suffix if present, as we'll add the correct path
	baseEndpoint = strings.TrimSuffix(baseEndpoint, "/api/v1")

	req, err := http.NewRequest("GET", baseEndpoint+"/api/v1/org", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// Set API key in header (plain text)
	req.Header.Set("X-API-Key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("invalid API key")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Read response body for debugging
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// Debug: print response body
	if os.Getenv("DEBUG") == "true" {
		fmt.Fprintf(os.Stderr, "Response body: %s\n", string(body))
	}

	// Parse response
	var orgInfo struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	if err := json.Unmarshal(body, &orgInfo); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v, body: %s", err, string(body))
	}

	if orgInfo.Name == "" {
		return nil, fmt.Errorf("organization name is empty")
	}

	return &OrgInfo{
		Name: orgInfo.Name,
		Slug: orgInfo.Slug,
	}, nil
}

// GetDevicesURL returns the devices URL for the current profile
func (pm *ProfileManager) GetDevicesURL() (string, error) {
	current := pm.GetCurrent()
	if current == nil {
		return "", errors.New(ErrNoCurrentProfile)
	}

	devicesURL := pm.buildDevicesURL(current)
	if devicesURL == "" {
		return "", fmt.Errorf("current profile does not have org_slug. Please run 'gbox profile add' to update your profile")
	}

	return devicesURL, nil
}

// GetDevicesURLByID returns the devices URL for a specific profile by ID
func (pm *ProfileManager) GetDevicesURLByID(id string) (string, error) {
	profile := pm.GetProfile(id)
	if profile == nil {
		return "", fmt.Errorf(ErrProfileNotFound, id)
	}

	devicesURL := pm.buildDevicesURL(profile)
	if devicesURL == "" {
		return "", fmt.Errorf("profile '%s' does not have org_slug. Please run 'gbox profile add' to update your profile", id)
	}

	return devicesURL, nil
}

// buildDevicesURL builds the devices URL for a given profile
func (pm *ProfileManager) buildDevicesURL(profile *Profile) string {
	baseURL := profile.BaseURL
	if baseURL == "" {
		baseURL = pm.config.Defaults.BaseURL
	}

	// Ensure baseURL doesn't end with slash
	baseEndpoint := strings.TrimSuffix(baseURL, "/")
	baseEndpoint = strings.TrimSuffix(baseEndpoint, "/api/v1")

	// If org_slug is available, use it to build the devices URL
	if profile.OrgSlug != "" {
		return fmt.Sprintf("%s/%s/devices", baseEndpoint, profile.OrgSlug)
	}

	// If no org_slug (old profile), return empty string
	// This indicates that the profile needs to be updated with org_slug
	return ""
}

// needsMigration checks if any base URLs need to be migrated to the new format
func (pm *ProfileManager) needsMigration() bool {
	// Check defaults.base_url
	if pm.needsURLMigration(pm.config.Defaults.BaseURL) {
		return true
	}

	// Check all profile base URLs
	for _, profile := range pm.config.Profiles {
		if pm.needsURLMigration(profile.BaseURL) {
			return true
		}
	}

	return false
}

// needsURLMigration checks if a specific URL needs migration
func (pm *ProfileManager) needsURLMigration(url string) bool {
	if url == "" {
		return false
	}

	// Remove trailing slash for comparison
	cleanURL := strings.TrimSuffix(url, "/")

	// Check if URL needs migration (doesn't end with /api/v1)
	return !strings.HasSuffix(cleanURL, "/api/v1")
}

// performMigration migrates all base URLs from old format to new format
func (pm *ProfileManager) performMigration() {
	migrated := false

	// Migrate defaults.base_url
	if pm.needsURLMigration(pm.config.Defaults.BaseURL) {
		pm.config.Defaults.BaseURL = pm.migrateURL(pm.config.Defaults.BaseURL)
		migrated = true
	}

	// Migrate all profile base URLs
	for id, profile := range pm.config.Profiles {
		if pm.needsURLMigration(profile.BaseURL) {
			profile.BaseURL = pm.migrateURL(profile.BaseURL)
			pm.config.Profiles[id] = profile
			migrated = true
		}
	}

	// Save the migrated configuration
	if migrated {
		if err := pm.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to save migrated configuration: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "Info: migrated base URLs to new format\n")
		}
	}
}

// migrateURL migrates a single URL to the new format
func (pm *ProfileManager) migrateURL(url string) string {
	if url == "" {
		return config.GetBaseURL()
	}

	// Remove trailing slash
	cleanURL := strings.TrimSuffix(url, "/")

	// If it's the main gbox.ai domain, use the default
	if cleanURL == "https://gbox.ai" {
		return config.GetBaseURL()
	}

	// If URL already ends with /api/v1, return as is
	if strings.HasSuffix(cleanURL, "/api/v1") {
		return cleanURL
	}

	// For other domains, append /api/v1
	return cleanURL + "/api/v1"
}
