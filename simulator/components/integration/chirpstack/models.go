package chirpstack

// DeviceCreateRequest represents the ChirpStack v4 device creation request
type DeviceCreateRequest struct {
	Device Device `json:"device"`
}

// Device represents a ChirpStack device
type Device struct {
	DevEUI          string            `json:"devEui"`
	Name            string            `json:"name"`
	Description     string            `json:"description,omitempty"`
	ApplicationID   string            `json:"applicationId"`
	DeviceProfileID string            `json:"deviceProfileId"`
	SkipFcntCheck   bool              `json:"skipFcntCheck,omitempty"`
	IsDisabled      bool              `json:"isDisabled,omitempty"`
	Variables       map[string]string `json:"variables,omitempty"`
	Tags            map[string]string `json:"tags,omitempty"`
}

// DeviceKeysRequest represents the device keys update request
type DeviceKeysRequest struct {
	DeviceKeys DeviceKeys `json:"deviceKeys"`
}

// DeviceKeys contains the OTAA keys for a device
type DeviceKeys struct {
	DevEUI string `json:"devEui,omitempty"`
	NwkKey string `json:"nwkKey"`
	AppKey string `json:"appKey,omitempty"`
}

// DeviceProfile represents a ChirpStack device profile
type DeviceProfile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// DeviceProfileListResponse represents the list device profiles response
type DeviceProfileListResponse struct {
	Result     []DeviceProfile `json:"result"`
	TotalCount int             `json:"totalCount"`
}

// ErrorResponse represents a ChirpStack API error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// DeviceActivationRequest represents the ABP activation request
type DeviceActivationRequest struct {
	DeviceActivation DeviceActivation `json:"deviceActivation"`
}

// DeviceActivation contains ABP session keys for a device
// For LoRaWAN 1.0.x, all NwkS*Key fields should be set to the same value (NwkSKey)
type DeviceActivation struct {
	DevAddr     string `json:"devAddr"`
	AppSKey     string `json:"appSKey"`
	NwkSEncKey  string `json:"nwkSEncKey"`  // LoRaWAN 1.1 (set to NwkSKey for 1.0.x)
	FNwkSIntKey string `json:"fNwkSIntKey"` // LoRaWAN 1.1 (set to NwkSKey for 1.0.x)
	SNwkSIntKey string `json:"sNwkSIntKey"` // LoRaWAN 1.1 (set to NwkSKey for 1.0.x)
}

// GatewayCreateRequest represents the ChirpStack v4 gateway creation request
type GatewayCreateRequest struct {
	Gateway Gateway `json:"gateway"`
}

// Gateway represents a ChirpStack gateway
type Gateway struct {
	GatewayID     string            `json:"gatewayId"`
	Name          string            `json:"name"`
	Description   string            `json:"description,omitempty"`
	TenantID      string            `json:"tenantId"`
	StatsInterval int               `json:"statsInterval,omitempty"`
	Location      GatewayLocation   `json:"location"`
	Tags          map[string]string `json:"tags,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// GatewayLocation represents a gateway's geographic location
type GatewayLocation struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitude  float64 `json:"altitude"`
	Source    string  `json:"source,omitempty"`
}
