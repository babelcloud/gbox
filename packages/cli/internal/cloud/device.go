package cloud

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
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
	RegId     string `json:"regId,omitempty"`
	Ownership string `json:"ownership,omitempty"`
	OwnerId   string `json:"ownerId,omitempty"`
	Metadata  struct {
		Serialno  string `json:"serialno,omitempty"`
		AndroidId string `json:"androidId,omitempty"`
		Type      string `json:"type,omitempty"`
		Resolution string `json:"resolution,omitempty"`
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

type Box struct {
	Id     string                 `json:"id,omitempty"`
	Type   string                 `json:"type,omitempty"`
	Status string                 `json:"status,omitempty"`
	Config map[string]interface{} `json:"config,omitempty"`
}

type AccessPointToken struct {
	Token string `json:"token"`
}

type DeviceAPI struct {
	client *http.Client
}

func NewDeviceAPI() *DeviceAPI {
	return &DeviceAPI{
		client: &http.Client{},
	}
}

// getCurrentProfile gets the current profile dynamically to support profile switching
func (d *DeviceAPI) getCurrentProfile() *profile.Profile {
	return profile.Default.GetCurrent()
}

// getDevices is a generic method to query devices with query parameters
func (d *DeviceAPI) getDevices(queries url.Values) (*DeviceList, error) {
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

func (d *DeviceAPI) GetBySerialnoAndAndroidId(serialno string, androidId string) (*DeviceList, error) {
	queries := url.Values{}
	queries.Set("serialno", serialno)
	queries.Set("androidId", androidId)
	return d.getDevices(queries)
}

func (d *DeviceAPI) GetByRegId(regId string) (*DeviceList, error) {
	queries := url.Values{}
	queries.Set("regId", regId)
	return d.getDevices(queries)
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

func (d *DeviceAPI) List(page, pageSize int) (*DeviceList, error) {
	u, err := d.buildUrlFromEndpoint("/api/v1/devices")
	if err != nil {
		return nil, errors.Wrap(err, "failed to build url")
	}
	q := u.Query()
	if page > 0 {
		q.Set("page", fmt.Sprintf("%d", page))
	}
	if pageSize > 0 {
		q.Set("pageSize", fmt.Sprintf("%d", pageSize))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create request from url: %s", u.String())
	}

	d.setCommonRequestHeaders(req)

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get devices: %s", u.String())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, errors.Errorf("get devices api respond %d: %s", resp.StatusCode, string(body))
	}

	decoder := json.NewDecoder(resp.Body)
	list := &DeviceList{}
	if err := decoder.Decode(list); err != nil {
		return nil, errors.Wrapf(err, "failed to parse response from get devices api")
	}
	return list, nil
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
	currentProfile := d.getCurrentProfile()
	if currentProfile == nil {
		return nil, errors.New("no current profile set")
	}

	url, err := url.Parse(currentProfile.BaseURL)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse base url: %s", currentProfile.BaseURL)
	}

	url.Path = endpoint

	return url, nil
}

func (d *DeviceAPI) DeviceToBox(deviceId string, force bool) (*Box, error) {
	url, err := d.buildUrlFromEndpoint(path.Join("/api/v1/devices", deviceId, "box"))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to build url")
	}

	reqBody, err := json.Marshal(map[string]interface{}{
		"force": force,
	})
	if err != nil {
		return nil, errors.Wrap(err, "fail to marshal device to box request body to json")
	}

	req, err := http.NewRequest(http.MethodPost, url.String(), bytes.NewReader(reqBody))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create request from url: %s", url.String())
	}

	d.setCommonRequestHeaders(req)

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to post device to box: %s", url.String())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, errors.Errorf("post device to box api respond %d: %s", resp.StatusCode, string(body))
	}

	decoder := json.NewDecoder(resp.Body)
	box := &Box{}
	if err := decoder.Decode(box); err != nil {
		return nil, errors.Wrapf(err, "failed to parse response from post device to box api")
	}

	return box, nil
}

func (d *DeviceAPI) setCommonRequestHeaders(req *http.Request) {
	currentProfile := d.getCurrentProfile()
	if currentProfile == nil {
		return
	}

	req.Header.Set("x-device-ap", "true")
	req.Header.Set("content-type", "application/json")
	decodedBytes, _ := base64.StdEncoding.DecodeString(currentProfile.APIKey)
	apiKey := string(decodedBytes)
	if strings.HasPrefix(apiKey, "gbox-rack_") {
		req.Header.Set("x-rack-api-key", apiKey)
	} else {
		req.Header.Set("x-api-key", apiKey)
	}
}
