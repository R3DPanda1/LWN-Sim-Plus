package integration

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrIntegrationNotFound = errors.New("integration not found")
	ErrInvalidIntegration  = errors.New("invalid integration")
)

// IntegrationType represents the type of integration
type IntegrationType string

const (
	IntegrationTypeChirpStack IntegrationType = "chirpstack"
)

// Integration represents a network server integration configuration
type Integration struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	Type          IntegrationType `json:"type"`
	URL           string          `json:"url"`
	APIKey        string          `json:"apiKey,omitempty"`
	TenantID      string          `json:"tenantId"`
	ApplicationID string          `json:"applicationId"`
	Enabled       bool            `json:"enabled"`
}

// NewIntegration creates a new integration with auto-generated ID
func NewIntegration(name string, intType IntegrationType, url, apiKey, tenantID, appID string) *Integration {
	i := &Integration{
		Name:          name,
		Type:          intType,
		URL:           normalizeURL(url),
		APIKey:        apiKey,
		TenantID:      tenantID,
		ApplicationID: appID,
		Enabled:       true,
	}
	i.ID = i.generateID()
	return i
}

// generateID creates a unique ID based on name and type
func (i *Integration) generateID() string {
	hash := sha256.Sum256([]byte(i.Name + string(i.Type) + i.URL))
	return hex.EncodeToString(hash[:])[:16]
}

// Validate checks if the integration has all required fields
func (i *Integration) Validate() error {
	if strings.TrimSpace(i.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidIntegration)
	}
	if strings.TrimSpace(i.URL) == "" {
		return fmt.Errorf("%w: URL is required", ErrInvalidIntegration)
	}
	if strings.TrimSpace(i.APIKey) == "" {
		return fmt.Errorf("%w: API key is required", ErrInvalidIntegration)
	}
	if strings.TrimSpace(i.TenantID) == "" {
		return fmt.Errorf("%w: tenant ID is required", ErrInvalidIntegration)
	}
	if strings.TrimSpace(i.ApplicationID) == "" {
		return fmt.Errorf("%w: application ID is required", ErrInvalidIntegration)
	}
	return nil
}

// PublicCopy returns a copy without the API key for public responses
func (i *Integration) PublicCopy() *Integration {
	return &Integration{
		ID:            i.ID,
		Name:          i.Name,
		Type:          i.Type,
		URL:           i.URL,
		TenantID:      i.TenantID,
		ApplicationID: i.ApplicationID,
		Enabled:       i.Enabled,
		// APIKey is intentionally omitted
	}
}

// Clone returns a deep copy of the integration
func (i *Integration) Clone() *Integration {
	return &Integration{
		ID:            i.ID,
		Name:          i.Name,
		Type:          i.Type,
		URL:           i.URL,
		APIKey:        i.APIKey,
		TenantID:      i.TenantID,
		ApplicationID: i.ApplicationID,
		Enabled:       i.Enabled,
	}
}

// normalizeURL ensures the URL doesn't have a trailing slash
func normalizeURL(url string) string {
	return strings.TrimSuffix(strings.TrimSpace(url), "/")
}
