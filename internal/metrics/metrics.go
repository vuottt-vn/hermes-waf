package metrics

import (
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics collects WAF metrics
type Metrics struct {
	mu sync.RWMutex

	// Request counters
	TotalRequests    atomic.Int64
	BlockedRequests  atomic.Int64
	AllowedRequests  atomic.Int64

	// Per-tenant metrics
	TenantRequests   map[string]*atomic.Int64
	TenantBlocked    map[string]*atomic.Int64

	// Latency tracking
	RequestLatencies []time.Duration
	latencyMu        sync.Mutex

	// WAF rule hits
	RuleHits map[string]*atomic.Int64
}

// NewMetrics creates a new metrics collector
func NewMetrics() *Metrics {
	return &Metrics{
		TenantRequests:   make(map[string]*atomic.Int64),
		TenantBlocked:    make(map[string]*atomic.Int64),
		RequestLatencies: make([]time.Duration, 0),
		RuleHits:         make(map[string]*atomic.Int64),
	}
}

// RecordRequest records a request
func (m *Metrics) RecordRequest(tenantID string, blocked bool, latency time.Duration) {
	m.TotalRequests.Add(1)

	if blocked {
		m.BlockedRequests.Add(1)
		m.getTenantBlocked(tenantID).Add(1)
	} else {
		m.AllowedRequests.Add(1)
	}

	m.getTenantRequests(tenantID).Add(1)

	// Record latency
	m.latencyMu.Lock()
	m.RequestLatencies = append(m.RequestLatencies, latency)
	// Keep only last 1000 latencies
	if len(m.RequestLatencies) > 1000 {
		m.RequestLatencies = m.RequestLatencies[len(m.RequestLatencies)-1000:]
	}
	m.latencyMu.Unlock()
}

// RecordRuleHit records a rule hit
func (m *Metrics) RecordRuleHit(ruleID string) {
	m.getRuleHits(ruleID).Add(1)
}

func (m *Metrics) getTenantRequests(tenantID string) *atomic.Int64 {
	m.mu.RLock()
	counter, exists := m.TenantRequests[tenantID]
	m.mu.RUnlock()

	if exists {
		return counter
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Double-check after acquiring write lock
	if counter, exists = m.TenantRequests[tenantID]; exists {
		return counter
	}

	counter = &atomic.Int64{}
	m.TenantRequests[tenantID] = counter
	return counter
}

func (m *Metrics) getTenantBlocked(tenantID string) *atomic.Int64 {
	m.mu.RLock()
	counter, exists := m.TenantBlocked[tenantID]
	m.mu.RUnlock()

	if exists {
		return counter
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	
	if counter, exists = m.TenantBlocked[tenantID]; exists {
		return counter
	}

	counter = &atomic.Int64{}
	m.TenantBlocked[tenantID] = counter
	return counter
}

func (m *Metrics) getRuleHits(ruleID string) *atomic.Int64 {
	m.mu.RLock()
	counter, exists := m.RuleHits[ruleID]
	m.mu.RUnlock()

	if exists {
		return counter
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	
	if counter, exists = m.RuleHits[ruleID]; exists {
		return counter
	}

	counter = &atomic.Int64{}
	m.RuleHits[ruleID] = counter
	return counter
}

// GetAverageLatency returns average request latency
func (m *Metrics) GetAverageLatency() time.Duration {
	m.latencyMu.Lock()
	defer m.latencyMu.Unlock()

	if len(m.RequestLatencies) == 0 {
		return 0
	}

	var total time.Duration
	for _, l := range m.RequestLatencies {
		total += l
	}
	return total / time.Duration(len(m.RequestLatencies))
}

// Handler returns HTTP handler for metrics endpoint
func (m *Metrics) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")

		// Prometheus format
		fmt.Fprintf(w, "# HELP waf_requests_total Total number of requests\n")
		fmt.Fprintf(w, "# TYPE waf_requests_total counter\n")
		fmt.Fprintf(w, "waf_requests_total %d\n", m.TotalRequests.Load())

		fmt.Fprintf(w, "# HELP waf_requests_blocked Total number of blocked requests\n")
		fmt.Fprintf(w, "# TYPE waf_requests_blocked counter\n")
		fmt.Fprintf(w, "waf_requests_blocked %d\n", m.BlockedRequests.Load())

		fmt.Fprintf(w, "# HELP waf_requests_allowed Total number of allowed requests\n")
		fmt.Fprintf(w, "# TYPE waf_requests_allowed counter\n")
		fmt.Fprintf(w, "waf_requests_allowed %d\n", m.AllowedRequests.Load())

		fmt.Fprintf(w, "# HELP waf_request_latency_seconds Average request latency\n")
		fmt.Fprintf(w, "# TYPE waf_request_latency_seconds gauge\n")
		fmt.Fprintf(w, "waf_request_latency_seconds %f\n", m.GetAverageLatency().Seconds())

		// Per-tenant metrics
		m.mu.RLock()
		for tenantID, counter := range m.TenantRequests {
			fmt.Fprintf(w, "waf_tenant_requests_total{tenant=\"%s\"} %d\n", tenantID, counter.Load())
		}
		for tenantID, counter := range m.TenantBlocked {
			fmt.Fprintf(w, "waf_tenant_blocked_total{tenant=\"%s\"} %d\n", tenantID, counter.Load())
		}
		m.mu.RUnlock()

		// Rule hits
		m.mu.RLock()
		for ruleID, counter := range m.RuleHits {
			fmt.Fprintf(w, "waf_rule_hits_total{rule=\"%s\"} %d\n", ruleID, counter.Load())
		}
		m.mu.RUnlock()
	})
}
