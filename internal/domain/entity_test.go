// SPDX-License-Identifier: MIT
// Copyright (c) 2026 GREEN-API

package domain

import (
	"encoding/json"
	"testing"
)

// ─── SendMessageRequest ───────────────────────────────────────────────────────

func TestSendMessageRequest_JSON(t *testing.T) {
	req := SendMessageRequest{
		ChatID:          "71234567890@c.us",
		Message:         "Hello",
		QuotedMessageID: "msg-123",
		LinkPreview:     true,
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got SendMessageRequest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.ChatID != req.ChatID {
		t.Errorf("ChatID: got %q, want %q", got.ChatID, req.ChatID)
	}
	if got.Message != req.Message {
		t.Errorf("Message: got %q, want %q", got.Message, req.Message)
	}
	if got.QuotedMessageID != req.QuotedMessageID {
		t.Errorf("QuotedMessageID: got %q, want %q", got.QuotedMessageID, req.QuotedMessageID)
	}
	if got.LinkPreview != req.LinkPreview {
		t.Errorf("LinkPreview: got %v, want %v", got.LinkPreview, req.LinkPreview)
	}
}

func TestSendMessageRequest_OmitEmpty(t *testing.T) {
	req := SendMessageRequest{
		ChatID:  "71234567890@c.us",
		Message: "Hi",
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := m["quotedMessageId"]; ok {
		t.Error("quotedMessageId should be omitted when empty")
	}
	if _, ok := m["linkPreview"]; ok {
		t.Error("linkPreview should be omitted when false")
	}
}

// ─── SendMessageResponse ──────────────────────────────────────────────────────

func TestSendMessageResponse_JSON(t *testing.T) {
	raw := `{"idMessage":"abc-123"}`
	var resp SendMessageResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.IDMessage != "abc-123" {
		t.Errorf("IDMessage: got %q, want %q", resp.IDMessage, "abc-123")
	}
}

// ─── SendFileByURLRequest ─────────────────────────────────────────────────────

func TestSendFileByURLRequest_JSON(t *testing.T) {
	req := SendFileByURLRequest{
		ChatID:   "71234567890@c.us",
		URL:      "https://example.com/file.pdf",
		FileName: "file.pdf",
		Caption:  "Important document",
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["urlFile"] != "https://example.com/file.pdf" {
		t.Errorf("urlFile: got %v", m["urlFile"])
	}
}

// ─── SendLocationRequest ──────────────────────────────────────────────────────

func TestSendLocationRequest_JSON(t *testing.T) {
	req := SendLocationRequest{
		ChatID:    "71234567890@c.us",
		Latitude:  51.5074,
		Longitude: -0.1278,
		Name:      "London",
		Address:   "UK",
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got SendLocationRequest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Latitude != 51.5074 {
		t.Errorf("Latitude: got %f, want %f", got.Latitude, 51.5074)
	}
	if got.Longitude != -0.1278 {
		t.Errorf("Longitude: got %f, want %f", got.Longitude, -0.1278)
	}
}

// ─── Contact & SendContactRequest ─────────────────────────────────────────────

func TestContact_JSON(t *testing.T) {
	c := Contact{
		PhoneNumber: 79001234567,
		FirstName:   "John",
		LastName:    "Doe",
		Company:     "Acme",
	}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got Contact
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.PhoneNumber != 79001234567 {
		t.Errorf("PhoneNumber: got %d", got.PhoneNumber)
	}
	if got.FirstName != "John" {
		t.Errorf("FirstName: got %q", got.FirstName)
	}
}

// ─── PollOption & SendPollRequest ─────────────────────────────────────────────

func TestSendPollRequest_JSON(t *testing.T) {
	req := SendPollRequest{
		ChatID:          "71234567890@c.us",
		Message:         "Vote?",
		Options:         []PollOption{{OptionName: "Yes"}, {OptionName: "No"}},
		MultipleAnswers: true,
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got SendPollRequest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got.Options) != 2 {
		t.Fatalf("Options: got %d, want 2", len(got.Options))
	}
	if got.Options[0].OptionName != "Yes" {
		t.Errorf("Options[0]: got %q", got.Options[0].OptionName)
	}
	if !got.MultipleAnswers {
		t.Error("MultipleAnswers: expected true")
	}
}

// ─── ForwardMessagesRequest/Response ──────────────────────────────────────────

func TestForwardMessagesRequest_JSON(t *testing.T) {
	req := ForwardMessagesRequest{
		ChatID:     "71111111111@c.us",
		ChatIDFrom: "72222222222@c.us",
		Messages:   []string{"msg-1", "msg-2"},
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got ForwardMessagesRequest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got.Messages) != 2 {
		t.Errorf("Messages count: got %d, want 2", len(got.Messages))
	}
}

// ─── EditMessageRequest/Response ──────────────────────────────────────────────

func TestEditMessageRequest_JSON(t *testing.T) {
	req := EditMessageRequest{
		ChatID:    "71234567890@c.us",
		IDMessage: "msg-old",
		Message:   "updated text",
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got EditMessageRequest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.IDMessage != "msg-old" {
		t.Errorf("IDMessage: got %q", got.IDMessage)
	}
}

// ─── DeleteMessageRequest ─────────────────────────────────────────────────────

func TestDeleteMessageRequest_JSON(t *testing.T) {
	req := DeleteMessageRequest{
		ChatID:    "71234567890@c.us",
		IDMessage: "msg-del",
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["chatId"] != "71234567890@c.us" {
		t.Errorf("chatId: got %v", m["chatId"])
	}
	if m["idMessage"] != "msg-del" {
		t.Errorf("idMessage: got %v", m["idMessage"])
	}
}

// ─── InstanceCredentials ──────────────────────────────────────────────────────

func TestInstanceCredentials_Fields(t *testing.T) {
	creds := InstanceCredentials{
		InstanceID: 12345,
		APIToken:   "token-abc",
		APIURL:     "https://api.green-api.com",
	}
	if creds.InstanceID != 12345 {
		t.Errorf("InstanceID: got %d", creds.InstanceID)
	}
	if creds.APIToken != "token-abc" {
		t.Errorf("APIToken: got %q", creds.APIToken)
	}
	if creds.APIURL != "https://api.green-api.com" {
		t.Errorf("APIURL: got %q", creds.APIURL)
	}
}

// ─── StateInstanceResponse ────────────────────────────────────────────────────

func TestStateInstanceResponse_JSON(t *testing.T) {
	raw := `{"stateInstance":"authorized"}`
	var resp StateInstanceResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.StateInstance != "authorized" {
		t.Errorf("StateInstance: got %q", resp.StateInstance)
	}
}

// ─── SettingsResponse ─────────────────────────────────────────────────────────

func TestSettingsResponse_JSON(t *testing.T) {
	raw := `{"webhookUrl":"https://example.com/hook","delaySendMessagesMilliseconds":500,"markIncomingMessagesReaded":true}`
	var resp SettingsResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.WebhookURL != "https://example.com/hook" {
		t.Errorf("WebhookURL: got %q", resp.WebhookURL)
	}
	if resp.DelaySendMessagesMilliseconds != 500 {
		t.Errorf("Delay: got %d", resp.DelaySendMessagesMilliseconds)
	}
	if !resp.MarkIncomingMessagesReaded {
		t.Error("MarkIncomingMessagesReaded: expected true")
	}
}

// ─── SetSettingsRequest ───────────────────────────────────────────────────────

func TestSetSettingsRequest_OmitEmpty(t *testing.T) {
	req := SetSettingsRequest{
		WebhookURL: "https://example.com/hook",
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Only webhookUrl should be present
	if _, ok := m["webhookUrl"]; !ok {
		t.Error("webhookUrl should be present")
	}
	if _, ok := m["proxyHost"]; ok {
		t.Error("proxyHost should be omitted")
	}
}

// ─── QRResponse ───────────────────────────────────────────────────────────────

func TestQRResponse_JSON(t *testing.T) {
	raw := `{"type":"qrCode","message":"base64data"}`
	var resp QRResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Type != "qrCode" {
		t.Errorf("Type: got %q", resp.Type)
	}
	if resp.Message != "base64data" {
		t.Errorf("Message: got %q", resp.Message)
	}
}

// ─── CheckWhatsapp ────────────────────────────────────────────────────────────

func TestCheckWhatsappResponse_JSON(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{"exists", `{"existsWhatsapp":true}`, true},
		{"not exists", `{"existsWhatsapp":false}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp CheckWhatsappResponse
			if err := json.Unmarshal([]byte(tt.raw), &resp); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if resp.ExistsWhatsapp != tt.want {
				t.Errorf("ExistsWhatsapp: got %v, want %v", resp.ExistsWhatsapp, tt.want)
			}
		})
	}
}

// ─── ContactInfo ──────────────────────────────────────────────────────────────

func TestContactInfo_JSON(t *testing.T) {
	raw := `{"avatar":"https://img.example.com/a.jpg","name":"Alice","description":"Dev","isGroup":false}`
	var info ContactInfo
	if err := json.Unmarshal([]byte(raw), &info); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if info.Name != "Alice" {
		t.Errorf("Name: got %q", info.Name)
	}
	if info.Description != "Dev" {
		t.Errorf("Description: got %q", info.Description)
	}
	if info.IsGroup {
		t.Error("IsGroup: expected false")
	}
}

// ─── NotificationBody ─────────────────────────────────────────────────────────

func TestNotificationBody_JSON(t *testing.T) {
	raw := `{"receiptId":42,"body":{"typeWebhook":"incomingMessageReceived","senderData":{"chatId":"71234567890@c.us"}}}`
	var notif NotificationBody
	if err := json.Unmarshal([]byte(raw), &notif); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if notif.ReceiptID != 42 {
		t.Errorf("ReceiptID: got %d", notif.ReceiptID)
	}
	if notif.Body == nil {
		t.Fatal("Body should not be nil")
	}
	// Body is json.RawMessage — verify it can be further unmarshalled.
	var body map[string]interface{}
	if err := json.Unmarshal(notif.Body, &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body["typeWebhook"] != "incomingMessageReceived" {
		t.Errorf("typeWebhook: got %v", body["typeWebhook"])
	}
}

// ─── Group entities ───────────────────────────────────────────────────────────

func TestCreateGroupRequest_JSON(t *testing.T) {
	req := CreateGroupRequest{
		GroupName: "Test Group",
		ChatIDs:   []string{"71111111111@c.us", "72222222222@c.us"},
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got CreateGroupRequest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.GroupName != "Test Group" {
		t.Errorf("GroupName: got %q", got.GroupName)
	}
	if len(got.ChatIDs) != 2 {
		t.Errorf("ChatIDs count: got %d", len(got.ChatIDs))
	}
}

func TestCreateGroupResponse_JSON(t *testing.T) {
	raw := `{"created":true,"chatId":"120363001234567890@g.us"}`
	var resp CreateGroupResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !resp.Created {
		t.Error("Created: expected true")
	}
	if resp.ChatID != "120363001234567890@g.us" {
		t.Errorf("ChatID: got %q", resp.ChatID)
	}
}

func TestGroupData_JSON(t *testing.T) {
	raw := `{
		"groupId":"120363001234567890@g.us",
		"owner":"71234567890@c.us",
		"subject":"Dev Team",
		"creation":1680000000,
		"participants":[
			{"id":"71234567890@c.us","isAdmin":true,"isSuperAdmin":true},
			{"id":"79876543210@c.us","isAdmin":false,"isSuperAdmin":false}
		]
	}`
	var gd GroupData
	if err := json.Unmarshal([]byte(raw), &gd); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if gd.Subject != "Dev Team" {
		t.Errorf("Subject: got %q", gd.Subject)
	}
	if len(gd.Participants) != 2 {
		t.Fatalf("Participants: got %d, want 2", len(gd.Participants))
	}
	if !gd.Participants[0].IsAdmin {
		t.Error("first participant should be admin")
	}
	if !gd.Participants[0].IsSuperAdmin {
		t.Error("first participant should be super admin")
	}
}

func TestGroupParticipantRequest_JSON(t *testing.T) {
	req := GroupParticipantRequest{
		GroupID:           "120363001234567890@g.us",
		ParticipantChatID: "71234567890@c.us",
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["groupId"] != "120363001234567890@g.us" {
		t.Errorf("groupId: got %v", m["groupId"])
	}
	if m["participantChatId"] != "71234567890@c.us" {
		t.Errorf("participantChatId: got %v", m["participantChatId"])
	}
}

func TestGroupParticipantResponse_Added(t *testing.T) {
	raw := `{"addParticipant":true}`
	var resp GroupParticipantResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Added == nil || !*resp.Added {
		t.Error("Added: expected true")
	}
	if resp.Removed != nil {
		t.Error("Removed: expected nil")
	}
}

func TestGroupParticipantResponse_Removed(t *testing.T) {
	raw := `{"removeParticipant":true}`
	var resp GroupParticipantResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Removed == nil || !*resp.Removed {
		t.Error("Removed: expected true")
	}
	if resp.Added != nil {
		t.Error("Added: expected nil")
	}
}
