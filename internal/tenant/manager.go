package tenant

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/vinahost/waf/internal/waf"
	"go.uber.org/zap"
)

// Tenant represents a WAF tenant
type Tenant struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Domains     []string          `json:"domains"`
	Enabled     bool              `json:"enabled"`
	Rules       []string          `json:"rules"`
	Config      *TenantConfig     `json:"config"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// TenantConfig contains tenant-specific WAF configuration
type TenantConfig struct {
	DefaultAction      string `json:"default_action"` // "block" or "log"
	RequestBodyAccess  bool   `json:"request_body_access"`
	ResponseBodyAccess bool   `json:"response_body_access"`
	RequestBodyLimit   int64  `json:"request_body_limit"`
	ResponseBodyLimit  int64  `json:"response_body_limit"`
	AuditLogEnabled    bool   `json:"audit_log_enabled"`
	RateLimitPerMin    int    `json:"rate_limit_per_min"`
	GeoBlock           []string `json:"geo_block,omitempty"` // Country codes to block
}

// TenantManager manages multiple WAF tenants
type TenantManager struct {
	mu       sync.RWMutex
	tenants  map[string]*Tenant          // tenant ID -> tenant
	domains  map[string]*Tenant          // domain -> tenant
	wafs     map[string]*waf.Engine      // tenant ID -> WAF engine
	logger   *zap.Logger
}

// NewTenantManager creates a new tenant manager
func NewTenantManager(logger *zap.Logger) *TenantManager {
	return &TenantManager{
		tenants: make(map[string]*Tenant),
		domains: make(map[string]*Tenant),
		wafs:    make(map[string]*waf.Engine),
		logger:  logger,
	}
}

// CreateTenant creates a new tenant
func (tm *TenantManager) CreateTenant(ctx context.Context, tenant *Tenant) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Validate tenant
	if tenant.ID == "" {
		return fmt.Errorf("tenant ID is required")
	}
	if tenant.Name == "" {
		return fmt.Errorf("tenant name is required")
	}
	if len(tenant.Domains) == 0 {
		return fmt.Errorf("at least one domain is required")
	}

	// Check if tenant already exists
	if _, exists := tm.tenants[tenant.ID]; exists {
		return fmt.Errorf("tenant %s already exists", tenant.ID)
	}

	// Check if domains are already mapped
	for _, domain := range tenant.Domains {
		if _, exists := tm.domains[domain]; exists {
			return fmt.Errorf("domain %s already mapped to another tenant", domain)
		}
	}

	// Set defaults
	if tenant.Config == nil {
		tenant.Config = &TenantConfig{
			DefaultAction:      "block",
			RequestBodyAccess:  true,
			ResponseBodyAccess: true,
			RequestBodyLimit:   13 * 1024 * 1024,  // 13MB
			ResponseBodyLimit:  512 * 1024,        // 512KB
			AuditLogEnabled:    true,
			RateLimitPerMin:    1000,
		}
	}

	tenant.CreatedAt = time.Now()
	tenant.UpdatedAt = time.Now()

	// Create WAF engine for tenant
	wafEngine, err := tm.createWAFEngine(tenant)
	if err != nil {
		return fmt.Errorf("failed to create WAF engine: %w", err)
	}

	// Store tenant
	tm.tenants[tenant.ID] = tenant
	tm.wafs[tenant.ID] = wafEngine

	// Map domains
	for _, domain := range tenant.Domains {
		tm.domains[domain] = tenant
	}

	tm.logger.Info("Tenant created",
		zap.String("tenant_id", tenant.ID),
		zap.String("name", tenant.Name),
		zap.Strings("domains", tenant.Domains),
	)

	return nil
}

// GetTenant returns a tenant by ID
func (tm *TenantManager) GetTenant(tenantID string) (*Tenant, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	tenant, exists := tm.tenants[tenantID]
	if !exists {
		return nil, fmt.Errorf("tenant %s not found", tenantID)
	}

	return tenant, nil
}

// GetTenantByDomain returns a tenant by domain
func (tm *TenantManager) GetTenantByDomain(domain string) (*Tenant, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	tenant, exists := tm.domains[domain]
	if !exists {
		return nil, fmt.Errorf("no tenant found for domain %s", domain)
	}

	return tenant, nil
}

// GetWAFEngine returns the WAF engine for a tenant
func (tm *TenantManager) GetWAFEngine(tenantID string) (*waf.Engine, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	waf, exists := tm.wafs[tenantID]
	if !exists {
		return nil, fmt.Errorf("WAF engine not found for tenant %s", tenantID)
	}

	return waf, nil
}

// UpdateTenant updates a tenant
func (tm *TenantManager) UpdateTenant(ctx context.Context, tenant *Tenant) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	existing, exists := tm.tenants[tenant.ID]
	if !exists {
		return fmt.Errorf("tenant %s not found", tenant.ID)
	}

	// Remove old domain mappings
	for _, domain := range existing.Domains {
		delete(tm.domains, domain)
	}

	// Close old WAF engine
	if oldWAF, exists := tm.wafs[tenant.ID]; exists {
		oldWAF.Close()
	}

	// Create new WAF engine
	wafEngine, err := tm.createWAFEngine(tenant)
	if err != nil {
		return fmt.Errorf("failed to create WAF engine: %w", err)
	}

	tenant.UpdatedAt = time.Now()

	// Update tenant
	tm.tenants[tenant.ID] = tenant
	tm.wafs[tenant.ID] = wafEngine

	// Map new domains
	for _, domain := range tenant.Domains {
		tm.domains[domain] = tenant
	}

	tm.logger.Info("Tenant updated",
		zap.String("tenant_id", tenant.ID),
		zap.String("name", tenant.Name),
	)

	return nil
}

// DeleteTenant deletes a tenant
func (tm *TenantManager) DeleteTenant(tenantID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tenant, exists := tm.tenants[tenantID]
	if !exists {
		return fmt.Errorf("tenant %s not found", tenantID)
	}

	// Remove domain mappings
	for _, domain := range tenant.Domains {
		delete(tm.domains, domain)
	}

	// Close WAF engine
	if waf, exists := tm.wafs[tenantID]; exists {
		waf.Close()
	}

	// Remove tenant
	delete(tm.tenants, tenantID)
	delete(tm.wafs, tenantID)

	tm.logger.Info("Tenant deleted",
		zap.String("tenant_id", tenantID),
	)

	return nil
}

// ListTenants returns all tenants
func (tm *TenantManager) ListTenants() []*Tenant {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	tenants := make([]*Tenant, 0, len(tm.tenants))
	for _, tenant := range tm.tenants {
		tenants = append(tenants, tenant)
	}

	return tenants
}

// createWAFEngine creates a WAF engine for a tenant
func (tm *TenantManager) createWAFEngine(tenant *Tenant) (*waf.Engine, error) {
	// Create WAF config from tenant config
	wafConfig := waf.Config{
		RulesFiles:         tenant.Rules,
		RequestBodyAccess:  tenant.Config.RequestBodyAccess,
		RequestBodyLimit:   tenant.Config.RequestBodyLimit,
		ResponseBodyAccess: tenant.Config.ResponseBodyAccess,
		ResponseBodyLimit:  tenant.Config.ResponseBodyLimit,
		AuditLogEnabled:    tenant.Config.AuditLogEnabled,
		DefaultAction:      tenant.Config.DefaultAction,
	}

	return waf.NewEngine(wafConfig, tm.logger)
}

// Close closes all WAF engines
func (tm *TenantManager) Close() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	for _, waf := range tm.wafs {
		waf.Close()
	}

	tm.logger.Info("Tenant manager closed")
}
