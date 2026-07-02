# Vinahost WAF

A production-ready Web Application Firewall (WAF) built on top of Coraza WAF engine, designed for multi-tenant hosting environments.

## Features

- **Coraza WAF Engine**: Built on Go-based Coraza WAF with ModSecurity compatibility
- **OWASP CRS Support**: Full compatibility with OWASP Core Rule Set v4
- **Reverse Proxy**: High-performance HTTP reverse proxy with WAF inspection
- **Multi-Tenancy Ready**: Architecture designed for multi-tenant isolation (Phase 2)
- **Real-time Logging**: Structured JSON logging with audit trail
- **Flexible Configuration**: YAML-based configuration with hot-reload support

## Architecture

```
Internet → Vinahost WAF (Go + Coraza) → Upstream Server
                ↓
         WAF Engine (Request/Response Inspection)
                ↓
         Rule Evaluation (OWASP CRS + Custom Rules)
                ↓
         Logging (JSON Audit Logs)
```

## Quick Start

### Prerequisites

- Go 1.21 or higher
- Docker (optional)

### Build from Source

```bash
# Clone repository
git clone https://github.com/vinahost/waf.git
cd waf

# Build
go build -o waf ./cmd/waf

# Run
./waf -config configs/config.yaml
```

### Docker

```bash
# Build image
docker build -t vinahost-waf .

# Run container
docker run -p 8080:8080 -v $(pwd)/configs:/app/configs vinahost-waf
```

### Docker Compose

```bash
# Start WAF with test upstream
docker-compose up -d

# Check logs
docker-compose logs -f waf
```

## Configuration

Edit `configs/config.yaml`:

```yaml
server:
  listen_addr: ":8080"
  workers: 4

proxy:
  upstream_url: "http://localhost:8081"

waf:
  rules_files:
    - "configs/rules/coreruleset.conf"
    - "configs/rules/custom.conf"
  request_body_access: true
  response_body_access: true
  default_action: "block"  # or "log"
```

## Adding Rules

### Custom Rules

Edit `configs/rules/custom.conf`:

```
SecRule REQUEST_URI "@rx ^/admin" \
    "id:10001,\
    phase:1,\
    deny,\
    status:403,\
    msg:'Admin access blocked'"
```

### OWASP Core Rule Set

1. Clone CRS repository:
```bash
git clone https://github.com/coreruleset/coreruleset.git
```

2. Copy rules to `configs/rules/`

3. Update `config.yaml` to include CRS rules

## Testing

### Test WAF Blocking

```bash
# Normal request (should pass)
curl http://localhost:8080/

# SQL injection attempt (should be blocked)
curl "http://localhost:8080/?id=1' OR '1'='1"

# XSS attempt (should be blocked)
curl "http://localhost:8080/?q=<script>alert('xss')</script>"
```

### Check Logs

```bash
# View WAF logs
tail -f /var/log/vinahost-waf/waf.log

# View audit logs
tail -f /var/log/vinahost-waf/audit.log
```

## API Endpoints

- `GET /health` - Health check endpoint
- `GET /metrics` - Prometheus metrics (Phase 3)

## Development

### Project Structure

```
vinahost-waf/
├── cmd/waf/           # Main entry point
├── internal/
│   ├── config/        # Configuration management
│   ├── waf/           # WAF engine wrapper
│   ├── proxy/         # Reverse proxy implementation
│   └── logging/       # Logging utilities
├── configs/           # Configuration files
│   ├── config.yaml    # Main config
│   └── rules/         # WAF rules
├── deployments/       # Deployment configs
└── test/              # Test files
```

### Run Tests

```bash
go test ./...
```

## Roadmap

- [x] Phase 1: Core WAF Engine + Reverse Proxy
- [ ] Phase 2: Multi-Tenancy Support
- [ ] Phase 3: Control Plane API
- [ ] Phase 4: Admin Portal
- [ ] Phase 5: Customer Portal
- [ ] Phase 6: Performance Optimization
- [ ] Phase 7: Production Hardening

## License

Apache 2.0

## Support

For support, email support@vinahost.vn or visit https://vinahost.vn
