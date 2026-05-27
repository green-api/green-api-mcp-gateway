// SPDX-License-Identifier: MIT
// Copyright (c) 2026 GREEN-API

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/green-api/green-api-mcp-gateway/internal/application"
	"github.com/green-api/green-api-mcp-gateway/internal/domain"
	"github.com/green-api/green-api-mcp-gateway/internal/infrastructure"
	"github.com/green-api/green-api-mcp-gateway/internal/infrastructure/config"
	"github.com/green-api/green-api-mcp-gateway/internal/infrastructure/greenapi"
	mcpserver "github.com/green-api/green-api-mcp-gateway/internal/infrastructure/mcp"
	"github.com/green-api/green-api-mcp-gateway/internal/infrastructure/monitoring"
	"github.com/green-api/green-api-mcp-gateway/internal/infrastructure/ratelimit"
	"github.com/green-api/green-api-mcp-gateway/internal/infrastructure/webhook"
)

// version is injected at build time via -ldflags "-X main.version=<tag>".
// Falls back to "dev" when building without tags.
var version = "dev"

var configFile = flag.String("config", "", "path to YAML config file (optional; env vars are always applied on top)")

func main() {
	flag.Parse()

	log.SetOutput(os.Stderr)
	log.Printf("green-api-mcp-gateway starting... version=%s", version)

	// --- Load configuration ---
	// Auto-detect config/config.yaml when --config flag is not provided.
	cfgPath := *configFile
	if cfgPath == "" {
		if _, err := os.Stat("config/config.yaml"); err == nil {
			cfgPath = "config/config.yaml"
			log.Printf("using default config: %s", cfgPath)
		}
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// --- Configure logger ---
	if cfg.Logging.Format == "json" {
		log.SetFlags(0)
	}

	// --- Credential store (application port, infrastructure adapter) ---
	credentialStore := infrastructure.NewCredentialManager()
	for _, inst := range cfg.GreenAPI.Instances {
		credentialStore.AddInstance(&domain.InstanceCredentials{
			InstanceID: inst.ID,
			APIToken:   inst.APIToken,
			APIURL:     inst.APIURL,
		})
		log.Printf("loaded instance %d → %s", inst.ID, inst.APIURL)
	}

	if len(cfg.GreenAPI.Instances) == 0 {
		log.Println("warning: no Green API instances configured — running without instances")
		log.Println("  set GREEN_API_INSTANCE_ID + GREEN_API_TOKEN env vars, or use --config flag")
	}

	// --- Signal context ---
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// --- Metrics (Prometheus) ---
	var metricsProvider application.MetricsProvider
	if cfg.Metrics.Enabled {
		metricsPort := cfg.Metrics.Port
		if metricsPort == 0 {
			metricsPort = 9090
		}
		promMetrics := monitoring.NewMetrics(nil)
		promMetrics.ActiveInstances.Set(float64(len(cfg.GreenAPI.Instances)))
		metricsProvider = promMetrics
		metricsServer := monitoring.NewMetricsServer(metricsPort, nil)
		metricsServer.Start(ctx)
		log.Printf("metrics enabled — Prometheus /metrics on port %d", metricsPort)
	} else {
		metricsProvider = monitoring.NoopMetricsProvider{}
		log.Println("metrics disabled (set metrics.enabled=true in config to enable)")
	}

	// --- WhatsApp client (HTTP-based adapter implementing application.WhatsAppClient) ---
	apiClient := greenapi.NewClient(credentialStore, metricsProvider, cfg)

	// --- MCP server (infrastructure adapter implementing application.MCPTransport) ---
	// --- Rate Limiter ---
	var rateLimiter *ratelimit.Limiter
	if cfg.RateLimit.RequestsPerSecond > 0 {
		rateLimiter = ratelimit.NewLimiter(ratelimit.Config{
			RequestsPerSecond: cfg.RateLimit.RequestsPerSecond,
			Burst:             cfg.RateLimit.Burst,
			Enabled:           true,
		})
	}

	server := mcpserver.NewServer(credentialStore, apiClient, metricsProvider, rateLimiter, version)
	server.SetTLS(cfg.Server.TLS)

	// --- Webhook Bridge (optional, polling mode) ---

	// notificationHandler is shared between polling bridge and HTTP receiver.
	notificationHandler := func(instanceID uint64, n *domain.NotificationBody) {
		// TODO: forward to MCP server notifications when SSE transport is added.
		log.Printf("[webhook] instance %d received notification receiptId=%d",
			instanceID, n.ReceiptID)
	}

	switch cfg.Webhook.Mode {
	case "polling":
		if len(cfg.GreenAPI.Instances) == 0 {
			log.Println("webhook polling mode: no instances configured — bridge idle")
			break
		}
		bridge := webhook.NewBridge(
			credentialStore,
			apiClient,
			notificationHandler,
			metricsProvider,
			0, // use default retry delay (5 s)
		)
		if err := bridge.Start(ctx); err != nil {
			log.Printf("warning: webhook bridge failed to start: %v", err)
		} else {
			defer bridge.Stop()
			log.Println("webhook bridge started (polling mode)")
		}
	case "receiver":
		receiverPort := cfg.Webhook.ReceiverPort
		if receiverPort == 0 {
			receiverPort = 8091
		}
		receiver := webhook.NewReceiver(receiverPort, notificationHandler)
		if err := receiver.Start(ctx); err != nil {
			log.Printf("warning: webhook receiver failed to start: %v", err)
		} else {
			defer receiver.Stop()
			log.Printf("webhook receiver started on port %d (receiver mode)", receiverPort)
		}
	default:
		log.Printf("webhook bridge disabled (mode=%q, instances=%d)",
			cfg.Webhook.Mode, len(cfg.GreenAPI.Instances))
	}

	// --- Serve ---
	log.Printf("transport: %s", cfg.Server.Transport)

	// Resolve public base URL for OAuth metadata and redirect URIs.
	// Respects GREEN_API_BASE_URL env, otherwise derives from port + TLS config.
	issuerURL := os.Getenv("GREEN_API_BASE_URL")
	if issuerURL == "" {
		scheme := "http"
		if cfg.Server.TLS.Enabled {
			scheme = "https"
		}
		issuerURL = fmt.Sprintf("%s://localhost:%d", scheme, cfg.Server.Port)
	}

	switch cfg.Server.Transport {
	case "stdio", "":
		if err := server.ServeStdio(ctx); err != nil {
			log.Fatalf("server error: %v", err)
		}
	case "sse":
		log.Printf("starting SSE server on port %d (auth mode: %s)", cfg.Server.Port, cfg.Auth.Mode)

		if cfg.Auth.Mode == "proxy" {
			proxyAuth := infrastructure.NewProxyAuthManager(apiClient, credentialStore, cfg.Auth.CacheTTL)
			oauthSrv := infrastructure.NewOAuthServer(issuerURL, apiClient, credentialStore)
			proxyAuth.SetOAuthServer(oauthSrv)
			log.Printf("OAuth 2.0 enabled — issuer: %s", issuerURL)

			if err := server.ServeSSEWithAuth(ctx, cfg.Server.Port, proxyAuth); err != nil {
				log.Fatalf("SSE server with proxy auth error: %v", err)
			}
		} else {
			// Traditional config-based auth
			if err := server.ServeSSE(ctx, cfg.Server.Port); err != nil {
				log.Fatalf("SSE server error: %v", err)
			}
		}
	case "http":
		log.Printf("starting Streamable HTTP server on port %d (auth mode: %s)", cfg.Server.Port, cfg.Auth.Mode)

		if cfg.Auth.Mode == "proxy" {
			proxyAuth := infrastructure.NewProxyAuthManager(apiClient, credentialStore, cfg.Auth.CacheTTL)
			oauthSrv := infrastructure.NewOAuthServer(issuerURL, apiClient, credentialStore)
			proxyAuth.SetOAuthServer(oauthSrv)
			log.Printf("OAuth 2.0 enabled — issuer: %s", issuerURL)

			if err := server.ServeStreamableHTTPWithAuth(ctx, cfg.Server.Port, proxyAuth); err != nil {
				log.Fatalf("Streamable HTTP server with proxy auth error: %v", err)
			}
		} else {
			if err := server.ServeStreamableHTTP(ctx, cfg.Server.Port); err != nil {
				log.Fatalf("Streamable HTTP server error: %v", err)
			}
		}
	case "hybrid":
		log.Printf("starting hybrid SSE+HTTP server on port %d (auth mode: %s)", cfg.Server.Port, cfg.Auth.Mode)

		if cfg.Auth.Mode == "proxy" {
			proxyAuth := infrastructure.NewProxyAuthManager(apiClient, credentialStore, cfg.Auth.CacheTTL)
			oauthSrv := infrastructure.NewOAuthServer(issuerURL, apiClient, credentialStore)
			proxyAuth.SetOAuthServer(oauthSrv)
			log.Printf("OAuth 2.0 enabled — issuer: %s", issuerURL)

			if err := server.ServeHybridWithAuth(ctx, cfg.Server.Port, proxyAuth); err != nil {
				log.Fatalf("hybrid server with proxy auth error: %v", err)
			}
		} else {
			// Hybrid without auth: same port, both transports, no credential injection.
			log.Println("warning: hybrid mode without proxy auth — using config-based instances only")
			if err := server.ServeSSE(ctx, cfg.Server.Port); err != nil {
				log.Fatalf("hybrid server error: %v", err)
			}
		}
	default:
		log.Printf("transport %q is not implemented; falling back to stdio", cfg.Server.Transport)
		if err := server.ServeStdio(ctx); err != nil {
			log.Fatalf("server error: %v", err)
		}
	}
}
