package cert

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

// ACMEManager handles Let's Encrypt certificate provisioning via ACME protocol
type ACMEManager struct {
	mu          sync.RWMutex
	cache       *autocert.Manager
	certManager *Manager
	logger      *zap.Logger
	email       string
	dirURL      string // Let's Encrypt directory URL
	dataDir     string
	challenges  map[string]string // token -> keyAuth for HTTP-01
}

// ACMEConfig holds ACME configuration
type ACMEConfig struct {
	Email       string
	DirectoryURL string // e.g., "https://acme-v02.api.letsencrypt.org/directory"
	DataDir     string
	AcceptTOS   bool
}

// NewACMEManager creates a new ACME manager for Let's Encrypt
func NewACMEManager(cfg ACMEConfig, certManager *Manager, logger *zap.Logger) (*ACMEManager, error) {
	if cfg.DirectoryURL == "" {
		cfg.DirectoryURL = acme.LetsEncryptURL
	}

	cacheDir := filepath.Join(cfg.DataDir, "acme-cache")
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create ACME cache dir: %w", err)
	}

	am := &ACMEManager{
		certManager: certManager,
		logger:      logger,
		email:       cfg.Email,
		dirURL:      cfg.DirectoryURL,
		dataDir:     cfg.DataDir,
		challenges:  make(map[string]string),
	}

	// Create autocert manager
	am.cache = &autocert.Manager{
		Prompt: func(tosURL string) bool {
			if cfg.AcceptTOS {
				logger.Info("Accepting Let's Encrypt TOS", zap.String("url", tosURL))
				return true
			}
			return false
		},
		Cache:      autocert.DirCache(cacheDir),
		Email:      cfg.Email,
		Client:     &acme.Client{DirectoryURL: cfg.DirectoryURL},
	}

	return am, nil
}

// GetCertificate is the SNI callback for ACME-managed certs
func (am *ACMEManager) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	// Try to get cert from autocert cache
	cert, err := am.cache.GetCertificate(hello)
	if err == nil {
		return cert, nil
	}

	// If not cached, trigger auto-provisioning in background
	name := strings.ToLower(strings.TrimSpace(hello.ServerName))
	if name == "" {
		return nil, fmt.Errorf("no server name provided")
	}

	am.logger.Info("Auto-provisioning certificate via ACME",
		zap.String("domain", name),
	)

	// Trigger async provisioning
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		if err := am.provisionCert(ctx, name); err != nil {
			am.logger.Error("Failed to auto-provision certificate",
				zap.String("domain", name),
				zap.Error(err),
			)
		}
	}()

	// Return error for now; next request will get the cached cert
	return nil, fmt.Errorf("certificate for %s not yet available, provisioning in background", name)
}

// provisionCert obtains a certificate for the given domain via ACME
func (am *ACMEManager) provisionCert(ctx context.Context, domain string) error {
	// Generate private key
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate key: %w", err)
	}

	// Create ACME order
	order, err := am.cache.Client.AuthorizeOrder(ctx, acme.DomainIDs(domain))
	if err != nil {
		return fmt.Errorf("failed to create order: %w", err)
	}

	// Wait for authorizations
	for _, authzURL := range order.AuthzURLs {
		authz, err := am.cache.Client.GetAuthorization(ctx, authzURL)
		if err != nil {
			return fmt.Errorf("failed to get authorization: %w", err)
		}

		// Find HTTP-01 challenge
		var challenge *acme.Challenge
		for _, c := range authz.Challenges {
			if c.Type == "http-01" {
				challenge = c
				break
			}
		}
		if challenge == nil {
			return fmt.Errorf("no HTTP-01 challenge found")
		}

		// Respond to challenge
		token := challenge.Token
		keyAuth, err := am.cache.Client.HTTP01ChallengeResponse(token)
		if err != nil {
			return fmt.Errorf("failed to compute challenge response: %w", err)
		}

		// Store challenge for HTTP handler
		am.mu.Lock()
		am.challenges[token] = keyAuth
		am.mu.Unlock()

		// Accept challenge
		if _, err := am.cache.Client.Accept(ctx, challenge); err != nil {
			return fmt.Errorf("failed to accept challenge: %w", err)
		}

		// Wait for authorization
		if _, err := am.cache.Client.WaitAuthorization(ctx, authzURL); err != nil {
			return fmt.Errorf("authorization failed: %w", err)
		}

		// Clean up challenge
		am.mu.Lock()
		delete(am.challenges, token)
		am.mu.Unlock()
	}

	// Generate CSR
	csr, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject:  pkix.Name{CommonName: domain},
		DNSNames: []string{domain},
	}, key)
	if err != nil {
		return fmt.Errorf("failed to create CSR: %w", err)
	}

	// Finalize order
	der, _, err := am.cache.Client.CreateOrderCert(ctx, order.FinalizeURL, csr, true)
	if err != nil {
		return fmt.Errorf("failed to finalize order: %w", err)
	}

	// Encode cert to PEM
	var certPEM []byte
	for _, b := range der {
		certPEM = append(certPEM, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: b})...)
	}

	// Encode key to PEM
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return fmt.Errorf("failed to marshal key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})

	// Store via cert manager
	_, err = am.certManager.AddUploadedCert("acme", []string{domain}, certPEM, keyPEM)
	if err != nil {
		return fmt.Errorf("failed to store cert: %w", err)
	}

	am.logger.Info("Successfully provisioned certificate via ACME",
		zap.String("domain", domain),
	)

	return nil
}

// HTTPHandler returns an HTTP handler for ACME HTTP-01 challenges
func (am *ACMEManager) HTTPHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/.well-known/acme-challenge/") {
			http.NotFound(w, r)
			return
		}

		token := strings.TrimPrefix(r.URL.Path, "/.well-known/acme-challenge/")
		am.mu.RLock()
		keyAuth, ok := am.challenges[token]
		am.mu.RUnlock()

		if !ok {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(keyAuth))
	})
}

// GenerateSelfSignedCert generates a self-signed certificate for testing
func GenerateSelfSignedCert(domains []string) (certPEM, keyPEM []byte, err error) {
	// Generate private key
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate key: %w", err)
	}

	// Create certificate template
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate serial: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Vinahost WAF Test"},
			CommonName:   domains[0],
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              domains,
	}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	// Encode to PEM
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal key: %w", err)
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})

	return certPEM, keyPEM, nil
}

// RenewalChecker periodically checks for expiring certs and renews them
func (am *ACMEManager) StartRenewalChecker(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				am.checkAndRenew(ctx)
			}
		}
	}()
}

func (am *ACMEManager) checkAndRenew(ctx context.Context) {
	// Check for certs expiring within 30 days
	expiring := am.certManager.ExpiringSoon(30 * 24 * time.Hour)
	if len(expiring) == 0 {
		return
	}

	am.logger.Info("Found expiring certificates", zap.Int("count", len(expiring)))

	for _, sc := range expiring {
		if sc.Source != CertSourceLetsEncrypt {
			continue // Only auto-renew Let's Encrypt certs
		}

		for _, domain := range sc.Domains {
			am.logger.Info("Renewing certificate",
				zap.String("cert_id", sc.ID),
				zap.String("domain", domain),
			)

			if err := am.provisionCert(ctx, domain); err != nil {
				am.logger.Error("Failed to renew certificate",
					zap.String("cert_id", sc.ID),
					zap.String("domain", domain),
					zap.Error(err),
				)
			}
		}
	}
}
