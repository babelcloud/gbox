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
	"time"

	"github.com/babelcloud/gbox/packages/cli/config"
	"github.com/babelcloud/gbox/packages/cli/internal/cloud"

	"os/exec"
	"runtime"

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

	// Updated OAuth configuration with new parameters
	oauth2Config = &oauth2.Config{
		ClientID:     "Ov23liVORYhMLpBvAtMs",
		ClientSecret: config.GetGithubClientSecret(),
		RedirectURL:  "http://localhost:18088/login/github",
		Scopes:       []string{"user:email"},
		Endpoint:     github.Endpoint,
	}
)

type TokenResponse struct {
	Token string `json:"token"`
}

// checkBrowserEnvironment checks if the user has a browser environment (GUI and browser available)
func checkBrowserEnvironment() bool {
	// On Windows or Mac, assume browser is available
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return true
	}
	// If running in SSH session and no DISPLAY/WAYLAND, assume no GUI
	if os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_TTY") != "" {
		if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
			return false
		}
	}
	// If no GUI session variables, assume no GUI
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" && os.Getenv("XDG_SESSION_TYPE") == "" {
		return false
	}
	// If BROWSER env is set, assume browser is available
	if os.Getenv("BROWSER") != "" {
		return true
	}
	// If xdg-open is available, assume browser can be launched
	if _, err := exec.LookPath("xdg-open"); err == nil {
		return true
	}
	// Check for common browser executables in PATH
	browsers := []string{"firefox", "google-chrome", "chromium", "brave", "opera", "konqueror"}
	for _, b := range browsers {
		if _, err := exec.LookPath(b); err == nil {
			return true
		}
	}
	// No browser environment detected
	return false
}

// startLocalServer starts a local server to handle OAuth callback
func startLocalServer(codeChan chan string, errorChan chan error) {
	server := &http.Server{
		Addr: ":18088",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/login/github" {
				code := r.URL.Query().Get("code")
				if code != "" {
					w.Write([]byte("<html><body><h1>Authentication successful!</h1><p>You can close this window and return to the terminal.</p></body></html>"))
					codeChan <- code
				} else {
					errorChan <- fmt.Errorf("no authorization code received")
				}
			} else {
				http.NotFound(w, r)
			}
		}),
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errorChan <- err
		}
	}()

	// Give server time to start
	time.Sleep(1 * time.Second)
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login using GitHub OAuth",
	Long:  `Authenticate using GitHub OAuth. This will detect your environment and use the appropriate authentication method.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		
		// Check if user has browser environment
		hasBrowser := checkBrowserEnvironment()
		
		var accessToken string
		var err error
		
		if hasBrowser {
			fmt.Println("Browser environment detected. Using OAuth authorization code flow...")
			accessToken, err = authenticateWithBrowser(ctx)
		} else {
			fmt.Println("No browser detected. Using device authorization flow...")
			accessToken, err = authenticateWithDevice(ctx)
		}
		
		if err != nil {
			return fmt.Errorf("authentication failed: %v", err)
		}

		_, err = getLocalToken(accessToken)
		if err != nil {
			return fmt.Errorf("failed to get local token: %v", err)
		}

		return nil
	},
}

// authenticateWithBrowser uses OAuth authorization code flow
func authenticateWithBrowser(ctx context.Context) (string, error) {
	// Start local server to handle callback
	codeChan := make(chan string, 1)
	errorChan := make(chan error, 1)
	startLocalServer(codeChan, errorChan)

	// Generate authorization URL
	authURL := oauth2Config.AuthCodeURL("state", oauth2.AccessTypeOffline)
	
	fmt.Println("Opening browser for authentication...")

	debugMode := os.Getenv("DEBUG") == "true"
	if debugMode {
		fmt.Printf("If the browser does not open automatically, please visit the following URL manually:\n%s\n", authURL)
	}
	if err := browser.OpenURL(authURL); err != nil {
		fmt.Println("Failed to open browser automatically. Please copy and paste the URL above into your browser.")
	}

	// Wait for authorization code
	select {
	case code := <-codeChan:
		// Exchange code for token
		token, err := oauth2Config.Exchange(ctx, code)
		if err != nil {
			return "", fmt.Errorf("failed to exchange code for token: %v", err)
		}
		return token.AccessToken, nil
	case err := <-errorChan:
		return "", fmt.Errorf("authentication error: %v", err)
	case <-time.After(5 * time.Minute):
		return "", fmt.Errorf("authentication timeout")
	}
}

// authenticateWithDevice uses OAuth device authorization flow
func authenticateWithDevice(ctx context.Context) (string, error) {
	deviceAuth, err := oauth2Config.DeviceAuth(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get device code: %v", err)
	}

	fmt.Printf("Device code: %s\n", deviceAuth.UserCode)
	fmt.Printf("To authenticate, please visit: %s\n", deviceAuth.VerificationURI)
	fmt.Println("Attempting to open browser...")
	if err := browser.OpenURL(deviceAuth.VerificationURI); err != nil {
		fmt.Println("Failed to open browser automatically. Please copy and paste the URL above into your browser.")
	}

	fmt.Println("Waiting for authentication to complete...")
	token, err := oauth2Config.DeviceAccessToken(ctx, deviceAuth)
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %v", err)
	}

	return token.AccessToken, nil
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

	// Attempt to obtain the token either from the response body (JSON) or the 'token' cookie.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var tokenResp TokenResponse
	// 1. Try to parse token from JSON body (backward compatibility).
	if len(body) > 0 {
		_ = json.Unmarshal(body, &tokenResp) // ignore error; we'll fall back to cookie if needed
	}

	// 2. Fallback to 'token' cookie if JSON body didn't contain it.
	if tokenResp.Token == "" {
		for _, c := range resp.Cookies() {
			if c.Name == "token" {
				tokenResp.Token = c.Value
				break
			}
		}
	}

	if tokenResp.Token == "" {
		return "", fmt.Errorf("failed to obtain token from response, body: %s", string(body))
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
		fmt.Printf("Created API key: %s. Login process successfully.\n", apiKeyInfo.KeyName)
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
		fmt.Println("No organizations found for your account.")
		return nil, nil
	}

	if len(organizations) == 1 {
		org := organizations[0]
		fmt.Printf("Automatically selected organization: %s (%s)\n", org.Name, org.ID)
		return &org, nil
	}

	fmt.Println("Available organizations:")
	for i, org := range organizations {
		fmt.Printf("%d. %s (%s)\n", i+1, org.Name, org.ID)
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Select an organization by entering the number: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("failed to read input: %v", err)
		}

		input = strings.TrimSpace(input)
		choice, err := strconv.Atoi(input)
		if err != nil {
			fmt.Println("Invalid input. Please enter a valid number.")
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
