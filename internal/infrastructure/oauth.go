// SPDX-License-Identifier: MIT
// Copyright (c) 2026 GREEN-API

package infrastructure

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/green-api/green-api-mcp-gateway/internal/application"
	"github.com/green-api/green-api-mcp-gateway/internal/domain"
)

//go:embed templates/authorize.html
var authorizeFormHTML string

//go:embed templates/greenapi.png
var greenapiLogoPNG []byte

//go:embed templates/qr_app.html
var qrAppHTML string

// QRAppHTML is the self-contained MCP App HTML for the QR code widget (ui://qr resource).
var QRAppHTML = qrAppHTML

//go:embed templates/contacts_app.html
var contactsAppHTML string

// ContactsAppHTML is the self-contained MCP App HTML for the contacts list widget (ui://contacts resource).
var ContactsAppHTML = contactsAppHTML

var greenapiLogoDataURL = "data:image/png;base64," + base64.StdEncoding.EncodeToString(greenapiLogoPNG)

// GreenapiLogoPNG is the raw PNG bytes of the GREEN-API logo (exported for HTTP favicon handlers).
var GreenapiLogoPNG = greenapiLogoPNG

// GreenapiLogoDataURL is the data: URI of the GREEN-API logo (exported for MCP icon fields).
var GreenapiLogoDataURL = greenapiLogoDataURL

const (
	authCodeTTL    = 10 * time.Minute
	accessTokenTTL = 24 * time.Hour
)

// OAuthServer implements the OAuth 2.0 Authorization Code flow with PKCE (RFC 7636).
// It bridges the MCP OAuth protocol with Green API credentials (instanceId + apiToken),
// presenting an HTML form where the user enters their Green API credentials to obtain
// a bearer token that Claude Desktop can use for subsequent MCP requests.
type OAuthServer struct {
	issuerURL string
	client    application.WhatsAppClient
	store     application.CredentialStore

	codeMu sync.Mutex
	codes  map[string]*pendingCode

	tokenMu sync.RWMutex
	tokens  map[string]*issuedToken
}

// pendingCode holds an issued authorization code awaiting exchange.
type pendingCode struct {
	creds         *domain.InstanceCredentials
	codeChallenge string
	redirectURI   string
	clientID      string
	state         string
	expiresAt     time.Time
}

// issuedToken holds credentials associated with an active bearer token.
type issuedToken struct {
	creds     *domain.InstanceCredentials
	expiresAt time.Time
}

// NewOAuthServer creates a new OAuth 2.0 server.
// issuerURL is the public base URL of this server (e.g. https://example.com).
func NewOAuthServer(issuerURL string, client application.WhatsAppClient, store application.CredentialStore) *OAuthServer {
	return &OAuthServer{
		issuerURL: strings.TrimRight(issuerURL, "/"),
		client:    client,
		store:     store,
		codes:     make(map[string]*pendingCode),
		tokens:    make(map[string]*issuedToken),
	}
}

// LookupToken returns credentials for a valid, non-expired bearer token.
func (o *OAuthServer) LookupToken(token string) (*domain.InstanceCredentials, error) {
	o.tokenMu.RLock()
	entry, ok := o.tokens[token]
	o.tokenMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown token")
	}
	if time.Now().After(entry.expiresAt) {
		o.tokenMu.Lock()
		delete(o.tokens, token)
		o.tokenMu.Unlock()
		return nil, fmt.Errorf("token expired")
	}
	return entry.creds, nil
}

// WWWAuthenticateHeader returns the value for the WWW-Authenticate header on 401 responses.
// This tells OAuth clients where to discover the authorization server (RFC 9728).
func (o *OAuthServer) WWWAuthenticateHeader() string {
	return fmt.Sprintf(`Bearer realm=%q, resource_metadata=%q`,
		o.issuerURL,
		o.issuerURL+"/.well-known/oauth-protected-resource",
	)
}

// CleanExpired removes expired codes and tokens from memory.
func (o *OAuthServer) CleanExpired() {
	now := time.Now()

	o.codeMu.Lock()
	for k, v := range o.codes {
		if now.After(v.expiresAt) {
			delete(o.codes, k)
		}
	}
	o.codeMu.Unlock()

	o.tokenMu.Lock()
	for k, v := range o.tokens {
		if now.After(v.expiresAt) {
			delete(o.tokens, k)
		}
	}
	o.tokenMu.Unlock()
}

// ── RFC 9728: Protected Resource Metadata ────────────────────────────────────

// ProtectedResourceHandler serves GET /.well-known/oauth-protected-resource (RFC 9728).
// Claude Desktop uses this to discover which authorization server protects this resource.
func (o *OAuthServer) ProtectedResourceHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		doc := map[string]interface{}{
			"resource":                 o.issuerURL,
			"authorization_servers":    []string{o.issuerURL},
			"bearer_methods_supported": []string{"header"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(doc)
	}
}

// ── RFC 8414: Authorization Server Metadata ──────────────────────────────────

// MetadataHandler serves GET /.well-known/oauth-authorization-server (RFC 8414).
func (o *OAuthServer) MetadataHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		meta := map[string]interface{}{
			"issuer":                                o.issuerURL,
			"authorization_endpoint":                o.issuerURL + "/authorize",
			"token_endpoint":                        o.issuerURL + "/token",
			"response_types_supported":              []string{"code"},
			"grant_types_supported":                 []string{"authorization_code"},
			"code_challenge_methods_supported":      []string{"S256"},
			"token_endpoint_auth_methods_supported": []string{"none"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(meta)
	}
}

// ── Authorization endpoint ────────────────────────────────────────────────────

// AuthorizeHandler handles GET /authorize (show form) and POST /authorize (process credentials).
func (o *OAuthServer) AuthorizeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			o.showForm(w, r, "", "")
		case http.MethodPost:
			o.processForm(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// ── Token endpoint ────────────────────────────────────────────────────────────

// TokenHandler handles POST /token — exchanges an authorization code for a bearer token.
func (o *OAuthServer) TokenHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseForm(); err != nil {
			writeTokenError(w, "invalid_request", "could not parse request body")
			return
		}

		grantType := r.FormValue("grant_type")
		code := r.FormValue("code")
		redirectURI := r.FormValue("redirect_uri")
		codeVerifier := r.FormValue("code_verifier")

		if grantType != "authorization_code" {
			writeTokenError(w, "unsupported_grant_type", "only authorization_code is supported")
			return
		}
		if code == "" || codeVerifier == "" {
			writeTokenError(w, "invalid_request", "code and code_verifier are required")
			return
		}

		// Consume the authorization code (one-time use).
		o.codeMu.Lock()
		pending, ok := o.codes[code]
		if ok {
			delete(o.codes, code)
		}
		o.codeMu.Unlock()

		if !ok {
			writeTokenError(w, "invalid_grant", "unknown or already-used authorization code")
			return
		}
		if time.Now().After(pending.expiresAt) {
			writeTokenError(w, "invalid_grant", "authorization code expired")
			return
		}
		if redirectURI != "" && redirectURI != pending.redirectURI {
			writeTokenError(w, "invalid_grant", "redirect_uri mismatch")
			return
		}

		// Verify PKCE S256: BASE64URL(SHA256(code_verifier)) must equal stored code_challenge.
		if !verifyPKCE(codeVerifier, pending.codeChallenge) {
			writeTokenError(w, "invalid_grant", "code_verifier does not match code_challenge")
			return
		}

		// Issue bearer token.
		token, err := randomToken(32)
		if err != nil {
			writeTokenError(w, "server_error", "could not generate access token")
			return
		}

		o.tokenMu.Lock()
		o.tokens[token] = &issuedToken{
			creds:     pending.creds,
			expiresAt: time.Now().Add(accessTokenTTL),
		}
		o.tokenMu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": token,
			"token_type":   "Bearer", // RFC 6749 §7.1 — capital B
			"expires_in":   int(accessTokenTTL.Seconds()),
		})
	}
}

// ── Internal helpers ──────────────────────────────────────────────────────────

func (o *OAuthServer) showForm(w http.ResponseWriter, r *http.Request, instanceID, errMsg string) {
	q := r.URL.Query()
	clientID := q.Get("client_id")

	// If instance ID is not yet known (first load) and client_id looks like a numeric
	// instance ID, pre-fill the field so the user only needs to enter the API token.
	if instanceID == "" {
		if _, err := strconv.ParseUint(clientID, 10, 64); err == nil {
			instanceID = clientID
		}
	}

	data := authorizeFormData{
		ClientID:            clientID,
		RedirectURI:         q.Get("redirect_uri"),
		CodeChallenge:       q.Get("code_challenge"),
		CodeChallengeMethod: q.Get("code_challenge_method"),
		State:               q.Get("state"),
		InstanceID:          instanceID,
		Error:               errMsg,
		LogoDataURL:         template.URL(greenapiLogoDataURL),
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = authorizeFormTmpl.Execute(w, data)
}

func (o *OAuthServer) processForm(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	redirectURI := r.FormValue("redirect_uri")
	clientID := r.FormValue("client_id")
	codeChallenge := r.FormValue("code_challenge")
	codeChallengeMethod := r.FormValue("code_challenge_method")
	state := r.FormValue("state")
	instanceIDStr := r.FormValue("instance_id")
	apiToken := r.FormValue("api_token")

	// Validate required OAuth parameters.
	if redirectURI == "" || codeChallenge == "" {
		http.Error(w, "invalid_request: missing OAuth parameters", http.StatusBadRequest)
		return
	}
	if codeChallengeMethod != "S256" {
		http.Error(w, "invalid_request: only S256 code_challenge_method is supported", http.StatusBadRequest)
		return
	}
	if _, err := url.ParseRequestURI(redirectURI); err != nil {
		http.Error(w, "invalid_request: malformed redirect_uri", http.StatusBadRequest)
		return
	}

	isAJAX := r.Header.Get("X-Requested-With") == "fetch"

	// Helper to return an error — JSON for AJAX, HTML form re-render otherwise.
	showError := func(msg string) {
		if isAJAX {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnprocessableEntity)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
			return
		}
		data := authorizeFormData{
			ClientID:            clientID,
			RedirectURI:         redirectURI,
			CodeChallenge:       codeChallenge,
			CodeChallengeMethod: codeChallengeMethod,
			State:               state,
			InstanceID:          instanceIDStr,
			LogoDataURL:         template.URL(greenapiLogoDataURL),
			Error:               msg,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = authorizeFormTmpl.Execute(w, data)
	}

	instanceID, err := strconv.ParseUint(instanceIDStr, 10, 64)
	if err != nil || instanceID == 0 {
		showError("Instance ID must be a positive number")
		return
	}
	if apiToken == "" {
		showError("API Token is required")
		return
	}

	creds := &domain.InstanceCredentials{
		InstanceID: instanceID,
		APIToken:   apiToken,
		APIURL:     "https://api.green-api.com",
	}

	if err := o.validateCredentials(r.Context(), creds); err != nil {
		showError("Invalid credentials — please check your Instance ID and API Token")
		return
	}

	// Generate and store an authorization code.
	code, err := randomToken(16)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	o.codeMu.Lock()
	o.codes[code] = &pendingCode{
		creds:         creds,
		codeChallenge: codeChallenge,
		redirectURI:   redirectURI,
		clientID:      clientID,
		state:         state,
		expiresAt:     time.Now().Add(authCodeTTL),
	}
	o.codeMu.Unlock()

	// Build callback URL with code and state.
	// Cache-Control: no-store prevents the authorization code from being cached (RFC 9700).
	cbURL, _ := url.ParseRequestURI(redirectURI)
	cbQ := cbURL.Query()
	cbQ.Set("code", code)
	if state != "" {
		cbQ.Set("state", state)
	}
	cbURL.RawQuery = cbQ.Encode()
	w.Header().Set("Cache-Control", "no-store")

	if isAJAX {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"redirect": cbURL.String()})
		return
	}
	http.Redirect(w, r, cbURL.String(), http.StatusFound)
}

// validateCredentials temporarily registers credentials, calls GetStateInstance,
// then restores the previous state of the store entry.
func (o *OAuthServer) validateCredentials(ctx context.Context, creds *domain.InstanceCredentials) error {
	orig, hadOriginal := func() (*domain.InstanceCredentials, bool) {
		if c, err := o.store.GetCredentials(creds.InstanceID); err == nil {
			return c, true
		}
		return nil, false
	}()

	o.store.AddInstance(creds)
	defer func() {
		if hadOriginal {
			o.store.AddInstance(orig)
		} else {
			o.store.RemoveInstance(creds.InstanceID)
		}
	}()

	_, err := o.client.GetStateInstance(ctx, creds.InstanceID)
	return err
}

// verifyPKCE returns true if BASE64URL(SHA256(codeVerifier)) equals codeChallenge.
func verifyPKCE(codeVerifier, codeChallenge string) bool {
	h := sha256.Sum256([]byte(codeVerifier))
	return base64.RawURLEncoding.EncodeToString(h[:]) == codeChallenge
}

// randomToken generates a URL-safe random token of n bytes.
func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// setCORSHeaders adds permissive CORS headers required for browser-based OAuth clients.
func setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Access-Control-Max-Age", "86400")
}

// writeTokenError writes an OAuth token error response (RFC 6749 §5.2).
func writeTokenError(w http.ResponseWriter, code, description string) {
	status := http.StatusBadRequest
	if code == "server_error" {
		status = http.StatusInternalServerError
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":             code,
		"error_description": description,
	})
}

// ── HTML form ─────────────────────────────────────────────────────────────────

type authorizeFormData struct {
	ClientID            string
	RedirectURI         string
	CodeChallenge       string
	CodeChallengeMethod string
	State               string
	InstanceID          string
	Error               string
	LogoDataURL         template.URL
}

var authorizeFormTmpl = template.Must(template.New("authorize").Parse(authorizeFormHTML))
