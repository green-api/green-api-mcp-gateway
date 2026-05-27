// SPDX-License-Identifier: MIT
// Copyright (c) 2026 GREEN-API

// Package greenapi implements the WhatsAppClient port using a custom HTTP client.
package greenapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/green-api/green-api-mcp-gateway/internal/application"
	"github.com/green-api/green-api-mcp-gateway/internal/domain"
	"github.com/green-api/green-api-mcp-gateway/internal/infrastructure/config"
)

// Client implements WhatsAppClient using a custom HTTP client.
type Client struct {
	credentials application.CredentialStore
	metrics     application.MetricsProvider
	config      *config.Config
	httpClient  *http.Client
}

// NewClient creates a new HTTP-based Green API client.
func NewClient(credentials application.CredentialStore, metrics application.MetricsProvider, cfg *config.Config) *Client {
	return &Client{
		credentials: credentials,
		metrics:     metrics,
		config:      cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// buildURL constructs the full URL for a Green API method call.
// All requests go to the api_url stored in the instance credentials.
func (c *Client) buildURL(instanceID uint64, method string) (string, error) {
	creds, err := c.credentials.GetCredentials(instanceID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/waInstance%d/%s/%s",
		strings.TrimRight(creds.APIURL, "/"), instanceID, method, creds.APIToken), nil
}

// httpRequest performs an HTTP request and returns the raw JSON response body.
func (c *Client) httpRequest(ctx context.Context, httpMethod, url string, body interface{}) (json.RawMessage, error) {
	var reqBody io.Reader
	if body != nil && (httpMethod == http.MethodPost || httpMethod == http.MethodPut) {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(data)
	}

	req, err := http.NewRequestWithContext(ctx, httpMethod, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http %s %s: %w", httpMethod, maskURL(url), err)
	}
	defer func() { _ = resp.Body.Close() }()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, &APIError{StatusCode: resp.StatusCode, Body: string(respData)}
	}

	// Empty body (e.g. receiveNotification with no pending message) → return nil.
	if len(bytes.TrimSpace(respData)) == 0 {
		return nil, nil //nolint:nilnil
	}

	return json.RawMessage(respData), nil
}

// multipartFile represents a file to be uploaded as multipart form data.
type multipartFile struct {
	Filename string
	Data     []byte
}

// multipartRequest makes a multipart/form-data POST request for file uploads.
// files maps form field name to multipartFile.
func (c *Client) multipartRequest(ctx context.Context, url string, fields map[string]string, files map[string]multipartFile) (json.RawMessage, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	for key, value := range fields {
		if err := w.WriteField(key, value); err != nil {
			return nil, fmt.Errorf("write field %s: %w", key, err)
		}
	}
	for fieldName, f := range files {
		fw, err := w.CreateFormFile(fieldName, f.Filename)
		if err != nil {
			return nil, fmt.Errorf("create form file %s: %w", fieldName, err)
		}
		if _, err := fw.Write(f.Data); err != nil {
			return nil, fmt.Errorf("write file data %s: %w", fieldName, err)
		}
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return nil, fmt.Errorf("create multipart request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("multipart request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read multipart response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, &APIError{StatusCode: resp.StatusCode, Body: string(respData)}
	}
	return json.RawMessage(respData), nil
}

// toStruct unmarshals a JSON raw message into a typed struct.
func toStruct[T any](data json.RawMessage) (*T, error) {
	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &result, nil
}

// httpMethodFor returns the HTTP verb to use for a given Green API method name.
// The mapping mirrors what the upstream GREEN-API service expects.
func httpMethodFor(method string) string {
	switch method {
	case domain.MethodDeleteNotification, "clearWebhooksQueue":
		return http.MethodDelete
	case
		domain.MethodGetStateInstance,
		"getStatusInstance",
		domain.MethodGetSettings,
		"getWaSettings",
		domain.MethodGetQR,
		domain.MethodReboot,
		domain.MethodLogout,
		domain.MethodGetContacts,
		"getChats",
		domain.MethodReceiveNotification,
		"showMessagesQueue",
		"getMessagesCount",
		"clearMessagesQueue",
		"lastIncomingMessages",
		"lastOutgoingMessages",
		"getIncomingStatuses",
		"getOutgoingStatuses",
		"getStatusStatistic",
		"getWebhooksCount":
		return http.MethodGet
	default:
		return http.MethodPost
	}
}

// ─── Interface implementation ─────────────────────────────────────────────────

// CallMethod sends an HTTP request with optional JSON body to the given Green API method.
func (c *Client) CallMethod(ctx context.Context, instanceID uint64, method string, body interface{}) (json.RawMessage, error) {
	start := time.Now()

	url, err := c.buildURL(instanceID, method)
	if err != nil {
		c.metrics.RecordAPIRequest(method, start, err)
		return nil, err
	}

	verb := httpMethodFor(method)
	// If caller supplies a body override the verb to POST unless it's an explicit GET/DELETE method.
	if body != nil && verb == http.MethodGet {
		verb = http.MethodPost
	}

	resp, err := c.httpRequest(ctx, verb, url, body)
	if err != nil {
		wrapped := fmt.Errorf("green api %s: %w", method, err)
		c.metrics.RecordAPIRequest(method, start, wrapped)
		return nil, wrapped
	}

	c.metrics.RecordAPIRequest(method, start, nil)
	return resp, nil
}

// GetMethod sends a GET request to the given Green API method.
func (c *Client) GetMethod(ctx context.Context, instanceID uint64, method string) (json.RawMessage, error) {
	start := time.Now()

	url, err := c.buildURL(instanceID, method)
	if err != nil {
		c.metrics.RecordAPIRequest(method, start, err)
		return nil, err
	}

	resp, err := c.httpRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		wrapped := fmt.Errorf("green api %s: %w", method, err)
		c.metrics.RecordAPIRequest(method, start, wrapped)
		return nil, wrapped
	}

	c.metrics.RecordAPIRequest(method, start, nil)
	return resp, nil
}

// SendMessage sends a text message.
func (c *Client) SendMessage(ctx context.Context, instanceID uint64, req domain.SendMessageRequest) (*domain.SendMessageResponse, error) {
	start := time.Now()

	url, err := c.buildURL(instanceID, domain.MethodSendMessage)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodSendMessage, start, err)
		return nil, err
	}

	raw, err := c.httpRequest(ctx, http.MethodPost, url, req)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodSendMessage, start, err)
		return nil, fmt.Errorf("SendMessage: %w", err)
	}
	c.metrics.RecordAPIRequest(domain.MethodSendMessage, start, nil)
	return toStruct[domain.SendMessageResponse](raw)
}

// SendFileByURL sends a file by URL.
func (c *Client) SendFileByURL(ctx context.Context, instanceID uint64, req domain.SendFileByURLRequest) (*domain.SendMessageResponse, error) {
	start := time.Now()

	url, err := c.buildURL(instanceID, domain.MethodSendFileByURL)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodSendFileByURL, start, err)
		return nil, err
	}

	raw, err := c.httpRequest(ctx, http.MethodPost, url, req)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodSendFileByURL, start, err)
		return nil, fmt.Errorf("SendFileByURL: %w", err)
	}
	c.metrics.RecordAPIRequest(domain.MethodSendFileByURL, start, nil)
	return toStruct[domain.SendMessageResponse](raw)
}

// SendLocation sends a geolocation.
func (c *Client) SendLocation(ctx context.Context, instanceID uint64, req domain.SendLocationRequest) (*domain.SendMessageResponse, error) {
	start := time.Now()

	url, err := c.buildURL(instanceID, domain.MethodSendLocation)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodSendLocation, start, err)
		return nil, err
	}

	raw, err := c.httpRequest(ctx, http.MethodPost, url, req)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodSendLocation, start, err)
		return nil, fmt.Errorf("SendLocation: %w", err)
	}
	c.metrics.RecordAPIRequest(domain.MethodSendLocation, start, nil)
	return toStruct[domain.SendMessageResponse](raw)
}

// SendContact sends a contact card.
func (c *Client) SendContact(ctx context.Context, instanceID uint64, req domain.SendContactRequest) (*domain.SendMessageResponse, error) {
	start := time.Now()

	url, err := c.buildURL(instanceID, domain.MethodSendContact)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodSendContact, start, err)
		return nil, err
	}

	raw, err := c.httpRequest(ctx, http.MethodPost, url, req)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodSendContact, start, err)
		return nil, fmt.Errorf("SendContact: %w", err)
	}
	c.metrics.RecordAPIRequest(domain.MethodSendContact, start, nil)
	return toStruct[domain.SendMessageResponse](raw)
}

// SendPoll sends a poll message.
func (c *Client) SendPoll(ctx context.Context, instanceID uint64, req domain.SendPollRequest) (*domain.SendMessageResponse, error) {
	start := time.Now()

	url, err := c.buildURL(instanceID, domain.MethodSendPoll)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodSendPoll, start, err)
		return nil, err
	}

	raw, err := c.httpRequest(ctx, http.MethodPost, url, req)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodSendPoll, start, err)
		return nil, fmt.Errorf("SendPoll: %w", err)
	}
	c.metrics.RecordAPIRequest(domain.MethodSendPoll, start, nil)
	return toStruct[domain.SendMessageResponse](raw)
}

// ForwardMessages forwards one or more messages to another chat.
func (c *Client) ForwardMessages(ctx context.Context, instanceID uint64, req domain.ForwardMessagesRequest) (*domain.ForwardMessagesResponse, error) {
	start := time.Now()

	url, err := c.buildURL(instanceID, domain.MethodForwardMessages)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodForwardMessages, start, err)
		return nil, err
	}

	raw, err := c.httpRequest(ctx, http.MethodPost, url, req)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodForwardMessages, start, err)
		return nil, fmt.Errorf("ForwardMessages: %w", err)
	}
	c.metrics.RecordAPIRequest(domain.MethodForwardMessages, start, nil)
	return toStruct[domain.ForwardMessagesResponse](raw)
}

// EditMessage edits a previously sent message.
func (c *Client) EditMessage(ctx context.Context, instanceID uint64, req domain.EditMessageRequest) (*domain.EditMessageResponse, error) {
	start := time.Now()

	url, err := c.buildURL(instanceID, domain.MethodEditMessage)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodEditMessage, start, err)
		return nil, err
	}

	raw, err := c.httpRequest(ctx, http.MethodPost, url, req)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodEditMessage, start, err)
		return nil, fmt.Errorf("EditMessage: %w", err)
	}
	c.metrics.RecordAPIRequest(domain.MethodEditMessage, start, nil)
	return toStruct[domain.EditMessageResponse](raw)
}

// DeleteMessage deletes a message.
func (c *Client) DeleteMessage(ctx context.Context, instanceID uint64, req domain.DeleteMessageRequest) error {
	start := time.Now()

	url, err := c.buildURL(instanceID, domain.MethodDeleteMessage)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodDeleteMessage, start, err)
		return err
	}

	_, err = c.httpRequest(ctx, http.MethodPost, url, req)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodDeleteMessage, start, err)
		return fmt.Errorf("DeleteMessage: %w", err)
	}
	c.metrics.RecordAPIRequest(domain.MethodDeleteMessage, start, nil)
	return nil
}

// GetStateInstance returns the current authorization state of the instance.
func (c *Client) GetStateInstance(ctx context.Context, instanceID uint64) (*domain.StateInstanceResponse, error) {
	start := time.Now()

	url, err := c.buildURL(instanceID, domain.MethodGetStateInstance)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodGetStateInstance, start, err)
		return nil, err
	}

	raw, err := c.httpRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodGetStateInstance, start, err)
		return nil, fmt.Errorf("GetStateInstance: %w", err)
	}
	c.metrics.RecordAPIRequest(domain.MethodGetStateInstance, start, nil)
	return toStruct[domain.StateInstanceResponse](raw)
}

// GetSettings returns the settings of the instance.
func (c *Client) GetSettings(ctx context.Context, instanceID uint64) (*domain.SettingsResponse, error) {
	start := time.Now()

	url, err := c.buildURL(instanceID, domain.MethodGetSettings)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodGetSettings, start, err)
		return nil, err
	}

	raw, err := c.httpRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodGetSettings, start, err)
		return nil, fmt.Errorf("GetSettings: %w", err)
	}
	c.metrics.RecordAPIRequest(domain.MethodGetSettings, start, nil)
	return toStruct[domain.SettingsResponse](raw)
}

// SetSettings updates the settings of the instance.
func (c *Client) SetSettings(ctx context.Context, instanceID uint64, req domain.SetSettingsRequest) (*domain.SetSettingsResponse, error) {
	start := time.Now()

	url, err := c.buildURL(instanceID, domain.MethodSetSettings)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodSetSettings, start, err)
		return nil, err
	}

	raw, err := c.httpRequest(ctx, http.MethodPost, url, req)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodSetSettings, start, err)
		return nil, fmt.Errorf("SetSettings: %w", err)
	}
	c.metrics.RecordAPIRequest(domain.MethodSetSettings, start, nil)
	return toStruct[domain.SetSettingsResponse](raw)
}

// GetQR retrieves the QR code for instance authorization.
func (c *Client) GetQR(ctx context.Context, instanceID uint64) (*domain.QRResponse, error) {
	start := time.Now()

	url, err := c.buildURL(instanceID, domain.MethodGetQR)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodGetQR, start, err)
		return nil, err
	}

	raw, err := c.httpRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodGetQR, start, err)
		return nil, fmt.Errorf("GetQR: %w", err)
	}
	c.metrics.RecordAPIRequest(domain.MethodGetQR, start, nil)
	return toStruct[domain.QRResponse](raw)
}

// CheckWhatsapp checks whether a phone number has an active WhatsApp account.
func (c *Client) CheckWhatsapp(ctx context.Context, instanceID uint64, req domain.CheckWhatsappRequest) (*domain.CheckWhatsappResponse, error) {
	start := time.Now()

	url, err := c.buildURL(instanceID, domain.MethodCheckWhatsapp)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodCheckWhatsapp, start, err)
		return nil, err
	}

	raw, err := c.httpRequest(ctx, http.MethodPost, url, req)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodCheckWhatsapp, start, err)
		return nil, fmt.Errorf("CheckWhatsapp: %w", err)
	}
	c.metrics.RecordAPIRequest(domain.MethodCheckWhatsapp, start, nil)
	return toStruct[domain.CheckWhatsappResponse](raw)
}

// GetContacts retrieves the full contacts list.
func (c *Client) GetContacts(ctx context.Context, instanceID uint64) (json.RawMessage, error) {
	start := time.Now()

	url, err := c.buildURL(instanceID, domain.MethodGetContacts)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodGetContacts, start, err)
		return nil, err
	}

	raw, err := c.httpRequest(ctx, http.MethodGet, url, nil)
	c.metrics.RecordAPIRequest(domain.MethodGetContacts, start, err)
	return raw, err
}

// GetContactInfo retrieves detailed information about a specific contact or group.
func (c *Client) GetContactInfo(ctx context.Context, instanceID uint64, req domain.GetContactInfoRequest) (*domain.ContactInfo, error) {
	start := time.Now()

	url, err := c.buildURL(instanceID, domain.MethodGetContactInfo)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodGetContactInfo, start, err)
		return nil, err
	}

	raw, err := c.httpRequest(ctx, http.MethodPost, url, req)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodGetContactInfo, start, err)
		return nil, fmt.Errorf("GetContactInfo: %w", err)
	}
	c.metrics.RecordAPIRequest(domain.MethodGetContactInfo, start, nil)
	return toStruct[domain.ContactInfo](raw)
}

// ReceiveNotification long-polls for the next notification in the instance queue.
// Returns nil without error when there are no pending notifications.
func (c *Client) ReceiveNotification(ctx context.Context, instanceID uint64) (*domain.NotificationBody, error) {
	start := time.Now()

	url, err := c.buildURL(instanceID, domain.MethodReceiveNotification)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodReceiveNotification, start, err)
		return nil, err
	}

	raw, err := c.httpRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodReceiveNotification, start, err)
		return nil, fmt.Errorf("ReceiveNotification: %w", err)
	}
	c.metrics.RecordAPIRequest(domain.MethodReceiveNotification, start, nil)

	// Empty queue — server returns empty body or null.
	if raw == nil || string(raw) == "null" {
		return nil, nil //nolint:nilnil
	}

	return toStruct[domain.NotificationBody](raw)
}

// DeleteNotification confirms receipt and removes a notification from the queue.
// URL format: {backend}/waInstance{id}/deleteNotification/{receiptId}/{token}
func (c *Client) DeleteNotification(ctx context.Context, instanceID uint64, receiptID int) error {
	start := time.Now()

	creds, err := c.credentials.GetCredentials(instanceID)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodDeleteNotification, start, err)
		return err
	}

	backendBase := strings.TrimRight(creds.APIURL, "/")

	deleteURL := fmt.Sprintf("%s/waInstance%d/%s/%d/%s",
		backendBase, instanceID, domain.MethodDeleteNotification, receiptID, creds.APIToken)

	_, err = c.httpRequest(ctx, http.MethodDelete, deleteURL, nil)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodDeleteNotification, start, err)
		return fmt.Errorf("DeleteNotification: %w", err)
	}
	c.metrics.RecordAPIRequest(domain.MethodDeleteNotification, start, nil)
	return nil
}

// CreateGroup creates a new WhatsApp group.
func (c *Client) CreateGroup(ctx context.Context, instanceID uint64, req domain.CreateGroupRequest) (*domain.CreateGroupResponse, error) {
	start := time.Now()

	url, err := c.buildURL(instanceID, domain.MethodCreateGroup)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodCreateGroup, start, err)
		return nil, err
	}

	raw, err := c.httpRequest(ctx, http.MethodPost, url, req)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodCreateGroup, start, err)
		return nil, fmt.Errorf("CreateGroup: %w", err)
	}
	c.metrics.RecordAPIRequest(domain.MethodCreateGroup, start, nil)
	return toStruct[domain.CreateGroupResponse](raw)
}

// GetGroupData retrieves data about a group.
func (c *Client) GetGroupData(ctx context.Context, instanceID uint64, groupID string) (*domain.GroupData, error) {
	start := time.Now()

	url, err := c.buildURL(instanceID, domain.MethodGetGroupData)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodGetGroupData, start, err)
		return nil, err
	}

	raw, err := c.httpRequest(ctx, http.MethodPost, url, map[string]string{"groupId": groupID})
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodGetGroupData, start, err)
		return nil, fmt.Errorf("GetGroupData: %w", err)
	}
	c.metrics.RecordAPIRequest(domain.MethodGetGroupData, start, nil)
	return toStruct[domain.GroupData](raw)
}

// AddGroupParticipant adds a participant to a group.
func (c *Client) AddGroupParticipant(ctx context.Context, instanceID uint64, req domain.GroupParticipantRequest) (json.RawMessage, error) {
	start := time.Now()

	url, err := c.buildURL(instanceID, domain.MethodAddGroupParticipant)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodAddGroupParticipant, start, err)
		return nil, err
	}

	raw, err := c.httpRequest(ctx, http.MethodPost, url, req)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodAddGroupParticipant, start, err)
		return nil, fmt.Errorf("AddGroupParticipant: %w", err)
	}
	c.metrics.RecordAPIRequest(domain.MethodAddGroupParticipant, start, nil)
	return raw, nil
}

// SetGroupAdmin grants admin rights to a group participant.
func (c *Client) SetGroupAdmin(ctx context.Context, instanceID uint64, req domain.GroupParticipantRequest) (json.RawMessage, error) {
	start := time.Now()

	url, err := c.buildURL(instanceID, domain.MethodSetGroupAdmin)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodSetGroupAdmin, start, err)
		return nil, err
	}

	raw, err := c.httpRequest(ctx, http.MethodPost, url, req)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodSetGroupAdmin, start, err)
		return nil, fmt.Errorf("SetGroupAdmin: %w", err)
	}
	c.metrics.RecordAPIRequest(domain.MethodSetGroupAdmin, start, nil)
	return raw, nil
}

// RemoveGroupAdmin revokes admin rights from a group participant.
func (c *Client) RemoveGroupAdmin(ctx context.Context, instanceID uint64, req domain.GroupParticipantRequest) (json.RawMessage, error) {
	start := time.Now()

	url, err := c.buildURL(instanceID, domain.MethodRemoveGroupAdmin)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodRemoveGroupAdmin, start, err)
		return nil, err
	}

	raw, err := c.httpRequest(ctx, http.MethodPost, url, req)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodRemoveGroupAdmin, start, err)
		return nil, fmt.Errorf("RemoveGroupAdmin: %w", err)
	}
	c.metrics.RecordAPIRequest(domain.MethodRemoveGroupAdmin, start, nil)
	return raw, nil
}

// LeaveGroup leaves a WhatsApp group.
func (c *Client) LeaveGroup(ctx context.Context, instanceID uint64, req domain.LeaveGroupRequest) (json.RawMessage, error) {
	start := time.Now()

	url, err := c.buildURL(instanceID, domain.MethodLeaveGroup)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodLeaveGroup, start, err)
		return nil, err
	}

	raw, err := c.httpRequest(ctx, http.MethodPost, url, req)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodLeaveGroup, start, err)
		return nil, fmt.Errorf("LeaveGroup: %w", err)
	}
	c.metrics.RecordAPIRequest(domain.MethodLeaveGroup, start, nil)
	return raw, nil
}

// GetChatHistory returns chat message history.
func (c *Client) GetChatHistory(ctx context.Context, instanceID uint64, req domain.GetChatHistoryRequest) (json.RawMessage, error) {
	start := time.Now()

	url, err := c.buildURL(instanceID, domain.MethodGetChatHistory)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodGetChatHistory, start, err)
		return nil, err
	}

	raw, err := c.httpRequest(ctx, http.MethodPost, url, req)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodGetChatHistory, start, err)
		return nil, fmt.Errorf("GetChatHistory: %w", err)
	}
	c.metrics.RecordAPIRequest(domain.MethodGetChatHistory, start, nil)
	return raw, nil
}

// GetMessage returns a specific message by ID.
func (c *Client) GetMessage(ctx context.Context, instanceID uint64, req domain.GetMessageRequest) (json.RawMessage, error) {
	start := time.Now()

	url, err := c.buildURL(instanceID, domain.MethodGetMessage)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodGetMessage, start, err)
		return nil, err
	}

	raw, err := c.httpRequest(ctx, http.MethodPost, url, req)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodGetMessage, start, err)
		return nil, fmt.Errorf("GetMessage: %w", err)
	}
	c.metrics.RecordAPIRequest(domain.MethodGetMessage, start, nil)
	return raw, nil
}

// LastIncomingMessages returns recent incoming messages.
func (c *Client) LastIncomingMessages(ctx context.Context, instanceID uint64, minutes int) (json.RawMessage, error) {
	start := time.Now()

	baseURL, err := c.buildURL(instanceID, domain.MethodLastIncomingMessages)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodLastIncomingMessages, start, err)
		return nil, err
	}

	url := fmt.Sprintf("%s?minutes=%d", baseURL, minutes)
	raw, err := c.httpRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodLastIncomingMessages, start, err)
		return nil, fmt.Errorf("LastIncomingMessages: %w", err)
	}
	c.metrics.RecordAPIRequest(domain.MethodLastIncomingMessages, start, nil)
	return raw, nil
}

// LastOutgoingMessages returns recent outgoing messages.
func (c *Client) LastOutgoingMessages(ctx context.Context, instanceID uint64, minutes int) (json.RawMessage, error) {
	start := time.Now()

	baseURL, err := c.buildURL(instanceID, domain.MethodLastOutgoingMessages)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodLastOutgoingMessages, start, err)
		return nil, err
	}

	url := fmt.Sprintf("%s?minutes=%d", baseURL, minutes)
	raw, err := c.httpRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodLastOutgoingMessages, start, err)
		return nil, fmt.Errorf("LastOutgoingMessages: %w", err)
	}
	c.metrics.RecordAPIRequest(domain.MethodLastOutgoingMessages, start, nil)
	return raw, nil
}

// ReadChat marks messages in a chat as read.
func (c *Client) ReadChat(ctx context.Context, instanceID uint64, req domain.ReadChatRequest) (json.RawMessage, error) {
	start := time.Now()

	url, err := c.buildURL(instanceID, domain.MethodReadChat)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodReadChat, start, err)
		return nil, err
	}

	raw, err := c.httpRequest(ctx, http.MethodPost, url, req)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodReadChat, start, err)
		return nil, fmt.Errorf("ReadChat: %w", err)
	}
	c.metrics.RecordAPIRequest(domain.MethodReadChat, start, nil)
	return raw, nil
}

// GetAuthorizationCode retrieves an authorization code for phone-based login.
func (c *Client) GetAuthorizationCode(ctx context.Context, instanceID uint64, req domain.GetAuthorizationCodeRequest) (json.RawMessage, error) {
	start := time.Now()

	url, err := c.buildURL(instanceID, domain.MethodGetAuthorizationCode)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodGetAuthorizationCode, start, err)
		return nil, err
	}

	raw, err := c.httpRequest(ctx, http.MethodPost, url, req)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodGetAuthorizationCode, start, err)
		return nil, fmt.Errorf("GetAuthorizationCode: %w", err)
	}
	c.metrics.RecordAPIRequest(domain.MethodGetAuthorizationCode, start, nil)
	return raw, nil
}

// GetContactAvatar retrieves the avatar URL of a contact.
func (c *Client) GetContactAvatar(ctx context.Context, instanceID uint64, req domain.GetAvatarRequest) (json.RawMessage, error) {
	start := time.Now()

	url, err := c.buildURL(instanceID, domain.MethodGetAvatar)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodGetAvatar, start, err)
		return nil, err
	}

	raw, err := c.httpRequest(ctx, http.MethodPost, url, req)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodGetAvatar, start, err)
		return nil, fmt.Errorf("GetContactAvatar: %w", err)
	}
	c.metrics.RecordAPIRequest(domain.MethodGetAvatar, start, nil)
	return raw, nil
}

// UploadFile uploads a file to Green API and returns the hosted file URL.
func (c *Client) UploadFile(ctx context.Context, instanceID uint64, filename string, fileData []byte) (json.RawMessage, error) {
	start := time.Now()

	url, err := c.buildURL(instanceID, domain.MethodUploadFile)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodUploadFile, start, err)
		return nil, err
	}

	raw, err := c.multipartRequest(ctx, url,
		map[string]string{},
		map[string]multipartFile{
			"file": {Filename: filename, Data: fileData},
		},
	)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodUploadFile, start, err)
		return nil, fmt.Errorf("UploadFile: %w", err)
	}
	c.metrics.RecordAPIRequest(domain.MethodUploadFile, start, nil)
	return raw, nil
}

// SendFileByUpload sends a file directly to a chat via multipart upload.
func (c *Client) SendFileByUpload(ctx context.Context, instanceID uint64, chatID, caption, filename string, fileData []byte) (json.RawMessage, error) {
	start := time.Now()

	url, err := c.buildURL(instanceID, domain.MethodSendFileByUpload)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodSendFileByUpload, start, err)
		return nil, err
	}

	fields := map[string]string{
		"chatId":   chatID,
		"caption":  caption,
		"fileName": filename,
	}
	raw, err := c.multipartRequest(ctx, url, fields, map[string]multipartFile{
		"file": {Filename: filename, Data: fileData},
	})
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodSendFileByUpload, start, err)
		return nil, fmt.Errorf("SendFileByUpload: %w", err)
	}
	c.metrics.RecordAPIRequest(domain.MethodSendFileByUpload, start, nil)
	return raw, nil
}

// RemoveGroupParticipant removes a participant from a group.
func (c *Client) RemoveGroupParticipant(ctx context.Context, instanceID uint64, req domain.GroupParticipantRequest) (json.RawMessage, error) {
	start := time.Now()

	url, err := c.buildURL(instanceID, domain.MethodRemoveGroupParticipant)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodRemoveGroupParticipant, start, err)
		return nil, err
	}

	raw, err := c.httpRequest(ctx, http.MethodPost, url, req)
	if err != nil {
		c.metrics.RecordAPIRequest(domain.MethodRemoveGroupParticipant, start, err)
		return nil, fmt.Errorf("RemoveGroupParticipant: %w", err)
	}
	c.metrics.RecordAPIRequest(domain.MethodRemoveGroupParticipant, start, nil)
	return raw, nil
}
