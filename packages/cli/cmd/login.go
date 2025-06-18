package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/babelcloud/gbox/packages/cli/config"

	"github.com/adrg/xdg"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

const (
	configDirName   = "gbox"
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

	apiURL := config.GetAPIURL() + "/api/v1/auth/github/callback/token"
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

	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create config directory: %v", err)
	}

	credentials := map[string]string{
		"token": tokenResp.Token,
	}
	credentialsData, err := json.MarshalIndent(credentials, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to serialize credentials: %v", err)
	}

	if err := os.WriteFile(credentialsPath, credentialsData, 0o600); err != nil {
		return "", fmt.Errorf("failed to save credentials: %v", err)
	}

	return tokenResp.Token, nil
}

func init() {
	rootCmd.AddCommand(loginCmd)
}
