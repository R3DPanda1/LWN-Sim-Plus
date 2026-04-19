package thingsboard

// EntityID is the common id wrapper used by ThingsBoard ({"id": "uuid", "entityType": "..."}).
type EntityID struct {
	ID         string `json:"id"`
	EntityType string `json:"entityType,omitempty"`
}

// DeviceCreateRequest is the body for POST /api/device.
type DeviceCreateRequest struct {
	Name            string    `json:"name"`
	Label           string    `json:"label,omitempty"`
	DeviceProfileID EntityID  `json:"deviceProfileId"`
	CustomerID      *EntityID `json:"customerId,omitempty"`
}

// Customer is the slim shape returned to the frontend.
type Customer struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// customerListResponse matches GET /api/customers.
type customerListResponse struct {
	Data []struct {
		ID    EntityID `json:"id"`
		Title string   `json:"title"`
		Name  string   `json:"name"`
	} `json:"data"`
	TotalElements int `json:"totalElements"`
}

// DeviceResponse captures the fields we care about from POST /api/device and GET /api/device/{id}.
type DeviceResponse struct {
	ID              EntityID `json:"id"`
	Name            string   `json:"name"`
	Label           string   `json:"label,omitempty"`
	DeviceProfileID EntityID `json:"deviceProfileId"`
}

// DeviceProfile is the slim shape returned to the frontend.
type DeviceProfile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// deviceProfileListResponse matches GET /api/deviceProfiles.
type deviceProfileListResponse struct {
	Data []struct {
		ID   EntityID `json:"id"`
		Name string   `json:"name"`
	} `json:"data"`
	TotalElements int `json:"totalElements"`
}

// errorResponse mirrors the {"status","message","errorCode"} envelope TB returns for 4xx/5xx.
type errorResponse struct {
	Status    int    `json:"status"`
	Message   string `json:"message"`
	ErrorCode int    `json:"errorCode"`
}

// DeviceCredentials captures the response from GET /api/device/{id}/credentials.
// For devices using the default ACCESS_TOKEN scheme, CredentialsID holds the token.
type DeviceCredentials struct {
	CredentialsType string `json:"credentialsType"`
	CredentialsID   string `json:"credentialsId"`
}
