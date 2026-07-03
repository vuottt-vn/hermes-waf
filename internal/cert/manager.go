package cert

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// CertSource indicates where the certificate came from
type CertSource string

const (
	CertSourceUploaded CertSource = "uploaded"
	CertSourceLetsEncrypt CertSource = "letsencrypt"
	CertSourceSelfSigned  CertSource = "selfsigned"
)

// StoredCert holds metadata + paths for a certificate managed for a tenant
type StoredCert struct {
	ID         string     `json:"id"`
	TenantID   string     `json:"tenant_id"`
	Domains    []string   `json:"domains"`
	Source     CertSource `json:"source"`
	Issuer     string     `json:"issuer"`
	NotBefore  time.Time  `json:"not_before"`
	NotAfter   time.Time  `json:"not_after"`
	Serial     string     `json:"serial"`
	CertPath   string     `json:"cert_path"`
	KeyPath    string     `json:"key_path"`
	CreatedAt  time.Time  `json:"created_at"`
	AutoRenew  bool       `json:"auto_renew"`
}

// Manager manages per-tenant TLS certificates and provides the SNI callback
type Manager struct {
	mu          sync.RWMutex
	certs       map[string]*StoredCert   // cert ID -> metadata
	domainIndex map[string]string        // domain -> cert ID
	dataDir     string
	logger      *zap.Logger
	acme        *ACMEManager             // optional, for Let's Encrypt
}

// NewManager creates a new certificate manager
func NewManager(dataDir string, logger *zap.Logger) (*Manager, error) {
	certsDir := filepath.Join(dataDir, "certs")
	if err := os.MkdirAll(certsDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create certs dir: %w", err)
	}

	m := &Manager{
		certs:       make(map[string]*StoredCert),
		domainIndex: make(map[string]string),
		dataDir:     certsDir,
		logger:      logger,
	}

	// Load existing certs from disk
	if err := m.loadFromDisk(); err != nil {
		logger.Warn("Failed to load some certs from disk", zap.Error(err))
	}

	return m, nil
}

// SetACMEManager attaches the ACME manager for Let's Encrypt support
func (m *Manager) SetACMEManager(am *ACMEManager) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.acme = am
}

// GetCertificate is the tls.Config.GetCertificate callback for SNI routing.
// It looks up a certificate by the ServerName in the TLS ClientHello.
func (m *Manager) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	name := strings.ToLower(strings.TrimSpace(hello.ServerName))
	if name == "" {
		// Fall back to first available cert (for tests / default)
		m.mu.RLock()
		defer m.mu.RUnlock()
		for _, sc := range m.certs {
			return m.loadTLSCert(sc)
		}
		return nil, fmt.Errorf("no certificates available")
	}

	m.mu.RLock()
	certID, ok := m.domainIndex[name]
	if ok {
		sc := m.certs[certID]
		m.mu.RUnlock()
		return m.loadTLSCert(sc)
	}

	// Try wildcard match: *.example.com for foo.example.com
	if idx := strings.Index(name, "."); idx > 0 {
		wildcard := "*" + name[idx:]
		if certID, ok := m.domainIndex[wildcard]; ok {
			sc := m.certs[certID]
			m.mu.RUnlock()
			return m.loadTLSCert(sc)
		}
	}
	m.mu.RUnlock()

	// Fall back to ACME auto-cert if configured
	if m.acme != nil {
		return m.acme.GetCertificate(hello)
	}

	return nil, fmt.Errorf("no certificate for domain %q", name)
}

// AddUploadedCert stores an uploaded cert+key for a tenant and indexes it
func (m *Manager) AddUploadedCert(tenantID string, domains []string, certPEM, keyPEM []byte) (*StoredCert, error) {
	// Validate the cert/key pair
	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("invalid cert/key pair: %w", err)
	}

	x509Cert, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Determine source
	source := CertSourceUploaded
	if x509Cert.Issuer.CommonName == "R3" || strings.Contains(x509Cert.Issuer.String(), "Let's Encrypt") {
		source = CertSourceLetsEncrypt
	} else if len(x509Cert.DNSNames) == 0 || (len(x509Cert.DNSNames) == 1 && x509Cert.DNSNames[0] == "") {
		// Self-signed heuristic: issuer == subject
		if x509Cert.Issuer.String() == x509Cert.Subject.String() {
			source = CertSourceSelfSigned
		}
	}

	// Generate cert ID
	certID := generateCertID(tenantID, x509Cert)

	// Create tenant dir
	tenantDir := filepath.Join(m.dataDir, tenantID)
	if err := os.MkdirAll(tenantDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create tenant dir: %w", err)
	}

	certPath := filepath.Join(tenantDir, certID+".crt")
	keyPath := filepath.Join(tenantDir, certID+".key")

	// Write cert+key to disk (atomic-ish: write then rename)
	if err := atomicWrite(keyPath, keyPEM, 0600); err != nil {
		return nil, fmt.Errorf("failed to write key: %w", err)
	}
	if err := atomicWrite(certPath, certPEM, 0644); err != nil {
		os.Remove(keyPath)
		return nil, fmt.Errorf("failed to write cert: %w", err)
	}

	// If domains not provided, use SANs from cert
	if len(domains) == 0 {
		domains = x509Cert.DNSNames
	}
	if len(domains) == 0 && x509Cert.Subject.CommonName != "" {
		domains = []string{x509Cert.Subject.CommonName}
	}

	sc := &StoredCert{
		ID:        certID,
		TenantID:  tenantID,
		Domains:   domains,
		Source:    source,
		Issuer:    x509Cert.Issuer.CommonName,
		NotBefore: x509Cert.NotBefore,
		NotAfter:  x509Cert.NotAfter,
		Serial:    x509Cert.SerialNumber.String(),
		CertPath:  certPath,
		KeyPath:   keyPath,
		CreatedAt: time.Now(),
		AutoRenew: false,
	}

	// Persist metadata
	if err := m.writeMeta(sc); err != nil {
		os.Remove(certPath)
		os.Remove(keyPath)
		return nil, fmt.Errorf("failed to write metadata: %w", err)
	}

	m.index(sc)

	m.logger.Info("Certificate added",
		zap.String("cert_id", certID),
		zap.String("tenant_id", tenantID),
		zap.Strings("domains", domains),
		zap.Time("not_after", sc.NotAfter),
		zap.String("source", string(source)),
	)

	return sc, nil
}

// GetCert returns metadata for a specific cert
func (m *Manager) GetCert(tenantID, certID string) (*StoredCert, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sc, ok := m.certs[certID]
	if !ok || sc.TenantID != tenantID {
		return nil, fmt.Errorf("certificate %s not found for tenant %s", certID, tenantID)
	}
	return sc, nil
}

// ListCerts returns all certs for a tenant
func (m *Manager) ListCerts(tenantID string) []*StoredCert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]*StoredCert, 0)
	for _, sc := range m.certs {
		if sc.TenantID == tenantID {
			out = append(out, sc)
		}
	}
	return out
}

// DeleteCert removes a cert from disk and index
func (m *Manager) DeleteCert(tenantID, certID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sc, ok := m.certs[certID]
	if !ok || sc.TenantID != tenantID {
		return fmt.Errorf("certificate %s not found for tenant %s", certID, tenantID)
	}

	// Remove files
	os.Remove(sc.CertPath)
	os.Remove(sc.KeyPath)
	os.Remove(sc.CertPath + ".json")

	// Remove from index
	for _, d := range sc.Domains {
		if m.domainIndex[strings.ToLower(d)] == certID {
			delete(m.domainIndex, strings.ToLower(d))
		}
	}
	delete(m.certs, certID)

	m.logger.Info("Certificate deleted",
		zap.String("cert_id", certID),
		zap.String("tenant_id", tenantID),
	)
	return nil
}

// ExpiringSoon returns certs that will expire within the given duration
func (m *Manager) ExpiringSoon(within time.Duration) []*StoredCert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cutoff := time.Now().Add(within)
	out := make([]*StoredCert, 0)
	for _, sc := range m.certs {
		if sc.AutoRenew && sc.NotAfter.Before(cutoff) {
			out = append(out, sc)
		}
	}
	return out
}

// index adds a cert to the in-memory maps (caller must hold write lock)
func (m *Manager) index(sc *StoredCert) {
	m.certs[sc.ID] = sc
	for _, d := range sc.Domains {
		m.domainIndex[strings.ToLower(d)] = sc.ID
	}
}

// loadTLSCert loads the actual tls.Certificate from disk
func (m *Manager) loadTLSCert(sc *StoredCert) (*tls.Certificate, error) {
	cert, err := tls.LoadX509KeyPair(sc.CertPath, sc.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load cert %s: %w", sc.ID, err)
	}
	return &cert, nil
}

// loadFromDisk reads all stored certs at startup
func (m *Manager) loadFromDisk() error {
	entries, err := os.ReadDir(m.dataDir)
	if err != nil {
		return err
	}

	loaded := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		tenantID := entry.Name()
		tenantDir := filepath.Join(m.dataDir, tenantID)
		files, err := os.ReadDir(tenantDir)
		if err != nil {
			m.logger.Warn("Failed to read tenant cert dir",
				zap.String("tenant_id", tenantID),
				zap.Error(err),
			)
			continue
		}
		for _, f := range files {
			if strings.HasSuffix(f.Name(), ".json") {
				metaPath := filepath.Join(tenantDir, f.Name())
				sc, err := m.readMeta(metaPath)
				if err != nil {
					m.logger.Warn("Failed to load cert metadata",
						zap.String("path", metaPath),
						zap.Error(err),
					)
					continue
				}
				// Verify cert files still exist
				if _, err := os.Stat(sc.CertPath); err != nil {
					continue
				}
				if _, err := os.Stat(sc.KeyPath); err != nil {
					continue
				}
				m.index(sc)
				loaded++
			}
		}
	}

	m.logger.Info("Loaded certificates from disk", zap.Int("count", loaded))
	return nil
}

func (m *Manager) writeMeta(sc *StoredCert) error {
	data, err := json.MarshalIndent(sc, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(sc.CertPath+".json", data, 0644)
}

func (m *Manager) readMeta(path string) (*StoredCert, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var sc StoredCert
	if err := json.Unmarshal(data, &sc); err != nil {
		return nil, err
	}
	return &sc, nil
}

// generateCertID creates a short unique-ish ID for the cert
func generateCertID(tenantID string, cert *x509.Certificate) string {
	serial := cert.SerialNumber.String()
	if len(serial) > 12 {
		serial = serial[len(serial)-12:]
	}
	return fmt.Sprintf("%s-%s", tenantID, serial)
}

// atomicWrite writes data to a temp file then renames it into place
func atomicWrite(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ParsePEMCertDomains extracts DNS names from a PEM-encoded certificate
func ParsePEMCertDomains(certPEM []byte) ([]string, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	domains := append([]string{}, cert.DNSNames...)
	if cert.Subject.CommonName != "" {
		found := false
		for _, d := range domains {
			if d == cert.Subject.CommonName {
				found = true
				break
			}
		}
		if !found {
			domains = append(domains, cert.Subject.CommonName)
		}
	}
	return domains, nil
}
