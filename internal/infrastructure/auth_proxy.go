// SPDX-License-Identifier: MIT
// Copyright (c) 2026 GREEN-API

package infrastructure

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/green-api/green-api-mcp-gateway/internal/application"
	"github.com/green-api/green-api-mcp-gateway/internal/domain"
)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

const credentialsContextKey contextKey = "credentials"
const instanceIDContextKey contextKey = "instanceID"

// AuthInstanceIDKey is the exported context key used to propagate the authenticated
// instance ID from HTTP middleware into MCP tool handler contexts.
var AuthInstanceIDKey = instanceIDContextKey

// GetInstanceIDFromContext returns the authenticated instance ID stored in the context.
// Returns 0 and false if not set (e.g. non-proxy auth mode).
func GetInstanceIDFromContext(ctx context.Context) (uint64, bool) {
	v, ok := ctx.Value(instanceIDContextKey).(uint64)
	return v, ok
}

// ProxyAuthManager implements credential extraction and validation for public MCP server mode.
// It supports OAuth 2.0 Bearer tokens, Authorization header (Basic auth), and custom headers
// (X-Instance-Id + X-Api-Token).
type ProxyAuthManager struct {
	client        application.WhatsAppClient
	fallbackStore application.CredentialStore
	oauth         *OAuthServer
	cacheTTL      time.Duration
	cache         map[string]*cachedCredential
	cacheMu       sync.RWMutex
}

type cachedCredential struct {
	creds      *domain.InstanceCredentials
	validUntil time.Time
}

// NewProxyAuthManager creates a new proxy auth manager.
func NewProxyAuthManager(client application.WhatsAppClient, fallbackStore application.CredentialStore, cacheTTLSeconds int) *ProxyAuthManager {
	return &ProxyAuthManager{
		client:        client,
		fallbackStore: fallbackStore,
		cacheTTL:      time.Duration(cacheTTLSeconds) * time.Second,
		cache:         make(map[string]*cachedCredential),
	}
}

// SetOAuthServer attaches an OAuth server to the auth manager.
// When set, Authorization: Bearer tokens are resolved via the OAuth server.
func (m *ProxyAuthManager) SetOAuthServer(o *OAuthServer) {
	m.oauth = o
}

// ExtractCredentialsFromRequest extracts credentials from HTTP request headers.
// Returns credentials or an error if extraction fails.
func (m *ProxyAuthManager) ExtractCredentialsFromRequest(r *http.Request) (*domain.InstanceCredentials, error) {
	// Try Authorization header first (Basic auth: instanceId:apiToken)
	if auth := r.Header.Get("Authorization"); auth != "" {
		if strings.HasPrefix(auth, "Basic ") {
			creds, err := m.parseBasicAuth(auth)
			if err != nil {
				return nil, fmt.Errorf("invalid basic auth: %w", err)
			}
			return creds, nil
		}
	}

	// Try custom headers (X-Instance-Id + X-Api-Token)
	instanceIDHeader := r.Header.Get("X-Instance-Id")
	apiTokenHeader := r.Header.Get("X-Api-Token")
	if instanceIDHeader != "" && apiTokenHeader != "" {
		instanceID, err := strconv.ParseUint(instanceIDHeader, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid X-Instance-Id header: %w", err)
		}

		// Use default API URL or try to extract from X-Api-Url header
		apiURL := "https://api.green-api.com"
		if customURL := r.Header.Get("X-Api-Url"); customURL != "" {
			apiURL = customURL
		}

		return &domain.InstanceCredentials{
			InstanceID: instanceID,
			APIToken:   apiTokenHeader,
			APIURL:     apiURL,
		}, nil
	}

	// No proxy credentials found
	return nil, errors.New("no proxy credentials found in request headers")
}

// parseBasicAuth parses Basic authentication header and returns credentials.
func (m *ProxyAuthManager) parseBasicAuth(authHeader string) (*domain.InstanceCredentials, error) {
	// Remove "Basic " prefix
	encoded := strings.TrimPrefix(authHeader, "Basic ")

	// Decode base64
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	// Split on first colon: instanceId:apiToken
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return nil, errors.New("basic auth must be in format instanceId:apiToken")
	}

	// Parse instance ID
	instanceID, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid instance ID: %w", err)
	}

	return &domain.InstanceCredentials{
		InstanceID: instanceID,
		APIToken:   parts[1],
		APIURL:     "https://api.green-api.com", // default, can be overridden by X-Api-Url
	}, nil
}

// ValidateCredentials validates credentials by calling the Green API getStateInstance endpoint.
// Results are cached for the configured TTL.
func (m *ProxyAuthManager) ValidateCredentials(ctx context.Context, creds *domain.InstanceCredentials) error {
	cacheKey := m.getCacheKey(creds)

	// Check cache first
	m.cacheMu.RLock()
	if cached, ok := m.cache[cacheKey]; ok && time.Now().Before(cached.validUntil) {
		m.cacheMu.RUnlock()
		return nil // Valid cached result
	}
	m.cacheMu.RUnlock()

	// Add the credentials temporarily to the fallback store for validation
	originalCreds, hadOriginal := func() (*domain.InstanceCredentials, bool) {
		if orig, err := m.fallbackStore.GetCredentials(creds.InstanceID); err == nil {
			return orig, true
		}
		return nil, false
	}()

	// Temporarily add the credentials for validation
	m.fallbackStore.AddInstance(creds)
	defer func() {
		if hadOriginal {
			// Restore original credentials
			m.fallbackStore.AddInstance(originalCreds)
		} else {
			// Remove the temporary credentials
			m.fallbackStore.RemoveInstance(creds.InstanceID)
		}
	}()

	// Try to call getStateInstance to validate credentials
	_, err := m.client.GetStateInstance(ctx, creds.InstanceID)
	if err != nil {
		return fmt.Errorf("credential validation failed: %w", err)
	}

	// Cache the successful validation
	m.cacheMu.Lock()
	m.cache[cacheKey] = &cachedCredential{
		creds:      creds,
		validUntil: time.Now().Add(m.cacheTTL),
	}
	m.cacheMu.Unlock()

	return nil
}

// GetCredentialsForRequest extracts and validates credentials from a request.
// Falls back to config-based credentials if no proxy credentials are found.
func (m *ProxyAuthManager) GetCredentialsForRequest(ctx context.Context, r *http.Request) (*domain.InstanceCredentials, error) {
	// Fast path: OAuth 2.0 Bearer token — already validated during the OAuth flow.
	// The Bearer scheme is case-insensitive per RFC 7235.
	if m.oauth != nil {
		if auth := r.Header.Get("Authorization"); strings.HasPrefix(strings.ToLower(auth), "bearer ") {
			token := auth[7:] // skip "bearer " (7 bytes), preserve token's original case
			creds, err := m.oauth.LookupToken(token)
			if err != nil {
				return nil, fmt.Errorf("invalid bearer token: %w", err)
			}
			return creds, nil
		}
	}

	// Try to extract proxy credentials (Basic auth or custom headers).
	creds, err := m.ExtractCredentialsFromRequest(r)
	if err == nil {
		// Validate the proxy credentials
		if validateErr := m.ValidateCredentials(ctx, creds); validateErr != nil {
			return nil, fmt.Errorf("proxy credential validation failed: %w", validateErr)
		}
		return creds, nil
	}

	// Fall back to config-based credentials (backward compatibility)
	// For fallback, we need an instance ID - try to get it from query params or use first available
	instanceIDStr := r.URL.Query().Get("instance_id")
	if instanceIDStr == "" {
		// Use the first available instance from config
		instances := m.fallbackStore.ListInstances()
		if len(instances) == 0 {
			return nil, errors.New("no proxy credentials in request and no config instances available")
		}
		instanceIDStr = strconv.FormatUint(instances[0], 10)
	}

	instanceID, parseErr := strconv.ParseUint(instanceIDStr, 10, 64)
	if parseErr != nil {
		return nil, fmt.Errorf("invalid instance_id parameter: %w", parseErr)
	}

	fallbackCreds, fallbackErr := m.fallbackStore.GetCredentials(instanceID)
	if fallbackErr != nil {
		return nil, fmt.Errorf("proxy credentials failed (%v) and fallback failed: %w", err, fallbackErr)
	}

	return fallbackCreds, nil
}

// getCacheKey generates a cache key for credentials.
func (m *ProxyAuthManager) getCacheKey(creds *domain.InstanceCredentials) string {
	return fmt.Sprintf("%d:%s:%s", creds.InstanceID, creds.APIToken, creds.APIURL)
}

// CleanExpiredCache removes expired entries from the credential validation cache.
// Should be called periodically to prevent memory leaks.
func (m *ProxyAuthManager) CleanExpiredCache() {
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()

	now := time.Now()
	for key, cached := range m.cache {
		if now.After(cached.validUntil) {
			delete(m.cache, key)
		}
	}

	if m.oauth != nil {
		m.oauth.CleanExpired()
	}
}

// OAuthProtectedResourceHandler returns the /.well-known/oauth-protected-resource handler (RFC 9728).
// Returns nil if no OAuth server is configured.
func (m *ProxyAuthManager) OAuthProtectedResourceHandler() http.HandlerFunc {
	if m.oauth == nil {
		return nil
	}
	return m.oauth.ProtectedResourceHandler()
}

// OAuthMetadataHandler returns the /.well-known/oauth-authorization-server handler.
// Returns nil if no OAuth server is configured.
func (m *ProxyAuthManager) OAuthMetadataHandler() http.HandlerFunc {
	if m.oauth == nil {
		return nil
	}
	return m.oauth.MetadataHandler()
}

// OAuthAuthorizeHandler returns the /authorize handler.
// Returns nil if no OAuth server is configured.
func (m *ProxyAuthManager) OAuthAuthorizeHandler() http.HandlerFunc {
	if m.oauth == nil {
		return nil
	}
	return m.oauth.AuthorizeHandler()
}

// OAuthTokenHandler returns the /token handler.
// Returns nil if no OAuth server is configured.
func (m *ProxyAuthManager) OAuthTokenHandler() http.HandlerFunc {
	if m.oauth == nil {
		return nil
	}
	return m.oauth.TokenHandler()
}

// ProxyAuthMiddleware returns HTTP middleware that extracts and validates credentials.
func (m *ProxyAuthManager) ProxyAuthMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Extract and validate credentials
			creds, err := m.GetCredentialsForRequest(ctx, r)
			if err != nil {
				// WWW-Authenticate tells OAuth clients where to discover the auth server (RFC 6750 §3).
				if m.oauth != nil {
					w.Header().Set("WWW-Authenticate", m.oauth.WWWAuthenticateHeader())
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = fmt.Fprintf(w, `{"error":"unauthorized","error_description":%q}`, err.Error())
				return
			}

			// Add credentials and instance ID to context for downstream handlers
			ctx = context.WithValue(ctx, credentialsContextKey, creds)
			ctx = context.WithValue(ctx, instanceIDContextKey, creds.InstanceID)
			r = r.WithContext(ctx)

			next.ServeHTTP(w, r)
		})
	}
}

// GetCredentialsFromContext extracts credentials from request context.
func GetCredentialsFromContext(ctx context.Context) (*domain.InstanceCredentials, bool) {
	creds, ok := ctx.Value(credentialsContextKey).(*domain.InstanceCredentials)
	return creds, ok
}
