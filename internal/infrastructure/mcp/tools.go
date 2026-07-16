// SPDX-License-Identifier: MIT
// Copyright (c) 2026 GREEN-API

package mcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"golang.org/x/image/draw"

	"github.com/green-api/green-api-mcp-gateway/internal/domain"
	"github.com/green-api/green-api-mcp-gateway/internal/infrastructure"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"
)

const partnerAPIBaseURL = "https://api.green-api.com"

// partnerDo executes a Partner API HTTP request and returns the response body.
func partnerDo(ctx context.Context, method, url string, body any) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("partner API error %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// registerTools adds all Green API tools to the MCP server.
func registerTools(s *Server) {
	// ── Session Auth ─────────────────────────────────────────────────────────

	// whatsapp_connect
	s.addTool(
		mcp.NewTool("whatsapp_connect",
			mcp.WithDescription("Connect a WhatsApp instance. Call this before using other tools. Validates credentials via getStateInstance."),
			mcp.WithNumber("instance_id", mcp.Required(), mcp.Description("WhatsApp instance ID (from console.green-api.com)")),
			mcp.WithString("api_token", mcp.Required(), mcp.Description("Instance API token")),
			mcp.WithString("api_url", mcp.Description("API URL (defaults to https://api.green-api.com)")),
		),
		mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			instanceID, err := resolveInstanceID(ctx, req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			apiToken, err := req.RequireString("api_token")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			apiURL := req.GetString("api_url", "https://api.green-api.com")

			// Register credentials
			creds := &domain.InstanceCredentials{
				InstanceID: instanceID,
				APIToken:   apiToken,
				APIURL:     apiURL,
			}
			s.credentials.AddInstance(creds)

			// Validate by calling getStateInstance
			result, err := s.client.GetMethod(ctx, instanceID, domain.MethodGetStateInstance)
			if err != nil {
				// Remove invalid credentials
				s.credentials.RemoveInstance(instanceID)
				return mcp.NewToolResultError(fmt.Sprintf("Connection failed: %s", err.Error())), nil
			}

			return mcp.NewToolResultText(fmt.Sprintf(`{"connected": true, "instance_id": %d, "state": %s}`, instanceID, string(result))), nil
		}),
	)

	// whatsapp_disconnect
	s.addTool(
		mcp.NewTool("whatsapp_disconnect",
			mcp.WithDescription("Disconnect a WhatsApp instance and remove credentials from the session"),
			mcp.WithNumber("instance_id", mcp.Required(), mcp.Description("WhatsApp instance ID")),
		),
		mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			instanceID, err := resolveInstanceID(ctx, req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			s.credentials.RemoveInstance(instanceID)
			return mcp.NewToolResultText(fmt.Sprintf(`{"disconnected": true, "instance_id": %d}`, instanceID)), nil
		}),
	)

	// ── Messaging ────────────────────────────────────────────────────────────

	// whatsapp_send_message
	s.addTool(
		mcp.NewTool("whatsapp_send_message",
			mcp.WithDescription("Send a text message via WhatsApp"),
			mcp.WithNumber("instance_id", mcp.Required(), mcp.Description("WhatsApp instance ID")),
			mcp.WithString("chat_id", mcp.Required(), mcp.Description("Chat ID (79001234567@c.us or 120363XXX@g.us)")),
			mcp.WithString("message", mcp.Required(), mcp.Description("Message text")),
			mcp.WithString("quoted_message_id", mcp.Description("ID of the message to quote (optional)")),
			mcp.WithBoolean("link_preview", mcp.Description("Show link previews")),
		),
		mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			instanceID, err := resolveInstanceID(ctx, req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			chatID, err := req.RequireString("chat_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			message, err := req.RequireString("message")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			body := domain.SendMessageRequest{
				ChatID:          chatID,
				Message:         message,
				QuotedMessageID: req.GetString("quoted_message_id", ""),
				LinkPreview:     req.GetBool("link_preview", false),
			}

			result, err := s.client.CallMethod(ctx, instanceID, domain.MethodSendMessage, body)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(result)), nil
		}),
	)

	// whatsapp_send_file
	s.addTool(
		mcp.NewTool("whatsapp_send_file",
			mcp.WithDescription("Send a file by URL"),
			mcp.WithNumber("instance_id", mcp.Required(), mcp.Description("WhatsApp instance ID")),
			mcp.WithString("chat_id", mcp.Required(), mcp.Description("Chat ID")),
			mcp.WithString("url", mcp.Required(), mcp.Description("File URL")),
			mcp.WithString("file_name", mcp.Description("File name")),
			mcp.WithString("caption", mcp.Description("File caption")),
		),
		mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			instanceID, err := resolveInstanceID(ctx, req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			chatID, err := req.RequireString("chat_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			url, err := req.RequireString("url")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			body := domain.SendFileByURLRequest{
				ChatID:   chatID,
				URL:      url,
				FileName: req.GetString("file_name", ""),
				Caption:  req.GetString("caption", ""),
			}
			result, err := s.client.CallMethod(ctx, instanceID, domain.MethodSendFileByURL, body)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(result)), nil
		}),
	)

	// whatsapp_upload_file
	s.addTool(
		mcp.NewTool("whatsapp_upload_file",
			mcp.WithDescription("Upload a file to GREEN-API storage and return its URL. Use the returned URL with whatsapp_send_file."),
			mcp.WithNumber("instance_id", mcp.Required(), mcp.Description("WhatsApp instance ID")),
			mcp.WithString("file_base64", mcp.Required(), mcp.Description("File contents encoded as base64")),
			mcp.WithString("filename", mcp.Required(), mcp.Description("File name with extension (e.g. photo.jpg)")),
		),
		mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			instanceID, err := resolveInstanceID(ctx, req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			fileBase64, err := req.RequireString("file_base64")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			filename, err := req.RequireString("filename")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			fileData, err := base64.StdEncoding.DecodeString(fileBase64)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid base64: %s", err.Error())), nil
			}

			result, err := s.client.UploadFile(ctx, instanceID, filename, fileData)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(result)), nil
		}),
	)

	// whatsapp_send_file_by_upload
	s.addTool(
		mcp.NewTool("whatsapp_send_file_by_upload",
			mcp.WithDescription("Send a file directly to a chat via multipart upload (no separate URL step)"),
			mcp.WithNumber("instance_id", mcp.Required(), mcp.Description("WhatsApp instance ID")),
			mcp.WithString("chat_id", mcp.Required(), mcp.Description("Chat ID (79001234567@c.us or 120363XXX@g.us)")),
			mcp.WithString("file_base64", mcp.Required(), mcp.Description("File contents encoded as base64")),
			mcp.WithString("filename", mcp.Required(), mcp.Description("File name with extension (e.g. photo.jpg)")),
			mcp.WithString("caption", mcp.Description("File caption (optional)")),
		),
		mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			instanceID, err := resolveInstanceID(ctx, req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			chatID, err := req.RequireString("chat_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			fileBase64, err := req.RequireString("file_base64")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			filename, err := req.RequireString("filename")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			caption := req.GetString("caption", "")

			fileData, err := base64.StdEncoding.DecodeString(fileBase64)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid base64: %s", err.Error())), nil
			}

			result, err := s.client.SendFileByUpload(ctx, instanceID, chatID, caption, filename, fileData)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(result)), nil
		}),
	)

	// whatsapp_send_location
	s.addTool(
		mcp.NewTool("whatsapp_send_location",
			mcp.WithDescription("Send a location"),
			mcp.WithNumber("instance_id", mcp.Required(), mcp.Description("WhatsApp instance ID")),
			mcp.WithString("chat_id", mcp.Required(), mcp.Description("Chat ID")),
			mcp.WithNumber("latitude", mcp.Required(), mcp.Description("Latitude")),
			mcp.WithNumber("longitude", mcp.Required(), mcp.Description("Longitude")),
			mcp.WithString("name", mcp.Description("Place name")),
			mcp.WithString("address", mcp.Description("Address")),
		),
		mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			instanceID, err := resolveInstanceID(ctx, req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			chatID, err := req.RequireString("chat_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			lat, err := req.RequireFloat("latitude")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			lon, err := req.RequireFloat("longitude")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			body := domain.SendLocationRequest{
				ChatID:    chatID,
				Latitude:  lat,
				Longitude: lon,
				Name:      req.GetString("name", ""),
				Address:   req.GetString("address", ""),
			}
			result, err := s.client.CallMethod(ctx, instanceID, domain.MethodSendLocation, body)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(result)), nil
		}),
	)

	// whatsapp_send_contact
	s.addTool(
		mcp.NewTool("whatsapp_send_contact",
			mcp.WithDescription("Send a contact card via WhatsApp"),
			mcp.WithNumber("instance_id", mcp.Required(), mcp.Description("WhatsApp instance ID")),
			mcp.WithString("chat_id", mcp.Required(), mcp.Description("Chat ID")),
			mcp.WithNumber("phone_number", mcp.Required(), mcp.Description("Contact phone number")),
			mcp.WithString("first_name", mcp.Description("Contact first name")),
			mcp.WithString("last_name", mcp.Description("Contact last name")),
			mcp.WithString("company", mcp.Description("Contact company")),
		),
		mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			instanceID, err := resolveInstanceID(ctx, req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			chatID, err := req.RequireString("chat_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			phoneNumber, err := req.RequireFloat("phone_number")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			body := domain.SendContactRequest{
				ChatID: chatID,
				Contact: domain.Contact{
					PhoneNumber: int64(phoneNumber),
					FirstName:   req.GetString("first_name", ""),
					LastName:    req.GetString("last_name", ""),
					Company:     req.GetString("company", ""),
				},
			}
			result, err := s.client.CallMethod(ctx, instanceID, domain.MethodSendContact, body)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(result)), nil
		}),
	)

	// whatsapp_send_poll
	s.addTool(
		mcp.NewTool("whatsapp_send_poll",
			mcp.WithDescription("Send a poll via WhatsApp"),
			mcp.WithNumber("instance_id", mcp.Required(), mcp.Description("WhatsApp instance ID")),
			mcp.WithString("chat_id", mcp.Required(), mcp.Description("Chat ID")),
			mcp.WithString("message", mcp.Required(), mcp.Description("Poll question")),
			mcp.WithArray("options", mcp.Required(), mcp.Description("Answer options (strings)"), mcp.WithStringItems()),
			mcp.WithBoolean("multiple_answers", mcp.Description("Allow multiple answers")),
		),
		mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			instanceID, err := resolveInstanceID(ctx, req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			chatID, err := req.RequireString("chat_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			message, err := req.RequireString("message")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			optionStrs, err := req.RequireStringSlice("options")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			options := make([]domain.PollOption, len(optionStrs))
			for i, o := range optionStrs {
				options[i] = domain.PollOption{OptionName: o}
			}

			body := domain.SendPollRequest{
				ChatID:          chatID,
				Message:         message,
				Options:         options,
				MultipleAnswers: req.GetBool("multiple_answers", false),
			}
			result, err := s.client.CallMethod(ctx, instanceID, domain.MethodSendPoll, body)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(result)), nil
		}),
	)

	// whatsapp_forward_messages
	s.addTool(
		mcp.NewTool("whatsapp_forward_messages",
			mcp.WithDescription("Forward messages to another chat"),
			mcp.WithNumber("instance_id", mcp.Required(), mcp.Description("WhatsApp instance ID")),
			mcp.WithString("chat_id", mcp.Required(), mcp.Description("Destination chat ID")),
			mcp.WithString("chat_id_from", mcp.Required(), mcp.Description("Source chat ID")),
			mcp.WithArray("messages", mcp.Required(), mcp.Description("Message IDs to forward"), mcp.WithStringItems()),
		),
		mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			instanceID, err := resolveInstanceID(ctx, req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			chatID, err := req.RequireString("chat_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			chatIDFrom, err := req.RequireString("chat_id_from")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			messages, err := req.RequireStringSlice("messages")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			body := domain.ForwardMessagesRequest{
				ChatID:     chatID,
				ChatIDFrom: chatIDFrom,
				Messages:   messages,
			}
			result, err := s.client.CallMethod(ctx, instanceID, domain.MethodForwardMessages, body)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(result)), nil
		}),
	)

	// whatsapp_edit_message
	s.addTool(
		mcp.NewTool("whatsapp_edit_message",
			mcp.WithDescription("Edit a previously sent message"),
			mcp.WithNumber("instance_id", mcp.Required(), mcp.Description("WhatsApp instance ID")),
			mcp.WithString("chat_id", mcp.Required(), mcp.Description("Chat ID")),
			mcp.WithString("id_message", mcp.Required(), mcp.Description("ID of the message to edit")),
			mcp.WithString("message", mcp.Required(), mcp.Description("New message text")),
		),
		mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			instanceID, err := resolveInstanceID(ctx, req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			chatID, err := req.RequireString("chat_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			idMessage, err := req.RequireString("id_message")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			message, err := req.RequireString("message")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			body := domain.EditMessageRequest{
				ChatID:    chatID,
				IDMessage: idMessage,
				Message:   message,
			}
			result, err := s.client.CallMethod(ctx, instanceID, domain.MethodEditMessage, body)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(result)), nil
		}),
	)

	// whatsapp_delete_message
	s.addTool(
		mcp.NewTool("whatsapp_delete_message",
			mcp.WithDescription("Delete a message"),
			mcp.WithNumber("instance_id", mcp.Required(), mcp.Description("WhatsApp instance ID")),
			mcp.WithString("chat_id", mcp.Required(), mcp.Description("Chat ID")),
			mcp.WithString("id_message", mcp.Required(), mcp.Description("ID of the message to delete")),
		),
		mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			instanceID, err := resolveInstanceID(ctx, req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			chatID, err := req.RequireString("chat_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			idMessage, err := req.RequireString("id_message")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			body := domain.DeleteMessageRequest{
				ChatID:    chatID,
				IDMessage: idMessage,
			}
			result, err := s.client.CallMethod(ctx, instanceID, domain.MethodDeleteMessage, body)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(result)), nil
		}),
	)

	// whatsapp_get_state
	s.addTool(
		mcp.NewTool("whatsapp_get_state",
			mcp.WithDescription("Get instance state (authorized, notAuthorized, blocked, sleepMode)"),
			mcp.WithNumber("instance_id", mcp.Required(), mcp.Description("WhatsApp instance ID")),
		),
		mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			instanceID, err := resolveInstanceID(ctx, req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			result, err := s.client.GetMethod(ctx, instanceID, domain.MethodGetStateInstance)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(result)), nil
		}),
	)

	// whatsapp_get_settings
	s.addTool(
		mcp.NewTool("whatsapp_get_settings",
			mcp.WithDescription("Get instance settings"),
			mcp.WithNumber("instance_id", mcp.Required(), mcp.Description("WhatsApp instance ID")),
		),
		mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			instanceID, err := resolveInstanceID(ctx, req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			result, err := s.client.GetMethod(ctx, instanceID, domain.MethodGetSettings)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(result)), nil
		}),
	)

	// whatsapp_set_settings
	s.addTool(
		mcp.NewTool("whatsapp_set_settings",
			mcp.WithDescription("Update instance settings (webhooks, proxy, etc.)"),
			mcp.WithNumber("instance_id", mcp.Required(), mcp.Description("WhatsApp instance ID")),
			mcp.WithString("webhook_url", mcp.Description("URL for incoming webhooks")),
			mcp.WithString("webhook_url_token", mcp.Description("Token for the webhook URL")),
			mcp.WithBoolean("outgoing_webhook", mcp.Description("Enable outgoing message webhook")),
			mcp.WithBoolean("incoming_webhook", mcp.Description("Enable incoming message webhook")),
			mcp.WithBoolean("device_webhook", mcp.Description("Enable device webhook")),
			mcp.WithBoolean("status_instance_webhook", mcp.Description("Enable instance status webhook")),
			mcp.WithBoolean("state_webhook", mcp.Description("Enable state webhook")),
			mcp.WithBoolean("mark_incoming_messages_readed", mcp.Description("Mark incoming messages as read")),
			mcp.WithNumber("delay_message", mcp.Description("Message send delay (ms)")),
		),
		mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			instanceID, err := resolveInstanceID(ctx, req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			body := domain.SetSettingsRequest{
				WebhookURL:      req.GetString("webhook_url", ""),
				WebhookURLToken: req.GetString("webhook_url_token", ""),
			}
			// Optional booleans — build as map for SDK
			args := req.GetArguments()
			if v, ok := args["outgoing_webhook"]; ok {
				body.OutgoingWebhook = toBoolStr(v)
			}
			if v, ok := args["incoming_webhook"]; ok {
				body.IncomingWebhook = toBoolStr(v)
			}
			if v, ok := args["device_webhook"]; ok {
				body.DeviceWebhook = toBoolStr(v)
			}
			if v, ok := args["status_instance_webhook"]; ok {
				body.StatusInstanceWebhook = toBoolStr(v)
			}
			if v, ok := args["state_webhook"]; ok {
				body.StateWebhook = toBoolStr(v)
			}
			if v, ok := args["mark_incoming_messages_readed"]; ok {
				b := toBoolPtr(v)
				body.MarkIncomingMessagesReaded = b
			}
			if v, ok := args["delay_message"]; ok {
				if f, ok2 := toFloat64(v); ok2 {
					i := int(f)
					body.DelaySendMessagesMilliseconds = &i
				}
			}

			result, err := s.client.CallMethod(ctx, instanceID, domain.MethodSetSettings, body)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(result)), nil
		}),
	)

	// whatsapp_get_qr
	s.addTool(
		mcp.NewTool("whatsapp_get_qr",
			mcp.WithDescription("Get the QR code for instance authorization. The result contains the QR as an image and a qrLink URL to an official GREEN-API page with a full-size auto-refreshing QR code. Always include the qrLink in your response as a clickable markdown link so the user can open it in the browser and scan comfortably."),
			mcp.WithNumber("instance_id", mcp.Required(), mcp.Description("WhatsApp instance ID")),
		),
		mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			instanceID, err := resolveInstanceID(ctx, req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			raw, err := s.client.GetMethod(ctx, instanceID, domain.MethodGetQR)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			var qr domain.QRResponse
			if err := json.Unmarshal(raw, &qr); err != nil {
				return mcp.NewToolResultText(string(raw)), nil
			}
			if qr.Type == "qrCode" {
				small, err := normalizeQR(qr.Message)
				if err != nil {
					small = qr.Message
				}
				text := "Scan this QR code to authorize the instance"
				// Optional plain-text fallback for hosts that cannot render
				// image content (enable with GREEN_API_QR_UNICODE=1). Shown
				// after the image so image-capable hosts display the PNG first.
				if os.Getenv("GREEN_API_QR_UNICODE") == "1" {
					if unicode, uerr := pngToUnicode(qr.Message); uerr == nil {
						text += "\n```\n" + unicode + "\n```"
					}
				}
				res := &mcp.CallToolResult{
					Content: []mcp.Content{
						mcp.ImageContent{Type: "image", Data: small, MIMEType: "image/png"},
						mcp.TextContent{Type: "text", Text: text},
					},
				}
				res.Meta = qrToolResultMeta(small, "image/png")
				res.StructuredContent = map[string]any{"type": "qrCode", "message": small, "mimeType": "image/png"}
				return res, nil
			}
			res := mcp.NewToolResultText(fmt.Sprintf(`{"type":%q,"message":%q}`, qr.Type, qr.Message))
			res.Meta = &mcp.Meta{AdditionalFields: map[string]any{"ui": map[string]any{"resourceUri": "ui://qr"}}}
			res.StructuredContent = map[string]any{"type": qr.Type, "message": qr.Message}
			return res, nil
		}),
	)

	// whatsapp_check_whatsapp
	s.addTool(
		mcp.NewTool("whatsapp_check_whatsapp",
			mcp.WithDescription("Check whether a phone number has a WhatsApp account"),
			mcp.WithNumber("instance_id", mcp.Required(), mcp.Description("WhatsApp instance ID")),
			mcp.WithNumber("phone_number", mcp.Required(), mcp.Description("Phone number to check")),
		),
		mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			instanceID, err := resolveInstanceID(ctx, req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			phoneNumber, err := req.RequireFloat("phone_number")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			body := domain.CheckWhatsappRequest{
				PhoneNumber: int64(phoneNumber),
			}
			result, err := s.client.CallMethod(ctx, instanceID, domain.MethodCheckWhatsapp, body)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(result)), nil
		}),
	)

	// whatsapp_get_contacts
	s.addTool(
		mcp.NewTool("whatsapp_get_contacts",
			mcp.WithDescription("Get the contact list"),
			mcp.WithNumber("instance_id", mcp.Required(), mcp.Description("WhatsApp instance ID")),
		),
		mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			instanceID, err := resolveInstanceID(ctx, req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			result, err := s.client.GetContacts(ctx, instanceID)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			res := mcp.NewToolResultText(string(result))
			res.Meta = &mcp.Meta{AdditionalFields: map[string]any{"ui": map[string]any{"resourceUri": "ui://contacts"}}}
			return res, nil
		}),
	)

	// whatsapp_get_contact_info
	s.addTool(
		mcp.NewTool("whatsapp_get_contact_info",
			mcp.WithDescription("Get information about a contact or group"),
			mcp.WithNumber("instance_id", mcp.Required(), mcp.Description("WhatsApp instance ID")),
			mcp.WithString("chat_id", mcp.Required(), mcp.Description("Contact or group ID")),
		),
		mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			instanceID, err := resolveInstanceID(ctx, req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			chatID, err := req.RequireString("chat_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			body := domain.GetContactInfoRequest{
				ChatID: chatID,
			}
			result, err := s.client.CallMethod(ctx, instanceID, domain.MethodGetContactInfo, body)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(result)), nil
		}),
	)

	// whatsapp_receive_notification
	s.addTool(
		mcp.NewTool("whatsapp_receive_notification",
			mcp.WithDescription("Get one notification from the queue (manual polling)"),
			mcp.WithNumber("instance_id", mcp.Required(), mcp.Description("WhatsApp instance ID")),
		),
		mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			instanceID, err := resolveInstanceID(ctx, req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			result, err := s.client.GetMethod(ctx, instanceID, domain.MethodReceiveNotification)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(result)), nil
		}),
	)

	// whatsapp_create_group
	s.addTool(
		mcp.NewTool("whatsapp_create_group",
			mcp.WithDescription("Create a WhatsApp group"),
			mcp.WithNumber("instance_id", mcp.Required(), mcp.Description("WhatsApp instance ID")),
			mcp.WithString("group_name", mcp.Required(), mcp.Description("Group name")),
			mcp.WithArray("chat_ids", mcp.Required(), mcp.Description("Participant list (chat IDs, e.g. 79001234567@c.us)"), mcp.WithStringItems()),
		),
		mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			instanceID, err := resolveInstanceID(ctx, req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			groupName, err := req.RequireString("group_name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			chatIDs, err := req.RequireStringSlice("chat_ids")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			body := domain.CreateGroupRequest{
				GroupName: groupName,
				ChatIDs:   chatIDs,
			}
			result, err := s.client.CreateGroup(ctx, instanceID, body)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			data, err := marshalJSON(result)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(data)), nil
		}),
	)

	// whatsapp_get_group_data
	s.addTool(
		mcp.NewTool("whatsapp_get_group_data",
			mcp.WithDescription("Get group data (members, name, invite link)"),
			mcp.WithNumber("instance_id", mcp.Required(), mcp.Description("WhatsApp instance ID")),
			mcp.WithString("group_id", mcp.Required(), mcp.Description("Group ID (e.g. 120363XXX@g.us)")),
		),
		mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			instanceID, err := resolveInstanceID(ctx, req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			groupID, err := req.RequireString("group_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			result, err := s.client.GetGroupData(ctx, instanceID, groupID)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			data, err := marshalJSON(result)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(data)), nil
		}),
	)

	// whatsapp_add_group_participant
	s.addTool(
		mcp.NewTool("whatsapp_add_group_participant",
			mcp.WithDescription("Add a participant to a WhatsApp group"),
			mcp.WithNumber("instance_id", mcp.Required(), mcp.Description("WhatsApp instance ID")),
			mcp.WithString("group_id", mcp.Required(), mcp.Description("Group ID")),
			mcp.WithString("participant_chat_id", mcp.Required(), mcp.Description("Participant chat ID (79001234567@c.us)")),
		),
		mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			instanceID, err := resolveInstanceID(ctx, req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			groupID, err := req.RequireString("group_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			participant, err := req.RequireString("participant_chat_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			body := domain.GroupParticipantRequest{
				GroupID:           groupID,
				ParticipantChatID: participant,
			}
			result, err := s.client.AddGroupParticipant(ctx, instanceID, body)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(result)), nil
		}),
	)

	// whatsapp_remove_group_participant
	s.addTool(
		mcp.NewTool("whatsapp_remove_group_participant",
			mcp.WithDescription("Remove a participant from a WhatsApp group"),
			mcp.WithNumber("instance_id", mcp.Required(), mcp.Description("WhatsApp instance ID")),
			mcp.WithString("group_id", mcp.Required(), mcp.Description("Group ID")),
			mcp.WithString("participant_chat_id", mcp.Required(), mcp.Description("Participant chat ID (79001234567@c.us)")),
		),
		mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			instanceID, err := resolveInstanceID(ctx, req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			groupID, err := req.RequireString("group_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			participant, err := req.RequireString("participant_chat_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			body := domain.GroupParticipantRequest{
				GroupID:           groupID,
				ParticipantChatID: participant,
			}
			result, err := s.client.RemoveGroupParticipant(ctx, instanceID, body)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(result)), nil
		}),
	)

	// ── Instance management ─────────────────────────────────────────────────

	// whatsapp_reboot
	s.addTool(
		mcp.NewTool("whatsapp_reboot",
			mcp.WithDescription("Reboot the WhatsApp instance"),
			mcp.WithNumber("instance_id", mcp.Required(), mcp.Description("WhatsApp instance ID")),
		),
		mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			instanceID, err := resolveInstanceID(ctx, req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			result, err := s.client.GetMethod(ctx, instanceID, domain.MethodReboot)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if result == nil {
				return mcp.NewToolResultText(`{"rebooted": true}`), nil
			}
			return mcp.NewToolResultText(string(result)), nil
		}),
	)

	// whatsapp_logout
	s.addTool(
		mcp.NewTool("whatsapp_logout",
			mcp.WithDescription("Log out the WhatsApp instance"),
			mcp.WithNumber("instance_id", mcp.Required(), mcp.Description("WhatsApp instance ID")),
		),
		mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			instanceID, err := resolveInstanceID(ctx, req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			result, err := s.client.GetMethod(ctx, instanceID, domain.MethodLogout)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if result == nil {
				return mcp.NewToolResultText(`{"logout": true}`), nil
			}
			return mcp.NewToolResultText(string(result)), nil
		}),
	)

	// whatsapp_get_authorization_code
	s.addTool(
		mcp.NewTool("whatsapp_get_authorization_code",
			mcp.WithDescription("Get an authorization code for sign-in by phone number"),
			mcp.WithNumber("instance_id", mcp.Required(), mcp.Description("WhatsApp instance ID")),
			mcp.WithString("phone_number", mcp.Required(), mcp.Description("Phone number (e.g. 79001234567)")),
		),
		mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			instanceID, err := resolveInstanceID(ctx, req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			phoneStr, err := req.RequireString("phone_number")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			// Strip any non-digit characters (spaces, dashes, +) that an LLM might include.
			digitsOnly := strings.Map(func(r rune) rune {
				if r >= '0' && r <= '9' {
					return r
				}
				return -1
			}, phoneStr)
			phoneNumber, err := strconv.ParseInt(digitsOnly, 10, 64)
			if err != nil || phoneNumber <= 0 {
				return mcp.NewToolResultError("phone_number must be a positive integer containing only digits"), nil
			}
			body := domain.GetAuthorizationCodeRequest{PhoneNumber: phoneNumber}
			result, err := s.client.GetAuthorizationCode(ctx, instanceID, body)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(result)), nil
		}),
	)

	// whatsapp_get_wa_settings
	s.addTool(
		mcp.NewTool("whatsapp_get_wa_settings",
			mcp.WithDescription("Get WhatsApp account settings (name, description, etc.)"),
			mcp.WithNumber("instance_id", mcp.Required(), mcp.Description("WhatsApp instance ID")),
		),
		mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			instanceID, err := resolveInstanceID(ctx, req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			result, err := s.client.GetMethod(ctx, instanceID, domain.MethodGetWaSettings)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(result)), nil
		}),
	)

	// ── History & Reading ────────────────────────────────────────────────────

	// whatsapp_get_chat_history
	s.addTool(
		mcp.NewTool("whatsapp_get_chat_history",
			mcp.WithDescription("Get chat message history. Use a small 'count' (default 50, max 100) to keep responses focused."),
			mcp.WithNumber("instance_id", mcp.Required(), mcp.Description("WhatsApp instance ID")),
			mcp.WithString("chat_id", mcp.Required(), mcp.Description("Chat ID (79001234567@c.us or 120363XXX@g.us)")),
			mcp.WithNumber("count", mcp.Description("Number of messages (default 50, max 100)")),
		),
		mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			instanceID, err := resolveInstanceID(ctx, req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			chatID, err := req.RequireString("chat_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			count := int(req.GetFloat("count", 50))
			if count < 1 {
				count = 50
			}
			if count > 100 {
				count = 100
			}
			body := domain.GetChatHistoryRequest{ChatID: chatID, Count: count}
			result, err := s.client.GetChatHistory(ctx, instanceID, body)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(result)), nil
		}),
	)

	// whatsapp_get_message
	s.addTool(
		mcp.NewTool("whatsapp_get_message",
			mcp.WithDescription("Get a message by ID"),
			mcp.WithNumber("instance_id", mcp.Required(), mcp.Description("WhatsApp instance ID")),
			mcp.WithString("chat_id", mcp.Required(), mcp.Description("Chat ID")),
			mcp.WithString("id_message", mcp.Required(), mcp.Description("Message ID")),
		),
		mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			instanceID, err := resolveInstanceID(ctx, req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			chatID, err := req.RequireString("chat_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			idMessage, err := req.RequireString("id_message")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			body := domain.GetMessageRequest{ChatID: chatID, IDMessage: idMessage}
			result, err := s.client.GetMessage(ctx, instanceID, body)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(result)), nil
		}),
	)

	// whatsapp_last_incoming_messages
	s.addTool(
		mcp.NewTool("whatsapp_last_incoming_messages",
			mcp.WithDescription("Get recent incoming messages from the last N minutes (default 60, max 1440 = 24 hours)."),
			mcp.WithNumber("instance_id", mcp.Required(), mcp.Description("WhatsApp instance ID")),
			mcp.WithNumber("minutes", mcp.Description("Time window in minutes (default 60, max 1440)")),
		),
		mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			instanceID, err := resolveInstanceID(ctx, req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			minutes := int(req.GetFloat("minutes", 60))
			if minutes < 1 {
				minutes = 60
			}
			if minutes > 1440 {
				minutes = 1440
			}
			result, err := s.client.LastIncomingMessages(ctx, instanceID, minutes)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(result)), nil
		}),
	)

	// whatsapp_last_outgoing_messages
	s.addTool(
		mcp.NewTool("whatsapp_last_outgoing_messages",
			mcp.WithDescription("Get recent outgoing messages from the last N minutes (default 60, max 1440 = 24 hours)."),
			mcp.WithNumber("instance_id", mcp.Required(), mcp.Description("WhatsApp instance ID")),
			mcp.WithNumber("minutes", mcp.Description("Time window in minutes (default 60, max 1440)")),
		),
		mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			instanceID, err := resolveInstanceID(ctx, req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			minutes := int(req.GetFloat("minutes", 60))
			if minutes < 1 {
				minutes = 60
			}
			if minutes > 1440 {
				minutes = 1440
			}
			result, err := s.client.LastOutgoingMessages(ctx, instanceID, minutes)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(result)), nil
		}),
	)

	// whatsapp_read_chat
	s.addTool(
		mcp.NewTool("whatsapp_read_chat",
			mcp.WithDescription("Mark chat messages as read"),
			mcp.WithNumber("instance_id", mcp.Required(), mcp.Description("WhatsApp instance ID")),
			mcp.WithString("chat_id", mcp.Required(), mcp.Description("Chat ID")),
			mcp.WithString("id_message", mcp.Description("ID of a specific message (optional)")),
		),
		mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			instanceID, err := resolveInstanceID(ctx, req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			chatID, err := req.RequireString("chat_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			body := domain.ReadChatRequest{
				ChatID:    chatID,
				IDMessage: req.GetString("id_message", ""),
			}
			result, err := s.client.ReadChat(ctx, instanceID, body)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(result)), nil
		}),
	)

	// whatsapp_delete_notification
	s.addTool(
		mcp.NewTool("whatsapp_delete_notification",
			mcp.WithDescription("Delete a notification from the queue (acknowledge receipt)"),
			mcp.WithNumber("instance_id", mcp.Required(), mcp.Description("WhatsApp instance ID")),
			mcp.WithNumber("receipt_id", mcp.Required(), mcp.Description("Notification ID (receiptId from receiveNotification)")),
		),
		mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			instanceID, err := resolveInstanceID(ctx, req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			receiptIDFloat, err := req.RequireFloat("receipt_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if err := s.client.DeleteNotification(ctx, instanceID, int(receiptIDFloat)); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(`{"deleted": true}`), nil
		}),
	)

	// ── Group admin ──────────────────────────────────────────────────────────

	// whatsapp_set_group_admin
	s.addTool(
		mcp.NewTool("whatsapp_set_group_admin",
			mcp.WithDescription("Promote a group member to admin"),
			mcp.WithNumber("instance_id", mcp.Required(), mcp.Description("WhatsApp instance ID")),
			mcp.WithString("group_id", mcp.Required(), mcp.Description("Group ID (120363XXX@g.us)")),
			mcp.WithString("participant_chat_id", mcp.Required(), mcp.Description("Participant chat ID (79001234567@c.us)")),
		),
		mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			instanceID, err := resolveInstanceID(ctx, req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			groupID, err := req.RequireString("group_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			participant, err := req.RequireString("participant_chat_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			body := domain.GroupParticipantRequest{GroupID: groupID, ParticipantChatID: participant}
			result, err := s.client.SetGroupAdmin(ctx, instanceID, body)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(result)), nil
		}),
	)

	// whatsapp_remove_group_admin
	s.addTool(
		mcp.NewTool("whatsapp_remove_group_admin",
			mcp.WithDescription("Revoke admin rights from a group member"),
			mcp.WithNumber("instance_id", mcp.Required(), mcp.Description("WhatsApp instance ID")),
			mcp.WithString("group_id", mcp.Required(), mcp.Description("Group ID (120363XXX@g.us)")),
			mcp.WithString("participant_chat_id", mcp.Required(), mcp.Description("Participant chat ID (79001234567@c.us)")),
		),
		mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			instanceID, err := resolveInstanceID(ctx, req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			groupID, err := req.RequireString("group_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			participant, err := req.RequireString("participant_chat_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			body := domain.GroupParticipantRequest{GroupID: groupID, ParticipantChatID: participant}
			result, err := s.client.RemoveGroupAdmin(ctx, instanceID, body)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(result)), nil
		}),
	)

	// whatsapp_leave_group
	s.addTool(
		mcp.NewTool("whatsapp_leave_group",
			mcp.WithDescription("Leave a WhatsApp group"),
			mcp.WithNumber("instance_id", mcp.Required(), mcp.Description("WhatsApp instance ID")),
			mcp.WithString("group_id", mcp.Required(), mcp.Description("Group ID (120363XXX@g.us)")),
		),
		mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			instanceID, err := resolveInstanceID(ctx, req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			groupID, err := req.RequireString("group_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			body := domain.LeaveGroupRequest{GroupID: groupID}
			result, err := s.client.LeaveGroup(ctx, instanceID, body)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(result)), nil
		}),
	)

	// ── Media & Contacts ─────────────────────────────────────────────────────

	// whatsapp_get_contact_avatar
	s.addTool(
		mcp.NewTool("whatsapp_get_contact_avatar",
			mcp.WithDescription("Get the avatar of a contact or group"),
			mcp.WithNumber("instance_id", mcp.Required(), mcp.Description("WhatsApp instance ID")),
			mcp.WithString("chat_id", mcp.Required(), mcp.Description("Contact or group ID")),
		),
		mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			instanceID, err := resolveInstanceID(ctx, req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			chatID, err := req.RequireString("chat_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			body := domain.GetAvatarRequest{ChatID: chatID}
			result, err := s.client.GetContactAvatar(ctx, instanceID, body)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(result)), nil
		}),
	)

	// ── Partner API tools ────────────────────────────────────────────────────
	// Partner API is not covered by the Green API Go SDK, so we call it directly
	// via HTTP. Partner token is passed as a tool argument.

	// whatsapp_create_instance
	s.addTool(
		mcp.NewTool("whatsapp_create_instance",
			mcp.WithDescription("Create a new WhatsApp instance via the GREEN-API Partner API"),
			mcp.WithString("partner_token", mcp.Required(), mcp.Description("Partner Token from your GREEN-API dashboard")),
		),
		mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			partnerToken, err := req.RequireString("partner_token")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			url := fmt.Sprintf("%s/partner/createInstance/%s", partnerAPIBaseURL, partnerToken)
			result, err := partnerDo(ctx, http.MethodPost, url, nil)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(result)), nil
		}),
	)

	// whatsapp_delete_instance
	s.addTool(
		mcp.NewTool("whatsapp_delete_instance",
			mcp.WithDescription("Delete a WhatsApp instance via the GREEN-API Partner API"),
			mcp.WithString("partner_token", mcp.Required(), mcp.Description("Partner Token from your GREEN-API dashboard")),
			mcp.WithNumber("instance_id", mcp.Required(), mcp.Description("Instance ID to delete")),
		),
		mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			partnerToken, err := req.RequireString("partner_token")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			instanceID, err := resolveInstanceID(ctx, req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			url := fmt.Sprintf("%s/partner/deleteInstance/%s", partnerAPIBaseURL, partnerToken)
			body := map[string]any{"idInstance": instanceID}
			result, err := partnerDo(ctx, http.MethodPost, url, body)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(result)), nil
		}),
	)

	// whatsapp_get_instances
	s.addTool(
		mcp.NewTool("whatsapp_get_instances",
			mcp.WithDescription("List all partner instances via the GREEN-API Partner API"),
			mcp.WithString("partner_token", mcp.Required(), mcp.Description("Partner Token from your GREEN-API dashboard")),
		),
		mcpgo.ToolHandlerFunc(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			partnerToken, err := req.RequireString("partner_token")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			url := fmt.Sprintf("%s/partner/getInstances/%s", partnerAPIBaseURL, partnerToken)
			result, err := partnerDo(ctx, http.MethodGet, url, nil)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(result)), nil
		}),
	)
}

// marshalJSON marshals a value to JSON bytes.
func marshalJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}

func qrToolResultMeta(base64, mimeType string) *mcp.Meta {
	return &mcp.Meta{AdditionalFields: map[string]any{
		"ui": map[string]any{"resourceUri": "ui://qr"},
		"qr": map[string]any{
			"type":     "qrCode",
			"message":  base64,
			"mimeType": mimeType,
		},
	}}
}

// resolveInstanceID returns the instance ID for a tool call.
// Priority: explicit argument > authenticated context (OAuth/proxy mode).
// This allows instance_id to be omitted when the client authenticated via OAuth.
func resolveInstanceID(ctx context.Context, req mcp.CallToolRequest) (uint64, error) {
	// Try explicit argument first.
	args := req.GetArguments()
	if _, ok := args["instance_id"]; ok {
		f, err := req.RequireFloat("instance_id")
		if err != nil {
			return 0, err
		}
		return uint64(f), nil
	}
	// Fall back to authenticated context (set by proxy auth middleware).
	if id, ok := infrastructure.GetInstanceIDFromContext(ctx); ok && id > 0 {
		return id, nil
	}
	return 0, fmt.Errorf("instance_id is required")
}

// toBoolPtr converts an interface{} to *bool.
func toBoolPtr(v interface{}) *bool {
	switch b := v.(type) {
	case bool:
		return &b
	case float64:
		val := b != 0
		return &val
	}
	return nil
}

// toBoolStr converts an interface{} to a string "yes"/"no" for the SDK.
func toBoolStr(v interface{}) string {
	switch b := v.(type) {
	case bool:
		if b {
			return "yes"
		}
		return "no"
	case float64:
		if b != 0 {
			return "yes"
		}
		return "no"
	}
	return ""
}

// toFloat64 converts an interface{} to float64.
func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	}
	return 0, false
}

// pngToUnicode decodes a base64-encoded PNG QR code and returns a Unicode
// half-block representation using ▀ ▄ █ and space (2 module rows → 1 line).
// The QR module size is detected from the image so each module maps to
// exactly one character column — no information is lost to downscaling.
func pngToUnicode(b64 string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", err
	}
	src, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	dark := func(x, y int) bool {
		r, g, b, _ := src.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
		return (r+g+b)/3 < 0x8000
	}

	// Detect module size: the shortest run of same-colored pixels across
	// several scan lines. QR modules are square, so this works per-axis.
	module := w
	for _, y := range []int{h / 4, h / 2, 3 * h / 4} {
		run := 1
		prev := dark(0, y)
		for x := 1; x < w; x++ {
			d := dark(x, y)
			if d == prev {
				run++
				continue
			}
			if run < module {
				module = run
			}
			run = 1
			prev = d
		}
	}
	if module < 1 {
		module = 1
	}
	cols := w / module
	rows := h / module
	if cols < 21 { // smallest valid QR is 21 modules
		return "", fmt.Errorf("could not detect QR modules (got %d columns)", cols)
	}

	// Sample the center of each module.
	cell := func(cx, cy int) bool {
		if cy >= rows {
			return false
		}
		return dark(cx*module+module/2, cy*module+module/2)
	}

	const margin = 2
	pad := strings.Repeat(" ", margin)
	var sb strings.Builder
	sb.WriteString("\n")
	for y := 0; y < rows; y += 2 {
		sb.WriteString(pad)
		for x := 0; x < cols; x++ {
			top := cell(x, y)
			bot := cell(x, y+1)
			switch {
			case top && bot:
				sb.WriteRune('█')
			case top:
				sb.WriteRune('▀')
			case bot:
				sb.WriteRune('▄')
			default:
				sb.WriteRune(' ')
			}
		}
		sb.WriteString(pad)
		sb.WriteRune('\n')
	}
	return sb.String(), nil
}

// normalizeQR keeps the QR PNG large enough to display and scan comfortably:
// images wider than 512px are downscaled to 512, images smaller than 300px
// are upscaled by an integer factor (nearest-neighbor keeps modules sharp).
func normalizeQR(b64 string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", err
	}
	src, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	w := src.Bounds().Dx()
	switch {
	case w > 512:
		return resizeQR(b64, 512)
	case w < 300:
		factor := (300 + w - 1) / w
		dst := image.NewRGBA(image.Rect(0, 0, w*factor, src.Bounds().Dy()*factor))
		draw.NearestNeighbor.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
		var buf bytes.Buffer
		if err := png.Encode(&buf, dst); err != nil {
			return "", err
		}
		return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
	default:
		return b64, nil
	}
}

// resizeQR decodes a base64-encoded PNG, resizes it to maxSide pixels
// using nearest-neighbor (keeps QR sharp), and returns a new base64 string.
func resizeQR(b64 string, maxSide int) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", err
	}
	src, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= maxSide && h <= maxSide {
		return b64, nil // already small enough
	}
	// Scale proportionally
	nw, nh := maxSide, maxSide
	if w > h {
		nh = h * maxSide / w
	} else if h > w {
		nw = w * maxSide / h
	}
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	draw.NearestNeighbor.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)
	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
