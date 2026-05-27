// SPDX-License-Identifier: MIT
// Copyright (c) 2026 GREEN-API

// Package config handles YAML configuration loading with env-var fallback.
package config

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

// TLSConfig holds TLS/HTTPS settings for HTTP transports.
type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`   // enable HTTPS
	CertFile string `yaml:"cert_file"` // path to PEM certificate
	KeyFile  string `yaml:"key_file"`  // path to PEM private key
}

// ServerConfig holds MCP server transport settings.
type ServerConfig struct {
	Transport string    `yaml:"transport"` // stdio | sse | http
	Port      int       `yaml:"port"`      // used for SSE/HTTP transports
	TLS       TLSConfig `yaml:"tls"`
}

// InstanceConfig holds credentials for a single Green API instance.
type InstanceConfig struct {
	ID       uint64 `yaml:"id"`
	APIToken string `yaml:"api_token"`
	APIURL   string `yaml:"api_url"`
}

// GreenAPIConfig holds all Green API instance configs.
type GreenAPIConfig struct {
	Instances []InstanceConfig `yaml:"instances"`
}

// AuthConfig controls authentication mode for the MCP server.
type AuthConfig struct {
	Mode     string `yaml:"mode"`      // config | proxy
	CacheTTL int    `yaml:"cache_ttl"` // seconds for credential cache (proxy mode only)
}

// WebhookConfig controls how notifications are received.
type WebhookConfig struct {
	Mode           string `yaml:"mode"`            // polling | receiver
	PollingTimeout int    `yaml:"polling_timeout"` // seconds for long-poll
	ReceiverPort   int    `yaml:"receiver_port"`   // HTTP port when mode=receiver
}

// LoggingConfig controls log output.
type LoggingConfig struct {
	Level  string `yaml:"level"`  // debug | info | warn | error
	Format string `yaml:"format"` // text | json
}

// MetricsConfig controls Prometheus metrics.
type MetricsConfig struct {
	Enabled bool `yaml:"enabled"`
	Port    int  `yaml:"port"`
}

// RateLimitConfig controls per-instance rate limiting.
type RateLimitConfig struct {
	RequestsPerSecond float64 `yaml:"requests_per_second"`
	Burst             int     `yaml:"burst"`
	Enabled           bool    `yaml:"enabled"`
}

// Config is the root configuration structure.
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Auth      AuthConfig      `yaml:"auth"`
	GreenAPI  GreenAPIConfig  `yaml:"green_api"`
	Webhook   WebhookConfig   `yaml:"webhook"`
	Logging   LoggingConfig   `yaml:"logging"`
	Metrics   MetricsConfig   `yaml:"metrics"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
}

// Default returns a Config with sensible defaults.
func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Transport: "stdio",
			Port:      8090,
		},
		Auth: AuthConfig{
			Mode:     "config",
			CacheTTL: 300, // 5 minutes
		},
		Webhook: WebhookConfig{
			Mode:           "polling",
			PollingTimeout: 20,
			ReceiverPort:   8091,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
		},
		Metrics: MetricsConfig{
			Enabled: false,
			Port:    9090,
		},
		RateLimit: RateLimitConfig{
			RequestsPerSecond: 10,
			Burst:             20,
			Enabled:           true,
		},
	}
}

// Load loads configuration from a YAML file (if path is non-empty) and then
// applies env-var overrides. Missing file path → use only defaults + env vars.
func Load(filePath string) (*Config, error) {
	cfg := Default()

	if filePath != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("reading config file %q: %w", filePath, err)
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parsing config file %q: %w", filePath, err)
		}
	}

	// Apply env-var overrides (single-instance shortcut for simple deployments).
	applyEnvOverrides(cfg)

	return cfg, nil
}

// applyEnvOverrides overlays environment variables on top of the loaded config.
func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("GREEN_API_TRANSPORT"); v != "" {
		cfg.Server.Transport = v
	}
	if v := os.Getenv("GREEN_API_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = port
		}
	}

	// Single-instance env shortcut (backward-compatible).
	instanceIDStr := os.Getenv("GREEN_API_INSTANCE_ID")
	apiToken := os.Getenv("GREEN_API_TOKEN")
	apiURL := os.Getenv("GREEN_API_URL")

	if instanceIDStr != "" && apiToken != "" {
		instanceID, err := strconv.ParseUint(instanceIDStr, 10, 64)
		if err == nil {
			if apiURL == "" {
				apiURL = "https://api.green-api.com"
			}
			found := false
			for i, inst := range cfg.GreenAPI.Instances {
				if inst.ID == instanceID {
					cfg.GreenAPI.Instances[i].APIToken = apiToken
					cfg.GreenAPI.Instances[i].APIURL = apiURL
					found = true
					break
				}
			}
			if !found {
				cfg.GreenAPI.Instances = append(cfg.GreenAPI.Instances, InstanceConfig{
					ID:       instanceID,
					APIToken: apiToken,
					APIURL:   apiURL,
				})
			}
		}
	}

	if v := os.Getenv("GREEN_API_AUTH_MODE"); v != "" {
		cfg.Auth.Mode = v
	}
	if v := os.Getenv("GREEN_API_AUTH_CACHE_TTL"); v != "" {
		if ttl, err := strconv.Atoi(v); err == nil {
			cfg.Auth.CacheTTL = ttl
		}
	}
	if v := os.Getenv("GREEN_API_WEBHOOK_MODE"); v != "" {
		cfg.Webhook.Mode = v
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.Logging.Level = v
	}
	if v := os.Getenv("LOG_FORMAT"); v != "" {
		cfg.Logging.Format = v
	}
}
