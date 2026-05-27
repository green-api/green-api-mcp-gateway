// SPDX-License-Identifier: MIT
// Copyright (c) 2026 GREEN-API

package infrastructure

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/green-api/green-api-mcp-gateway/internal/domain"
)

// MockWhatsAppClient is a mock implementation of the WhatsAppClient interface for testing.
type MockWhatsAppClient struct {
	GetStateInstanceFunc func(ctx context.Context, instanceID uint64) (*domain.StateInstanceResponse, error)
}

func (m *MockWhatsAppClient) GetStateInstance(ctx context.Context, instanceID uint64) (*domain.StateInstanceResponse, error) {
	if m.GetStateInstanceFunc != nil {
		return m.GetStateInstanceFunc(ctx, instanceID)
	}
	return &domain.StateInstanceResponse{StateInstance: "authorized"}, nil
}

// Implement the rest of the interface methods with stubs
func (m *MockWhatsAppClient) CallMethod(ctx context.Context, instanceID uint64, method string, body interface{}) (json.RawMessage, error) {
	return nil, nil
}

func (m *MockWhatsAppClient) GetMethod(ctx context.Context, instanceID uint64, method string) (json.RawMessage, error) {
	return nil, nil
}

func (m *MockWhatsAppClient) SendMessage(ctx context.Context, instanceID uint64, req domain.SendMessageRequest) (*domain.SendMessageResponse, error) {
	return nil, nil
}

func (m *MockWhatsAppClient) SendFileByURL(ctx context.Context, instanceID uint64, req domain.SendFileByURLRequest) (*domain.SendMessageResponse, error) {
	return nil, nil
}

func (m *MockWhatsAppClient) SendLocation(ctx context.Context, instanceID uint64, req domain.SendLocationRequest) (*domain.SendMessageResponse, error) {
	return nil, nil
}

func (m *MockWhatsAppClient) SendContact(ctx context.Context, instanceID uint64, req domain.SendContactRequest) (*domain.SendMessageResponse, error) {
	return nil, nil
}

func (m *MockWhatsAppClient) SendPoll(ctx context.Context, instanceID uint64, req domain.SendPollRequest) (*domain.SendMessageResponse, error) {
	return nil, nil
}

func (m *MockWhatsAppClient) ForwardMessages(ctx context.Context, instanceID uint64, req domain.ForwardMessagesRequest) (*domain.ForwardMessagesResponse, error) {
	return nil, nil
}

func (m *MockWhatsAppClient) EditMessage(ctx context.Context, instanceID uint64, req domain.EditMessageRequest) (*domain.EditMessageResponse, error) {
	return nil, nil
}

func (m *MockWhatsAppClient) DeleteMessage(ctx context.Context, instanceID uint64, req domain.DeleteMessageRequest) error {
	return nil
}

func (m *MockWhatsAppClient) GetSettings(ctx context.Context, instanceID uint64) (*domain.SettingsResponse, error) {
	return nil, nil
}

func (m *MockWhatsAppClient) SetSettings(ctx context.Context, instanceID uint64, req domain.SetSettingsRequest) (*domain.SetSettingsResponse, error) {
	return nil, nil
}

func (m *MockWhatsAppClient) GetQR(ctx context.Context, instanceID uint64) (*domain.QRResponse, error) {
	return nil, nil
}

func (m *MockWhatsAppClient) CheckWhatsapp(ctx context.Context, instanceID uint64, req domain.CheckWhatsappRequest) (*domain.CheckWhatsappResponse, error) {
	return nil, nil
}

func (m *MockWhatsAppClient) GetContacts(ctx context.Context, instanceID uint64) (json.RawMessage, error) {
	return nil, nil
}

func (m *MockWhatsAppClient) GetContactInfo(ctx context.Context, instanceID uint64, req domain.GetContactInfoRequest) (*domain.ContactInfo, error) {
	return nil, nil
}

func (m *MockWhatsAppClient) ReceiveNotification(ctx context.Context, instanceID uint64) (*domain.NotificationBody, error) {
	return nil, nil
}

func (m *MockWhatsAppClient) DeleteNotification(ctx context.Context, instanceID uint64, receiptID int) error {
	return nil
}

func (m *MockWhatsAppClient) CreateGroup(ctx context.Context, instanceID uint64, req domain.CreateGroupRequest) (*domain.CreateGroupResponse, error) {
	return nil, nil
}

func (m *MockWhatsAppClient) GetGroupData(ctx context.Context, instanceID uint64, groupID string) (*domain.GroupData, error) {
	return nil, nil
}

func (m *MockWhatsAppClient) AddGroupParticipant(ctx context.Context, instanceID uint64, req domain.GroupParticipantRequest) (json.RawMessage, error) {
	return nil, nil
}

func (m *MockWhatsAppClient) RemoveGroupParticipant(ctx context.Context, instanceID uint64, req domain.GroupParticipantRequest) (json.RawMessage, error) {
	return nil, nil
}
func (m *MockWhatsAppClient) SetGroupAdmin(_ context.Context, _ uint64, _ domain.GroupParticipantRequest) (json.RawMessage, error) {
	return nil, nil
}
func (m *MockWhatsAppClient) RemoveGroupAdmin(_ context.Context, _ uint64, _ domain.GroupParticipantRequest) (json.RawMessage, error) {
	return nil, nil
}
func (m *MockWhatsAppClient) LeaveGroup(_ context.Context, _ uint64, _ domain.LeaveGroupRequest) (json.RawMessage, error) {
	return nil, nil
}
func (m *MockWhatsAppClient) GetChatHistory(_ context.Context, _ uint64, _ domain.GetChatHistoryRequest) (json.RawMessage, error) {
	return nil, nil
}
func (m *MockWhatsAppClient) GetMessage(_ context.Context, _ uint64, _ domain.GetMessageRequest) (json.RawMessage, error) {
	return nil, nil
}
func (m *MockWhatsAppClient) LastIncomingMessages(_ context.Context, _ uint64, _ int) (json.RawMessage, error) {
	return nil, nil
}
func (m *MockWhatsAppClient) LastOutgoingMessages(_ context.Context, _ uint64, _ int) (json.RawMessage, error) {
	return nil, nil
}
func (m *MockWhatsAppClient) ReadChat(_ context.Context, _ uint64, _ domain.ReadChatRequest) (json.RawMessage, error) {
	return nil, nil
}
func (m *MockWhatsAppClient) GetAuthorizationCode(_ context.Context, _ uint64, _ domain.GetAuthorizationCodeRequest) (json.RawMessage, error) {
	return nil, nil
}
func (m *MockWhatsAppClient) GetContactAvatar(_ context.Context, _ uint64, _ domain.GetAvatarRequest) (json.RawMessage, error) {
	return nil, nil
}

func (m *MockWhatsAppClient) UploadFile(_ context.Context, _ uint64, _ string, _ []byte) (json.RawMessage, error) {
	return nil, nil
}

func (m *MockWhatsAppClient) SendFileByUpload(_ context.Context, _ uint64, _, _, _ string, _ []byte) (json.RawMessage, error) {
	return nil, nil
}

func TestProxyAuthManager_ExtractCredentialsFromRequest(t *testing.T) {
	client := &MockWhatsAppClient{}
	fallbackStore := NewCredentialManager()
	manager := NewProxyAuthManager(client, fallbackStore, 300)

	tests := []struct {
		name       string
		setupReq   func() *http.Request
		wantErr    bool
		wantInstID uint64
		wantToken  string
		wantAPIURL string
	}{
		{
			name: "valid basic auth",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				creds := "1234567890:test-token"
				encoded := base64.StdEncoding.EncodeToString([]byte(creds))
				req.Header.Set("Authorization", "Basic "+encoded)
				return req
			},
			wantInstID: 1234567890,
			wantToken:  "test-token",
			wantAPIURL: "https://api.green-api.com",
		},
		{
			name: "valid custom headers",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.Header.Set("X-Instance-Id", "9876543210")
				req.Header.Set("X-Api-Token", "custom-token")
				req.Header.Set("X-Api-Url", "https://api.p03.green-api.com")
				return req
			},
			wantInstID: 9876543210,
			wantToken:  "custom-token",
			wantAPIURL: "https://api.p03.green-api.com",
		},
		{
			name: "custom headers without api url",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.Header.Set("X-Instance-Id", "1111111111")
				req.Header.Set("X-Api-Token", "another-token")
				return req
			},
			wantInstID: 1111111111,
			wantToken:  "another-token",
			wantAPIURL: "https://api.green-api.com",
		},
		{
			name: "invalid basic auth - no colon",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				encoded := base64.StdEncoding.EncodeToString([]byte("invalidformat"))
				req.Header.Set("Authorization", "Basic "+encoded)
				return req
			},
			wantErr: true,
		},
		{
			name: "invalid custom headers - missing token",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.Header.Set("X-Instance-Id", "1234567890")
				return req
			},
			wantErr: true,
		},
		{
			name: "no auth headers",
			setupReq: func() *http.Request {
				return httptest.NewRequest("GET", "/", nil)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.setupReq()
			creds, err := manager.ExtractCredentialsFromRequest(req)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ExtractCredentialsFromRequest() expected error, got none")
				}
				return
			}

			if err != nil {
				t.Errorf("ExtractCredentialsFromRequest() unexpected error: %v", err)
				return
			}

			if creds.InstanceID != tt.wantInstID {
				t.Errorf("ExtractCredentialsFromRequest() instanceID = %v, want %v", creds.InstanceID, tt.wantInstID)
			}

			if creds.APIToken != tt.wantToken {
				t.Errorf("ExtractCredentialsFromRequest() token = %v, want %v", creds.APIToken, tt.wantToken)
			}

			if creds.APIURL != tt.wantAPIURL {
				t.Errorf("ExtractCredentialsFromRequest() apiURL = %v, want %v", creds.APIURL, tt.wantAPIURL)
			}
		})
	}
}

func TestProxyAuthManager_ValidateCredentials(t *testing.T) {
	validInstanceID := uint64(1234567890)
	invalidInstanceID := uint64(9999999999)

	client := &MockWhatsAppClient{
		GetStateInstanceFunc: func(ctx context.Context, instanceID uint64) (*domain.StateInstanceResponse, error) {
			if instanceID == validInstanceID {
				return &domain.StateInstanceResponse{StateInstance: "authorized"}, nil
			}
			return nil, domain.ErrInstanceNotFound
		},
	}

	fallbackStore := NewCredentialManager()
	manager := NewProxyAuthManager(client, fallbackStore, 1) // 1 second cache for testing

	tests := []struct {
		name    string
		creds   *domain.InstanceCredentials
		wantErr bool
	}{
		{
			name: "valid credentials",
			creds: &domain.InstanceCredentials{
				InstanceID: validInstanceID,
				APIToken:   "valid-token",
				APIURL:     "https://api.green-api.com",
			},
			wantErr: false,
		},
		{
			name: "invalid credentials",
			creds: &domain.InstanceCredentials{
				InstanceID: invalidInstanceID,
				APIToken:   "invalid-token",
				APIURL:     "https://api.green-api.com",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := manager.ValidateCredentials(ctx, tt.creds)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateCredentials() expected error, got none")
				}
				return
			}

			if err != nil {
				t.Errorf("ValidateCredentials() unexpected error: %v", err)
				return
			}

			// Test cache hit
			err = manager.ValidateCredentials(ctx, tt.creds)
			if err != nil {
				t.Errorf("ValidateCredentials() cache hit failed: %v", err)
			}
		})
	}
}

func TestProxyAuthManager_GetCredentialsForRequest(t *testing.T) {
	client := &MockWhatsAppClient{}
	fallbackStore := NewCredentialManager()

	// Add a fallback instance
	fallbackCreds := &domain.InstanceCredentials{
		InstanceID: 1111111111,
		APIToken:   "fallback-token",
		APIURL:     "https://api.green-api.com",
	}
	fallbackStore.AddInstance(fallbackCreds)

	manager := NewProxyAuthManager(client, fallbackStore, 300)

	tests := []struct {
		name     string
		setupReq func() *http.Request
		wantErr  bool
		wantID   uint64
	}{
		{
			name: "proxy credentials in basic auth",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				creds := "1234567890:test-token"
				encoded := base64.StdEncoding.EncodeToString([]byte(creds))
				req.Header.Set("Authorization", "Basic "+encoded)
				return req
			},
			wantID: 1234567890,
		},
		{
			name: "fallback to config credentials",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/?instance_id=1111111111", nil)
				return req
			},
			wantID: 1111111111,
		},
		{
			name: "fallback to first available config instance",
			setupReq: func() *http.Request {
				return httptest.NewRequest("GET", "/", nil)
			},
			wantID: 1111111111, // first (and only) instance in fallback store
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			req := tt.setupReq()

			creds, err := manager.GetCredentialsForRequest(ctx, req)

			if tt.wantErr {
				if err == nil {
					t.Errorf("GetCredentialsForRequest() expected error, got none")
				}
				return
			}

			if err != nil {
				t.Errorf("GetCredentialsForRequest() unexpected error: %v", err)
				return
			}

			if creds.InstanceID != tt.wantID {
				t.Errorf("GetCredentialsForRequest() instanceID = %v, want %v", creds.InstanceID, tt.wantID)
			}
		})
	}
}

func TestProxyAuthManager_CleanExpiredCache(t *testing.T) {
	client := &MockWhatsAppClient{}
	fallbackStore := NewCredentialManager()
	manager := NewProxyAuthManager(client, fallbackStore, 0) // 0 second cache for immediate expiry

	// Add an item to cache
	creds := &domain.InstanceCredentials{
		InstanceID: 1234567890,
		APIToken:   "test-token",
		APIURL:     "https://api.green-api.com",
	}

	cacheKey := manager.getCacheKey(creds)
	manager.cache[cacheKey] = &cachedCredential{
		creds:      creds,
		validUntil: time.Now().Add(-1 * time.Second), // Already expired
	}

	// Verify item is in cache
	if len(manager.cache) != 1 {
		t.Errorf("Expected 1 item in cache, got %d", len(manager.cache))
	}

	// Clean expired cache
	manager.CleanExpiredCache()

	// Verify item was removed
	if len(manager.cache) != 0 {
		t.Errorf("Expected 0 items in cache after cleanup, got %d", len(manager.cache))
	}
}

func TestProxyAuthMiddleware(t *testing.T) {
	client := &MockWhatsAppClient{}
	fallbackStore := NewCredentialManager()

	// Add fallback credentials
	fallbackCreds := &domain.InstanceCredentials{
		InstanceID: 1111111111,
		APIToken:   "fallback-token",
		APIURL:     "https://api.green-api.com",
	}
	fallbackStore.AddInstance(fallbackCreds)

	manager := NewProxyAuthManager(client, fallbackStore, 300)
	middleware := manager.ProxyAuthMiddleware()

	// Test handler that checks for credentials in context
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		creds, ok := GetCredentialsFromContext(r.Context())
		if !ok {
			http.Error(w, "No credentials in context", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(strconv.FormatUint(creds.InstanceID, 10)))
	})

	wrappedHandler := middleware(testHandler)

	tests := []struct {
		name           string
		setupReq       func() *http.Request
		wantStatus     int
		wantInstanceID string
	}{
		{
			name: "valid proxy credentials",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				creds := "1234567890:test-token"
				encoded := base64.StdEncoding.EncodeToString([]byte(creds))
				req.Header.Set("Authorization", "Basic "+encoded)
				return req
			},
			wantStatus:     http.StatusOK,
			wantInstanceID: "1234567890",
		},
		{
			name: "fallback credentials",
			setupReq: func() *http.Request {
				return httptest.NewRequest("GET", "/", nil)
			},
			wantStatus:     http.StatusOK,
			wantInstanceID: "1111111111",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.setupReq()
			rr := httptest.NewRecorder()

			wrappedHandler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("ProxyAuthMiddleware() status = %v, want %v", rr.Code, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusOK && rr.Body.String() != tt.wantInstanceID {
				t.Errorf("ProxyAuthMiddleware() instanceID = %v, want %v", rr.Body.String(), tt.wantInstanceID)
			}
		})
	}
}
