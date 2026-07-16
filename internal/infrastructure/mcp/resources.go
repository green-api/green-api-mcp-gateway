// SPDX-License-Identifier: MIT
// Copyright (c) 2026 GREEN-API

package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/green-api/green-api-mcp-gateway/internal/domain"
	"github.com/green-api/green-api-mcp-gateway/internal/infrastructure"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"
)

// registerResources adds MCP Resources for Green API instances.
//
// Registered resources:
//   - ui://qr                           — interactive QR code widget (MCP App)
//   - ui://contacts                     — contacts list widget (MCP App)
//
// Registered resource templates:
//   - whatsapp://instance/{id}/state    — current authorization state of an instance
//   - whatsapp://instance/{id}/settings — settings of an instance
func registerResources(s *Server) {
	// ui://qr — MCP App widget for instance authorization via QR code
	qrResource := mcp.NewResource("ui://qr", "QR Code Widget",
		mcp.WithMIMEType("text/html;profile=mcp-app"),
		mcp.WithResourceDescription("Interactive QR code widget for authorizing a GREEN-API instance"),
	)
	qrResource.Meta = mcp.NewMetaFromMap(widgetResourceMeta("Interactive QR code widget for authorizing a GREEN-API instance", []string{}, ""))
	s.mcp.AddResource(
		qrResource,
		mcpgo.ResourceHandlerFunc(func(ctx context.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			// _meta.ui.domain must be the hash of the endpoint URL the client
			// actually connected to, so build it per request.
			meta := widgetResourceMeta("Interactive QR code widget for authorizing a GREEN-API instance", []string{}, publicBaseURLFromContext(ctx))
			return []mcp.ResourceContents{
				mcp.TextResourceContents{
					Meta:     meta,
					URI:      "ui://qr",
					MIMEType: "text/html;profile=mcp-app",
					Text:     infrastructure.QRAppHTML,
				},
			}, nil
		}),
	)

	// ui://contacts — MCP App widget for browsing the contacts list
	contactsResource := mcp.NewResource("ui://contacts", "Contacts List Widget",
		mcp.WithMIMEType("text/html;profile=mcp-app"),
		mcp.WithResourceDescription("Interactive contacts list with search and contact details"),
	)
	contactsResource.Meta = mcp.NewMetaFromMap(widgetResourceMeta("Interactive contacts list with search and contact details", []string{"https://pps.whatsapp.net"}, ""))
	s.mcp.AddResource(
		contactsResource,
		mcpgo.ResourceHandlerFunc(func(ctx context.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			meta := widgetResourceMeta("Interactive contacts list with search and contact details", []string{"https://pps.whatsapp.net"}, publicBaseURLFromContext(ctx))
			return []mcp.ResourceContents{
				mcp.TextResourceContents{
					Meta:     meta,
					URI:      "ui://contacts",
					MIMEType: "text/html;profile=mcp-app",
					Text:     infrastructure.ContactsAppHTML,
				},
			}, nil
		}),
	)
	// whatsapp://instance/{id}/state
	s.mcp.AddResourceTemplate(
		mcp.NewResourceTemplate(
			"whatsapp://instance/{id}/state",
			"Instance State",
			mcp.WithTemplateDescription("Current authorization state of the WhatsApp instance (authorized, notAuthorized, blocked, sleepMode)"),
			mcp.WithTemplateMIMEType("application/json"),
		),
		mcpgo.ResourceTemplateHandlerFunc(func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			instanceID, err := extractInstanceID(req)
			if err != nil {
				return nil, err
			}

			raw, err := s.client.GetMethod(ctx, instanceID, domain.MethodGetStateInstance)
			if err != nil {
				return nil, fmt.Errorf("whatsapp_get_state: %w", err)
			}

			return []mcp.ResourceContents{
				mcp.TextResourceContents{
					URI:      req.Params.URI,
					MIMEType: "application/json",
					Text:     string(raw),
				},
			}, nil
		}),
	)

	// whatsapp://instance/{id}/settings
	s.mcp.AddResourceTemplate(
		mcp.NewResourceTemplate(
			"whatsapp://instance/{id}/settings",
			"Instance Settings",
			mcp.WithTemplateDescription("WhatsApp instance settings (webhook URL, timeouts, proxy, etc.)"),
			mcp.WithTemplateMIMEType("application/json"),
		),
		mcpgo.ResourceTemplateHandlerFunc(func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			instanceID, err := extractInstanceID(req)
			if err != nil {
				return nil, err
			}

			raw, err := s.client.GetMethod(ctx, instanceID, domain.MethodGetSettings)
			if err != nil {
				return nil, fmt.Errorf("whatsapp_get_settings: %w", err)
			}

			return []mcp.ResourceContents{
				mcp.TextResourceContents{
					URI:      req.Params.URI,
					MIMEType: "application/json",
					Text:     string(raw),
				},
			}, nil
		}),
	)
}

// widgetResourceMeta builds the _meta map for an MCP App widget resource.
// requestBaseURL is the base URL the client connected to for this request
// (may be empty for static registration meta); env overrides take precedence.
func widgetResourceMeta(description string, resourceDomains []string, requestBaseURL string) map[string]any {
	domain := strings.TrimRight(os.Getenv("GREEN_API_WIDGET_DOMAIN"), "/")
	if domain == "" {
		domain = strings.TrimRight(os.Getenv("GREEN_API_BASE_URL"), "/")
	}
	if domain == "" {
		domain = strings.TrimRight(requestBaseURL, "/")
	}
	if domain == "" {
		// Production endpoint (see server.json remotes) — note: greenapi.com
		// with hyphen, matching the MCP Registry namespace.
		domain = "https://mcp.greenapi.com"
	}

	apiURL := strings.TrimRight(os.Getenv("GREEN_API_URL"), "/")
	if apiURL == "" {
		apiURL = "https://api.green-api.com"
	}
	connectDomains := []string{apiURL}
	standardCSP := map[string]any{
		"connectDomains":  connectDomains,
		"resourceDomains": resourceDomains,
	}
	legacyCSP := map[string]any{
		"connect_domains":  connectDomains,
		"resource_domains": resourceDomains,
	}

	ui := map[string]any{
		"prefersBorder": true,
		"csp":           standardCSP,
		"domain":        claudeWidgetDomain(domain),
	}

	return map[string]any{
		"ui":                         ui,
		"openai/widgetDescription":   description,
		"openai/widgetPrefersBorder": true,
		"openai/widgetCSP":           legacyCSP,
		"openai/widgetDomain":        domain,
	}
}

// claudeWidgetDomain returns the _meta.ui.domain value Claude.ai requires to
// place the widget iframe: sha256 of the streamable-HTTP endpoint URL
// (including the /mcp path), first 32 hex chars, under claudemcpcontent.com.
// Although optional in the MCP Apps spec, Claude silently never renders the
// iframe without it. A pre-computed value can be forced via
// GREEN_API_WIDGET_DOMAIN (must already end in .claudemcpcontent.com).
func claudeWidgetDomain(baseURL string) string {
	if d := os.Getenv("GREEN_API_WIDGET_DOMAIN"); strings.HasSuffix(d, ".claudemcpcontent.com") {
		return d
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/mcp"
	sum := sha256.Sum256([]byte(endpoint))
	return hex.EncodeToString(sum[:])[:32] + ".claudemcpcontent.com"
}

// extractInstanceID parses the {id} variable from a resource URI template match.
// The mcp-go library populates req.Params.Arguments with matched template variables.
func extractInstanceID(req mcp.ReadResourceRequest) (uint64, error) {
	args := req.Params.Arguments
	if args == nil {
		return 0, fmt.Errorf("missing URI template arguments (expected {id})")
	}

	raw, ok := args["id"]
	if !ok {
		return 0, fmt.Errorf("missing {id} in resource URI: %s", req.Params.URI)
	}

	switch v := raw.(type) {
	case string:
		var id uint64
		_, err := fmt.Sscanf(v, "%d", &id)
		if err != nil {
			return 0, fmt.Errorf("invalid instance id %q: %w", v, err)
		}
		return id, nil
	case float64:
		return uint64(v), nil
	default:
		return 0, fmt.Errorf("unexpected type for instance id: %T", raw)
	}
}
