package cloud

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/babelcloud/gbox/packages/cli/internal/profile"
)

type Client struct {
	httpClient *http.Client
	token      string
	baseURL    string
}

type Organization struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Add other fields from the API response as needed
}

type CreateAPIKeyRequest struct {
	KeyName string `json:"name"`
	OrgID   string `json:"organizationId"`
}

type CreateAPIKeyResponse struct {
	ID        string `json:"id"`
	KeyName   string `json:"name"`
	APIKey    string `json:"key"`
	OrgID     string `json:"organizationId"`
	CreatedAt string `json:"createdAt"`
	// Add other fields from the API response as needed
}

type CreateOrganizationRequest struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Website     string `json:"website,omitempty"`
	CompanySize string `json:"companySize,omitempty"`
	Industry    string `json:"industry,omitempty"`
	Logo        string `json:"logo,omitempty"`
}

func NewClient(token string) (*Client, error) {
	if token == "" {
		return nil, fmt.Errorf("token cannot be empty")
	}

	// Get base URL with proper priority handling
	baseURL := profile.Default.GetEffectiveBaseURL()

	return &Client{
		httpClient: &http.Client{},
		token:      token,
		baseURL:    baseURL,
	}, nil
}

func (c *Client) GetMyOrganizationList() ([]Organization, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/dashboard/v1/organization/get_my_organization_list", c.baseURL), nil)
	if err != nil {
		return nil, err
	}

	req.AddCookie(&http.Cookie{
		Name:  "token",
		Value: c.token,
	})
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get organization list: status code %d, body: %s", resp.StatusCode, string(body))
	}

	var organizations []Organization
	if err := json.NewDecoder(resp.Body).Decode(&organizations); err != nil {
		return nil, err
	}

	return organizations, nil
}

func (c *Client) CreateAPIKey(keyName, orgID string) (*CreateAPIKeyResponse, error) {
	requestBody := CreateAPIKeyRequest{
		KeyName: keyName,
		OrgID:   orgID,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/dashboard/v1/api_key/create_an_api_key", c.baseURL), bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.AddCookie(&http.Cookie{
		Name:  "token",
		Value: c.token,
	})
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create API key: status code %d, body: %s", resp.StatusCode, string(body))
	}

	var response CreateAPIKeyResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return &response, nil
}

func (c *Client) CreateOrganization(reqBody CreateOrganizationRequest) (*Organization, error) {
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/dashboard/v1/organization/create_an_organization", c.baseURL), bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.AddCookie(&http.Cookie{
		Name:  "token",
		Value: c.token,
	})
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create organization: status code %d, body: %s", resp.StatusCode, string(body))
	}

	var organization Organization
	if err := json.NewDecoder(resp.Body).Decode(&organization); err != nil {
		return nil, err
	}

	return &organization, nil
}

type UserInfo struct {
	ID    string  `json:"id"`
	Name  *string `json:"name"`
	Email *string `json:"email"`
}

func (c *Client) GetCurrentUserInfo() (*UserInfo, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/dashboard/v1/user/get_current_user_info", c.baseURL), nil)
	if err != nil {
		return nil, err
	}

	req.AddCookie(&http.Cookie{
		Name:  "token",
		Value: c.token,
	})
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get current user info: status code %d, body: %s", resp.StatusCode, string(body))
	}

	var info UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}

	return &info, nil
}
