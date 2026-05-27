// SPDX-License-Identifier: MIT
// Copyright (c) 2026 GREEN-API

// Package application_test verifies that the port interfaces are implementable
// via mock structs. This ensures interface contracts are stable.
package application_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/green-api/green-api-mcp-gateway/internal/application"
	"github.com/green-api/green-api-mcp-gateway/internal/domain"
)

// ─── MockCredentialStore ──────────────────────────────────────────────────────

type mockCredentialStore struct {
	instances map[uint64]*domain.InstanceCredentials
}

func newMockCredentialStore() *mockCredentialStore {
	return &mockCredentialStore{instances: make(map[uint64]*domain.InstanceCredentials)}
}

func (m *mockCredentialStore) AddInstance(creds *domain.InstanceCredentials) {
	m.instances[creds.InstanceID] = creds
}

func (m *mockCredentialStore) GetCredentials(id uint64) (*domain.InstanceCredentials, error) {
	c, ok := m.instances[id]
	if !ok {
		return nil, domain.ErrInstanceNotFound
	}
	return c, nil
}

func (m *mockCredentialStore) ListInstances() []uint64 {
	ids := make([]uint64, 0, len(m.instances))
	for id := range m.instances {
		ids = append(ids, id)
	}
	return ids
}

func (m *mockCredentialStore) RemoveInstance(id uint64) {
	delete(m.instances, id)
}

// Compile-time interface check.
var _ application.CredentialStore = (*mockCredentialStore)(nil)

func TestMockCredentialStore_Implements(t *testing.T) {
	store := newMockCredentialStore()

	store.AddInstance(&domain.InstanceCredentials{
		InstanceID: 100,
		APIToken:   "test-token",
		APIURL:     "https://api.green-api.com",
	})

	creds, err := store.GetCredentials(100)
	if err != nil {
		t.Fatalf("GetCredentials: %v", err)
	}
	if creds.APIToken != "test-token" {
		t.Errorf("APIToken: got %q", creds.APIToken)
	}

	ids := store.ListInstances()
	if len(ids) != 1 || ids[0] != 100 {
		t.Errorf("ListInstances: got %v", ids)
	}

	store.RemoveInstance(100)
	_, err = store.GetCredentials(100)
	if err != domain.ErrInstanceNotFound {
		t.Errorf("expected ErrInstanceNotFound, got: %v", err)
	}
}

// ─── MockWhatsAppClient ───────────────────────────────────────────────────────

type mockWhatsAppClient struct{}

func (m *mockWhatsAppClient) CallMethod(_ context.Context, _ uint64, _ string, _ interface{}) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}
func (m *mockWhatsAppClient) GetMethod(_ context.Context, _ uint64, _ string) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}
func (m *mockWhatsAppClient) SendMessage(_ context.Context, _ uint64, _ domain.SendMessageRequest) (*domain.SendMessageResponse, error) {
	return &domain.SendMessageResponse{IDMessage: "mock-msg"}, nil
}
func (m *mockWhatsAppClient) SendFileByURL(_ context.Context, _ uint64, _ domain.SendFileByURLRequest) (*domain.SendMessageResponse, error) {
	return &domain.SendMessageResponse{IDMessage: "mock-file"}, nil
}
func (m *mockWhatsAppClient) SendLocation(_ context.Context, _ uint64, _ domain.SendLocationRequest) (*domain.SendMessageResponse, error) {
	return &domain.SendMessageResponse{IDMessage: "mock-loc"}, nil
}
func (m *mockWhatsAppClient) SendContact(_ context.Context, _ uint64, _ domain.SendContactRequest) (*domain.SendMessageResponse, error) {
	return &domain.SendMessageResponse{IDMessage: "mock-contact"}, nil
}
func (m *mockWhatsAppClient) SendPoll(_ context.Context, _ uint64, _ domain.SendPollRequest) (*domain.SendMessageResponse, error) {
	return &domain.SendMessageResponse{IDMessage: "mock-poll"}, nil
}
func (m *mockWhatsAppClient) ForwardMessages(_ context.Context, _ uint64, _ domain.ForwardMessagesRequest) (*domain.ForwardMessagesResponse, error) {
	return &domain.ForwardMessagesResponse{Messages: []string{"fwd-1"}}, nil
}
func (m *mockWhatsAppClient) EditMessage(_ context.Context, _ uint64, _ domain.EditMessageRequest) (*domain.EditMessageResponse, error) {
	return &domain.EditMessageResponse{IDMessage: "mock-edit"}, nil
}
func (m *mockWhatsAppClient) DeleteMessage(_ context.Context, _ uint64, _ domain.DeleteMessageRequest) error {
	return nil
}
func (m *mockWhatsAppClient) GetStateInstance(_ context.Context, _ uint64) (*domain.StateInstanceResponse, error) {
	return &domain.StateInstanceResponse{StateInstance: "authorized"}, nil
}
func (m *mockWhatsAppClient) GetSettings(_ context.Context, _ uint64) (*domain.SettingsResponse, error) {
	return &domain.SettingsResponse{}, nil
}
func (m *mockWhatsAppClient) SetSettings(_ context.Context, _ uint64, _ domain.SetSettingsRequest) (*domain.SetSettingsResponse, error) {
	return &domain.SetSettingsResponse{SavedSettings: "true"}, nil
}
func (m *mockWhatsAppClient) GetQR(_ context.Context, _ uint64) (*domain.QRResponse, error) {
	return &domain.QRResponse{Type: "qrCode", Message: "base64"}, nil
}
func (m *mockWhatsAppClient) CheckWhatsapp(_ context.Context, _ uint64, _ domain.CheckWhatsappRequest) (*domain.CheckWhatsappResponse, error) {
	return &domain.CheckWhatsappResponse{ExistsWhatsapp: true}, nil
}
func (m *mockWhatsAppClient) GetContacts(_ context.Context, _ uint64) (json.RawMessage, error) {
	return json.RawMessage(`[]`), nil
}
func (m *mockWhatsAppClient) GetContactInfo(_ context.Context, _ uint64, _ domain.GetContactInfoRequest) (*domain.ContactInfo, error) {
	return &domain.ContactInfo{Name: "Mock"}, nil
}
func (m *mockWhatsAppClient) ReceiveNotification(_ context.Context, _ uint64) (*domain.NotificationBody, error) {
	return nil, nil
}
func (m *mockWhatsAppClient) DeleteNotification(_ context.Context, _ uint64, _ int) error {
	return nil
}
func (m *mockWhatsAppClient) CreateGroup(_ context.Context, _ uint64, _ domain.CreateGroupRequest) (*domain.CreateGroupResponse, error) {
	return &domain.CreateGroupResponse{Created: true, ChatID: "group@g.us"}, nil
}
func (m *mockWhatsAppClient) GetGroupData(_ context.Context, _ uint64, _ string) (*domain.GroupData, error) {
	return &domain.GroupData{Subject: "Test"}, nil
}
func (m *mockWhatsAppClient) AddGroupParticipant(_ context.Context, _ uint64, _ domain.GroupParticipantRequest) (json.RawMessage, error) {
	return json.RawMessage(`{"addParticipant":true}`), nil
}
func (m *mockWhatsAppClient) RemoveGroupParticipant(_ context.Context, _ uint64, _ domain.GroupParticipantRequest) (json.RawMessage, error) {
	return json.RawMessage(`{"removeParticipant":true}`), nil
}
func (m *mockWhatsAppClient) SetGroupAdmin(_ context.Context, _ uint64, _ domain.GroupParticipantRequest) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}
func (m *mockWhatsAppClient) RemoveGroupAdmin(_ context.Context, _ uint64, _ domain.GroupParticipantRequest) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}
func (m *mockWhatsAppClient) LeaveGroup(_ context.Context, _ uint64, _ domain.LeaveGroupRequest) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}
func (m *mockWhatsAppClient) GetChatHistory(_ context.Context, _ uint64, _ domain.GetChatHistoryRequest) (json.RawMessage, error) {
	return json.RawMessage(`[]`), nil
}
func (m *mockWhatsAppClient) GetMessage(_ context.Context, _ uint64, _ domain.GetMessageRequest) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}
func (m *mockWhatsAppClient) LastIncomingMessages(_ context.Context, _ uint64, _ int) (json.RawMessage, error) {
	return json.RawMessage(`[]`), nil
}
func (m *mockWhatsAppClient) LastOutgoingMessages(_ context.Context, _ uint64, _ int) (json.RawMessage, error) {
	return json.RawMessage(`[]`), nil
}
func (m *mockWhatsAppClient) ReadChat(_ context.Context, _ uint64, _ domain.ReadChatRequest) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}
func (m *mockWhatsAppClient) GetAuthorizationCode(_ context.Context, _ uint64, _ domain.GetAuthorizationCodeRequest) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}
func (m *mockWhatsAppClient) GetContactAvatar(_ context.Context, _ uint64, _ domain.GetAvatarRequest) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}

func (m *mockWhatsAppClient) UploadFile(_ context.Context, _ uint64, _ string, _ []byte) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}

func (m *mockWhatsAppClient) SendFileByUpload(_ context.Context, _ uint64, _, _, _ string, _ []byte) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}

// Compile-time check.
var _ application.WhatsAppClient = (*mockWhatsAppClient)(nil)

func TestMockWhatsAppClient_Implements(t *testing.T) {
	client := &mockWhatsAppClient{}
	ctx := context.Background()

	resp, err := client.SendMessage(ctx, 1, domain.SendMessageRequest{ChatID: "test@c.us", Message: "hi"})
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if resp.IDMessage != "mock-msg" {
		t.Errorf("IDMessage: got %q", resp.IDMessage)
	}

	state, err := client.GetStateInstance(ctx, 1)
	if err != nil {
		t.Fatalf("GetStateInstance: %v", err)
	}
	if state.StateInstance != "authorized" {
		t.Errorf("StateInstance: got %q", state.StateInstance)
	}
}

// ─── MockMCPTransport ─────────────────────────────────────────────────────────

type mockMCPTransport struct{}

func (m *mockMCPTransport) ServeStdio(_ context.Context) error { return nil }

var _ application.MCPTransport = (*mockMCPTransport)(nil)

func TestMockMCPTransport_Implements(t *testing.T) {
	transport := &mockMCPTransport{}
	err := transport.ServeStdio(context.Background())
	if err != nil {
		t.Errorf("ServeStdio: unexpected error: %v", err)
	}
}

// ─── MockMetricsProvider ──────────────────────────────────────────────────────

type mockMetricsProvider struct {
	toolCalls int
	apiCalls  int
}

func (m *mockMetricsProvider) RecordToolCall(_ string, _ time.Time, _ error)   { m.toolCalls++ }
func (m *mockMetricsProvider) RecordAPIRequest(_ string, _ time.Time, _ error) { m.apiCalls++ }
func (m *mockMetricsProvider) SetActiveWebhookConnections(_ float64)           {}
func (m *mockMetricsProvider) RecordRateLimit(_ uint64)                        {}

var _ application.MetricsProvider = (*mockMetricsProvider)(nil)

func TestMockMetricsProvider_Implements(t *testing.T) {
	mp := &mockMetricsProvider{}
	mp.RecordToolCall("test_tool", time.Now(), nil)
	mp.RecordAPIRequest("sendMessage", time.Now(), nil)
	mp.SetActiveWebhookConnections(5)
	if mp.toolCalls != 1 {
		t.Errorf("toolCalls: got %d, want 1", mp.toolCalls)
	}
	if mp.apiCalls != 1 {
		t.Errorf("apiCalls: got %d, want 1", mp.apiCalls)
	}
}

// ─── MockWebhookBridge ────────────────────────────────────────────────────────

type mockWebhookBridge struct {
	started bool
	stopped bool
}

func (m *mockWebhookBridge) Start(_ context.Context) error {
	m.started = true
	return nil
}

func (m *mockWebhookBridge) Stop() {
	m.stopped = true
}

var _ application.WebhookBridge = (*mockWebhookBridge)(nil)

func TestMockWebhookBridge_Implements(t *testing.T) {
	bridge := &mockWebhookBridge{}
	if err := bridge.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !bridge.started {
		t.Error("expected started=true")
	}
	bridge.Stop()
	if !bridge.stopped {
		t.Error("expected stopped=true")
	}
}
