package storage

import (
	"context"
	"sync"

	"github.com/vinahost/waf/internal/tenant"
)

// MemoryStore implements in-memory storage (for testing/fallback)
type MemoryStore struct {
	mu      sync.RWMutex
	tenants map[string]*tenant.Tenant
}

// NewMemoryStore creates a new in-memory storage
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		tenants: make(map[string]*tenant.Tenant),
	}
}

// SaveTenant saves a tenant to memory
func (s *MemoryStore) SaveTenant(ctx context.Context, t *tenant.Tenant) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tenants[t.ID] = t
	return nil
}

// GetTenant retrieves a tenant by ID
func (s *MemoryStore) GetTenant(ctx context.Context, id string) (*tenant.Tenant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	t, exists := s.tenants[id]
	if !exists {
		return nil, &NotFoundError{ID: id}
	}
	return t, nil
}

// ListTenants retrieves all tenants
func (s *MemoryStore) ListTenants(ctx context.Context) ([]*tenant.Tenant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	tenants := make([]*tenant.Tenant, 0, len(s.tenants))
	for _, t := range s.tenants {
		tenants = append(tenants, t)
	}
	return tenants, nil
}

// DeleteTenant deletes a tenant by ID
func (s *MemoryStore) DeleteTenant(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if _, exists := s.tenants[id]; !exists {
		return &NotFoundError{ID: id}
	}
	delete(s.tenants, id)
	return nil
}

// Close closes the storage (no-op for memory)
func (s *MemoryStore) Close() error {
	return nil
}

// NotFoundError represents a tenant not found error
type NotFoundError struct {
	ID string
}

func (e *NotFoundError) Error() string {
	return "tenant not found: " + e.ID
}
