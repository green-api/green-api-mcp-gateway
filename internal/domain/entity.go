// SPDX-License-Identifier: MIT
// Copyright (c) 2026 GREEN-API

// Package domain contains core business entities for the Green API MCP Gateway.
// Following clean architecture principles: domain has zero external dependencies.
package domain

import "encoding/json"

// InstanceCredentials holds credentials for a single Green API instance.
type InstanceCredentials struct {
	InstanceID uint64
	APIToken   string
	APIURL     string // e.g. https://api.p03.green-api.com
}

// ─── Messaging ────────────────────────────────────────────────────────────────

// SendMessageRequest is the body for sendMessage.
type SendMessageRequest struct {
	ChatID          string `json:"chatId"`
	Message         string `json:"message"`
	QuotedMessageID string `json:"quotedMessageId,omitempty"`
	LinkPreview     bool   `json:"linkPreview,omitempty"`
}

// SendMessageResponse is the response from sendMessage / sendFileByUrl /
// sendLocation / sendContact / sendPoll.
type SendMessageResponse struct {
	IDMessage string `json:"idMessage"`
}

// SendFileByURLRequest is the body for sendFileByUrl.
type SendFileByURLRequest struct {
	ChatID   string `json:"chatId"`
	URL      string `json:"urlFile"`
	FileName string `json:"fileName,omitempty"`
	Caption  string `json:"caption,omitempty"`
}

// SendLocationRequest is the body for sendLocation.
type SendLocationRequest struct {
	ChatID    string  `json:"chatId"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Name      string  `json:"nameLocation,omitempty"`
	Address   string  `json:"address,omitempty"`
}

// Contact represents a vCard contact to send.
type Contact struct {
	PhoneNumber int64  `json:"phoneContact"`
	FirstName   string `json:"firstName,omitempty"`
	MiddleName  string `json:"middleName,omitempty"`
	LastName    string `json:"lastName,omitempty"`
	Company     string `json:"company,omitempty"`
}

// SendContactRequest is the body for sendContact.
type SendContactRequest struct {
	ChatID  string  `json:"chatId"`
	Contact Contact `json:"contact"`
}

// PollOption represents a single option in a poll.
type PollOption struct {
	OptionName string `json:"optionName"`
}

// SendPollRequest is the body for sendPoll.
type SendPollRequest struct {
	ChatID          string       `json:"chatId"`
	Message         string       `json:"message"`
	Options         []PollOption `json:"options"`
	MultipleAnswers bool         `json:"multipleAnswers,omitempty"`
}

// ForwardMessagesRequest is the body for forwardMessages.
type ForwardMessagesRequest struct {
	ChatID     string   `json:"chatId"`
	ChatIDFrom string   `json:"chatIdFrom"`
	Messages   []string `json:"messages"`
}

// ForwardMessagesResponse is the response from forwardMessages.
type ForwardMessagesResponse struct {
	Messages []string `json:"messages"`
}

// EditMessageRequest is the body for editMessage.
type EditMessageRequest struct {
	ChatID    string `json:"chatId"`
	IDMessage string `json:"idMessage"`
	Message   string `json:"message"`
}

// EditMessageResponse is the response from editMessage.
type EditMessageResponse struct {
	IDMessage string `json:"idMessage"`
}

// DeleteMessageRequest is the body for deleteMessage.
type DeleteMessageRequest struct {
	ChatID    string `json:"chatId"`
	IDMessage string `json:"idMessage"`
}

// ─── Instance management ──────────────────────────────────────────────────────

// StateInstanceResponse is the response from getStateInstance.
type StateInstanceResponse struct {
	StateInstance string `json:"stateInstance"`
}

// SettingsResponse holds the full set of instance settings returned by getSettings.
type SettingsResponse struct {
	WebhookURL                    string `json:"webhookUrl"`
	WebhookURLToken               string `json:"webhookUrlToken,omitempty"`
	DelaySendMessagesMilliseconds int    `json:"delaySendMessagesMilliseconds,omitempty"`
	MarkIncomingMessagesReaded    bool   `json:"markIncomingMessagesReaded,omitempty"`
	OutgoingWebhook               string `json:"outgoingWebhook,omitempty"`
	OutgoingMessageWebhook        string `json:"outgoingMessageWebhook,omitempty"`
	IncomingWebhook               string `json:"incomingWebhook,omitempty"`
	DeviceWebhook                 string `json:"deviceWebhook,omitempty"`
	StatusInstanceWebhook         string `json:"statusInstanceWebhook,omitempty"`
	StateWebhook                  string `json:"stateWebhook,omitempty"`
	ProxyHost                     string `json:"proxyHost,omitempty"`
	ProxyPort                     string `json:"proxyPort,omitempty"`
	ProxyLogin                    string `json:"proxyLogin,omitempty"`
}

// SetSettingsRequest is the body for setSettings.
// All fields are optional — only set fields are updated.
type SetSettingsRequest struct {
	WebhookURL                    string `json:"webhookUrl,omitempty"`
	WebhookURLToken               string `json:"webhookUrlToken,omitempty"`
	DelaySendMessagesMilliseconds *int   `json:"delaySendMessagesMilliseconds,omitempty"`
	MarkIncomingMessagesReaded    *bool  `json:"markIncomingMessagesReaded,omitempty"`
	OutgoingWebhook               string `json:"outgoingWebhook,omitempty"`
	OutgoingMessageWebhook        string `json:"outgoingMessageWebhook,omitempty"`
	IncomingWebhook               string `json:"incomingWebhook,omitempty"`
	DeviceWebhook                 string `json:"deviceWebhook,omitempty"`
	StatusInstanceWebhook         string `json:"statusInstanceWebhook,omitempty"`
	StateWebhook                  string `json:"stateWebhook,omitempty"`
	ProxyHost                     string `json:"proxyHost,omitempty"`
	ProxyPort                     string `json:"proxyPort,omitempty"`
	ProxyLogin                    string `json:"proxyLogin,omitempty"`
	ProxyPassword                 string `json:"proxyPassword,omitempty"`
}

// SetSettingsResponse is the response from setSettings.
type SetSettingsResponse struct {
	SavedSettings string `json:"saveSettings"`
}

// QRResponse is the response from the qr (getQr) endpoint.
type QRResponse struct {
	Type    string `json:"type"`    // "qrCode" | "alreadyLogged" | "error"
	Message string `json:"message"` // base64 QR image or status text
}

// ─── Service methods ──────────────────────────────────────────────────────────

// CheckWhatsappRequest is the body for checkWhatsapp.
type CheckWhatsappRequest struct {
	PhoneNumber int64 `json:"phoneNumber"`
}

// CheckWhatsappResponse is the response from checkWhatsapp.
type CheckWhatsappResponse struct {
	ExistsWhatsapp bool `json:"existsWhatsapp"`
}

// GetContactInfoRequest is the body for getContactInfo.
type GetContactInfoRequest struct {
	ChatID string `json:"chatId"`
}

// ContactInfo holds details about a contact or group returned by getContactInfo.
type ContactInfo struct {
	Avatar      string `json:"avatar"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Email       string `json:"email,omitempty"`
	Category    string `json:"category,omitempty"`
	IsGroup     bool   `json:"isGroup,omitempty"`
}

// ─── Notifications ────────────────────────────────────────────────────────────

// NotificationBody represents a single notification returned by receiveNotification.
type NotificationBody struct {
	ReceiptID int             `json:"receiptId"`
	Body      json.RawMessage `json:"body"`
}

// ─── Groups ──────────────────────────────────────────────────────────────────

// CreateGroupRequest is the body for createGroup.
type CreateGroupRequest struct {
	GroupName string   `json:"groupName"`
	ChatIDs   []string `json:"chatIds"`
}

// CreateGroupResponse is the response from createGroup.
type CreateGroupResponse struct {
	Created bool   `json:"created"`
	ChatID  string `json:"chatId"`
}

// GetGroupDataRequest is the body for getGroupData.
type GetGroupDataRequest struct {
	GroupID string `json:"groupId"`
}

// GroupParticipant represents a single participant in a group.
type GroupParticipant struct {
	ID           string `json:"id"`
	IsAdmin      bool   `json:"isAdmin"`
	IsSuperAdmin bool   `json:"isSuperAdmin"`
}

// GroupData holds details about a group returned by getGroupData.
type GroupData struct {
	GroupID      string             `json:"groupId"`
	Owner        string             `json:"owner"`
	Subject      string             `json:"subject"`
	Creation     int64              `json:"creation"`
	Participants []GroupParticipant `json:"participants"`
	InviteLink   string             `json:"inviteLink,omitempty"`
}

// GroupParticipantRequest is the body for addGroupParticipant / removeGroupParticipant.
type GroupParticipantRequest struct {
	GroupID           string `json:"groupId"`
	ParticipantChatID string `json:"participantChatId"`
}

// GroupParticipantResponse is the response from addGroupParticipant / removeGroupParticipant.
type GroupParticipantResponse struct {
	Added   *bool `json:"addParticipant,omitempty"`
	Removed *bool `json:"removeParticipant,omitempty"`
}

// LeaveGroupRequest is the body for leaveGroup.
type LeaveGroupRequest struct {
	GroupID string `json:"groupId"`
}

// ─── History & Reading ────────────────────────────────────────────────────────

// GetChatHistoryRequest is the body for getChatHistory.
type GetChatHistoryRequest struct {
	ChatID string `json:"chatId"`
	Count  int    `json:"count,omitempty"`
}

// GetMessageRequest is the body for getMessage.
type GetMessageRequest struct {
	ChatID    string `json:"chatId"`
	IDMessage string `json:"idMessage"`
}

// ReadChatRequest is the body for readChat.
type ReadChatRequest struct {
	ChatID    string `json:"chatId"`
	IDMessage string `json:"idMessage,omitempty"`
}

// GetAuthorizationCodeRequest is the body for getAuthorizationCode.
type GetAuthorizationCodeRequest struct {
	PhoneNumber int64 `json:"phoneNumber"`
}

// GetAvatarRequest is the body for getAvatar.
type GetAvatarRequest struct {
	ChatID string `json:"chatId"`
}
