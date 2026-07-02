# Vinahost WAF - Deployment Guide

## 📋 Table of Contents

1. [Quick Start](#quick-start)
2. [Docker Deployment](#docker-deployment)
3. [Kubernetes Deployment](#kubernetes-deployment)
4. [Production Deployment](#production-deployment)
5. [Configuration](#configuration)
6. [Monitoring](#monitoring)

---

## 🚀 Quick Start

### Prerequisites

- Docker & Docker Compose
- Go 1.21+ (for building from source)
- Upstream web server (nginx, apache, etc.)

### 1. Clone Repository

```bash
git clone https://github.com/vuottt-vn/hermes-waf.git
cd hermes-waf
```

### 2. Configure

Edit `configs/config.yaml`:

```yaml
server:
  listen_addr: ":8080"
  
proxy:
  upstream_url: "http://your-upstream-server:80"
  
waf:
  rules_files:
    - "configs/rules/custom.conf"
```

### 3. Run with Docker Compose

```bash
docker-compose up -d
```

WAF will be available at `http://localhost:8080`

---

## 🐳 Docker Deployment

### Build Image

```bash
# Build locally
docker build -t vinahost-waf:latest .

# Or pull from GitHub Container Registry
docker pull ghcr.io/vuottt-vn/hermes-waf:latest
```

### Run Container

```bash
docker run -d \
  --name vinahost-waf \
  -p 8080:8080 \
  -v $(pwd)/configs:/app/configs:ro \
  -v waf-logs:/var/log/vinahost-waf \
  -e LOG_LEVEL=info \
  vinahost-waf:latest
```

### Docker Compose (Production)

```yaml
version: '3.8'

services:
  waf:
    image: ghcr.io/vuottt-vn/hermes-waf:latest
    ports:
      - "8080:8080"
    volumes:
      - ./configs:/app/configs:ro
      - waf-logs:/var/log/vinahost-waf
    environment:
      - LOG_LEVEL=info
      - CONFIG_FILE=/app/configs/config.yaml
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 512M
        reservations:
          cpus: '1'
          memory: 256M

volumes:
  waf-logs:
```

---

## ☸️ Kubernetes Deployment

### 1. Create Namespace

```bash
kubectl create namespace vinahost-waf
```

### 2. Create ConfigMap

```bash
kubectl create configmap waf-config \
  --from-file=configs/config.yaml \
  --from-file=configs/rules/ \
  -n vinahost-waf
```

### 3. Deploy

```bash
kubectl apply -f deployments/k8s/ -n vinahost-waf
```

### Kubernetes Manifests

**deployment.yaml:**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: vinahost-waf
  namespace: vinahost-waf
spec:
  replicas: 3
  selector:
    matchLabels:
      app: vinahost-waf
  template:
    metadata:
      labels:
        app: vinahost-waf
    spec:
      containers:
      - name: waf
        image: ghcr.io/vuottt-vn/hermes-waf:latest
        ports:
        - containerPort: 8080
        volumeMounts:
        - name: config
          mountPath: /app/configs
          readOnly: true
        - name: logs
          mountPath: /var/log/vinahost-waf
        resources:
          requests:
            cpu: "500m"
            memory: "256Mi"
          limits:
            cpu: "2000m"
            memory: "512Mi"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
      volumes:
      - name: config
        configMap:
          name: waf-config
      - name: logs
        emptyDir: {}
```

**service.yaml:**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: vinahost-waf
  namespace: vinahost-waf
spec:
  type: LoadBalancer
  ports:
  - port: 80
    targetPort: 8080
    protocol: TCP
  selector:
    app: vinahost-waf
```

**hpa.yaml (Horizontal Pod Autoscaler):**

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: vinahost-waf
  namespace: vinahost-waf
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: vinahost-waf
  minReplicas: 3
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

---

## 🏭 Production Deployment

### Architecture

```
Internet → Load Balancer (HAProxy/Cloud LB)
              ↓
         WAF Cluster (3+ nodes)
              ↓
         Upstream Servers
```

### Load Balancer Configuration (HAProxy)

```haproxy
frontend waf_frontend
    bind *:80
    bind *:443 ssl crt /etc/ssl/certs/
    
    # Rate limiting
    stick-table type ip size 100k expire 30s store http_req_rate(10s)
    http-request track-sc0 src
    http-request deny if { sc_http_req_rate(0) gt 100 }
    
    default_backend waf_backend

backend waf_backend
    balance roundrobin
    option httpchk GET /health
    
    server waf1 10.0.1.10:8080 check
    server waf2 10.0.1.11:8080 check
    server waf3 10.0.1.12:8080 check
```

### System Tuning

**/etc/sysctl.conf:**

```bash
# Increase file descriptors
fs.file-max = 1000000

# TCP tuning
net.core.somaxconn = 65535
net.ipv4.tcp_max_syn_backlog = 65535
net.ipv4.tcp_fin_timeout = 30
net.ipv4.tcp_keepalive_time = 120
net.ipv4.tcp_tw_reuse = 1

# Network buffer
net.core.rmem_max = 16777216
net.core.wmem_max = 16777216
```

Apply: `sysctl -p`

### Systemd Service

**/etc/systemd/system/vinahost-waf.service:**

```ini
[Unit]
Description=Vinahost WAF Service
After=network.target

[Service]
Type=simple
User=waf
Group=waf
WorkingDirectory=/opt/vinahost-waf
ExecStart=/opt/vinahost-waf/bin/waf -config /opt/vinahost-waf/configs/config.yaml
Restart=always
RestartSec=10
LimitNOFILE=65536

# Security
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable vinahost-waf
sudo systemctl start vinahost-waf
```

---

## ⚙️ Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `CONFIG_FILE` | `configs/config.yaml` | Path to config file |
| `WORKERS` | `4` | Number of worker goroutines |

### Config File (configs/config.yaml)

```yaml
server:
  listen_addr: ":8080"
  workers: 4
  read_timeout: 30
  write_timeout: 30

proxy:
  upstream_url: "http://backend:80"
  proxy_timeout: 60
  max_idle_conns: 100
  max_idle_conns_host: 10

waf:
  request_body_access: true
  request_body_limit: 13107200  # 12.5 MB
  response_body_access: true
  response_body_limit: 524288   # 512 KB
  rules_files:
    - "configs/rules/custom.conf"
  default_action: "block"  # or "log"
```

### WAF Rules

**Custom Rules (configs/rules/custom.conf):**

```apache
# Block SQL injection
SecRule ARGS "@detectSQLi" "id:1001,phase:2,deny,status:403,log,msg:'SQL Injection Detected'"

# Block XSS
SecRule ARGS "@detectXSS" "id:1002,phase:2,deny,status:403,log,msg:'XSS Attack Detected'"

# Block common tools
SecRule REQUEST_HEADERS:User-Agent "@rx (?:nikto|sqlmap|nmap)" \
    "id:1003,phase:1,deny,status:403,log,msg:'Security Scanner Blocked'"
```

---

## 📊 Monitoring

### Health Check Endpoint

```bash
curl http://localhost:8080/health
```

Response:
```json
{
  "status": "healthy",
  "version": "0.1.0",
  "uptime": "2h30m15s"
}
```

### Logs

Logs are written to `/var/log/vinahost-waf/waf.log` in JSON format:

```json
{
  "level": "info",
  "ts": "2026-07-02T14:57:49.480+0700",
  "caller": "proxy/proxy.go:156",
  "msg": "Request completed",
  "method": "GET",
  "url": "/api/users",
  "status": 200,
  "duration": 0.045,
  "client_ip": "192.168.1.100"
}
```

### Metrics (Prometheus)

Add to `docker-compose.yml`:

```yaml
services:
  prometheus:
    image: prom/prometheus:latest
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
    ports:
      - "9090:9090"

  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
```

---

## 🔧 Troubleshooting

### WAF not starting

```bash
# Check logs
docker logs vinahost-waf

# Verify config
docker exec vinahost-waf cat /app/configs/config.yaml

# Test rules syntax
docker exec vinahost-waf /app/waf -config /app/configs/config.yaml -test
```

### Upstream connection failed

```bash
# Check upstream is accessible
curl http://your-upstream:80/health

# Verify network
docker exec vinahost-waf ping your-upstream
```

### High memory usage

```bash
# Reduce body inspection limits
waf:
  request_body_limit: 1048576   # 1 MB
  response_body_limit: 524288   # 512 KB

# Disable response body inspection if not needed
waf:
  response_body_access: false
```

---

## 📚 Additional Resources

- [Coraza WAF Documentation](https://coraza.io/docs/)
- [OWASP Core Rule Set](https://github.com/coreruleset/coreruleset)
- [ModSecurity Reference Manual](https://github.com/SpiderLabs/ModSecurity/wiki/Reference-Manual)

---

## 🆘 Support

- **Issues:** https://github.com/vuottt-vn/hermes-waf/issues
- **Email:** support@vinahost.vn
- **Documentation:** https://docs.vinahost.vn/waf
