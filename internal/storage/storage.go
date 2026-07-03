package storage

import (
	"context"
	"github.com/vinahost/waf/internal/tenant"
)

// Storage defines the interface for tenant persistence
type Storage interface {
	// SaveTenant saves a tenant to storage
	SaveTenant(ctx context.Context, t *tenant.Tenant) error
	
	// GetTenant retrieves a tenant by ID
	GetTenant(ctx context.Context, id string) (*tenant.Tenant, error)
	
	// ListTenants retrieves all tenants
	ListTenants(ctx context.Context) ([]*tenant.Tenant, error)
	
	// DeleteTenant deletes a tenant by ID
	DeleteTenant(ctx context.Context, id string) error
	
	// Close closes the storage connection
	Close() error
}
