// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package server

import (
	"errors"
	"testing"

	"trpc.group/trpc-go/trpc-mcp-go/internal/auth"
)

// Test errors used by mocks
var (
	ErrClientNotFound       = errors.New("client not found")
	ErrGetClientFailed      = errors.New("get client failed")
	ErrRegisterClientFailed = errors.New("register client failed")
)

// mockGetClientSuccess returns a known client for id existing-client or ErrClientNotFound
func mockGetClientSuccess(clientID string) (*auth.OAuthClientInformationFull, error) {
	if clientID == "existing-client" {
		return &auth.OAuthClientInformationFull{
			OAuthClientInformation: auth.OAuthClientInformation{
				ClientID:     clientID,
				ClientSecret: "secret-123",
			},
			OAuthClientMetadata: auth.OAuthClientMetadata{
				RedirectURIs: []string{"https://example.com/callback"},
			},
		}, nil
	}
	return nil, ErrClientNotFound
}

// mockGetClientError always returns ErrGetClientFailed
func mockGetClientError(clientID string) (*auth.OAuthClientInformationFull, error) {
	return nil, ErrGetClientFailed
}

// mockRegisterClientSuccess simulates server generating client_id and client_secret
func mockRegisterClientSuccess(client auth.OAuthClientInformationFull) (*auth.OAuthClientInformationFull, error) {
	client.ClientID = "generated-client-id"
	client.ClientSecret = "generated-secret"
	return &client, nil
}

// mockRegisterClientError always returns ErrRegisterClientFailed
func mockRegisterClientError(client auth.OAuthClientInformationFull) (*auth.OAuthClientInformationFull, error) {
	return nil, ErrRegisterClientFailed
}

func TestNewOAuthClientStore(t *testing.T) {
	store := NewOAuthClientStore(mockGetClientSuccess)

	if store == nil {
		t.Fatal("NewOAuthClientStore returned nil")
	}

	// Should not support registration
	if store.SupportsRegistration() {
		t.Error("basic store should not support registration")
	}

	// GetClient should work
	client, err := store.GetClient("existing-client")
	if err != nil {
		t.Fatalf("GetClient failed: %v", err)
	}
	if client.ClientID != "existing-client" {
		t.Errorf("expected client ID 'existing-client', got %s", client.ClientID)
	}

	// RegisterClient should return error
	newClient := auth.OAuthClientInformationFull{
		OAuthClientMetadata: auth.OAuthClientMetadata{
			RedirectURIs: []string{"https://test.com/callback"},
		},
	}
	_, err = store.RegisterClient(newClient)
	if err == nil {
		t.Error("RegisterClient should fail on basic store")
	}
	if err.Error() != "dynamic client registration is not supported" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestNewOAuthClientStoreSupportDynamicRegistration(t *testing.T) {
	store := NewOAuthClientStoreSupportDynamicRegistration(mockGetClientSuccess, mockRegisterClientSuccess)

	if store == nil {
		t.Fatal("NewOAuthClientStoreSupportDynamicRegistration returned nil")
	}

	// Should support registration
	if !store.SupportsRegistration() {
		t.Error("dynamic registration store should support registration")
	}

	// GetClient should work
	client, err := store.GetClient("existing-client")
	if err != nil {
		t.Fatalf("GetClient failed: %v", err)
	}
	if client.ClientID != "existing-client" {
		t.Errorf("expected client ID 'existing-client', got %s", client.ClientID)
	}

	// RegisterClient should work
	newClient := auth.OAuthClientInformationFull{
		OAuthClientMetadata: auth.OAuthClientMetadata{
			RedirectURIs: []string{"https://test.com/callback"},
		},
	}
	registered, err := store.RegisterClient(newClient)
	if err != nil {
		t.Fatalf("RegisterClient failed: %v", err)
	}
	if registered.ClientID != "generated-client-id" {
		t.Errorf("expected generated client ID, got %s", registered.ClientID)
	}
	if registered.ClientSecret != "generated-secret" {
		t.Errorf("expected generated secret, got %s", registered.ClientSecret)
	}
}

func TestOAuthClientsStore_GetClient_Success(t *testing.T) {
	store := NewOAuthClientStore(mockGetClientSuccess)

	// Test existing client
	client, err := store.GetClient("existing-client")
	if err != nil {
		t.Fatalf("GetClient failed: %v", err)
	}
	if client.ClientID != "existing-client" {
		t.Errorf("expected client ID 'existing-client', got %s", client.ClientID)
	}
	if client.ClientSecret != "secret-123" {
		t.Errorf("expected secret 'secret-123', got %s", client.ClientSecret)
	}
	if len(client.RedirectURIs) != 1 || client.RedirectURIs[0] != "https://example.com/callback" {
		t.Errorf("unexpected redirect URIs: %v", client.RedirectURIs)
	}
}

func TestOAuthClientsStore_GetClient_NotFound(t *testing.T) {
	store := NewOAuthClientStore(mockGetClientSuccess)

	// Test non-existing client
	client, err := store.GetClient("non-existing-client")
	if err != ErrClientNotFound {
		t.Errorf("expected ErrClientNotFound, got %v", err)
	}
	if client != nil {
		t.Error("expected nil client for not found case")
	}
}

func TestOAuthClientsStore_GetClient_Error(t *testing.T) {
	store := NewOAuthClientStore(mockGetClientError)

	// Test error case
	client, err := store.GetClient("any-client")
	if err != ErrGetClientFailed {
		t.Errorf("expected ErrGetClientFailed, got %v", err)
	}
	if client != nil {
		t.Error("expected nil client for error case")
	}
}

func TestOAuthClientsStore_RegisterClient_Supported(t *testing.T) {
	store := NewOAuthClientStoreSupportDynamicRegistration(mockGetClientSuccess, mockRegisterClientSuccess)

	newClient := auth.OAuthClientInformationFull{
		OAuthClientMetadata: auth.OAuthClientMetadata{
			RedirectURIs: []string{"https://test.com/callback"},
			ClientName:   stringPtr("Test Client"),
		},
	}

	registered, err := store.RegisterClient(newClient)
	if err != nil {
		t.Fatalf("RegisterClient failed: %v", err)
	}

	if registered.ClientID != "generated-client-id" {
		t.Errorf("expected generated client ID, got %s", registered.ClientID)
	}
	if registered.ClientSecret != "generated-secret" {
		t.Errorf("expected generated secret, got %s", registered.ClientSecret)
	}
	// Original metadata should be preserved
	if registered.ClientName == nil || *registered.ClientName != "Test Client" {
		t.Errorf("client name not preserved")
	}
}

func TestOAuthClientsStore_RegisterClient_NotSupported(t *testing.T) {
	store := NewOAuthClientStore(mockGetClientSuccess)

	newClient := auth.OAuthClientInformationFull{
		OAuthClientMetadata: auth.OAuthClientMetadata{
			RedirectURIs: []string{"https://test.com/callback"},
		},
	}

	registered, err := store.RegisterClient(newClient)
	if err == nil {
		t.Error("RegisterClient should fail when not supported")
	}
	if err.Error() != "dynamic client registration is not supported" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
	if registered != nil {
		t.Error("expected nil result for unsupported registration")
	}
}

func TestOAuthClientsStore_RegisterClient_Error(t *testing.T) {
	store := NewOAuthClientStoreSupportDynamicRegistration(mockGetClientSuccess, mockRegisterClientError)

	newClient := auth.OAuthClientInformationFull{
		OAuthClientMetadata: auth.OAuthClientMetadata{
			RedirectURIs: []string{"https://test.com/callback"},
		},
	}

	registered, err := store.RegisterClient(newClient)
	if err != ErrRegisterClientFailed {
		t.Errorf("expected ErrRegisterClientFailed, got %v", err)
	}
	if registered != nil {
		t.Error("expected nil result for failed registration")
	}
}

func TestOAuthClientsStore_SupportsRegistration(t *testing.T) {
	// Test store without registration support
	basicStore := NewOAuthClientStore(mockGetClientSuccess)
	if basicStore.SupportsRegistration() {
		t.Error("basic store should not support registration")
	}

	// Test store with registration support
	dynamicStore := NewOAuthClientStoreSupportDynamicRegistration(mockGetClientSuccess, mockRegisterClientSuccess)
	if !dynamicStore.SupportsRegistration() {
		t.Error("dynamic store should support registration")
	}
}

func TestOAuthClientsStore_InterfaceCompliance(t *testing.T) {
	// Test that OAuthClientsStore implements OAuthClientsStoreInterface
	var _ OAuthClientsStoreInterface = &OAuthClientsStore{}

	// Test that stores with registration support implement SupportDynamicClientRegistration
	dynamicStore := NewOAuthClientStoreSupportDynamicRegistration(mockGetClientSuccess, mockRegisterClientSuccess)
	var _ SupportDynamicClientRegistration = dynamicStore

	// Verify interface methods work as expected
	store := NewOAuthClientStoreSupportDynamicRegistration(mockGetClientSuccess, mockRegisterClientSuccess)

	// Test as OAuthClientsStoreInterface
	var iface OAuthClientsStoreInterface = store
	client, err := iface.GetClient("existing-client")
	if err != nil {
		t.Fatalf("interface GetClient failed: %v", err)
	}
	if client.ClientID != "existing-client" {
		t.Errorf("interface GetClient returned wrong client ID")
	}

	// Test as SupportDynamicClientRegistration
	var regIface SupportDynamicClientRegistration = store
	newClient := auth.OAuthClientInformationFull{
		OAuthClientMetadata: auth.OAuthClientMetadata{
			RedirectURIs: []string{"https://test.com/callback"},
		},
	}
	registered, err := regIface.RegisterClient(newClient)
	if err != nil {
		t.Fatalf("interface RegisterClient failed: %v", err)
	}
	if registered.ClientID != "generated-client-id" {
		t.Errorf("interface RegisterClient returned wrong client ID")
	}
}

func TestOAuthClientsStore_EdgeCases(t *testing.T) {
	t.Run("empty client ID", func(t *testing.T) {
		store := NewOAuthClientStore(mockGetClientSuccess)
		client, err := store.GetClient("")
		if err != ErrClientNotFound {
			t.Errorf("expected ErrClientNotFound for empty client ID, got %v", err)
		}
		if client != nil {
			t.Error("expected nil client for empty client ID")
		}
	})

	t.Run("register client with empty metadata", func(t *testing.T) {
		store := NewOAuthClientStoreSupportDynamicRegistration(mockGetClientSuccess, mockRegisterClientSuccess)

		// Test registering a client with minimal metadata
		emptyClient := auth.OAuthClientInformationFull{}
		registered, err := store.RegisterClient(emptyClient)
		if err != nil {
			t.Errorf("registering empty client failed: %v", err)
		}
		if registered.ClientID != "generated-client-id" {
			t.Errorf("expected generated client ID even for empty client")
		}
	})
}

// stringPtr returns a pointer to s
func stringPtr(s string) *string {
	return &s
}
