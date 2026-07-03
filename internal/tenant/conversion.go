package tenant

import (
	"github.com/vinahost/waf/internal/storage"
)

// ToTenantData converts Tenant to storage.TenantData
func (t *Tenant) ToTenantData() *storage.TenantData {
	return &storage.TenantData{
		ID:        t.ID,
		Name:      t.Name,
		Domains:   t.Domains,
		Enabled:   t.Enabled,
		Rules:     t.Rules,
		Config:    t.Config,
		Metadata:  t.Metadata,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
}

// FromTenantData converts storage.TenantData to Tenant
func FromTenantData(d *storage.TenantData) *Tenant {
	// Type assert Config to TenantConfig
	var config *TenantConfig
	if d.Config != nil {
		if c, ok := d.Config.(*TenantConfig); ok {
			config = c
		}
	}

	// Type assert Metadata to map[string]string
	var metadata map[string]string
	if d.Metadata != nil {
		if m, ok := d.Metadata.(map[string]string); ok {
			metadata = m
		}
	}

	return &Tenant{
		ID:        d.ID,
		Name:      d.Name,
		Domains:   d.Domains,
		Enabled:   d.Enabled,
		Rules:     d.Rules,
		Config:    config,
		Metadata:  metadata,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
}
