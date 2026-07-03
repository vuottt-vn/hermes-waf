package tenant

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/vinahost/waf/internal/cache"
	"github.com/vinahost/waf/internal/storage"
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
	tenants  map[string]*Tenant          // tenant ID -> tenant (in-memory cache)
	domains  map[string]*Tenant          // domain -> tenant (in-memory cache)
	wafs     map[string]*waf.Engine      // tenant ID -> WAF engine
	cache    cache.Cache                 // cache for tenant configs
	storage  storage.Storage             // persistent storage
	logger   *zap.Logger
}

// NewTenantManager creates a new tenant manager
func NewTenantManager(cache cache.Cache, store storage.Storage, logger *zap.Logger) *TenantManager {
	return &TenantManager{
		tenants: make(map[string]*Tenant),
		domains: make(map[string]*Tenant),
		wafs:    make(map[string]*waf.Engine),
		cache:   cache,
		storage: store,
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

	// Store tenant in memory
	tm.tenants[tenant.ID] = tenant
	tm.wafs[tenant.ID] = wafEngine

	// Persist to storage
	if tm.storage != nil {
		if err := tm.storage.SaveTenant(ctx, tenant); err != nil {
			tm.logger.Error("Failed to persist tenant to storage",
				zap.String("tenant_id", tenant.ID),
				zap.Error(err),
			)
			// Rollback in-memory changes
			delete(tm.tenants, tenant.ID)
			delete(tm.wafs, tenant.ID)
			wafEngine.Close()
			return fmt.Errorf("failed to persist tenant: %w", err)
		}
	}

	// Cache tenant config
	if tm.cache != nil {
		cacheKey := "tenant:" + tenant.ID
		if err := tm.cache.Set(ctx, cacheKey, tenant); err != nil {
			tm.logger.Warn("Failed to cache tenant",
				zap.String("tenant_id", tenant.ID),
				zap.Error(err),
			)
		}
	}

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
func (tm *TenantManager) GetTenant(ctx context.Context, tenantID string) (*Tenant, error) {
	// Try in-memory cache first
	tm.mu.RLock()
	tenant, exists := tm.tenants[tenantID]
	tm.mu.RUnlock()
	
	if exists {
		return tenant, nil
	}

	// Try persistent storage
	if tm.storage != nil {
		tenant, err := tm.storage.GetTenant(ctx, tenantID)
		if err == nil {
			// Load into memory cache
			tm.mu.Lock()
			tm.tenants[tenantID] = tenant
			for _, domain := range tenant.Domains {
				tm.domains[domain] = tenant
			}
			tm.mu.Unlock()
			
			// Create WAF engine
			wafEngine, err := tm.createWAFEngine(tenant)
			if err != nil {
				return nil, fmt.Errorf("failed to create WAF engine: %w", err)
			}
			
			tm.mu.Lock()
			tm.wafs[tenantID] = wafEngine
			tm.mu.Unlock()
			
			return tenant, nil
		}
	}

	return nil, fmt.Errorf("tenant %s not found", tenantID)
}

// GetTenantByDomain returns a tenant by domain
func (tm *TenantManager) GetTenantByDomain(domain string) (*Tenant, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	// Try exact match first
	tenant, exists := tm.domains[domain]
	if exists {
		return tenant, nil
	}

	// Try wildcard catch-all "*"
	tenant, exists = tm.domains["*"]
	if exists {
		return tenant, nil
	}

	// Try wildcard match (e.g., *.example.com)
	parts := strings.Split(domain, ".")
	if len(parts) > 2 {
		wildcardDomain := "*." + strings.Join(parts[1:], ".")
		tenant, exists = tm.domains[wildcardDomain]
		if exists {
			return tenant, nil
		}
	}

	return nil, fmt.Errorf("no tenant found for domain %s", domain)
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
		// Try to load from storage
		if tm.storage != nil {
			var err error
			existing, err = tm.storage.GetTenant(ctx, tenant.ID)
			if err != nil {
				return fmt.Errorf("tenant %s not found", tenant.ID)
			}
		} else {
			return fmt.Errorf("tenant %s not found", tenant.ID)
		}
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

	// Update in memory
	tm.tenants[tenant.ID] = tenant
	tm.wafs[tenant.ID] = wafEngine

	// Persist to storage
	if tm.storage != nil {
		if err := tm.storage.SaveTenant(ctx, tenant); err != nil {
			tm.logger.Error("Failed to persist tenant update to storage",
				zap.String("tenant_id", tenant.ID),
				zap.Error(err),
			)
			return fmt.Errorf("failed to persist tenant update: %w", err)
		}
	}

	// Invalidate and update cache
	if tm.cache != nil {
		cacheKey := "tenant:" + tenant.ID
		tm.cache.Delete(ctx, cacheKey)
		if err := tm.cache.Set(ctx, cacheKey, tenant); err != nil {
			tm.logger.Warn("Failed to update cache for tenant",
				zap.String("tenant_id", tenant.ID),
				zap.Error(err),
			)
		}
	}

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
func (tm *TenantManager) DeleteTenant(ctx context.Context, tenantID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tenant, exists := tm.tenants[tenantID]
	if !exists {
		// Try to load from storage
		if tm.storage != nil {
			var err error
			tenant, err = tm.storage.GetTenant(ctx, tenantID)
			if err != nil {
				return fmt.Errorf("tenant %s not found", tenantID)
			}
		} else {
			return fmt.Errorf("tenant %s not found", tenantID)
		}
	}

	// Remove domain mappings
	for _, domain := range tenant.Domains {
		delete(tm.domains, domain)
	}

	// Close WAF engine
	if waf, exists := tm.wafs[tenantID]; exists {
		waf.Close()
	}

	// Remove from memory
	delete(tm.tenants, tenantID)
	delete(tm.wafs, tenantID)

	// Delete from storage
	if tm.storage != nil {
		if err := tm.storage.DeleteTenant(ctx, tenantID); err != nil {
			tm.logger.Error("Failed to delete tenant from storage",
				zap.String("tenant_id", tenantID),
				zap.Error(err),
			)
			return fmt.Errorf("failed to delete tenant from storage: %w", err)
		}
	}

	// Invalidate cache
	if tm.cache != nil {
		cacheKey := "tenant:" + tenantID
		if err := tm.cache.Delete(ctx, cacheKey); err != nil {
			tm.logger.Warn("Failed to delete cache for tenant",
				zap.String("tenant_id", tenantID),
				zap.Error(err),
			)
		}
	}

	tm.logger.Info("Tenant deleted",
		zap.String("tenant_id", tenantID),
	)

	return nil
}

// ListTenants returns all tenants
func (tm *TenantManager) ListTenants(ctx context.Context) []*Tenant {
	// Try storage first if available
	if tm.storage != nil {
		tenants, err := tm.storage.ListTenants(ctx)
		if err == nil && len(tenants) > 0 {
			// Update in-memory cache
			tm.mu.Lock()
			for _, tenant := range tenants {
				tm.tenants[tenant.ID] = tenant
				for _, domain := range tenant.Domains {
					tm.domains[domain] = tenant
				}
			}
			tm.mu.Unlock()
			return tenants
		}
	}

	// Fallback to in-memory
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

// GetCache returns the cache instance
func (tm *TenantManager) GetCache() cache.Cache {
	return tm.cache
}
