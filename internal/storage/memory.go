package storage

import (
	"context"
	"sync"
)

// MemoryStore implements in-memory storage (for testing/fallback)
type MemoryStore struct {
	mu      sync.RWMutex
	tenants map[string]*TenantData
}

// NewMemoryStore creates a new memory storage
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		tenants: make(map[string]*TenantData),
	}
}

// SaveTenant saves a tenant to memory
func (s *MemoryStore) SaveTenant(ctx context.Context, t *TenantData) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tenants[t.ID] = t
	return nil
}

// GetTenant retrieves a tenant by ID
func (s *MemoryStore) GetTenant(ctx context.Context, id string) (*TenantData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tenants[id]
	if !ok {
		return nil, ErrTenantNotFound
	}
	return t, nil
}

// ListTenants retrieves all tenants
func (s *MemoryStore) ListTenants(ctx context.Context) ([]*TenantData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tenants := make([]*TenantData, 0, len(s.tenants))
	for _, t := range s.tenants {
		tenants = append(tenants, t)
	}
	return tenants, nil
}

// DeleteTenant deletes a tenant by ID
func (s *MemoryStore) DeleteTenant(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tenants[id]; !ok {
		return ErrTenantNotFound
	}
	delete(s.tenants, id)
	return nil
}

// Close closes the storage (no-op for memory)
func (s *MemoryStore) Close() error {
	return nil
}

// Ensure MemoryStore implements Storage interface
var _ Storage = (*MemoryStore)(nil)
