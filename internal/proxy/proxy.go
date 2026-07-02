package proxy

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/corazawaf/coraza/v3/types"
	"go.uber.org/zap"

	"github.com/vinahost/waf/internal/config"
	"github.com/vinahost/waf/internal/waf"
)

// ReverseProxy wraps httputil.ReverseProxy with WAF inspection
type ReverseProxy struct {
	proxy  *httputil.ReverseProxy
	waf    *waf.Engine
	logger *zap.Logger
	config config.ProxyConfig
}

// NewReverseProxy creates a new reverse proxy with WAF integration
func NewReverseProxy(cfg config.ProxyConfig, wafEngine *waf.Engine, logger *zap.Logger) *ReverseProxy {
	// Parse upstream URL
	upstreamURL, err := url.Parse(cfg.UpstreamURL)
	if err != nil {
		logger.Fatal("Invalid upstream URL", zap.Error(err))
	}

	// Create reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(upstreamURL)

	// Configure transport
	proxy.Transport = &http.Transport{
		MaxIdleConns:        cfg.MaxIdleConns,
		MaxIdleConnsPerHost: cfg.MaxIdleConnsHost,
		IdleConnTimeout:     90 * time.Second,
	}

	// Custom error handler
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		logger.Error("Proxy error",
			zap.String("url", r.URL.String()),
			zap.Error(err),
		)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
	}

	return &ReverseProxy{
		proxy:  proxy,
		waf:    wafEngine,
		logger: logger,
		config: cfg,
	}
}

// ServeHTTP handles incoming HTTP requests with WAF inspection
func (rp *ReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	// Create WAF transaction
	tx := rp.waf.NewTransaction()
	defer tx.ProcessLogging()
	defer tx.Close()

	// Extract connection info
	clientIP, clientPort := extractClientInfo(r)
	serverIP, serverPort := extractServerInfo(r)

	// Phase 1: Process connection
	tx.ProcessConnection(clientIP, clientPort, serverIP, serverPort)

	// Phase 1: Process URI
	tx.ProcessURI(r.URL.String(), r.Method, r.Proto)

	// Phase 1: Add request headers
	for key, values := range r.Header {
		for _, value := range values {
			tx.AddRequestHeader(key, value)
		}
	}

	// Phase 1: Process request headers - check for interruption
	if it := tx.ProcessRequestHeaders(); it != nil {
		rp.handleInterruption(w, r, it, startTime)
		return
	}

	// Phase 2: Process request body if enabled
	if rp.waf.RequestBodyAccess() {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			rp.logger.Error("Failed to read request body", zap.Error(err))
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
		r.Body.Close()

		// Restore body for upstream proxy
		r.Body = io.NopCloser(bytes.NewBuffer(body))

		// Write body to WAF for inspection using ReadRequestBodyFrom
		if _, _, err := tx.ReadRequestBodyFrom(io.NopCloser(bytes.NewBuffer(body))); err != nil {
			rp.logger.Error("Failed to write request body to WAF", zap.Error(err))
		}

		// Process request body - check for interruption
		if it, err := tx.ProcessRequestBody(); err != nil {
			rp.logger.Error("Failed to process request body", zap.Error(err))
		} else if it != nil {
			rp.handleInterruption(w, r, it, startTime)
			return
		}
	}

	// Create response writer wrapper to capture response
	wrapper := &responseWriterWrapper{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		body:           &bytes.Buffer{},
	}

	// Forward request to upstream
	rp.proxy.ServeHTTP(wrapper, r)

	// Phase 3: Process response headers
	tx.ProcessResponseHeaders(wrapper.statusCode, r.Proto)

	// Phase 4: Process response body if enabled
	if rp.waf.ResponseBodyAccess() && wrapper.body.Len() > 0 {
		if _, _, err := tx.ReadResponseBodyFrom(io.NopCloser(bytes.NewBuffer(wrapper.body.Bytes()))); err != nil {
			rp.logger.Error("Failed to write response body to WAF", zap.Error(err))
		}

		if it, err := tx.ProcessResponseBody(); err != nil {
			rp.logger.Error("Failed to process response body", zap.Error(err))
		} else if it != nil {
			rp.logger.Warn("Response body matched WAF rule",
				zap.Int("rule_id", it.RuleID),
				zap.String("url", r.URL.String()),
			)
		}
	}

	// Log request completion
	duration := time.Since(startTime)
	rp.logger.Info("Request completed",
		zap.String("method", r.Method),
		zap.String("url", r.URL.String()),
		zap.Int("status", wrapper.statusCode),
		zap.Duration("duration", duration),
		zap.String("client_ip", clientIP),
	)
}

// handleInterruption handles WAF rule interruption (blocked request)
func (rp *ReverseProxy) handleInterruption(w http.ResponseWriter, r *http.Request, it *types.Interruption, startTime time.Time) {
	rp.logger.Warn("Request blocked by WAF",
		zap.Int("rule_id", it.RuleID),
		zap.String("action", it.Action),
		zap.Int("status", it.Status),
		zap.String("url", r.URL.String()),
		zap.String("client_ip", r.RemoteAddr),
	)

	// Return appropriate response
	switch it.Action {
	case "deny":
		w.WriteHeader(it.Status)
		w.Write([]byte("Request blocked by WAF"))
	case "redirect":
		// For redirect, we need to set Location header manually
		w.Header().Set("Location", "/")
		w.WriteHeader(it.Status)
	default:
		w.WriteHeader(it.Status)
	}

	// Log blocked request
	duration := time.Since(startTime)
	rp.logger.Info("Request blocked",
		zap.String("method", r.Method),
		zap.String("url", r.URL.String()),
		zap.Int("status", it.Status),
		zap.Duration("duration", duration),
	)
}

// extractClientInfo extracts client IP and port from request
func extractClientInfo(r *http.Request) (string, int) {
	// Check X-Forwarded-For header first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		ip := strings.TrimSpace(parts[0])
		return ip, 0
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri, 0
	}

	// Parse RemoteAddr
	host, portStr, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr, 0
	}

	port, _ := strconv.Atoi(portStr)
	return host, port
}

// extractServerInfo extracts server IP and port from request
func extractServerInfo(r *http.Request) (string, int) {
	host := r.Host
	if host == "" {
		return "127.0.0.1", 80
	}

	// Remove port if present
	h, portStr, err := net.SplitHostPort(host)
	if err != nil {
		// No port in host
		return host, 80
	}

	port, _ := strconv.Atoi(portStr)
	return h, port
}

// responseWriterWrapper wraps http.ResponseWriter to capture response
type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func (w *responseWriterWrapper) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *responseWriterWrapper) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}
