// SPDX-License-Identifier: MIT
// Copyright (c) 2026 GREEN-API

package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"
)

// registerPrompts adds all Green API prompt templates to the MCP server.
func registerPrompts(s *Server) {
	// whatsapp_customer_support — template for a customer support agent
	s.mcp.AddPrompt(
		mcp.NewPrompt("whatsapp_customer_support",
			mcp.WithPromptDescription("System prompt template for a WhatsApp customer support agent. Configurable per instance and language."),
			mcp.WithArgument("instance_id",
				mcp.RequiredArgument(),
				mcp.ArgumentDescription("WhatsApp instance ID used for support"),
			),
			mcp.WithArgument("language",
				mcp.RequiredArgument(),
				mcp.ArgumentDescription("Language to use with customers (e.g. en, ru, es)"),
			),
		),
		mcpgo.PromptHandlerFunc(func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			instanceID := req.Params.Arguments["instance_id"]
			language := req.Params.Arguments["language"]

			if instanceID == "" {
				return nil, fmt.Errorf("instance_id is required")
			}
			if language == "" {
				return nil, fmt.Errorf("language is required")
			}

			systemText := fmt.Sprintf(`You are a professional customer support agent operating via WhatsApp (instance: %s).

Communication language: %s

## Your responsibilities:
- Respond promptly and politely to customer inquiries
- Provide accurate information about products and services
- Escalate complex issues to human agents when necessary
- Keep responses concise and clear for messaging format

## Guidelines:
- Always greet the customer at the start of a conversation
- Use a friendly but professional tone
- If you cannot resolve an issue, acknowledge it and offer escalation
- Avoid sending multiple short messages; consolidate into one clear response
- Do not share sensitive internal information

## Available tools:
- whatsapp_send_message (instance_id: %s) — send text replies
- whatsapp_send_file — share documents or images
- whatsapp_get_contact_info — look up customer details

When responding, always identify yourself as the support assistant for this channel.`,
				instanceID, language, instanceID)

			return &mcp.GetPromptResult{
				Description: fmt.Sprintf("Customer support agent prompt for instance %s (%s)", instanceID, language),
				Messages: []mcp.PromptMessage{
					{
						Role: mcp.RoleUser,
						Content: mcp.TextContent{
							Type: "text",
							Text: systemText,
						},
					},
				},
			}, nil
		}),
	)

	// whatsapp_broadcast — template for mass message broadcasts
	s.mcp.AddPrompt(
		mcp.NewPrompt("whatsapp_broadcast",
			mcp.WithPromptDescription("Template for planning and executing a mass WhatsApp broadcast. Helps craft the message and prepare the recipient list."),
			mcp.WithArgument("instance_id",
				mcp.RequiredArgument(),
				mcp.ArgumentDescription("WhatsApp instance ID used for the broadcast"),
			),
			mcp.WithArgument("message_template",
				mcp.RequiredArgument(),
				mcp.ArgumentDescription("Message body template (variables {{name}}, {{company}}, etc. are supported)"),
			),
		),
		mcpgo.PromptHandlerFunc(func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			instanceID := req.Params.Arguments["instance_id"]
			messageTemplate := req.Params.Arguments["message_template"]

			if instanceID == "" {
				return nil, fmt.Errorf("instance_id is required")
			}
			if messageTemplate == "" {
				return nil, fmt.Errorf("message_template is required")
			}

			promptText := fmt.Sprintf(`You are a broadcast messaging assistant for WhatsApp instance %s.

## Message template to send:
---
%s
---

## Your task:
1. Review the message template above for clarity and compliance
2. Ask the user for the recipient list (phone numbers or chat IDs)
3. Personalize the message for each recipient if template variables ({{name}}, etc.) are present
4. Use whatsapp_send_message (instance_id: %s) to send to each recipient
5. Track sent/failed messages and provide a summary report

## Important rules:
- Respect WhatsApp messaging policies — no spam
- Add delays between messages to avoid rate limiting (recommended: 1-3 seconds)
- Verify recipients exist via whatsapp_check_whatsapp before sending if list is unverified
- Log any failures and retry once before marking as failed
- Maximum recommended batch size: 50 recipients per session

## Broadcast session format:
- Confirm recipient count before starting
- Report progress every 10 messages
- Provide final summary: total sent, failed, skipped`,
				instanceID, messageTemplate, instanceID)

			return &mcp.GetPromptResult{
				Description: fmt.Sprintf("Broadcast messaging prompt for instance %s", instanceID),
				Messages: []mcp.PromptMessage{
					{
						Role: mcp.RoleUser,
						Content: mcp.TextContent{
							Type: "text",
							Text: promptText,
						},
					},
				},
			}, nil
		}),
	)
}
