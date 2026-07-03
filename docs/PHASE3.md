# Phase 3: Persistence, Metrics & Rate Limiting

## Tổng quan

Phase 3 bổ sung các tính năng production-ready cho WAF multi-tenant:
- **PostgreSQL Persistence**: Lưu trữ tenants vào database thay vì chỉ in-memory
- **Rate Limiting**: Giới hạn request rate per client IP
- **Metrics & Monitoring**: Thu thập metrics và expose qua Prometheus endpoint

## 1. PostgreSQL Persistence

### Kiến trúc

```
TenantManager
    ↓
Storage Interface
    ↓
├── PostgresStore (production)
└── MemoryStore (fallback/testing)
```

### Schema

```sql
CREATE TABLE tenants (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    domains TEXT[] NOT NULL,
    enabled BOOLEAN DEFAULT true,
    config JSONB,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE tenant_rules (
    id SERIAL PRIMARY KEY,
    tenant_id VARCHAR(255) REFERENCES tenants(id) ON DELETE CASCADE,
    rule_path TEXT NOT NULL,
    UNIQUE(tenant_id, rule_path)
);
```

### Cấu hình

```yaml
storage:
  enabled: true
  type: "postgres"
  host: "postgres"
  port: 5432
  user: "waf"
  password: "waf_password"
  dbname: "vinahost_waf"
  sslmode: "disable"
```

### Fallback Strategy

Nếu PostgreSQL không khả dụng, hệ thống tự động fallback về MemoryStore:
- Tenants vẫn hoạt động nhưng không persist qua restart
- Log warning để admin biết

## 2. Rate Limiting

### Thuật toán

Sử dụng **Token Bucket** algorithm:
- Mỗi client IP có một bucket
- Bucket chứa tối đa `rate` tokens
- Mỗi request tiêu tốn 1 token
- Tokens được refill theo `interval`

### Cấu hình

```go
rateLimiter := ratelimit.NewRateLimiter(cache, 1000, time.Minute)
// 1000 requests per minute per IP
```

### Integration

Rate limiter được tích hợp vào WAF handler:
```go
if !rateLimiter.Allow(ctx, clientIP) {
    metrics.RecordRequest(tenantID, true, 0)  // blocked
    metrics.RecordRuleHit("rate_limit")
    http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
    return
}
```

### Cleanup

Background goroutine cleanup expired buckets mỗi 5 phút để tránh memory leak.

## 3. Metrics & Monitoring

### Metrics Collection

```go
type Metrics struct {
    TotalRequests    atomic.Int64
    BlockedRequests  atomic.Int64
    TenantRequests   map[string]*atomic.Int64
    RuleHits         map[string]*atomic.Int64
    LatencySum       atomic.Int64
    LatencyCount     atomic.Int64
}
```

### Prometheus Endpoint

Expose tại `/metrics` theo Prometheus format:

```
# HELP waf_requests_total Total number of requests
# TYPE waf_requests_total counter
waf_requests_total{tenant="default"} 1234
waf_requests_total{tenant="tenant1"} 567

# HELP waf_blocked_requests_total Total number of blocked requests
# TYPE waf_blocked_requests_total counter
waf_blocked_requests_total 42

# HELP waf_request_latency_seconds Request latency in seconds
# TYPE waf_request_latency_seconds histogram
waf_request_latency_seconds_bucket{le="0.1"} 100
waf_request_latency_seconds_bucket{le="0.5"} 200
waf_request_latency_seconds_bucket{le="1.0"} 250
```

### Grafana Dashboard

Có thể import dashboard JSON để visualize:
- Request rate per tenant
- Block rate
- Latency percentiles (p50, p95, p99)
- Top rule hits

## 4. API Enhancements

### Health Check

```bash
curl http://localhost:9080/health
```

Response:
```json
{
  "status": "healthy",
  "version": "0.4.0",
  "uptime": "2h30m",
  "go_version": "go1.21",
  "mode": "multi-tenant",
  "tls": false,
  "storage": "*storage.PostgresStore",
  "metrics": true
}
```

### Metrics Endpoint

```bash
curl http://localhost:9080/metrics
```

Returns Prometheus-format metrics.

## 5. Docker Compose Updates

### Services

```yaml
services:
  waf:
    depends_on:
      - redis
      - postgres
  
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
  
  postgres:
    image: postgres:15-alpine
    environment:
      - POSTGRES_USER=waf
      - POSTGRES_PASSWORD=waf_password
      - POSTGRES_DB=vinahost_waf
    ports:
      - "5432:5432"
```

### Ports

- WAF: 9080 (HTTP proxy)
- Redis: 6379 (cache)
- PostgreSQL: 5432 (storage)
- Upstream: 9081 (test server)

## 6. Migration từ Phase 2

### Backward Compatibility

- Memory storage vẫn hoạt động nếu không config PostgreSQL
- Tenants tạo qua API vẫn hoạt động như cũ
- Cache layer không thay đổi

### Migration Steps

1. Update docker-compose.yml
2. Update config.yaml với storage config
3. Restart services: `docker-compose down && docker-compose up -d`
4. Verify: `curl http://localhost:9080/health`

## 7. Testing

### Create Tenant with Persistence

```bash
curl -X POST http://localhost:9080/api/v1/tenants \
  -H "Content-Type: application/json" \
  -d '{
    "id": "tenant1",
    "name": "Test Tenant",
    "domains": ["test.example.com"],
    "enabled": true
  }'
```

### Verify Persistence

```bash
# Restart WAF
docker-compose restart waf

# Tenant should still exist
curl http://localhost:9080/api/v1/tenants/tenant1
```

### Test Rate Limiting

```bash
# Send 1001 requests quickly
for i in {1..1001}; do
  curl -s http://localhost:9080/ > /dev/null &
done
wait

# Last request should get 429
curl -i http://localhost:9080/
# HTTP/1.1 429 Too Many Requests
```

### Test Metrics

```bash
curl http://localhost:9080/metrics | grep waf_
```

## 8. Performance Considerations

### PostgreSQL

- Connection pooling: Max 25 connections
- Query optimization: Indexes on tenant_id, domains
- Batch operations: ListTenants uses single query

### Rate Limiting

- Memory usage: ~1KB per unique IP
- Cleanup: Every 5 minutes
- Cache backend: Uses Redis if available, else memory

### Metrics

- Atomic operations: Lock-free counters
- Memory usage: ~100 bytes per tenant + rule
- No persistence: Metrics reset on restart

## 9. Next Steps (Phase 4)

- Admin Dashboard (React frontend)
- Audit logging to database
- Tenant-specific rate limits
- Advanced analytics
- Alerting system

## 10. Troubleshooting

### PostgreSQL Connection Failed

```
WARN: Failed to connect to PostgreSQL, falling back to memory storage
```

**Solution**: Check PostgreSQL is running:
```bash
docker-compose ps postgres
docker-compose logs postgres
```

### Rate Limiter Not Working

**Check**: Rate limiter initialized in logs:
```
INFO: Rate limiter initialized rate=1000 interval=1m0s
```

### Metrics Not Updating

**Check**: Metrics endpoint accessible:
```bash
curl http://localhost:9080/metrics
```

## 11. Security Notes

- PostgreSQL password should be changed in production
- Use SSL for PostgreSQL in production (`sslmode: require`)
- Rate limiter prevents DDoS but not sophisticated attacks
- Metrics endpoint should be protected in production (auth/IP whitelist)
