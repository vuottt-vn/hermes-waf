package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the main configuration structure
type Config struct {
	Server  ServerConfig  `yaml:"server"`
	TLS     TLSConfig     `yaml:"tls"`
	Proxy   ProxyConfig   `yaml:"proxy"`
	WAF     WAFConfig     `yaml:"waf"`
	Cache   CacheConfig   `yaml:"cache"`
	Storage StorageConfig `yaml:"storage"`
	Logging LoggingConfig `yaml:"logging"`
}

// TLSConfig contains TLS/SSL settings for the WAF listener
type TLSConfig struct {
	Enabled     bool   `yaml:"enabled"`
	ListenAddr  string `yaml:"listen_addr"`
	CertDir     string `yaml:"cert_dir"`
	ACMEEmail   string `yaml:"acme_email"`
	ACMEDirURL  string `yaml:"acme_dir_url"`
	AcceptTOS   bool   `yaml:"accept_tos"`
	HTTP01Addr  string `yaml:"http01_addr"` // address for ACME HTTP-01 challenge server
}

// ServerConfig contains HTTP server settings
type ServerConfig struct {
	ListenAddr   string `yaml:"listen_addr"`
	Workers      int    `yaml:"workers"`
	ReadTimeout  int    `yaml:"read_timeout"`
	WriteTimeout int    `yaml:"write_timeout"`
	IdleTimeout  int    `yaml:"idle_timeout"`
}

// ProxyConfig contains reverse proxy settings
type ProxyConfig struct {
	UpstreamURL      string `yaml:"upstream_url"`
	ProxyTimeout     int    `yaml:"proxy_timeout"`
	MaxIdleConns     int    `yaml:"max_idle_conns"`
	MaxIdleConnsHost int    `yaml:"max_idle_conns_per_host"`
	EnableHTTPS      bool   `yaml:"enable_https"`
	TLSCertFile      string `yaml:"tls_cert_file"`
	TLSKeyFile       string `yaml:"tls_key_file"`
}

// WAFConfig contains WAF engine settings
type WAFConfig struct {
	RulesFiles        []string `yaml:"rules_files"`
	RequestBodyAccess bool     `yaml:"request_body_access"`
	RequestBodyLimit  int64    `yaml:"request_body_limit"`
	ResponseBodyAccess bool    `yaml:"response_body_access"`
	ResponseBodyLimit int64    `yaml:"response_body_limit"`
	AuditLogEnabled   bool     `yaml:"audit_log_enabled"`
	AuditLogFile      string   `yaml:"audit_log_file"`
	DebugLogEnabled   bool     `yaml:"debug_log_enabled"`
	DebugLogLevel     int      `yaml:"debug_log_level"`
	DefaultAction     string   `yaml:"default_action"` // "block" or "log"
}

// CacheConfig contains cache settings
type CacheConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Type     string `yaml:"type"` // "redis" or "memory"
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	Prefix   string `yaml:"prefix"`
	TTL      int    `yaml:"ttl"` // seconds
}

// StorageConfig contains storage settings for tenant persistence
type StorageConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Type     string `yaml:"type"` // "postgres" or "memory"
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
	SSLMode  string `yaml:"sslmode"`
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	Level      string `yaml:"level"`
	Format     string `yaml:"format"` // "json" or "console"
	OutputPath string `yaml:"output_path"`
}

// LoadConfig loads configuration from YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	setDefaults(&cfg)

	// Validate configuration
	if err := validateConfig(&cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// setDefaults sets default values for configuration
func setDefaults(cfg *Config) {
	// Server defaults
	if cfg.Server.ListenAddr == "" {
		cfg.Server.ListenAddr = ":8080"
	}
	if cfg.Server.Workers == 0 {
		cfg.Server.Workers = 4
	}
	if cfg.Server.ReadTimeout == 0 {
		cfg.Server.ReadTimeout = 30
	}
	if cfg.Server.WriteTimeout == 0 {
		cfg.Server.WriteTimeout = 30
	}
	if cfg.Server.IdleTimeout == 0 {
		cfg.Server.IdleTimeout = 120
	}

	// TLS defaults
	if cfg.TLS.ListenAddr == "" {
		cfg.TLS.ListenAddr = ":8443"
	}
	if cfg.TLS.CertDir == "" {
		cfg.TLS.CertDir = "/opt/vinahost-waf/certs"
	}
	if cfg.TLS.HTTP01Addr == "" {
		cfg.TLS.HTTP01Addr = ":80"
	}

	// Proxy defaults
	if cfg.Proxy.ProxyTimeout == 0 {
		cfg.Proxy.ProxyTimeout = 60
	}
	if cfg.Proxy.MaxIdleConns == 0 {
		cfg.Proxy.MaxIdleConns = 100
	}
	if cfg.Proxy.MaxIdleConnsHost == 0 {
		cfg.Proxy.MaxIdleConnsHost = 10
	}

	// WAF defaults
	if cfg.WAF.RequestBodyLimit == 0 {
		cfg.WAF.RequestBodyLimit = 13 * 1024 * 1024 // 13MB
	}
	if cfg.WAF.ResponseBodyLimit == 0 {
		cfg.WAF.ResponseBodyLimit = 512 * 1024 // 512KB
	}
	if cfg.WAF.DefaultAction == "" {
		cfg.WAF.DefaultAction = "block"
	}

	// Cache defaults
	if cfg.Cache.Type == "" {
		cfg.Cache.Type = "memory"
	}
	if cfg.Cache.Prefix == "" {
		cfg.Cache.Prefix = "vinahost-waf:"
	}
	if cfg.Cache.TTL == 0 {
		cfg.Cache.TTL = 300 // 5 minutes
	}

	// Storage defaults
	if cfg.Storage.Type == "" {
		cfg.Storage.Type = "memory"
	}
	if cfg.Storage.Port == 0 {
		cfg.Storage.Port = 5432
	}
	if cfg.Storage.DBName == "" {
		cfg.Storage.DBName = "vinahost_waf"
	}
	if cfg.Storage.SSLMode == "" {
		cfg.Storage.SSLMode = "disable"
	}

	// Logging defaults
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
	}
	if cfg.Logging.Format == "" {
		cfg.Logging.Format = "json"
	}
}

// validateConfig validates the configuration
func validateConfig(cfg *Config) error {
	if cfg.Proxy.UpstreamURL == "" {
		return fmt.Errorf("proxy.upstream_url is required")
	}

	if cfg.WAF.DefaultAction != "block" && cfg.WAF.DefaultAction != "log" {
		return fmt.Errorf("waf.default_action must be 'block' or 'log'")
	}

	return nil
}
