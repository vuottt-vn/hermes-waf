package tenant

import (
	"context"
	"net/http"
	"strings"

	"go.uber.org/zap"
)

// Router handles domain-based routing to tenants
type Router struct {
	manager *TenantManager
	logger  *zap.Logger
}

// NewRouter creates a new tenant router
func NewRouter(manager *TenantManager, logger *zap.Logger) *Router {
	return &Router{
		manager: manager,
		logger:  logger,
	}
}

// GetTenantFromRequest extracts tenant from HTTP request
func (r *Router) GetTenantFromRequest(req *http.Request) (*Tenant, error) {
	// Extract domain from Host header
	host := req.Host
	
	// Remove port if present
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}

	// Normalize domain (lowercase, trim spaces)
	host = strings.ToLower(strings.TrimSpace(host))

	// Try exact match first
	tenant, err := r.manager.GetTenantByDomain(host)
	if err == nil {
		return tenant, nil
	}

	// Try wildcard match (e.g., *.example.com)
	parts := strings.Split(host, ".")
	if len(parts) > 2 {
		// Try *.example.com
		wildcardDomain := "*." + strings.Join(parts[1:], ".")
		tenant, err = r.manager.GetTenantByDomain(wildcardDomain)
		if err == nil {
			return tenant, nil
		}
	}

	// No tenant found
	r.logger.Warn("No tenant found for domain",
		zap.String("domain", host),
		zap.String("remote_addr", req.RemoteAddr),
	)

	return nil, err
}

// Middleware returns HTTP middleware for tenant routing
func (r *Router) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		tenant, err := r.GetTenantFromRequest(req)
		if err != nil {
			// No tenant found, return 404
			http.Error(w, "Tenant not found", http.StatusNotFound)
			return
		}

		if !tenant.Enabled {
			// Tenant is disabled
			http.Error(w, "Tenant disabled", http.StatusServiceUnavailable)
			return
		}

		// Add tenant info to request context
		ctx := WithTenant(req.Context(), tenant)
		req = req.WithContext(ctx)

		// Log tenant routing
		r.logger.Debug("Request routed to tenant",
			zap.String("tenant_id", tenant.ID),
			zap.String("tenant_name", tenant.Name),
			zap.String("domain", req.Host),
		)

		next.ServeHTTP(w, req)
	})
}

// contextKey is a custom type for context keys
type contextKey string

const tenantContextKey contextKey = "tenant"

// WithTenant adds tenant to context
func WithTenant(ctx context.Context, tenant *Tenant) context.Context {
	return context.WithValue(ctx, tenantContextKey, tenant)
}

// FromContext extracts tenant from context
func FromContext(ctx context.Context) (*Tenant, bool) {
	tenant, ok := ctx.Value(tenantContextKey).(*Tenant)
	return tenant, ok
}
