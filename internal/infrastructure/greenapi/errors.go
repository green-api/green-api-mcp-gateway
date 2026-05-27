// SPDX-License-Identifier: MIT
// Copyright (c) 2026 GREEN-API

package greenapi

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// APIError represents an HTTP error response from the Green API.
// Its Error() method returns a human-readable, actionable description
// suitable for AI agents (Claude, etc.) rather than a raw HTTP dump.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	msg := parseErrorMessage(e.Body)

	switch {
	case e.StatusCode == 401:
		hint := "verify your Instance ID and API Token, or use whatsapp_connect to re-authorize"
		if msg != "" {
			return fmt.Sprintf("authentication error (HTTP 401): %s — %s", msg, hint)
		}
		return fmt.Sprintf("authentication error (HTTP 401): credentials are invalid or expired — %s", hint)

	case e.StatusCode == 403:
		hint := "check authorization status with whatsapp_get_state; if not authorized, scan the QR code with whatsapp_get_qr"
		if msg != "" {
			return fmt.Sprintf("access denied (HTTP 403): %s — %s", msg, hint)
		}
		return fmt.Sprintf("access denied (HTTP 403): instance is not authorized — %s", hint)

	case e.StatusCode == 404:
		if msg != "" {
			return fmt.Sprintf("not found (HTTP 404): %s", msg)
		}
		return "not found (HTTP 404): resource does not exist — check the instance ID"

	case e.StatusCode == 429:
		return "rate limit exceeded (HTTP 429): too many requests — wait a few seconds before retrying"

	case e.StatusCode >= 500:
		if msg != "" {
			return fmt.Sprintf("Green API server error (HTTP %d): %s — the service may be temporarily unavailable, try again later", e.StatusCode, msg)
		}
		return fmt.Sprintf("Green API server error (HTTP %d) — the service may be temporarily unavailable, try again later", e.StatusCode)

	default:
		if msg != "" {
			return fmt.Sprintf("Green API error (HTTP %d): %s", e.StatusCode, msg)
		}
		return fmt.Sprintf("Green API error (HTTP %d): %s", e.StatusCode, e.Body)
	}
}

// parseErrorMessage extracts a human-readable message from a Green API JSON error body.
// Green API typically uses {"error": "..."}, {"message": "..."}, or similar.
func parseErrorMessage(body string) string {
	if body == "" {
		return ""
	}
	var v map[string]interface{}
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		// Not JSON — return trimmed raw body if it's short enough to be useful.
		trimmed := strings.TrimSpace(body)
		if len(trimmed) <= 200 {
			return trimmed
		}
		return ""
	}
	for _, key := range []string{"error", "message", "description", "detail", "msg"} {
		if s, ok := v[key].(string); ok && s != "" {
			return s
		}
	}
	return ""
}

// maskURL removes the API token (last path segment) from a Green API URL so it can
// be safely included in error messages and logs.
// URL format: {base}/waInstance{id}/{method}/{token}
func maskURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "[url]"
	}
	parts := strings.Split(strings.TrimRight(u.Path, "/"), "/")
	if len(parts) >= 2 {
		// Replace last segment (token) with "***"
		parts[len(parts)-1] = "***"
		u.Path = strings.Join(parts, "/")
	}
	return u.String()
}
