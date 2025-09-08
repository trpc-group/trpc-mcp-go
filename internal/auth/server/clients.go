// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package server

import (
	"fmt"

	"trpc.group/trpc-go/trpc-mcp-go/internal/auth"
)

// OAuthClientsStoreInterface defines retrieval and optional dynamic registration for OAuth clients registered with this server
type OAuthClientsStoreInterface interface {
	// GetClient returns information about a registered client by its ID or nil if not found
	GetClient(clientId string) (*auth.OAuthClientInformationFull, error)

	// SupportDynamicClientRegistration adds optional dynamic client registration capability
	// Implementations may return a modified client to reflect server enforced values
	// Implementations should not delete expired client secrets in place
	// Middleware validates client_secret_expires_at and rejects expired secrets
	SupportDynamicClientRegistration
}

// SupportDynamicClientRegistration exposes the RegisterClient operation for dynamic client registration
type SupportDynamicClientRegistration interface {
	// RegisterClient registers a new OAuth client and returns the stored client record
	RegisterClient(client auth.OAuthClientInformationFull) (*auth.OAuthClientInformationFull, error)
}

// OAuthClientsStore is a functional store adapter for OAuth client retrieval and optional registration
type OAuthClientsStore struct {
	getClient      func(clientID string) (*auth.OAuthClientInformationFull, error)                        // lookup function injected by caller
	registerClient func(client auth.OAuthClientInformationFull) (*auth.OAuthClientInformationFull, error) // optional registration function injected by caller
}

// GetClient returns the client record for the given clientID or an error from the underlying store
func (s OAuthClientsStore) GetClient(clientID string) (*auth.OAuthClientInformationFull, error) {
	// Delegate to injected lookup function
	return s.getClient(clientID)
}

// RegisterClient registers a new client if dynamic registration is supported otherwise returns an error
func (s OAuthClientsStore) RegisterClient(client auth.OAuthClientInformationFull) (*auth.OAuthClientInformationFull, error) {
	// If no registration function is provided dynamic registration is not supported
	if s.registerClient == nil {
		return nil, fmt.Errorf("dynamic client registration is not supported")
	}
	// Delegate to injected registration function
	return s.registerClient(client)
}

// NewOAuthClientStoreSupportDynamicRegistration constructs a store with both lookup and registration support
func NewOAuthClientStoreSupportDynamicRegistration(
	getClient func(clientID string) (*auth.OAuthClientInformationFull, error),
	registerClient func(client auth.OAuthClientInformationFull) (*auth.OAuthClientInformationFull, error),
) *OAuthClientsStore {
	// Inject both handlers to enable dynamic registration support
	return &OAuthClientsStore{
		getClient:      getClient,
		registerClient: registerClient,
	}
}

// NewOAuthClientStore constructs a store that supports only client lookup
func NewOAuthClientStore(
	getClient func(clientID string) (*auth.OAuthClientInformationFull, error),
) *OAuthClientsStore {
	// Inject lookup handler only leaving registration unsupported
	return &OAuthClientsStore{
		getClient: getClient,
	}
}

// SupportsRegistration returns true if dynamic client registration is supported
func (s OAuthClientsStore) SupportsRegistration() bool {
	// Registration supported when a registration function is present
	return s.registerClient != nil
}
