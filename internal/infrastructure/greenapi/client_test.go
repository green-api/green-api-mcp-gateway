// SPDX-License-Identifier: MIT
// Copyright (c) 2026 GREEN-API

package greenapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/green-api/green-api-mcp-gateway/internal/domain"
	"github.com/green-api/green-api-mcp-gateway/internal/infrastructure"
	"github.com/green-api/green-api-mcp-gateway/internal/infrastructure/greenapi"
	"github.com/green-api/green-api-mcp-gateway/internal/infrastructure/monitoring"
)

const testInstanceID uint64 = 1234567890

// newClientWithServer creates an httptest.Server that responds with body, a
// CredentialManager pointing to that server, and a greenapi.Client.
// Call cleanup() to close the server.
func newClientWithServer(t *testing.T, statusCode int, body string) (*greenapi.Client, func()) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_, _ = w.Write([]byte(body))
	}))

	mgr := infrastructure.NewCredentialManager()
	mgr.AddInstance(&domain.InstanceCredentials{
		InstanceID: testInstanceID,
		APIToken:   "test-token",
		APIURL:     srv.URL,
	})

	// nil config → uses APIURL from credentials (pointing to test server).
	client := greenapi.NewClient(mgr, monitoring.NoopMetricsProvider{}, nil)
	return client, srv.Close
}

// newClientWithHandler allows full control over the handler.
func newClientWithHandler(t *testing.T, handler http.HandlerFunc) (*greenapi.Client, func()) {
	t.Helper()
	srv := httptest.NewServer(handler)

	mgr := infrastructure.NewCredentialManager()
	mgr.AddInstance(&domain.InstanceCredentials{
		InstanceID: testInstanceID,
		APIToken:   "test-token",
		APIURL:     srv.URL,
	})

	client := greenapi.NewClient(mgr, monitoring.NoopMetricsProvider{}, nil)
	return client, srv.Close
}

// ─── CredentialStore failures ──────────────────────────────────────────────────

func TestClient_UnknownInstance_ReturnsError(t *testing.T) {
	mgr := infrastructure.NewCredentialManager()
	client := greenapi.NewClient(mgr, monitoring.NoopMetricsProvider{}, nil)

	_, err := client.GetStateInstance(context.Background(), 99999)
	if err == nil {
		t.Fatal("expected error for unknown instance, got nil")
	}
}

// ─── CallMethod ───────────────────────────────────────────────────────────────

func TestClient_CallMethod_Success(t *testing.T) {
	want := `{"idMessage":"msg-001"}`
	client, cleanup := newClientWithServer(t, http.StatusOK, want)
	defer cleanup()

	raw, err := client.CallMethod(context.Background(), testInstanceID, domain.MethodSendMessage, map[string]any{
		"chatId":  "71234567890@c.us",
		"message": "hello",
	})
	if err != nil {
		t.Fatalf("CallMethod: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["idMessage"] != "msg-001" {
		t.Errorf("idMessage: got %v, want %q", got["idMessage"], "msg-001")
	}
}

func TestClient_CallMethod_ServerError(t *testing.T) {
	client, cleanup := newClientWithServer(t, http.StatusInternalServerError, "server error")
	defer cleanup()

	_, err := client.CallMethod(context.Background(), testInstanceID, "someMethod", nil)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

// ─── GetMethod ────────────────────────────────────────────────────────────────

func TestClient_GetMethod_Success(t *testing.T) {
	want := `{"stateInstance":"authorized"}`
	client, cleanup := newClientWithServer(t, http.StatusOK, want)
	defer cleanup()

	raw, err := client.GetMethod(context.Background(), testInstanceID, domain.MethodGetStateInstance)
	if err != nil {
		t.Fatalf("GetMethod: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["stateInstance"] != "authorized" {
		t.Errorf("stateInstance: got %v", got["stateInstance"])
	}
}

// ─── SendMessage ──────────────────────────────────────────────────────────────

func TestClient_SendMessage_Success(t *testing.T) {
	client, cleanup := newClientWithServer(t, http.StatusOK, `{"idMessage":"abc-123"}`)
	defer cleanup()

	resp, err := client.SendMessage(context.Background(), testInstanceID, domain.SendMessageRequest{
		ChatID:  "71234567890@c.us",
		Message: "Hello world",
	})
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if resp.IDMessage != "abc-123" {
		t.Errorf("IDMessage: got %q, want %q", resp.IDMessage, "abc-123")
	}
}

func TestClient_SendMessage_Error(t *testing.T) {
	client, cleanup := newClientWithServer(t, http.StatusBadRequest, `{"error":"bad request"}`)
	defer cleanup()

	_, err := client.SendMessage(context.Background(), testInstanceID, domain.SendMessageRequest{
		ChatID:  "71234567890@c.us",
		Message: "test",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ─── SendFileByURL ────────────────────────────────────────────────────────────

func TestClient_SendFileByURL_Success(t *testing.T) {
	client, cleanup := newClientWithServer(t, http.StatusOK, `{"idMessage":"file-456"}`)
	defer cleanup()

	resp, err := client.SendFileByURL(context.Background(), testInstanceID, domain.SendFileByURLRequest{
		ChatID:   "71234567890@c.us",
		URL:      "https://example.com/file.pdf",
		FileName: "file.pdf",
	})
	if err != nil {
		t.Fatalf("SendFileByURL: %v", err)
	}
	if resp.IDMessage != "file-456" {
		t.Errorf("IDMessage: got %q, want %q", resp.IDMessage, "file-456")
	}
}

// ─── SendLocation ─────────────────────────────────────────────────────────────

func TestClient_SendLocation_Success(t *testing.T) {
	client, cleanup := newClientWithServer(t, http.StatusOK, `{"idMessage":"loc-789"}`)
	defer cleanup()

	resp, err := client.SendLocation(context.Background(), testInstanceID, domain.SendLocationRequest{
		ChatID:    "71234567890@c.us",
		Latitude:  51.5074,
		Longitude: -0.1278,
		Name:      "London",
	})
	if err != nil {
		t.Fatalf("SendLocation: %v", err)
	}
	if resp.IDMessage != "loc-789" {
		t.Errorf("IDMessage: got %q, want %q", resp.IDMessage, "loc-789")
	}
}

// ─── SendContact ──────────────────────────────────────────────────────────────

func TestClient_SendContact_Success(t *testing.T) {
	client, cleanup := newClientWithServer(t, http.StatusOK, `{"idMessage":"contact-111"}`)
	defer cleanup()

	resp, err := client.SendContact(context.Background(), testInstanceID, domain.SendContactRequest{
		ChatID: "71234567890@c.us",
		Contact: domain.Contact{
			PhoneNumber: 79001234567,
			FirstName:   "John",
			LastName:    "Doe",
		},
	})
	if err != nil {
		t.Fatalf("SendContact: %v", err)
	}
	if resp.IDMessage != "contact-111" {
		t.Errorf("IDMessage: got %q, want %q", resp.IDMessage, "contact-111")
	}
}

// ─── SendPoll ─────────────────────────────────────────────────────────────────

func TestClient_SendPoll_Success(t *testing.T) {
	client, cleanup := newClientWithServer(t, http.StatusOK, `{"idMessage":"poll-222"}`)
	defer cleanup()

	resp, err := client.SendPoll(context.Background(), testInstanceID, domain.SendPollRequest{
		ChatID:  "71234567890@c.us",
		Message: "Vote?",
		Options: []domain.PollOption{{OptionName: "Yes"}, {OptionName: "No"}},
	})
	if err != nil {
		t.Fatalf("SendPoll: %v", err)
	}
	if resp.IDMessage != "poll-222" {
		t.Errorf("IDMessage: got %q, want %q", resp.IDMessage, "poll-222")
	}
}

// ─── ForwardMessages ──────────────────────────────────────────────────────────

func TestClient_ForwardMessages_Success(t *testing.T) {
	client, cleanup := newClientWithServer(t, http.StatusOK, `{"messages":["fwd-1","fwd-2"]}`)
	defer cleanup()

	resp, err := client.ForwardMessages(context.Background(), testInstanceID, domain.ForwardMessagesRequest{
		ChatID:     "71111111111@c.us",
		ChatIDFrom: "72222222222@c.us",
		Messages:   []string{"msg-a", "msg-b"},
	})
	if err != nil {
		t.Fatalf("ForwardMessages: %v", err)
	}
	if len(resp.Messages) != 2 {
		t.Errorf("Messages count: got %d, want %d", len(resp.Messages), 2)
	}
}

// ─── EditMessage ──────────────────────────────────────────────────────────────

func TestClient_EditMessage_Success(t *testing.T) {
	client, cleanup := newClientWithServer(t, http.StatusOK, `{"idMessage":"edited-333"}`)
	defer cleanup()

	resp, err := client.EditMessage(context.Background(), testInstanceID, domain.EditMessageRequest{
		ChatID:    "71234567890@c.us",
		IDMessage: "old-msg",
		Message:   "updated text",
	})
	if err != nil {
		t.Fatalf("EditMessage: %v", err)
	}
	if resp.IDMessage != "edited-333" {
		t.Errorf("IDMessage: got %q, want %q", resp.IDMessage, "edited-333")
	}
}

// ─── DeleteMessage ────────────────────────────────────────────────────────────

func TestClient_DeleteMessage_Success(t *testing.T) {
	// The SDK's deleteMessage returns a POST response; we just need status 200 + a JSON object.
	client, cleanup := newClientWithServer(t, http.StatusOK, `{}`)
	defer cleanup()

	err := client.DeleteMessage(context.Background(), testInstanceID, domain.DeleteMessageRequest{
		ChatID:    "71234567890@c.us",
		IDMessage: "msg-to-delete",
	})
	if err != nil {
		t.Fatalf("DeleteMessage: %v", err)
	}
}

func TestClient_DeleteMessage_Error(t *testing.T) {
	client, cleanup := newClientWithServer(t, http.StatusForbidden, `{"error":"forbidden"}`)
	defer cleanup()

	err := client.DeleteMessage(context.Background(), testInstanceID, domain.DeleteMessageRequest{
		ChatID:    "71234567890@c.us",
		IDMessage: "x",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ─── GetStateInstance ─────────────────────────────────────────────────────────

func TestClient_GetStateInstance_Authorized(t *testing.T) {
	client, cleanup := newClientWithServer(t, http.StatusOK, `{"stateInstance":"authorized"}`)
	defer cleanup()

	resp, err := client.GetStateInstance(context.Background(), testInstanceID)
	if err != nil {
		t.Fatalf("GetStateInstance: %v", err)
	}
	if resp.StateInstance != "authorized" {
		t.Errorf("StateInstance: got %q, want %q", resp.StateInstance, "authorized")
	}
}

func TestClient_GetStateInstance_NotAuthorized(t *testing.T) {
	client, cleanup := newClientWithServer(t, http.StatusOK, `{"stateInstance":"notAuthorized"}`)
	defer cleanup()

	resp, err := client.GetStateInstance(context.Background(), testInstanceID)
	if err != nil {
		t.Fatalf("GetStateInstance: %v", err)
	}
	if resp.StateInstance != "notAuthorized" {
		t.Errorf("StateInstance: got %q, want %q", resp.StateInstance, "notAuthorized")
	}
}

// ─── GetSettings ──────────────────────────────────────────────────────────────

func TestClient_GetSettings_Success(t *testing.T) {
	body := `{"webhookUrl":"https://example.com/hook","incomingWebhook":"yes"}`
	client, cleanup := newClientWithServer(t, http.StatusOK, body)
	defer cleanup()

	resp, err := client.GetSettings(context.Background(), testInstanceID)
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if resp.WebhookURL != "https://example.com/hook" {
		t.Errorf("WebhookURL: got %q", resp.WebhookURL)
	}
	if resp.IncomingWebhook != "yes" {
		t.Errorf("IncomingWebhook: got %q", resp.IncomingWebhook)
	}
}

// ─── SetSettings ──────────────────────────────────────────────────────────────

func TestClient_SetSettings_Success(t *testing.T) {
	client, cleanup := newClientWithServer(t, http.StatusOK, `{"saveSettings":"true"}`)
	defer cleanup()

	resp, err := client.SetSettings(context.Background(), testInstanceID, domain.SetSettingsRequest{
		WebhookURL: "https://example.com/new-hook",
	})
	if err != nil {
		t.Fatalf("SetSettings: %v", err)
	}
	if resp.SavedSettings != "true" {
		t.Errorf("SavedSettings: got %q, want %q", resp.SavedSettings, "true")
	}
}

// ─── GetQR ────────────────────────────────────────────────────────────────────

func TestClient_GetQR_Success(t *testing.T) {
	client, cleanup := newClientWithServer(t, http.StatusOK, `{"type":"qrCode","message":"base64data"}`)
	defer cleanup()

	resp, err := client.GetQR(context.Background(), testInstanceID)
	if err != nil {
		t.Fatalf("GetQR: %v", err)
	}
	if resp.Type != "qrCode" {
		t.Errorf("Type: got %q, want %q", resp.Type, "qrCode")
	}
	if resp.Message != "base64data" {
		t.Errorf("Message: got %q, want %q", resp.Message, "base64data")
	}
}

// ─── CheckWhatsapp ────────────────────────────────────────────────────────────

func TestClient_CheckWhatsapp_Exists(t *testing.T) {
	client, cleanup := newClientWithServer(t, http.StatusOK, `{"existsWhatsapp":true}`)
	defer cleanup()

	resp, err := client.CheckWhatsapp(context.Background(), testInstanceID, domain.CheckWhatsappRequest{
		PhoneNumber: 79001234567,
	})
	if err != nil {
		t.Fatalf("CheckWhatsapp: %v", err)
	}
	if !resp.ExistsWhatsapp {
		t.Error("ExistsWhatsapp: expected true")
	}
}

func TestClient_CheckWhatsapp_NotExists(t *testing.T) {
	client, cleanup := newClientWithServer(t, http.StatusOK, `{"existsWhatsapp":false}`)
	defer cleanup()

	resp, err := client.CheckWhatsapp(context.Background(), testInstanceID, domain.CheckWhatsappRequest{
		PhoneNumber: 79000000000,
	})
	if err != nil {
		t.Fatalf("CheckWhatsapp: %v", err)
	}
	if resp.ExistsWhatsapp {
		t.Error("ExistsWhatsapp: expected false")
	}
}

// ─── GetContacts ──────────────────────────────────────────────────────────────

func TestClient_GetContacts_Success(t *testing.T) {
	// SDK uses ArrayRequest for GetContacts → returns []any → server must return JSON array.
	body := `[{"id":"71234567890@c.us","name":"Alice"},{"id":"79876543210@c.us","name":"Bob"}]`
	client, cleanup := newClientWithServer(t, http.StatusOK, body)
	defer cleanup()

	raw, err := client.GetContacts(context.Background(), testInstanceID)
	if err != nil {
		t.Fatalf("GetContacts: %v", err)
	}

	var contacts []map[string]any
	if err := json.Unmarshal(raw, &contacts); err != nil {
		t.Fatalf("unmarshal contacts: %v", err)
	}
	if len(contacts) != 2 {
		t.Errorf("expected 2 contacts, got %d", len(contacts))
	}
	if contacts[0]["name"] != "Alice" {
		t.Errorf("first contact name: got %v", contacts[0]["name"])
	}
}

func TestClient_GetContacts_Empty(t *testing.T) {
	client, cleanup := newClientWithServer(t, http.StatusOK, `[]`)
	defer cleanup()

	raw, err := client.GetContacts(context.Background(), testInstanceID)
	if err != nil {
		t.Fatalf("GetContacts: %v", err)
	}
	var contacts []map[string]any
	if err := json.Unmarshal(raw, &contacts); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(contacts) != 0 {
		t.Errorf("expected 0 contacts, got %d", len(contacts))
	}
}

// ─── GetContactInfo ───────────────────────────────────────────────────────────

func TestClient_GetContactInfo_Success(t *testing.T) {
	body := `{"avatar":"https://example.com/avatar.jpg","name":"Alice Smith","isGroup":false}`
	client, cleanup := newClientWithServer(t, http.StatusOK, body)
	defer cleanup()

	resp, err := client.GetContactInfo(context.Background(), testInstanceID, domain.GetContactInfoRequest{
		ChatID: "71234567890@c.us",
	})
	if err != nil {
		t.Fatalf("GetContactInfo: %v", err)
	}
	if resp.Name != "Alice Smith" {
		t.Errorf("Name: got %q", resp.Name)
	}
	if resp.IsGroup {
		t.Error("IsGroup: expected false")
	}
}

// ─── ReceiveNotification ──────────────────────────────────────────────────────

func TestClient_ReceiveNotification_HasNotification(t *testing.T) {
	body := `{"receiptId":42,"body":{"typeWebhook":"incomingMessageReceived"}}`
	client, cleanup := newClientWithServer(t, http.StatusOK, body)
	defer cleanup()

	notif, err := client.ReceiveNotification(context.Background(), testInstanceID)
	if err != nil {
		t.Fatalf("ReceiveNotification: %v", err)
	}
	if notif == nil {
		t.Fatal("expected notification, got nil")
		return
	}
	if notif.ReceiptID != 42 {
		t.Errorf("ReceiptID: got %d, want %d", notif.ReceiptID, 42)
	}
}

func TestClient_ReceiveNotification_Empty(t *testing.T) {
	// Empty response body → SDK RawRequest returns nil → client returns nil, nil.
	client, cleanup := newClientWithServer(t, http.StatusOK, ``)
	defer cleanup()

	notif, err := client.ReceiveNotification(context.Background(), testInstanceID)
	if err != nil {
		t.Fatalf("ReceiveNotification: %v", err)
	}
	if notif != nil {
		t.Errorf("expected nil notification, got %+v", notif)
	}
}

// ─── DeleteNotification ───────────────────────────────────────────────────────

func TestClient_DeleteNotification_Success(t *testing.T) {
	// SDK DeleteNotification uses Request("DELETE", ...) → returns map[string]any.
	client, cleanup := newClientWithServer(t, http.StatusOK, `{"result":true}`)
	defer cleanup()

	err := client.DeleteNotification(context.Background(), testInstanceID, 42)
	if err != nil {
		t.Fatalf("DeleteNotification: %v", err)
	}
}

func TestClient_DeleteNotification_Error(t *testing.T) {
	client, cleanup := newClientWithServer(t, http.StatusNotFound, `{"error":"not found"}`)
	defer cleanup()

	err := client.DeleteNotification(context.Background(), testInstanceID, 99)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ─── Request body validation ──────────────────────────────────────────────────

func TestClient_SendMessage_RequestBody(t *testing.T) {
	var received []byte
	client, cleanup := newClientWithHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		var err error
		received = make([]byte, r.ContentLength)
		_, err = r.Body.Read(received)
		_ = err
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"idMessage":"test-123"}`))
	})
	defer cleanup()

	_, err := client.SendMessage(context.Background(), testInstanceID, domain.SendMessageRequest{
		ChatID:  "71234567890@c.us",
		Message: "body check",
	})
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
}
