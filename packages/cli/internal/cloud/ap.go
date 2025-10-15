package cloud

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/babelcloud/gbox/packages/cli/internal/profile"
	"github.com/pkg/errors"
)

type AccessPoint struct {
	Id       string `json:"id"`
	Endpoint string `json:"endpoint"`
	Metadata struct {
		Country   string `json:"country"`
		Region    string `json:"region"`
		City      string `json:"city"`
		Latitude  string `json:"latitude"`
		Longitude string `json:"longitude"`
		Timezone  string `json:"timezone"`
		Protocol  string `json:"protocol"`
	} `json:"metadata"`
}

type AccessPointList struct {
	Data     []*AccessPoint `json:"data"`
	Page     int            `json:"page"`
	PageSize int            `json:"pageSize"`
	Total    int            `json:"total"`
}

type AccessPointAPI struct {
	client  *http.Client
	profile *profile.Profile
}

func NewAccessPointAPI() *AccessPointAPI {
	return &AccessPointAPI{
		client:  &http.Client{},
		profile: profile.Default.GetCurrent(),
	}
}

func (ap *AccessPointAPI) List() (*AccessPointList, error) {
	url, err := ap.buildUrlFromEndpoint("/api/v1/access-points")
	if err != nil {
		return nil, errors.Wrap(err, "failed to build url")
	}

	req, err := http.NewRequest(http.MethodGet, url.String(), nil)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create request from url: %s", url.String())
	}

	ap.setCommonRequestHeaders(req)

	resp, err := ap.client.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get access points: %s", url.String())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, errors.Errorf("get access points api respond %d: %s", resp.StatusCode, string(body))
	}

	decoder := json.NewDecoder(resp.Body)
	apList := &AccessPointList{}
	if err := decoder.Decode(apList); err != nil {
		return nil, errors.Wrapf(err, "failed to parse response from get access points api")
	}

	return apList, nil
}

func (d *AccessPointAPI) buildUrlFromEndpoint(endpoint string) (*url.URL, error) {
	url, err := url.Parse(d.profile.BaseURL)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse base url: %s", d.profile.BaseURL)
	}

	url.Path = endpoint

	return url, nil
}

func (d *AccessPointAPI) setCommonRequestHeaders(req *http.Request) {
	req.Header.Set("content-type", "application/json")
	decodedBytes, _ := base64.StdEncoding.DecodeString(d.profile.APIKey)
	apiKey := string(decodedBytes)
	if strings.HasPrefix(apiKey, "gbox-rack_") {
		req.Header.Set("x-rack-api-key", apiKey)
	} else {
		req.Header.Set("x-api-key", apiKey)
	}
}
