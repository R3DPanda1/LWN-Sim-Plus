package thingsboard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is a ThingsBoard REST API client authenticated via X-Authorization: ApiKey.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new ThingsBoard API client.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) doRequest(method, path string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Authorization", "ApiKey "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp errorResponse
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Message != "" {
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Message)
		}
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// TestConnection verifies the API key has tenant-admin permissions by listing tenant devices.
// This touches the same authorization domain used for CreateDevice/DeleteDevice.
func (c *Client) TestConnection() error {
	_, err := c.doRequest("GET", "/api/tenant/devices?pageSize=1&page=0", nil)
	return err
}

// CreateDevice creates a device in ThingsBoard and returns its UUID. If customerID is
// non-empty the device is assigned to that customer at creation time.
func (c *Client) CreateDevice(name, label, deviceProfileID, customerID string) (string, error) {
	req := DeviceCreateRequest{
		Name:  name,
		Label: label,
		DeviceProfileID: EntityID{
			ID:         deviceProfileID,
			EntityType: "DEVICE_PROFILE",
		},
	}
	if customerID != "" {
		req.CustomerID = &EntityID{ID: customerID, EntityType: "CUSTOMER"}
	}
	respBody, err := c.doRequest("POST", "/api/device", req)
	if err != nil {
		return "", err
	}
	var resp DeviceResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}
	if resp.ID.ID == "" {
		return "", fmt.Errorf("ThingsBoard did not return a device id")
	}
	return resp.ID.ID, nil
}

// GetDeviceCredentials returns the access token for a ThingsBoard device. Only ACCESS_TOKEN credentials are supported.
func (c *Client) GetDeviceCredentials(deviceID string) (string, error) {
	respBody, err := c.doRequest("GET", "/api/device/"+deviceID+"/credentials", nil)
	if err != nil {
		return "", err
	}
	var creds DeviceCredentials
	if err := json.Unmarshal(respBody, &creds); err != nil {
		return "", fmt.Errorf("failed to parse credentials response: %w", err)
	}
	if creds.CredentialsType != "ACCESS_TOKEN" {
		return "", fmt.Errorf("device %s has unsupported credentials type %q (expected ACCESS_TOKEN)", deviceID, creds.CredentialsType)
	}
	if creds.CredentialsID == "" {
		return "", fmt.Errorf("ThingsBoard returned empty access token for device %s", deviceID)
	}
	return creds.CredentialsID, nil
}

// ListCustomers returns all customers visible to the tenant.
func (c *Client) ListCustomers(limit int) ([]Customer, error) {
	path := fmt.Sprintf("/api/customers?pageSize=%d&page=0", limit)
	respBody, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	var resp customerListResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	out := make([]Customer, 0, len(resp.Data))
	for _, c := range resp.Data {
		name := c.Title
		if name == "" {
			name = c.Name
		}
		out = append(out, Customer{ID: c.ID.ID, Name: name})
	}
	return out, nil
}

// DeleteDevice removes a device from ThingsBoard by its UUID.
func (c *Client) DeleteDevice(deviceID string) error {
	_, err := c.doRequest("DELETE", "/api/device/"+deviceID, nil)
	return err
}

// ListDeviceProfiles returns all device profiles accessible to the tenant.
func (c *Client) ListDeviceProfiles(limit int) ([]DeviceProfile, error) {
	path := fmt.Sprintf("/api/deviceProfiles?pageSize=%d&page=0", limit)
	respBody, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	var resp deviceProfileListResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	profiles := make([]DeviceProfile, 0, len(resp.Data))
	for _, p := range resp.Data {
		profiles = append(profiles, DeviceProfile{ID: p.ID.ID, Name: p.Name})
	}
	return profiles, nil
}
