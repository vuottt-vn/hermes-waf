package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"github.com/vinahost/waf/internal/tenant"
)

// PostgresStore implements persistent storage using PostgreSQL
type PostgresStore struct {
	db *sql.DB
}

// NewPostgresStore creates a new PostgreSQL storage
func NewPostgresStore(dsn string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	store := &PostgresStore{db: db}
	if err := store.migrate(); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return store, nil
}

// migrate creates necessary tables
func (s *PostgresStore) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS tenants (
		id VARCHAR(255) PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		domains TEXT[] NOT NULL,
		enabled BOOLEAN NOT NULL DEFAULT true,
		rules TEXT[] NOT NULL DEFAULT '{}',
		config JSONB NOT NULL DEFAULT '{}',
		metadata JSONB NOT NULL DEFAULT '{}',
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP NOT NULL DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_tenants_enabled ON tenants(enabled);
	CREATE INDEX IF NOT EXISTS idx_tenants_domains ON tenants USING GIN(domains);
	`

	_, err := s.db.Exec(schema)
	return err
}

// SaveTenant saves a tenant to database
func (s *PostgresStore) SaveTenant(ctx context.Context, t *tenant.Tenant) error {
	domains := fmt.Sprintf("{%s}", joinStrings(t.Domains))
	rules := fmt.Sprintf("{%s}", joinStrings(t.Rules))
	
	configJSON, err := json.Marshal(t.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	metadataJSON, err := json.Marshal(t.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO tenants (id, name, domains, enabled, rules, config, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			domains = EXCLUDED.domains,
			enabled = EXCLUDED.enabled,
			rules = EXCLUDED.rules,
			config = EXCLUDED.config,
			metadata = EXCLUDED.metadata,
			updated_at = EXCLUDED.updated_at
	`

	_, err = s.db.ExecContext(ctx, query,
		t.ID, t.Name, domains, t.Enabled, rules,
		configJSON, metadataJSON, t.CreatedAt, t.UpdatedAt,
	)

	return err
}

// GetTenant retrieves a tenant by ID
func (s *PostgresStore) GetTenant(ctx context.Context, id string) (*tenant.Tenant, error) {
	var t tenant.Tenant
	var domains, rules []string
	var configJSON, metadataJSON []byte

	query := `
		SELECT id, name, domains, enabled, rules, config, metadata, created_at, updated_at
		FROM tenants WHERE id = $1
	`

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&t.ID, &t.Name, &domains, &t.Enabled, &rules,
		&configJSON, &metadataJSON, &t.CreatedAt, &t.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("tenant not found")
	}
	if err != nil {
		return nil, err
	}

	t.Domains = domains
	t.Rules = rules

	if err := json.Unmarshal(configJSON, &t.Config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := json.Unmarshal(metadataJSON, &t.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &t, nil
}

// ListTenants retrieves all tenants
func (s *PostgresStore) ListTenants(ctx context.Context) ([]*tenant.Tenant, error) {
	query := `
		SELECT id, name, domains, enabled, rules, config, metadata, created_at, updated_at
		FROM tenants ORDER BY created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tenants []*tenant.Tenant
	for rows.Next() {
		var t tenant.Tenant
		var domains, rules []string
		var configJSON, metadataJSON []byte

		if err := rows.Scan(
			&t.ID, &t.Name, &domains, &t.Enabled, &rules,
			&configJSON, &metadataJSON, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, err
		}

		t.Domains = domains
		t.Rules = rules

		if err := json.Unmarshal(configJSON, &t.Config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config: %w", err)
		}

		if err := json.Unmarshal(metadataJSON, &t.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		tenants = append(tenants, &t)
	}

	return tenants, rows.Err()
}

// DeleteTenant deletes a tenant by ID
func (s *PostgresStore) DeleteTenant(ctx context.Context, id string) error {
	query := `DELETE FROM tenants WHERE id = $1`
	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("tenant not found")
	}

	return nil
}

// Close closes the database connection
func (s *PostgresStore) Close() error {
	return s.db.Close()
}

// Helper function to join strings for PostgreSQL array
func joinStrings(strs []string) string {
	result := ""
	for i, s := range strs {
		if i > 0 {
			result += ","
		}
		result += fmt.Sprintf(`"%s"`, s)
	}
	return result
}
