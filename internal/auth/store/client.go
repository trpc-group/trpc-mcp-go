package store

import (
	"sync"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth"
)

// ClientStore stores information about a registered OAuth client.
//
// This interface is equivalent to the parts of the OAuthClientProvider
// in the TypeScript SDK that are responsible for persisting and retrieving
// OAuth client information (clientInformation / saveClientInformation).
type ClientStore interface {
	// ClientInformation returns information about a registered client.
	// If none is stored, returns nil (equivalent to undefined in TypeScript).
	ClientInformation() *auth.OAuthClientInformation

	// SaveClientInformation saves information about a registered client.
	// The full registration response (OAuthClientInformationFull) should be stored,
	// though callers may only require the OAuthClientInformation portion.
	SaveClientInformation(full auth.OAuthClientInformationFull)
}

// InMemoryClientStore is an in-memory implementation of ClientStore.
//
// This is equivalent to the in-memory storage logic found in the
// InMemoryOAuthClientProvider in the TypeScript SDK.
// In production, you should persist client information securely.
type InMemoryClientStore struct {
	mu   sync.RWMutex
	full *auth.OAuthClientInformationFull
}

// NewInMemoryClientStore creates a new in-memory client store instance.
//
// In production, replace this with a persistent implementation.
func NewInMemoryClientStore() *InMemoryClientStore {
	return &InMemoryClientStore{}
}

// ClientInformation returns the stored client information, or nil if none exists.
//
// This matches the behavior of clientInformation() in the TypeScript SDK,
// which returns undefined when no information is stored.
func (s *InMemoryClientStore) ClientInformation() *auth.OAuthClientInformation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.full == nil {
		return nil
	}
	ci := s.full.OAuthClientInformation
	return &ci
}

// SaveClientInformation saves the full client registration information.
//
// This matches the behavior of saveClientInformation() in the TypeScript SDK,
// which stores the OAuthClientInformationFull object returned from dynamic
// client registration.
func (s *InMemoryClientStore) SaveClientInformation(full auth.OAuthClientInformationFull) {
	s.mu.Lock()
	s.full = &full
	s.mu.Unlock()
}
