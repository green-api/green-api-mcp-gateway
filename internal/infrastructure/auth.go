// SPDX-License-Identifier: MIT
// Copyright (c) 2026 GREEN-API

// Package infrastructure contains adapters for external systems.
package infrastructure

import (
	"fmt"
	"sync"

	"github.com/green-api/green-api-mcp-gateway/internal/domain"
)

// CredentialManager implements application.CredentialStore with an in-memory store.
type CredentialManager struct {
	mu        sync.RWMutex
	instances map[uint64]*domain.InstanceCredentials
}

// NewCredentialManager creates a new credential manager.
func NewCredentialManager() *CredentialManager {
	return &CredentialManager{
		instances: make(map[uint64]*domain.InstanceCredentials),
	}
}

// AddInstance registers credentials for an instance.
func (m *CredentialManager) AddInstance(creds *domain.InstanceCredentials) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.instances[creds.InstanceID] = creds
}

// GetCredentials returns credentials for a given instance ID.
func (m *CredentialManager) GetCredentials(instanceID uint64) (*domain.InstanceCredentials, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	creds, ok := m.instances[instanceID]
	if !ok {
		return nil, fmt.Errorf("%w: %d. Call whatsapp_connect first", domain.ErrInstanceNotFound, instanceID)
	}
	return creds, nil
}

// RemoveInstance removes credentials for an instance.
func (m *CredentialManager) RemoveInstance(instanceID uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.instances, instanceID)
}

// ListInstances returns all registered instance IDs.
func (m *CredentialManager) ListInstances() []uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]uint64, 0, len(m.instances))
	for id := range m.instances {
		ids = append(ids, id)
	}
	return ids
}
