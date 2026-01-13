package chirpstack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is a ChirpStack v4 API client
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new ChirpStack API client
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// doRequest performs an HTTP request with proper authentication
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
	req.Header.Set("Grpc-Metadata-Authorization", "Bearer "+c.apiKey)

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
		var errResp ErrorResponse
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Message != "" {
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Message)
		}
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// TestConnection verifies the API key and connectivity by listing device profiles
func (c *Client) TestConnection(tenantID string) error {
	_, err := c.doRequest("GET", "/api/device-profiles?limit=1&tenantId="+tenantID, nil)
	return err
}

// CreateDevice creates a device in ChirpStack
func (c *Client) CreateDevice(device *Device) error {
	req := DeviceCreateRequest{Device: *device}
	_, err := c.doRequest("POST", "/api/devices", req)
	return err
}

// DeleteDevice removes a device from ChirpStack
func (c *Client) DeleteDevice(devEUI string) error {
	_, err := c.doRequest("DELETE", "/api/devices/"+devEUI, nil)
	return err
}

// SetDeviceKeys sets the device OTAA keys (NwkKey/AppKey)
func (c *Client) SetDeviceKeys(devEUI string, nwkKey string) error {
	req := DeviceKeysRequest{
		DeviceKeys: DeviceKeys{
			NwkKey: nwkKey,
			AppKey: nwkKey, // Same for LoRaWAN 1.0.x OTAA
		},
	}
	_, err := c.doRequest("POST", "/api/devices/"+devEUI+"/keys", req)
	return err
}

// ActivateDeviceABP activates a device using ABP (Activation By Personalization)
// This sets the session keys directly, bypassing the OTAA join process
func (c *Client) ActivateDeviceABP(devEUI, devAddr, nwkSKey, appSKey string) error {
	req := DeviceActivationRequest{
		DeviceActivation: DeviceActivation{
			DevAddr:     devAddr,
			AppSKey:     appSKey,
			NwkSEncKey:  nwkSKey, // For LoRaWAN 1.0.x, all NwkS*Key are the same
			FNwkSIntKey: nwkSKey,
			SNwkSIntKey: nwkSKey,
		},
	}
	_, err := c.doRequest("POST", "/api/devices/"+devEUI+"/activate", req)
	return err
}

// ListDeviceProfiles returns available device profiles for a tenant
func (c *Client) ListDeviceProfiles(tenantID string, limit int) ([]DeviceProfile, error) {
	path := fmt.Sprintf("/api/device-profiles?limit=%d&tenantId=%s", limit, tenantID)
	respBody, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	var resp DeviceProfileListResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return resp.Result, nil
}

// DeviceExists checks if a device exists in ChirpStack
func (c *Client) DeviceExists(devEUI string) (bool, error) {
	_, err := c.doRequest("GET", "/api/devices/"+devEUI, nil)
	if err != nil {
		// Check if it's a 404 error
		return false, nil
	}
	return true, nil
}
