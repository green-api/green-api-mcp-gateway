// SPDX-License-Identifier: MIT
// Copyright (c) 2026 GREEN-API

package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/green-api/green-api-mcp-gateway/internal/infrastructure/config"
)

// writeTemp writes content to a temp file and returns its path.
// The file is removed when the test ends.
func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("writeTemp: %v", err)
	}
	return path
}

// ─── Default ──────────────────────────────────────────────────────────────────

func TestDefault(t *testing.T) {
	cfg := config.Default()

	if cfg.Server.Transport != "stdio" {
		t.Errorf("Server.Transport: got %q, want %q", cfg.Server.Transport, "stdio")
	}
	if cfg.Server.Port != 8090 {
		t.Errorf("Server.Port: got %d, want %d", cfg.Server.Port, 8090)
	}
	if cfg.Webhook.Mode != "polling" {
		t.Errorf("Webhook.Mode: got %q, want %q", cfg.Webhook.Mode, "polling")
	}
	if cfg.Webhook.PollingTimeout != 20 {
		t.Errorf("Webhook.PollingTimeout: got %d, want %d", cfg.Webhook.PollingTimeout, 20)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("Logging.Level: got %q, want %q", cfg.Logging.Level, "info")
	}
	if cfg.Logging.Format != "text" {
		t.Errorf("Logging.Format: got %q, want %q", cfg.Logging.Format, "text")
	}
	if cfg.Metrics.Enabled {
		t.Error("Metrics.Enabled: expected false by default")
	}
	if cfg.Metrics.Port != 9090 {
		t.Errorf("Metrics.Port: got %d, want %d", cfg.Metrics.Port, 9090)
	}
}

// ─── Load: empty path ─────────────────────────────────────────────────────────

func TestLoad_EmptyPath_ReturnsDefaults(t *testing.T) {
	// Clear env vars that could interfere.
	clearEnvVars(t)

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Transport != "stdio" {
		t.Errorf("Server.Transport: got %q, want %q", cfg.Server.Transport, "stdio")
	}
}

// ─── Load: valid YAML ─────────────────────────────────────────────────────────

func TestLoad_ValidYAML(t *testing.T) {
	clearEnvVars(t)

	yaml := `
server:
  transport: sse
  port: 9000
green_api:
  instances:
    - id: 111
      api_token: "tok111"
      api_url: "https://api.green-api.com"
logging:
  level: debug
  format: json
metrics:
  enabled: true
  port: 9091
`
	path := writeTemp(t, yaml)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Transport != "sse" {
		t.Errorf("Server.Transport: got %q, want %q", cfg.Server.Transport, "sse")
	}
	if cfg.Server.Port != 9000 {
		t.Errorf("Server.Port: got %d, want %d", cfg.Server.Port, 9000)
	}
	if len(cfg.GreenAPI.Instances) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(cfg.GreenAPI.Instances))
	}
	inst := cfg.GreenAPI.Instances[0]
	if inst.ID != 111 {
		t.Errorf("Instance.ID: got %d, want %d", inst.ID, 111)
	}
	if inst.APIToken != "tok111" {
		t.Errorf("Instance.APIToken: got %q, want %q", inst.APIToken, "tok111")
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("Logging.Level: got %q, want %q", cfg.Logging.Level, "debug")
	}
	if cfg.Logging.Format != "json" {
		t.Errorf("Logging.Format: got %q, want %q", cfg.Logging.Format, "json")
	}
	if !cfg.Metrics.Enabled {
		t.Error("Metrics.Enabled: expected true")
	}
	if cfg.Metrics.Port != 9091 {
		t.Errorf("Metrics.Port: got %d, want %d", cfg.Metrics.Port, 9091)
	}
}

// ─── Load: file not found ────────────────────────────────────────────────────

func TestLoad_FileNotFound(t *testing.T) {
	_, err := config.Load("/tmp/non-existent-config-xyz.yaml")
	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}
}

// ─── Load: invalid YAML ───────────────────────────────────────────────────────

func TestLoad_InvalidYAML(t *testing.T) {
	path := writeTemp(t, ":::: not valid yaml ::::")

	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

// ─── Env-var overrides ────────────────────────────────────────────────────────

func TestLoad_EnvOverride_Transport(t *testing.T) {
	clearEnvVars(t)
	t.Setenv("GREEN_API_TRANSPORT", "http")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Transport != "http" {
		t.Errorf("Server.Transport: got %q, want %q", cfg.Server.Transport, "http")
	}
}

func TestLoad_EnvOverride_Port(t *testing.T) {
	clearEnvVars(t)
	t.Setenv("GREEN_API_PORT", "7777")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Port != 7777 {
		t.Errorf("Server.Port: got %d, want %d", cfg.Server.Port, 7777)
	}
}

func TestLoad_EnvOverride_SingleInstance_New(t *testing.T) {
	clearEnvVars(t)
	t.Setenv("GREEN_API_INSTANCE_ID", "999")
	t.Setenv("GREEN_API_TOKEN", "secret-token")
	t.Setenv("GREEN_API_URL", "https://custom.green-api.com")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.GreenAPI.Instances) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(cfg.GreenAPI.Instances))
	}
	inst := cfg.GreenAPI.Instances[0]
	if inst.ID != 999 {
		t.Errorf("Instance.ID: got %d, want %d", inst.ID, 999)
	}
	if inst.APIToken != "secret-token" {
		t.Errorf("Instance.APIToken: got %q, want %q", inst.APIToken, "secret-token")
	}
	if inst.APIURL != "https://custom.green-api.com" {
		t.Errorf("Instance.APIURL: got %q, want %q", inst.APIURL, "https://custom.green-api.com")
	}
}

func TestLoad_EnvOverride_SingleInstance_DefaultURL(t *testing.T) {
	clearEnvVars(t)
	t.Setenv("GREEN_API_INSTANCE_ID", "111")
	t.Setenv("GREEN_API_TOKEN", "tok")
	// GREEN_API_URL not set → should default to https://api.green-api.com

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.GreenAPI.Instances) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(cfg.GreenAPI.Instances))
	}
	if cfg.GreenAPI.Instances[0].APIURL != "https://api.green-api.com" {
		t.Errorf("APIURL: got %q, want default", cfg.GreenAPI.Instances[0].APIURL)
	}
}

func TestLoad_EnvOverride_ExistingInstanceUpdated(t *testing.T) {
	clearEnvVars(t)

	yaml := `
green_api:
  instances:
    - id: 555
      api_token: "original"
      api_url: "https://api.green-api.com"
`
	path := writeTemp(t, yaml)

	t.Setenv("GREEN_API_INSTANCE_ID", "555")
	t.Setenv("GREEN_API_TOKEN", "updated-token")

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.GreenAPI.Instances) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(cfg.GreenAPI.Instances))
	}
	if cfg.GreenAPI.Instances[0].APIToken != "updated-token" {
		t.Errorf("APIToken: got %q, want %q", cfg.GreenAPI.Instances[0].APIToken, "updated-token")
	}
}

func TestLoad_EnvOverride_WebhookMode(t *testing.T) {
	clearEnvVars(t)
	t.Setenv("GREEN_API_WEBHOOK_MODE", "receiver")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Webhook.Mode != "receiver" {
		t.Errorf("Webhook.Mode: got %q, want %q", cfg.Webhook.Mode, "receiver")
	}
}

func TestLoad_EnvOverride_LogLevel(t *testing.T) {
	clearEnvVars(t)
	t.Setenv("LOG_LEVEL", "warn")
	t.Setenv("LOG_FORMAT", "json")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Logging.Level != "warn" {
		t.Errorf("Logging.Level: got %q, want %q", cfg.Logging.Level, "warn")
	}
	if cfg.Logging.Format != "json" {
		t.Errorf("Logging.Format: got %q, want %q", cfg.Logging.Format, "json")
	}
}

func TestLoad_EnvPort_InvalidValue_KeepsDefault(t *testing.T) {
	clearEnvVars(t)
	t.Setenv("GREEN_API_PORT", "not-a-number")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Port != 8090 {
		t.Errorf("Server.Port: got %d, want default 8090", cfg.Server.Port)
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────────

// clearEnvVars unsets all env vars used by applyEnvOverrides for the duration
// of the test, restoring them afterwards via t.Cleanup.
func clearEnvVars(t *testing.T) {
	t.Helper()
	vars := []string{
		"GREEN_API_TRANSPORT",
		"GREEN_API_PORT",
		"GREEN_API_INSTANCE_ID",
		"GREEN_API_TOKEN",
		"GREEN_API_URL",
		"GREEN_API_WEBHOOK_MODE",
		"LOG_LEVEL",
		"LOG_FORMAT",
	}
	for _, v := range vars {
		t.Setenv(v, "") // t.Setenv restores the original value on cleanup
		_ = os.Unsetenv(v)
	}
}
