package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/vinahost/waf/internal/cert"
	"github.com/vinahost/waf/internal/tenant"
	"go.uber.org/zap"
)

// Handler handles REST API requests
type Handler struct {
	manager     *tenant.TenantManager
	certManager *cert.Manager
	logger      *zap.Logger
}

// NewHandler creates a new API handler
func NewHandler(manager *tenant.TenantManager, certManager *cert.Manager, logger *zap.Logger) *Handler {
	return &Handler{
		manager:     manager,
		certManager: certManager,
		logger:      logger,
	}
}

// RegisterRoutes registers API routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/tenants", h.handleTenants)
	mux.HandleFunc("/api/v1/tenants/", h.handleTenant)
	mux.HandleFunc("/api/v1/cache/status", h.handleCacheStatus)
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
	
	// Check if this is a cert endpoint
	if strings.Contains(path, "/certs") {
		h.handleTenantCerts(w, r, path)
		return
	}
	
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
	tenants := h.manager.ListTenants(r.Context())

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
	t, err := h.manager.GetTenant(r.Context(), tenantID)
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
	if err := h.manager.DeleteTenant(r.Context(), tenantID); err != nil {
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

// handleTenantCerts handles /api/v1/tenants/{id}/certs and /api/v1/tenants/{id}/certs/{cert_id}
func (h *Handler) handleTenantCerts(w http.ResponseWriter, r *http.Request, path string) {
	// path is like "tenant123/certs" or "tenant123/certs/cert456"
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		http.Error(w, "Invalid cert path", http.StatusBadRequest)
		return
	}

	tenantID := parts[0]
	// parts[1] should be "certs"
	
	// Verify tenant exists
	if _, err := h.manager.GetTenant(r.Context(), tenantID); err != nil {
		http.Error(w, "Tenant not found", http.StatusNotFound)
		return
	}

	// Check if specific cert ID is provided
	if len(parts) >= 3 && parts[2] != "" {
		certID := parts[2]
		h.handleSpecificCert(w, r, tenantID, certID)
		return
	}

	// Handle /certs endpoint
	switch r.Method {
	case http.MethodGet:
		h.listCerts(w, r, tenantID)
	case http.MethodPost:
		h.uploadCert(w, r, tenantID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// listCerts lists all certificates for a tenant
func (h *Handler) listCerts(w http.ResponseWriter, r *http.Request, tenantID string) {
	certs := h.certManager.ListCerts(tenantID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"certs": certs,
		"count": len(certs),
	})
}

// uploadCert handles certificate upload
func (h *Handler) uploadCert(w http.ResponseWriter, r *http.Request, tenantID string) {
	// Parse multipart form
	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32MB max
		h.logger.Error("Failed to parse multipart form", zap.Error(err))
		http.Error(w, "Invalid multipart form", http.StatusBadRequest)
		return
	}

	// Get cert file
	certFile, _, err := r.FormFile("cert")
	if err != nil {
		http.Error(w, "cert file required", http.StatusBadRequest)
		return
	}
	defer certFile.Close()

	certPEM, err := io.ReadAll(certFile)
	if err != nil {
		http.Error(w, "Failed to read cert file", http.StatusBadRequest)
		return
	}

	// Get key file
	keyFile, _, err := r.FormFile("key")
	if err != nil {
		http.Error(w, "key file required", http.StatusBadRequest)
		return
	}
	defer keyFile.Close()

	keyPEM, err := io.ReadAll(keyFile)
	if err != nil {
		http.Error(w, "Failed to read key file", http.StatusBadRequest)
		return
	}

	// Get optional domains
	domains := r.FormValue("domains")
	var domainList []string
	if domains != "" {
		domainList = strings.Split(domains, ",")
		for i := range domainList {
			domainList[i] = strings.TrimSpace(domainList[i])
		}
	}

	// Store cert
	sc, err := h.certManager.AddUploadedCert(tenantID, domainList, certPEM, keyPEM)
	if err != nil {
		h.logger.Error("Failed to upload certificate",
			zap.String("tenant_id", tenantID),
			zap.Error(err),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.logger.Info("Certificate uploaded via API",
		zap.String("tenant_id", tenantID),
		zap.String("cert_id", sc.ID),
		zap.Strings("domains", sc.Domains),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Certificate uploaded successfully",
		"cert":    sc,
	})
}

// handleSpecificCert handles operations on a specific certificate
func (h *Handler) handleSpecificCert(w http.ResponseWriter, r *http.Request, tenantID, certID string) {
	switch r.Method {
	case http.MethodGet:
		h.getCert(w, r, tenantID, certID)
	case http.MethodDelete:
		h.deleteCert(w, r, tenantID, certID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// getCert gets a specific certificate
func (h *Handler) getCert(w http.ResponseWriter, r *http.Request, tenantID, certID string) {
	sc, err := h.certManager.GetCert(tenantID, certID)
	if err != nil {
		http.Error(w, "Certificate not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sc)
}

// deleteCert deletes a specific certificate
func (h *Handler) deleteCert(w http.ResponseWriter, r *http.Request, tenantID, certID string) {
	if err := h.certManager.DeleteCert(tenantID, certID); err != nil {
		http.Error(w, "Certificate not found", http.StatusNotFound)
		return
	}

	h.logger.Info("Certificate deleted via API",
		zap.String("tenant_id", tenantID),
		zap.String("cert_id", certID),
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Certificate deleted successfully",
	})
}

// handleCacheStatus returns cache status and statistics
func (h *Handler) handleCacheStatus(w http.ResponseWriter, r *http.Request) {
	// Get all tenants to check cache
	tenants := h.manager.ListTenants(r.Context())
	
	cachedCount := 0
	for _, t := range tenants {
		cacheKey := "tenant:" + t.ID
		exists, err := h.manager.GetCache().Exists(r.Context(), cacheKey)
		if err == nil && exists {
			cachedCount++
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"cache_enabled":  h.manager.GetCache() != nil,
		"cache_type":     fmt.Sprintf("%T", h.manager.GetCache()),
		"total_tenants":  len(tenants),
		"cached_tenants": cachedCount,
	})
}
