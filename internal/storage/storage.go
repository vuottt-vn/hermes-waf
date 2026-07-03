package storage

import (
	"context"
	"fmt"
	"time"
)

// TenantData represents tenant data for storage (avoids import cycle)
type TenantData struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Domains   []string    `json:"domains"`
	Enabled   bool        `json:"enabled"`
	Rules     []string    `json:"rules"`
	Config    interface{} `json:"config"`
	Metadata  interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// Storage defines the interface for tenant persistence
type Storage interface {
	// SaveTenant saves a tenant to storage
	SaveTenant(ctx context.Context, tenant *TenantData) error

	// GetTenant retrieves a tenant by ID
	GetTenant(ctx context.Context, tenantID string) (*TenantData, error)

	// DeleteTenant removes a tenant from storage
	DeleteTenant(ctx context.Context, tenantID string) error

	// ListTenants returns all tenants
	ListTenants(ctx context.Context) ([]*TenantData, error)

	// Close closes the storage connection
	Close() error
}

// ErrTenantNotFound is returned when a tenant is not found
var ErrTenantNotFound = fmt.Errorf("tenant not found")
