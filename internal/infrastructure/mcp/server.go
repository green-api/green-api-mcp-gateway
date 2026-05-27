// SPDX-License-Identifier: MIT
// Copyright (c) 2026 GREEN-API

// Package mcp implements the MCPTransport port using the mcp-go SDK.
package mcp

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/green-api/green-api-mcp-gateway/internal/application"
	"github.com/green-api/green-api-mcp-gateway/internal/infrastructure"
	"github.com/green-api/green-api-mcp-gateway/internal/infrastructure/config"
	"github.com/green-api/green-api-mcp-gateway/internal/infrastructure/ratelimit"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"
)

// Server wraps the mcp-go MCP server with Green API tooling.
type Server struct {
	mcp         *mcpgo.MCPServer
	credentials application.CredentialStore
	client      application.WhatsAppClient
	metrics     application.MetricsProvider
	rateLimiter *ratelimit.Limiter
	logger      *slog.Logger
	tlsCfg      config.TLSConfig
	toolIcon    *mcp.Icon
}

// NewServer creates a new MCP server with all Green API tools registered.
func NewServer(credentials application.CredentialStore, client application.WhatsAppClient, metrics application.MetricsProvider, rateLimiter *ratelimit.Limiter, version string) *Server {
	jsonHandler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(jsonHandler)

	s := &Server{
		mcp: mcpgo.NewMCPServer("green-api-mcp-gateway", version,
			mcpgo.WithHooks(&mcpgo.Hooks{
				OnAfterInitialize: []mcpgo.OnAfterInitializeFunc{
					func(_ context.Context, _ any, _ *mcp.InitializeRequest, result *mcp.InitializeResult) {
						// Advertise MCP Apps (SEP-1865) extension so hosts like Claude.ai
						// know they can load ui:// resources linked from tool _meta.
						if result.Capabilities.Extensions == nil {
							result.Capabilities.Extensions = make(map[string]any)
						}
						result.Capabilities.Extensions["io.modelcontextprotocol/ui"] = map[string]any{}
					},
				},
			}),
		),
		credentials: credentials,
		client:      client,
		metrics:     metrics,
		rateLimiter: rateLimiter,
		logger:      logger,
		toolIcon:    resolveToolIcon(),
	}
	registerTools(s)
	registerResources(s)
	registerPrompts(s)
	return s
}

// resolveToolIcon returns an Icon pointing at the /favicon.png endpoint when a public
// base URL is known. Returning nil (no GREEN_API_BASE_URL set, e.g. stdio dev runs)
// causes addTool to skip the icon entirely, keeping the inline data URL out of
// tools/list responses — embedding it on every tool inflates the payload past the
// silent ~1MB limit some clients enforce, which makes them drop the whole tool list.
func resolveToolIcon() *mcp.Icon {
	base := strings.TrimRight(os.Getenv("GREEN_API_BASE_URL"), "/")
	if base == "" {
		return nil
	}
	return &mcp.Icon{
		Src:      base + "/favicon.ico",
		MIMEType: "image/png",
	}
}

// addTool registers a tool and wraps its handler with rate limiting + metrics instrumentation.
// It automatically injects the GREEN-API icon into the tool metadata when configured.
func (s *Server) addTool(tool mcp.Tool, handler mcpgo.ToolHandlerFunc) {
	if s.toolIcon != nil {
		tool.Icons = append(tool.Icons, *s.toolIcon)
	}
	toolName := tool.Name
	s.mcp.AddTool(tool, mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Generate unique request ID
		requestID := generateRequestID()
		args := req.GetArguments()
		instanceID := getInstanceID(args)

		// Log the request
		logToolRequest(s.logger, toolName, instanceID, args, requestID)

		// Rate limit check: extract instance_id from arguments
		if s.rateLimiter != nil && instanceID > 0 {
			if err := s.rateLimiter.Allow(instanceID); err != nil {
				logRateLimit(s.logger, toolName, instanceID)
				s.metrics.RecordRateLimit(instanceID)
				return mcp.NewToolResultError(fmt.Sprintf("Rate limit exceeded for instance %d. %s", instanceID, err.Error())), nil
			}
		}

		start := time.Now()
		result, err := handler(ctx, req)
		duration := time.Since(start)

		// Log the result or error
		if err != nil {
			logToolError(s.logger, toolName, instanceID, err, duration, requestID)
		} else {
			logToolResult(s.logger, toolName, instanceID, result, duration, requestID)
		}

		s.metrics.RecordToolCall(toolName, start, err)
		return result, err
	}))
}

// ServeStdio runs the MCP server over stdio transport (blocking).
func (s *Server) ServeStdio(ctx context.Context) error {
	stdioSrv := mcpgo.NewStdioServer(s.mcp)
	return stdioSrv.Listen(ctx, os.Stdin, os.Stdout)
}

// healthHandler returns an http.HandlerFunc for health checks.
func healthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}
}

// faviconHandler serves the GREEN-API logo as a favicon so MCP clients
// (e.g. Claude Desktop) can display the connector icon.
func faviconHandler() http.HandlerFunc {
	png := infrastructure.GreenapiLogoPNG
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(png)
	}
}

// mcpContextFunc returns a context function that propagates the authenticated
// instance ID from the HTTP request context into the MCP tool handler context.
func mcpContextFunc(ctx context.Context, r *http.Request) context.Context {
	if id, ok := infrastructure.GetInstanceIDFromContext(r.Context()); ok {
		return context.WithValue(ctx, infrastructure.AuthInstanceIDKey, id)
	}
	return ctx
}

// corsMiddleware adds CORS headers so browser-based MCP clients (Inspector, web apps)
// can reach the server from a different origin.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Mcp-Session-Id, Mcp-Protocol-Version, X-Instance-Id, X-Api-Token, X-Api-Url")
		w.Header().Set("Access-Control-Expose-Headers", "Mcp-Session-Id")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// rootRedirectHandler redirects bare "/" requests to /mcp.
func rootRedirectHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/mcp", http.StatusMovedPermanently)
			return
		}
		http.NotFound(w, r)
	}
}

// ServeSSE runs the MCP server over SSE (HTTP) transport (blocking).
// It listens on the given port and shuts down gracefully when ctx is cancelled.
func (s *Server) ServeSSE(ctx context.Context, port int) error {
	addr := fmt.Sprintf(":%d", port)
	baseURL := s.baseURL(addr)

	sseSrv := mcpgo.NewSSEServer(s.mcp, mcpgo.WithBaseURL(baseURL))

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler())
	mux.HandleFunc("/healthz", healthHandler())
	mux.HandleFunc("/favicon.ico", faviconHandler())
	mux.HandleFunc("/favicon.png", faviconHandler())
	mux.Handle("/", sseSrv)

	httpSrv := &http.Server{
		Addr:    addr,
		Handler: corsMiddleware(mux),
	}

	go func() {
		<-ctx.Done()
		_ = sseSrv.Shutdown(context.Background())
		_ = httpSrv.Shutdown(context.Background())
	}()

	return s.listenAndServe(httpSrv)
}

// registerOAuthEndpoints adds the OAuth 2.0 discovery and flow endpoints to mux (no auth).
func registerOAuthEndpoints(mux *http.ServeMux, authManager *infrastructure.ProxyAuthManager) {
	if h := authManager.OAuthProtectedResourceHandler(); h != nil {
		mux.HandleFunc("/.well-known/oauth-protected-resource", h)
	}
	if h := authManager.OAuthMetadataHandler(); h != nil {
		mux.HandleFunc("/.well-known/oauth-authorization-server", h)
	}
	if h := authManager.OAuthAuthorizeHandler(); h != nil {
		mux.HandleFunc("/authorize", h)
	}
	if h := authManager.OAuthTokenHandler(); h != nil {
		mux.HandleFunc("/token", h)
	}
}

// startCacheCleanup runs periodic OAuth/credential cache cleanup until ctx is cancelled.
func startCacheCleanup(ctx context.Context, authManager *infrastructure.ProxyAuthManager) {
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				authManager.CleanExpiredCache()
			case <-ctx.Done():
				return
			}
		}
	}()
}

// ServeSSEWithAuth runs the MCP server over SSE transport with proxy authentication middleware.
func (s *Server) ServeSSEWithAuth(ctx context.Context, port int, authManager *infrastructure.ProxyAuthManager) error {
	addr := fmt.Sprintf(":%d", port)
	baseURL := os.Getenv("GREEN_API_BASE_URL")
	if baseURL == "" {
		baseURL = s.baseURL(addr)
	}

	sseSrv := mcpgo.NewSSEServer(s.mcp,
		mcpgo.WithBaseURL(baseURL),
		mcpgo.WithSSEContextFunc(mcpContextFunc),
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler())
	mux.HandleFunc("/healthz", healthHandler())
	mux.HandleFunc("/favicon.ico", faviconHandler())
	mux.HandleFunc("/favicon.png", faviconHandler())
	registerOAuthEndpoints(mux, authManager)

	auth := authManager.ProxyAuthMiddleware()
	mux.Handle("/", auth(s.injectCredentials(sseSrv)))

	httpSrv := &http.Server{Addr: addr, Handler: corsMiddleware(mux)}

	go func() {
		<-ctx.Done()
		_ = httpSrv.Shutdown(context.Background())
	}()
	startCacheCleanup(ctx, authManager)

	return s.listenAndServe(httpSrv)
}

// injectCredentials wraps a handler with credential injection from the request context.
// Credentials are removed from the store when a GET connection closes (SSE/stream),
// but kept alive during short-lived POST tool calls.
func (s *Server) injectCredentials(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		creds, ok := infrastructure.GetCredentialsFromContext(r.Context())
		if !ok {
			http.Error(w, "No credentials in context", http.StatusInternalServerError)
			return
		}
		s.credentials.AddInstance(creds)
		if r.Method == http.MethodGet {
			defer s.credentials.RemoveInstance(creds.InstanceID)
		}
		next.ServeHTTP(w, r)
	})
}

// ServeStreamableHTTP runs the MCP server over Streamable HTTP transport (new MCP spec).
// Endpoint: POST/GET /mcp — JSON-RPC with optional SSE upgrade for notifications.
// If config/localhost.crt and config/localhost.key exist, the server starts with TLS (HTTPS).
func (s *Server) ServeStreamableHTTP(ctx context.Context, port int) error {
	addr := fmt.Sprintf(":%d", port)

	streamSrv := mcpgo.NewStreamableHTTPServer(s.mcp,
		mcpgo.WithStateLess(false),
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler())
	mux.HandleFunc("/healthz", healthHandler())
	mux.Handle("/mcp", streamSrv)

	httpSrv := &http.Server{
		Addr:    addr,
		Handler: corsMiddleware(mux),
	}

	go func() {
		<-ctx.Done()
		_ = streamSrv.Shutdown(context.Background())
		_ = httpSrv.Shutdown(context.Background())
	}()

	return s.listenAndServe(httpSrv)
}

// SetTLS configures TLS settings for all HTTP transports.
func (s *Server) SetTLS(cfg config.TLSConfig) {
	s.tlsCfg = cfg
}

// baseURL returns the local base URL respecting the TLS setting.
func (s *Server) baseURL(addr string) string {
	scheme := "http"
	if s.tlsCfg.Enabled {
		scheme = "https"
	}
	return fmt.Sprintf("%s://localhost%s", scheme, addr)
}

// listenAndServe starts the HTTP server, using TLS when configured.
func (s *Server) listenAndServe(srv *http.Server) error {
	if s.tlsCfg.Enabled {
		s.logger.Info("starting HTTPS server", "addr", srv.Addr, "cert", s.tlsCfg.CertFile)
		srv.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		if err := srv.ListenAndServeTLS(s.tlsCfg.CertFile, s.tlsCfg.KeyFile); err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	}
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// ServeStreamableHTTPWithAuth runs the MCP server over Streamable HTTP transport with proxy
// authentication middleware. Endpoint: POST/GET /mcp.
// Claude.ai connectors and other MCP 2025-03-26 clients use this transport.
func (s *Server) ServeStreamableHTTPWithAuth(ctx context.Context, port int, authManager *infrastructure.ProxyAuthManager) error {
	addr := fmt.Sprintf(":%d", port)

	streamSrv := mcpgo.NewStreamableHTTPServer(s.mcp,
		mcpgo.WithStateLess(false),
		mcpgo.WithHTTPContextFunc(mcpContextFunc),
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler())
	mux.HandleFunc("/healthz", healthHandler())
	mux.HandleFunc("/favicon.ico", faviconHandler())
	mux.HandleFunc("/favicon.png", faviconHandler())
	mux.HandleFunc("/", rootRedirectHandler())
	registerOAuthEndpoints(mux, authManager)

	auth := authManager.ProxyAuthMiddleware()
	mux.Handle("/mcp", auth(s.injectCredentials(streamSrv)))

	httpSrv := &http.Server{Addr: addr, Handler: corsMiddleware(mux)}

	go func() {
		<-ctx.Done()
		_ = streamSrv.Shutdown(context.Background())
		_ = httpSrv.Shutdown(context.Background())
	}()
	startCacheCleanup(ctx, authManager)

	return s.listenAndServe(httpSrv)
}

// ServeHybridWithAuth runs both SSE (/sse, /message) and Streamable HTTP (/mcp) transports
// on the same port with proxy authentication middleware.
// Use this when you need to support both legacy SSE clients and new MCP 2025-03-26 clients.
func (s *Server) ServeHybridWithAuth(ctx context.Context, port int, authManager *infrastructure.ProxyAuthManager) error {
	addr := fmt.Sprintf(":%d", port)
	baseURL := os.Getenv("GREEN_API_BASE_URL")
	if baseURL == "" {
		baseURL = s.baseURL(addr)
	}

	sseSrv := mcpgo.NewSSEServer(s.mcp,
		mcpgo.WithBaseURL(baseURL),
		mcpgo.WithSSEContextFunc(mcpContextFunc),
	)
	streamSrv := mcpgo.NewStreamableHTTPServer(s.mcp,
		mcpgo.WithStateLess(false),
		mcpgo.WithHTTPContextFunc(mcpContextFunc),
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler())
	mux.HandleFunc("/healthz", healthHandler())
	mux.HandleFunc("/favicon.ico", faviconHandler())
	mux.HandleFunc("/favicon.png", faviconHandler())
	mux.HandleFunc("/", rootRedirectHandler())
	registerOAuthEndpoints(mux, authManager)

	auth := authManager.ProxyAuthMiddleware()

	// Legacy SSE transport: /sse (stream) + /message (tool calls)
	mux.Handle("/sse", auth(s.injectCredentials(sseSrv)))
	mux.Handle("/message", auth(s.injectCredentials(sseSrv)))

	// Streamable HTTP transport (MCP 2025-03-26): /mcp
	mux.Handle("/mcp", auth(s.injectCredentials(streamSrv)))

	httpSrv := &http.Server{Addr: addr, Handler: corsMiddleware(mux)}

	go func() {
		<-ctx.Done()
		_ = sseSrv.Shutdown(context.Background())
		_ = streamSrv.Shutdown(context.Background())
		_ = httpSrv.Shutdown(context.Background())
	}()
	startCacheCleanup(ctx, authManager)

	return s.listenAndServe(httpSrv)
}
