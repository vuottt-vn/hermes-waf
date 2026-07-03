package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/vinahost/waf/internal/tenant"
	"go.uber.org/zap"
)

// Handler handles REST API requests
type Handler struct {
	manager *tenant.TenantManager
	logger  *zap.Logger
}

// NewHandler creates a new API handler
func NewHandler(manager *tenant.TenantManager, logger *zap.Logger) *Handler {
	return &Handler{
		manager: manager,
		logger:  logger,
	}
}

// RegisterRoutes registers API routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/tenants", h.handleTenants)
	mux.HandleFunc("/api/v1/tenants/", h.handleTenant)
}

// handleTenants handles /api/v1/tenants endpoint
func (h *Handler) handleTenants(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listTenants(w, r)
	case http.MethodPost:
		h.createTenant(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleTenant handles /api/v1/tenants/{id} endpoint
func (h *Handler) handleTenant(w http.ResponseWriter, r *http.Request) {
	// Extract tenant ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/tenants/")
	tenantID := strings.TrimSuffix(path, "/")

	if tenantID == "" {
		http.Error(w, "Tenant ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getTenant(w, r, tenantID)
	case http.MethodPut:
		h.updateTenant(w, r, tenantID)
	case http.MethodDelete:
		h.deleteTenant(w, r, tenantID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// listTenants lists all tenants
func (h *Handler) listTenants(w http.ResponseWriter, r *http.Request) {
	tenants := h.manager.ListTenants()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tenants": tenants,
		"count":   len(tenants),
	})
}

// createTenant creates a new tenant
func (h *Handler) createTenant(w http.ResponseWriter, r *http.Request) {
	var t tenant.Tenant
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.manager.CreateTenant(r.Context(), &t); err != nil {
		h.logger.Error("Failed to create tenant", zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.logger.Info("Tenant created via API",
		zap.String("tenant_id", t.ID),
		zap.String("name", t.Name),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Tenant created successfully",
		"tenant":  t,
	})
}

// getTenant gets a tenant by ID
func (h *Handler) getTenant(w http.ResponseWriter, r *http.Request, tenantID string) {
	t, err := h.manager.GetTenant(tenantID)
	if err != nil {
		http.Error(w, "Tenant not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(t)
}

// updateTenant updates a tenant
func (h *Handler) updateTenant(w http.ResponseWriter, r *http.Request, tenantID string) {
	var t tenant.Tenant
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	t.ID = tenantID

	if err := h.manager.UpdateTenant(r.Context(), &t); err != nil {
		h.logger.Error("Failed to update tenant", zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.logger.Info("Tenant updated via API",
		zap.String("tenant_id", tenantID),
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Tenant updated successfully",
		"tenant":  t,
	})
}

// deleteTenant deletes a tenant
func (h *Handler) deleteTenant(w http.ResponseWriter, r *http.Request, tenantID string) {
	if err := h.manager.DeleteTenant(tenantID); err != nil {
		http.Error(w, "Tenant not found", http.StatusNotFound)
		return
	}

	h.logger.Info("Tenant deleted via API",
		zap.String("tenant_id", tenantID),
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Tenant deleted successfully",
	})
}
