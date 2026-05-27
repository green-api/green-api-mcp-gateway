// SPDX-License-Identifier: MIT
// Copyright (c) 2026 GREEN-API

package mcp

import (
	"context"
	"fmt"

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
	s.mcp.AddResource(
		mcp.NewResource("ui://qr", "QR Code Widget",
			mcp.WithMIMEType("text/html;profile=mcp-app"),
			mcp.WithResourceDescription("Interactive QR code widget for authorizing a GREEN-API instance"),
		),
		mcpgo.ResourceHandlerFunc(func(_ context.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			return []mcp.ResourceContents{
				mcp.TextResourceContents{
					URI:      "ui://qr",
					MIMEType: "text/html;profile=mcp-app",
					Text:     infrastructure.QRAppHTML,
				},
			}, nil
		}),
	)

	// ui://contacts — MCP App widget for browsing the contacts list
	s.mcp.AddResource(
		mcp.NewResource("ui://contacts", "Contacts List Widget",
			mcp.WithMIMEType("text/html;profile=mcp-app"),
			mcp.WithResourceDescription("Interactive contacts list with search and contact details"),
		),
		mcpgo.ResourceHandlerFunc(func(_ context.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			return []mcp.ResourceContents{
				mcp.TextResourceContents{
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
