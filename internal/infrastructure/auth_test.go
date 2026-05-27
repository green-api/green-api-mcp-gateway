// SPDX-License-Identifier: MIT
// Copyright (c) 2026 GREEN-API

package infrastructure_test

import (
	"errors"
	"sort"
	"sync"
	"testing"

	"github.com/green-api/green-api-mcp-gateway/internal/domain"
	"github.com/green-api/green-api-mcp-gateway/internal/infrastructure"
)

func TestCredentialManager_AddAndGet(t *testing.T) {
	m := infrastructure.NewCredentialManager()

	creds := &domain.InstanceCredentials{
		InstanceID: 12345,
		APIToken:   "token-abc",
		APIURL:     "https://api.green-api.com",
	}
	m.AddInstance(creds)

	got, err := m.GetCredentials(12345)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.InstanceID != creds.InstanceID {
		t.Errorf("InstanceID: got %d, want %d", got.InstanceID, creds.InstanceID)
	}
	if got.APIToken != creds.APIToken {
		t.Errorf("APIToken: got %q, want %q", got.APIToken, creds.APIToken)
	}
	if got.APIURL != creds.APIURL {
		t.Errorf("APIURL: got %q, want %q", got.APIURL, creds.APIURL)
	}
}

func TestCredentialManager_GetNotFound(t *testing.T) {
	m := infrastructure.NewCredentialManager()

	_, err := m.GetCredentials(9999)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, domain.ErrInstanceNotFound) {
		t.Errorf("expected ErrInstanceNotFound, got: %v", err)
	}
}

func TestCredentialManager_ListInstances_Empty(t *testing.T) {
	m := infrastructure.NewCredentialManager()
	ids := m.ListInstances()
	if len(ids) != 0 {
		t.Errorf("expected empty list, got %v", ids)
	}
}

func TestCredentialManager_ListInstances_Multiple(t *testing.T) {
	m := infrastructure.NewCredentialManager()

	want := []uint64{1, 2, 3}
	for _, id := range want {
		m.AddInstance(&domain.InstanceCredentials{
			InstanceID: id,
			APIToken:   "tok",
			APIURL:     "https://api.green-api.com",
		})
	}

	got := m.ListInstances()
	if len(got) != len(want) {
		t.Fatalf("expected %d instances, got %d", len(want), len(got))
	}

	sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
	for i, id := range want {
		if got[i] != id {
			t.Errorf("ListInstances()[%d] = %d, want %d", i, got[i], id)
		}
	}
}

func TestCredentialManager_OverwriteInstance(t *testing.T) {
	m := infrastructure.NewCredentialManager()

	m.AddInstance(&domain.InstanceCredentials{InstanceID: 1, APIToken: "old", APIURL: "https://old.example.com"})
	m.AddInstance(&domain.InstanceCredentials{InstanceID: 1, APIToken: "new", APIURL: "https://new.example.com"})

	got, err := m.GetCredentials(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.APIToken != "new" {
		t.Errorf("expected overwritten token %q, got %q", "new", got.APIToken)
	}
}

func TestCredentialManager_RemoveInstance(t *testing.T) {
	m := infrastructure.NewCredentialManager()

	m.AddInstance(&domain.InstanceCredentials{InstanceID: 1, APIToken: "tok", APIURL: "https://api.green-api.com"})
	m.AddInstance(&domain.InstanceCredentials{InstanceID: 2, APIToken: "tok2", APIURL: "https://api.green-api.com"})

	m.RemoveInstance(1)

	_, err := m.GetCredentials(1)
	if !errors.Is(err, domain.ErrInstanceNotFound) {
		t.Errorf("expected ErrInstanceNotFound after remove, got: %v", err)
	}

	// Instance 2 should still be present.
	got, err := m.GetCredentials(2)
	if err != nil {
		t.Fatalf("unexpected error for instance 2: %v", err)
	}
	if got.APIToken != "tok2" {
		t.Errorf("APIToken: got %q, want %q", got.APIToken, "tok2")
	}

	ids := m.ListInstances()
	if len(ids) != 1 {
		t.Errorf("expected 1 instance after remove, got %d", len(ids))
	}
}

func TestCredentialManager_RemoveInstance_NonExistent(t *testing.T) {
	m := infrastructure.NewCredentialManager()
	// Should not panic.
	m.RemoveInstance(9999)
	ids := m.ListInstances()
	if len(ids) != 0 {
		t.Errorf("expected 0 instances, got %d", len(ids))
	}
}

func TestCredentialManager_ConcurrentAccess(t *testing.T) {
	m := infrastructure.NewCredentialManager()

	var wg sync.WaitGroup
	for i := uint64(0); i < 100; i++ {
		wg.Add(1)
		go func(id uint64) {
			defer wg.Done()
			m.AddInstance(&domain.InstanceCredentials{
				InstanceID: id,
				APIToken:   "tok",
				APIURL:     "https://api.green-api.com",
			})
		}(i)
	}
	wg.Wait()

	ids := m.ListInstances()
	if len(ids) != 100 {
		t.Errorf("expected 100 instances, got %d", len(ids))
	}

	// Concurrent reads.
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id uint64) {
			defer wg.Done()
			_, _ = m.GetCredentials(id)
		}(uint64(i))
	}
	wg.Wait()
}
