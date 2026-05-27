// SPDX-License-Identifier: MIT
// Copyright (c) 2026 GREEN-API

// Package application defines the ports (interfaces) that the domain uses
// to communicate with external systems. Infrastructure adapters implement these.
// Follows the Ports & Adapters (hexagonal) architecture pattern,
// consistent with sw-pool-manager's internal/usecase/instance/types.go approach.
package application

import (
	"context"
	"encoding/json"
	"time"

	"github.com/green-api/green-api-mcp-gateway/internal/domain"
)

// CredentialStore provides access to Green API instance credentials.
// Infrastructure adapter: in-memory auth manager.
type CredentialStore interface {
	// AddInstance registers credentials for an instance.
	AddInstance(creds *domain.InstanceCredentials)

	// GetCredentials returns credentials for a given instance ID.
	GetCredentials(instanceID uint64) (*domain.InstanceCredentials, error)

	// ListInstances returns all registered instance IDs.
	ListInstances() []uint64

	// RemoveInstance removes credentials for an instance.
	RemoveInstance(instanceID uint64)
}

// WhatsAppClient is the primary port for interacting with the Green API.
// Infrastructure adapter: SDK-based client wrapping the official Go SDK.
type WhatsAppClient interface {
	// CallMethod sends a POST request with a JSON body to the given Green API method.
	CallMethod(ctx context.Context, instanceID uint64, method string, body interface{}) (json.RawMessage, error)

	// GetMethod sends a GET request to the given Green API method.
	GetMethod(ctx context.Context, instanceID uint64, method string) (json.RawMessage, error)

	// SendMessage sends a text message.
	SendMessage(ctx context.Context, instanceID uint64, req domain.SendMessageRequest) (*domain.SendMessageResponse, error)

	// SendFileByURL sends a file by URL.
	SendFileByURL(ctx context.Context, instanceID uint64, req domain.SendFileByURLRequest) (*domain.SendMessageResponse, error)

	// SendLocation sends a geolocation.
	SendLocation(ctx context.Context, instanceID uint64, req domain.SendLocationRequest) (*domain.SendMessageResponse, error)

	// SendContact sends a contact card.
	SendContact(ctx context.Context, instanceID uint64, req domain.SendContactRequest) (*domain.SendMessageResponse, error)

	// SendPoll sends a poll message.
	SendPoll(ctx context.Context, instanceID uint64, req domain.SendPollRequest) (*domain.SendMessageResponse, error)

	// ForwardMessages forwards one or more messages to another chat.
	ForwardMessages(ctx context.Context, instanceID uint64, req domain.ForwardMessagesRequest) (*domain.ForwardMessagesResponse, error)

	// EditMessage edits a previously sent message.
	EditMessage(ctx context.Context, instanceID uint64, req domain.EditMessageRequest) (*domain.EditMessageResponse, error)

	// DeleteMessage deletes a message.
	DeleteMessage(ctx context.Context, instanceID uint64, req domain.DeleteMessageRequest) error

	// GetStateInstance returns the current authorization state of the instance.
	GetStateInstance(ctx context.Context, instanceID uint64) (*domain.StateInstanceResponse, error)

	// GetSettings returns the settings of the instance.
	GetSettings(ctx context.Context, instanceID uint64) (*domain.SettingsResponse, error)

	// SetSettings updates the settings of the instance.
	SetSettings(ctx context.Context, instanceID uint64, req domain.SetSettingsRequest) (*domain.SetSettingsResponse, error)

	// GetQR retrieves the QR code for instance authorization.
	GetQR(ctx context.Context, instanceID uint64) (*domain.QRResponse, error)

	// CheckWhatsapp checks whether a phone number has an active WhatsApp account.
	CheckWhatsapp(ctx context.Context, instanceID uint64, req domain.CheckWhatsappRequest) (*domain.CheckWhatsappResponse, error)

	// GetContacts retrieves the full contacts list.
	GetContacts(ctx context.Context, instanceID uint64) (json.RawMessage, error)

	// GetContactInfo retrieves detailed information about a specific contact or group.
	GetContactInfo(ctx context.Context, instanceID uint64, req domain.GetContactInfoRequest) (*domain.ContactInfo, error)

	// ReceiveNotification long-polls for the next notification in the instance queue.
	ReceiveNotification(ctx context.Context, instanceID uint64) (*domain.NotificationBody, error)

	// DeleteNotification confirms receipt and removes a notification from the queue.
	DeleteNotification(ctx context.Context, instanceID uint64, receiptID int) error

	// CreateGroup creates a new WhatsApp group.
	CreateGroup(ctx context.Context, instanceID uint64, req domain.CreateGroupRequest) (*domain.CreateGroupResponse, error)

	// GetGroupData retrieves data about a group.
	GetGroupData(ctx context.Context, instanceID uint64, groupID string) (*domain.GroupData, error)

	// AddGroupParticipant adds a participant to a group.
	AddGroupParticipant(ctx context.Context, instanceID uint64, req domain.GroupParticipantRequest) (json.RawMessage, error)

	// RemoveGroupParticipant removes a participant from a group.
	RemoveGroupParticipant(ctx context.Context, instanceID uint64, req domain.GroupParticipantRequest) (json.RawMessage, error)

	// SetGroupAdmin grants admin rights to a group participant.
	SetGroupAdmin(ctx context.Context, instanceID uint64, req domain.GroupParticipantRequest) (json.RawMessage, error)

	// RemoveGroupAdmin revokes admin rights from a group participant.
	RemoveGroupAdmin(ctx context.Context, instanceID uint64, req domain.GroupParticipantRequest) (json.RawMessage, error)

	// LeaveGroup leaves a WhatsApp group.
	LeaveGroup(ctx context.Context, instanceID uint64, req domain.LeaveGroupRequest) (json.RawMessage, error)

	// GetChatHistory returns chat message history.
	GetChatHistory(ctx context.Context, instanceID uint64, req domain.GetChatHistoryRequest) (json.RawMessage, error)

	// GetMessage returns a specific message by ID.
	GetMessage(ctx context.Context, instanceID uint64, req domain.GetMessageRequest) (json.RawMessage, error)

	// LastIncomingMessages returns recent incoming messages.
	LastIncomingMessages(ctx context.Context, instanceID uint64, minutes int) (json.RawMessage, error)

	// LastOutgoingMessages returns recent outgoing messages.
	LastOutgoingMessages(ctx context.Context, instanceID uint64, minutes int) (json.RawMessage, error)

	// ReadChat marks messages in a chat as read.
	ReadChat(ctx context.Context, instanceID uint64, req domain.ReadChatRequest) (json.RawMessage, error)

	// GetAuthorizationCode retrieves an authorization code for phone-based login.
	GetAuthorizationCode(ctx context.Context, instanceID uint64, req domain.GetAuthorizationCodeRequest) (json.RawMessage, error)

	// GetContactAvatar retrieves the avatar URL of a contact.
	GetContactAvatar(ctx context.Context, instanceID uint64, req domain.GetAvatarRequest) (json.RawMessage, error)

	// UploadFile uploads a file to Green API and returns the hosted file URL.
	UploadFile(ctx context.Context, instanceID uint64, filename string, fileData []byte) (json.RawMessage, error)

	// SendFileByUpload sends a file directly (multipart upload) to a chat.
	SendFileByUpload(ctx context.Context, instanceID uint64, chatID, caption, filename string, fileData []byte) (json.RawMessage, error)
}

// MCPTransport is the port for serving MCP protocol.
// Infrastructure adapter: mcp-go based server.
type MCPTransport interface {
	// ServeStdio runs the MCP server over stdio transport (blocking).
	ServeStdio(ctx context.Context) error
}

// MetricsProvider is the port for recording observability metrics.
// Infrastructure adapter: Prometheus-based implementation in infrastructure/monitoring.
type MetricsProvider interface {
	// RecordToolCall increments the MCP tool call counter and records its duration.
	RecordToolCall(tool string, start time.Time, err error)

	// RecordAPIRequest increments the outbound Green API request counter and records latency.
	RecordAPIRequest(method string, start time.Time, err error)

	// SetActiveWebhookConnections updates the gauge of active webhook polling goroutines.
	SetActiveWebhookConnections(count float64)

	// RecordRateLimit increments the rate limited requests counter for the given instance.
	RecordRateLimit(instanceID uint64)
}

// WebhookBridge is the port for receiving Green API notifications in the
// background and forwarding them to interested parties (e.g. the MCP server).
// Infrastructure adapter: polling bridge in infrastructure/webhook.
type WebhookBridge interface {
	// Start launches the background polling goroutines. Returns an error only
	// if the bridge cannot initialise (e.g. no instances). Goroutines run until
	// ctx is cancelled or Stop is called.
	Start(ctx context.Context) error

	// Stop signals all goroutines to terminate and waits for them to finish.
	// Safe to call multiple times.
	Stop()
}
