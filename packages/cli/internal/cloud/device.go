package cloud

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/babelcloud/gbox/packages/cli/internal/profile"
	"github.com/pkg/errors"
)

type Device struct {
	Id        string `json:"id,omitempty"`
	Ownership string `json:"ownership,omitempty"`
	OwnerId   string `json:"ownerId,omitempty"`
	Metadata  struct {
		Serialno  string `json:"serialno,omitempty"`
		AndroidId string `json:"androidId,omitempty"`
	} `json:"metadata,omitzero"`
	Labels        map[string]string `json:"labels,omitempty"`
	AccessPointId string            `json:"accessPointId,omitempty"`
	Connected     bool              `json:"connected,omitempty"`
	Available     bool              `json:"available,omitempty"`
	LastOnlineAt  time.Time         `json:"lastOnlineAt,omitzero"`
}

type DeviceList struct {
	Data     []*Device `json:"data"`
	Page     int       `json:"page"`
	PageSize int       `json:"pageSize"`
	Total    int       `json:"total"`
}

type AccessPointToken struct {
	Token string `json:"token"`
}

type DeviceAPI struct {
	client  *http.Client
	profile *profile.Profile
}

func NewDeviceAPI() *DeviceAPI {
	return &DeviceAPI{
		client:  &http.Client{},
		profile: profile.Default.GetCurrent(),
	}
}

func (d *DeviceAPI) GetBySerialnoAndAndroidId(serialno string, androidId string) (*DeviceList, error) {
	queries := url.Values{}
	queries.Set("serialno", serialno)
	queries.Set("androidId", androidId)

	url, err := d.buildUrlFromEndpoint("/api/v1/devices")
	if err != nil {
		return nil, errors.Wrap(err, "failed to build url")
	}

	url.RawQuery = queries.Encode()

	req, err := http.NewRequest(http.MethodGet, url.String(), nil)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create request from url: %s", url.String())
	}

	d.setCommonRequestHeaders(req)

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get devices: %s", url.String())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, errors.Errorf("get devices api respond %d: %s", resp.StatusCode, string(body))
	}

	decoder := json.NewDecoder(resp.Body)
	deviceList := &DeviceList{}
	if err := decoder.Decode(deviceList); err != nil {
		return nil, errors.Wrapf(err, "failed to parse response from get devices api")
	}

	return deviceList, nil
}

func (d *DeviceAPI) Create(device *Device) (*Device, error) {
	url, err := d.buildUrlFromEndpoint("/api/v1/devices")

	if err != nil {
		return nil, errors.Wrapf(err, "failed to build url")
	}

	reqBody, err := json.Marshal(device)
	if err != nil {
		return nil, errors.Wrap(err, "fail to marshal device to json")
	}

	req, err := http.NewRequest(http.MethodPost, url.String(), bytes.NewReader(reqBody))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create request from url: %s", url.String())
	}

	d.setCommonRequestHeaders(req)

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to post devices: %s", url.String())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, errors.Errorf("post devices api respond %d: %s", resp.StatusCode, string(body))
	}

	decoder := json.NewDecoder(resp.Body)
	device = &Device{}
	if err := decoder.Decode(device); err != nil {
		return nil, errors.Wrapf(err, "failed to parse response from post devices api")
	}

	return device, nil
}

func (d *DeviceAPI) Delete(deviceId string) error {
	url, err := d.buildUrlFromEndpoint(path.Join("/api/v1/devices", deviceId))
	if err != nil {
		return errors.Wrapf(err, "failed to build url")
	}

	req, err := http.NewRequest(http.MethodDelete, url.String(), nil)
	if err != nil {
		return errors.Wrapf(err, "failed to create request from url: %s", url.String())
	}

	d.setCommonRequestHeaders(req)

	resp, err := d.client.Do(req)
	if err != nil {
		return errors.Wrapf(err, "failed to delete devices: %s", url.String())
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusNoContent {
		return errors.Errorf("delete devices api respond %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (d *DeviceAPI) GenerateAccessPointToken(deviceId, requestEndpoint string) (*AccessPointToken, error) {
	url, err := d.buildUrlFromEndpoint(path.Join("/api/v1/devices", deviceId, "generate-access-point-token"))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to build url")
	}

	reqBody, err := json.Marshal(map[string]any{
		"requestEndpoint": requestEndpoint,
	})
	if err != nil {
		return nil, errors.Wrap(err, "fail to marshal generate access point request body to json")
	}

	req, err := http.NewRequest(http.MethodPost, url.String(), bytes.NewReader(reqBody))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create request from url: %s", url.String())
	}

	d.setCommonRequestHeaders(req)

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to generate access point token: %s", url.String())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, errors.Errorf("generate access point token api respond %d: %s", resp.StatusCode, string(body))
	}

	decoder := json.NewDecoder(resp.Body)
	apToken := &AccessPointToken{}
	if err := decoder.Decode(apToken); err != nil {
		return nil, errors.Wrapf(err, "failed to parse response from generate access point token api")
	}

	return apToken, nil
}

func (d *DeviceAPI) buildUrlFromEndpoint(endpoint string) (*url.URL, error) {
	url, err := url.Parse(d.profile.BaseURL)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse base url: %s", d.profile.BaseURL)
	}

	url.Path = endpoint

	return url, nil
}

func (d *DeviceAPI) setCommonRequestHeaders(req *http.Request) {
	req.Header.Set("x-device-ap", "true")
	req.Header.Set("content-type", "application/json")
	decodedBytes, _ := base64.StdEncoding.DecodeString(d.profile.APIKey)
	apiKey := string(decodedBytes)
	if strings.HasPrefix(apiKey, "gbox-rack_") {
		req.Header.Set("x-rack-api-key", apiKey)
	} else {
		req.Header.Set("x-api-key", apiKey)
	}
}
