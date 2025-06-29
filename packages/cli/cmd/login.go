package cmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/babelcloud/gbox/packages/cli/config"
	"github.com/babelcloud/gbox/packages/cli/internal/cloud"

	"github.com/adrg/xdg"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

const (
	configDirName   = ".gbox"
	credentialsFile = "credentials.json"
)

var (
	configDir       = filepath.Join(xdg.Home, configDirName)
	credentialsPath = filepath.Join(configDir, credentialsFile)

	oauth2Config = &oauth2.Config{
		ClientID: "Ov23lilXASZX16JRBl7b",
		Scopes:   []string{"user:email"},
		Endpoint: github.Endpoint,
	}
)

type TokenResponse struct {
	Token string `json:"token"`
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login using GitHub OAuth Device Flow",
	Long:  `Authenticate using GitHub OAuth Device Flow. This method doesn't require opening a browser, but uses a device code to complete authentication.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		deviceAuth, err := oauth2Config.DeviceAuth(ctx)
		if err != nil {
			return fmt.Errorf("failed to get device code: %v", err)
		}

		fmt.Printf("Device code: %s\n", deviceAuth.UserCode)
		fmt.Printf("Please visit this link to complete authentication: %s\n", deviceAuth.VerificationURI)
		fmt.Println("Attempting to open browser...")
		if err := browser.OpenURL(deviceAuth.VerificationURI); err != nil {
			fmt.Println("Failed to open browser automatically, please visit the link above manually")
		}

		fmt.Println("Waiting for authentication...")
		token, err := oauth2Config.DeviceAccessToken(ctx, deviceAuth)
		if err != nil {
			return fmt.Errorf("failed to get access token: %v", err)
		}

		_, err = getLocalToken(token.AccessToken)
		if err != nil {
			return fmt.Errorf("failed to get local token: %v", err)
		}

		fmt.Println("SUCCESS")
		return nil
	},
}

func getLocalToken(githubToken string) (string, error) {
	reqBody := map[string]string{
		"token": githubToken,
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}
	
	apiURL := config.GetCloudAPIURL() + "/api/public/v1/auth/github/callback/token"
	resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %v, response content: %s", err, string(body))
	}

	// Get organization list and let user select
	selectedOrg, err := selectOrganization(tokenResp.Token)
	if err != nil {
		return "", fmt.Errorf("failed to select organization: %v", err)
	}

	// Create API key
	var apiKeyInfo *cloud.CreateAPIKeyResponse
	if selectedOrg != nil {
		client, err := cloud.NewClient(tokenResp.Token)
		if err != nil {
			return "", fmt.Errorf("failed to create cloud client: %v", err)
		}

		// Generate API key name
		apiKeyName := fmt.Sprintf("gbox-cli-%s", selectedOrg.Name)
		apiKeyInfo, err = client.CreateAPIKey(apiKeyName, selectedOrg.ID)
		if err != nil {
			return "", fmt.Errorf("failed to create API key: %v", err)
		}
		fmt.Printf("Created API key: %s\n", apiKeyInfo.KeyName)
	}

	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create config directory: %v", err)
	}

	credentials := map[string]string{
		"token": tokenResp.Token,
	}
	if selectedOrg != nil {
		credentials["organization_id"] = selectedOrg.ID
		credentials["organization_name"] = selectedOrg.Name
	}

	credentialsData, err := json.MarshalIndent(credentials, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to serialize credentials: %v", err)
	}

	if err := os.WriteFile(credentialsPath, credentialsData, 0o600); err != nil {
		return "", fmt.Errorf("failed to save credentials: %v", err)
	}

	// Save API key related data to profile.json
	if apiKeyInfo != nil && selectedOrg != nil {
		pm := NewProfileManager()
		if err := pm.Load(); err != nil {
			return "", fmt.Errorf("failed to load profile manager: %v", err)
		}

		if err := pm.Add(apiKeyInfo.APIKey, apiKeyInfo.KeyName, selectedOrg.Name); err != nil {
			return "", fmt.Errorf("failed to add profile: %v", err)
		}
	}

	return tokenResp.Token, nil
}

func selectOrganization(token string) (*cloud.Organization, error) {
	client, err := cloud.NewClient(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create cloud client: %v", err)
	}

	organizations, err := client.GetMyOrganizationList()
	if err != nil {
		return nil, fmt.Errorf("failed to get organization list: %v", err)
	}

	if len(organizations) == 0 {
		fmt.Println("No organizations found.")
		return nil, nil
	}

	if len(organizations) == 1 {
		org := organizations[0]
		fmt.Printf("Selected organization: %s (%s)\n", org.Name, org.ID)
		return &org, nil
	}

	fmt.Println("Available organizations:")
	for i, org := range organizations {
		fmt.Printf("%d. %s (%s)\n", i+1, org.Name, org.ID)
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Please select an organization (enter number): ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("failed to read input: %v", err)
		}

		input = strings.TrimSpace(input)
		choice, err := strconv.Atoi(input)
		if err != nil {
			fmt.Println("Invalid input. Please enter a number.")
			continue
		}

		if choice < 1 || choice > len(organizations) {
			fmt.Printf("Please enter a number between 1 and %d.\n", len(organizations))
			continue
		}

		selectedOrg := organizations[choice-1]
		fmt.Printf("Selected organization: %s (%s)\n", selectedOrg.Name, selectedOrg.ID)
		return &selectedOrg, nil
	}
}

func init() {
	rootCmd.AddCommand(loginCmd)
}
