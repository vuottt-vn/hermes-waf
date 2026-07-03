# Phase 2.5: Certificate Management (TLS/SNI)

## Overview

Phase 2.5 adds comprehensive TLS/SSL certificate management to the Vinahost WAF, enabling per-tenant HTTPS with automatic SNI routing and Let's Encrypt integration.

## Features Implemented

### 1. Certificate Manager Module (`internal/cert/manager.go`)

**Core Functionality:**
- Per-tenant certificate storage and management
- TLS SNI callback for dynamic certificate selection
- Certificate metadata tracking (issuer, validity, domains)
- Persistent storage with JSON metadata files
- Domain-to-certificate indexing with wildcard support

**Key Methods:**
- `GetCertificate(hello *tls.ClientHelloInfo)` - TLS SNI callback
- `AddUploadedCert(tenantID, domains, certPEM, keyPEM)` - Store uploaded certs
- `ListCerts(tenantID)` - List all certs for a tenant
- `GetCert(tenantID, certID)` - Get specific cert metadata
- `DeleteCert(tenantID, certID)` - Remove certificate
- `ExpiringSoon(within)` - Find certs expiring within duration

**Storage Structure:**
```
/opt/vinahost-waf/certs/
├── tenant1/
│   ├── cert-id-1.crt
│   ├── cert-id-1.key
│   └── cert-id-1.crt.json (metadata)
└── tenant2/
    ├── cert-id-2.crt
    └── cert-id-2.key
```

### 2. Let's Encrypt ACME Integration (`internal/cert/acme.go`)

**Features:**
- Automatic certificate provisioning via ACME protocol
- HTTP-01 challenge support
- Background certificate renewal (30 days before expiry)
- Integration with autocert cache for performance

**Key Components:**
- `ACMEManager` - Manages ACME client and certificate provisioning
- `HTTPHandler()` - Serves ACME HTTP-01 challenges
- `StartRenewalChecker()` - Periodic renewal check (24h interval)
- `GenerateSelfSignedCert()` - Test certificate generation

**Configuration:**
```yaml
tls:
  enabled: true
  listen_addr: ":8443"
  cert_dir: "/opt/vinahost-waf/certs"
  acme_email: "admin@example.com"
  acme_dir_url: "https://acme-v02.api.letsencrypt.org/directory"
  accept_tos: true
  http01_addr: ":80"
```

### 3. API Endpoints (`internal/api/handler.go`)

**Certificate Management API:**

```
POST   /api/v1/tenants/{id}/certs          # Upload certificate
GET    /api/v1/tenants/{id}/certs          # List certificates
GET    /api/v1/tenants/{id}/certs/{cert_id} # Get certificate details
DELETE /api/v1/tenants/{id}/certs/{cert_id} # Delete certificate
```

**Upload Example:**
```bash
curl -X POST http://localhost:8080/api/v1/tenants/tenant1/certs \
  -F "cert=@cert.pem" \
  -F "key=@key.pem" \
  -F "domains=example.com,www.example.com"
```

**Response:**
```json
{
  "message": "Certificate uploaded successfully",
  "cert": {
    "id": "tenant1-1234567890",
    "tenant_id": "tenant1",
    "domains": ["example.com", "www.example.com"],
    "source": "uploaded",
    "issuer": "Let's Encrypt Authority X3",
    "not_before": "2024-01-01T00:00:00Z",
    "not_after": "2024-04-01T00:00:00Z",
    "serial": "1234567890",
    "auto_renew": false
  }
}
```

### 4. HTTPS Server with SNI (`cmd/waf/main.go`)

**Changes:**
- Dual HTTP/HTTPS server support
- TLS configuration with SNI callback
- Certificate manager initialization
- ACME manager setup (optional)
- Graceful shutdown for both servers

**Server Configuration:**
```yaml
server:
  listen_addr: ":8080"  # HTTP

tls:
  enabled: true
  listen_addr: ":8443"  # HTTPS
```

### 5. Configuration Updates (`internal/config/config.go`)

**New TLS Configuration:**
```go
type TLSConfig struct {
    Enabled     bool   `yaml:"enabled"`
    ListenAddr  string `yaml:"listen_addr"`
    CertDir     string `yaml:"cert_dir"`
    ACMEEmail   string `yaml:"acme_email"`
    ACMEDirURL  string `yaml:"acme_dir_url"`
    AcceptTOS   bool   `yaml:"accept_tos"`
    HTTP01Addr  string `yaml:"http01_addr"`
}
```

## Usage Examples

### 1. Enable TLS in Configuration

```yaml
# configs/config.yaml
server:
  listen_addr: ":8080"

tls:
  enabled: true
  listen_addr: ":8443"
  cert_dir: "/opt/vinahost-waf/certs"
  acme_email: "admin@vinahost.com"
  accept_tos: true
```

### 2. Upload Certificate for Tenant

```bash
# Generate self-signed cert for testing
openssl req -x509 -newkey rsa:2048 -keyout key.pem -out cert.pem \
  -days 365 -nodes -subj "/CN=example.com"

# Upload via API
curl -X POST http://localhost:8080/api/v1/tenants/tenant1/certs \
  -F "cert=@cert.pem" \
  -F "key=@key.pem" \
  -F "domains=example.com"
```

### 3. Test HTTPS with SNI

```bash
# Test with curl (skip verification for self-signed)
curl -k --resolve "example.com:8443:127.0.0.1" \
  https://example.com:8443/health

# Expected response:
# {"status":"healthy","version":"0.3.0","tls":true}
```

### 4. Let's Encrypt Auto-Provisioning

For production domains with public DNS:

```yaml
tls:
  enabled: true
  acme_email: "admin@example.com"
  accept_tos: true
  acme_dir_url: "https://acme-v02.api.letsencrypt.org/directory"
```

The ACME manager will:
1. Automatically provision certificates when first accessed
2. Handle HTTP-01 challenges on port 80
3. Renew certificates 30 days before expiry
4. Store certificates in the cert directory

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    HTTPS Request                         │
│                  (with SNI hostname)                     │
└────────────────────┬────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────┐
│              TLS Server (port 8443)                      │
│  ┌──────────────────────────────────────────────────┐  │
│  │  tls.Config.GetCertificate()                     │  │
│  │  - Extract SNI hostname                          │  │
│  │  - Lookup certificate in domain index            │  │
│  │  - Support wildcard matching                     │  │
│  │  - Fallback to ACME auto-cert                    │  │
│  └──────────────────────────────────────────────────┘  │
└────────────────────┬────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────┐
│              Certificate Manager                         │
│  ┌──────────────────────────────────────────────────┐  │
│  │  - Domain index (domain -> cert_id)              │  │
│  │  - Certificate storage (per-tenant)              │  │
│  │  - Metadata tracking                             │  │
│  │  - ACME integration (optional)                   │  │
│  └──────────────────────────────────────────────────┘  │
└────────────────────┬────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────┐
│              Tenant Router                               │
│  - Extract tenant from Host header                      │
│  - Route to tenant-specific WAF engine                  │
└────────────────────┬────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────┐
│              WAF Proxy                                   │
│  - Apply tenant-specific WAF rules                      │
│  - Forward to upstream                                  │
└─────────────────────────────────────────────────────────┘
```

## Testing

Run the test script:
```bash
chmod +x test-phase2.5.sh
./test-phase2.5.sh
```

The script tests:
1. WAF health check
2. Tenant creation
3. Self-signed certificate generation
4. Certificate upload via API
5. Certificate listing
6. HTTPS endpoint accessibility
7. SNI routing verification

## Security Considerations

1. **Certificate Storage**: Certificates are stored with restrictive permissions (0600 for keys, 0644 for certs)
2. **TLS Version**: Minimum TLS 1.2 enforced
3. **SNI Validation**: Only serves certificates for configured domains
4. **ACME TOS**: Must explicitly accept Let's Encrypt Terms of Service
5. **Auto-Renewal**: Only renews Let's Encrypt certificates, not uploaded ones

## Dependencies

Added to `go.mod`:
```
golang.org/x/crypto v0.24.0  # For ACME protocol support
```

## Files Modified/Created

**New Files:**
- `internal/cert/manager.go` (411 lines) - Certificate manager with SNI support
- `internal/cert/acme.go` (334 lines) - Let's Encrypt ACME integration
- `test-phase2.5.sh` - Test script

**Modified Files:**
- `internal/config/config.go` - Added TLSConfig structure
- `internal/api/handler.go` - Added certificate management endpoints
- `cmd/waf/main.go` - Added HTTPS server with SNI routing
- `go.mod` - Added golang.org/x/crypto dependency

## Version

Updated to v0.3.0 to reflect Phase 2.5 completion.

## Next Steps (Phase 3)

- Rate limiting per tenant
- Geo-blocking support
- Advanced caching strategies
- Metrics and monitoring
- Admin dashboard
