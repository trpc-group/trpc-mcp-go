// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package client

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"trpc.group/trpc-go/trpc-mcp-go/internal/auth"
)

func TestNewInMemoryOAuthClientProvider(t *testing.T) {
	redirectURL := "http://localhost:8080/callback"
	clientName := "Test Client"
	scope := "read write"
	clientMetadata := auth.OAuthClientMetadata{
		ClientName: &clientName, // Use pointer to string
		Scope:      &scope,      // Use pointer to string
	}

	// Test with onRedirect callback
	var redirectCalled bool
	onRedirect := func(u *url.URL) error {
		redirectCalled = true
		return nil
	}

	provider := NewInMemoryOAuthClientProvider(redirectURL, clientMetadata, onRedirect)

	if provider.redirectURL != redirectURL {
		t.Errorf("Expected redirectURL %s, got %s", redirectURL, provider.redirectURL)
	}

	if *provider.clientMetadata.ClientName != *clientMetadata.ClientName {
		t.Errorf("Expected clientMetadata.ClientName %s, got %s",
			*clientMetadata.ClientName, *provider.clientMetadata.ClientName)
	}

	if provider.onRedirect == nil {
		t.Error("Expected onRedirect to be set")
	}

	// Test that onRedirect was properly set by calling it
	testURL, _ := url.Parse("https://example.com")
	provider.onRedirect(testURL)
	if !redirectCalled {
		t.Error("Expected onRedirect to be called")
	}

	// Test with nil onRedirect
	provider2 := NewInMemoryOAuthClientProvider(redirectURL, clientMetadata, nil)
	if provider2.onRedirect == nil {
		t.Error("Expected default onRedirect to be set when nil is passed")
	}

	// Test default onRedirect doesn't panic
	err := provider2.onRedirect(&url.URL{})
	if err != nil {
		t.Errorf("Expected default onRedirect to return nil, got %v", err)
	}
}

func TestRedirectURL(t *testing.T) {
	redirectURL := "http://localhost:8080/callback"
	provider := NewInMemoryOAuthClientProvider(redirectURL, auth.OAuthClientMetadata{}, nil)

	if provider.RedirectURL() != redirectURL {
		t.Errorf("Expected %s, got %s", redirectURL, provider.RedirectURL())
	}
}

func TestClientMetadata(t *testing.T) {
	clientName := "Test Client"
	scope := "read write"
	clientMetadata := auth.OAuthClientMetadata{
		ClientName: &clientName,
		Scope:      &scope,
	}
	provider := NewInMemoryOAuthClientProvider("", clientMetadata, nil)

	result := provider.ClientMetadata()
	if *result.ClientName != *clientMetadata.ClientName {
		t.Errorf("Expected ClientName %s, got %s", *clientMetadata.ClientName, *result.ClientName)
	}
	if *result.Scope != *clientMetadata.Scope {
		t.Errorf("Expected Scope %s, got %s", *clientMetadata.Scope, *result.Scope)
	}
}

func TestClientInformation(t *testing.T) {
	provider := NewInMemoryOAuthClientProvider("", auth.OAuthClientMetadata{}, nil)

	// Test when no client information is saved
	clientInfo := provider.ClientInformation()
	if clientInfo != nil {
		t.Error("Expected nil client information when none is saved")
	}

	// Test after saving client information
	fullClientInfo := auth.OAuthClientInformationFull{
		OAuthClientInformation: auth.OAuthClientInformation{
			ClientID:              "test-client-id",
			ClientSecret:          "test-client-secret",
			ClientIDIssuedAt:      &[]int64{time.Now().Unix()}[0],
			ClientSecretExpiresAt: &[]int64{time.Now().Add(time.Hour * 24).Unix()}[0],
		},
	}

	err := provider.SaveClientInformation(fullClientInfo)
	if err != nil {
		t.Errorf("Expected no error saving client information, got %v", err)
	}

	clientInfo = provider.ClientInformation()
	if clientInfo == nil {
		t.Fatal("Expected client information to be saved")
	}

	if clientInfo.ClientID != fullClientInfo.ClientID {
		t.Errorf("Expected ClientID %s, got %s", fullClientInfo.ClientID, clientInfo.ClientID)
	}

	if clientInfo.ClientSecret != fullClientInfo.ClientSecret {
		t.Errorf("Expected ClientSecret %s, got %s", fullClientInfo.ClientSecret, clientInfo.ClientSecret)
	}
}

func TestTokens(t *testing.T) {
	provider := NewInMemoryOAuthClientProvider("", auth.OAuthClientMetadata{}, nil)

	// Test when no tokens are saved
	tokens, err := provider.Tokens()
	if err != nil {
		t.Errorf("Expected no error getting tokens, got %v", err)
	}
	if tokens != nil {
		t.Error("Expected nil tokens when none are saved")
	}

	// Test saving and retrieving tokens
	testTokens := auth.OAuthTokens{
		AccessToken:  "test-access-token",
		RefreshToken: &[]string{"test-refresh-token"}[0], // Use pointer
		TokenType:    "Bearer",
		ExpiresIn:    &[]int64{3600}[0], // Use pointer
	}

	err = provider.SaveTokens(testTokens)
	if err != nil {
		t.Errorf("Expected no error saving tokens, got %v", err)
	}

	tokens, err = provider.Tokens()
	if err != nil {
		t.Errorf("Expected no error getting tokens, got %v", err)
	}

	if tokens == nil {
		t.Fatal("Expected tokens to be saved")
	}

	if tokens.AccessToken != testTokens.AccessToken {
		t.Errorf("Expected AccessToken %s, got %s", testTokens.AccessToken, tokens.AccessToken)
	}

	if *tokens.RefreshToken != *testTokens.RefreshToken {
		t.Errorf("Expected RefreshToken %s, got %s", *testTokens.RefreshToken, *tokens.RefreshToken)
	}
}

func TestRedirectToAuthorization(t *testing.T) {
	var capturedURL *url.URL
	onRedirect := func(u *url.URL) error {
		capturedURL = u
		return nil
	}

	provider := NewInMemoryOAuthClientProvider("", auth.OAuthClientMetadata{}, onRedirect)

	testURL, _ := url.Parse("https://auth.example.com/authorize?response_type=code&client_id=123")
	err := provider.RedirectToAuthorization(testURL)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if capturedURL == nil {
		t.Error("Expected onRedirect to be called")
	}

	if capturedURL.String() != testURL.String() {
		t.Errorf("Expected URL %s, got %s", testURL.String(), capturedURL.String())
	}
}

func TestCodeVerifier(t *testing.T) {
	provider := NewInMemoryOAuthClientProvider("", auth.OAuthClientMetadata{}, nil)

	// Test when no code verifier is saved
	verifier, err := provider.CodeVerifier()
	if err == nil {
		t.Error("Expected error when no code verifier is saved")
	}
	if verifier != "" {
		t.Error("Expected empty verifier when none is saved")
	}

	// Test saving and retrieving code verifier
	testVerifier := "test-code-verifier-12345"
	err = provider.SaveCodeVerifier(testVerifier)
	if err != nil {
		t.Errorf("Expected no error saving code verifier, got %v", err)
	}

	verifier, err = provider.CodeVerifier()
	if err != nil {
		t.Errorf("Expected no error getting code verifier, got %v", err)
	}

	if verifier != testVerifier {
		t.Errorf("Expected verifier %s, got %s", testVerifier, verifier)
	}
}

func TestState(t *testing.T) {
	provider := NewInMemoryOAuthClientProvider("", auth.OAuthClientMetadata{}, nil)

	state1, err := provider.State()
	if err != nil {
		t.Errorf("Expected no error generating state, got %v", err)
	}

	if state1 == "" {
		t.Error("Expected non-empty state")
	}

	// Test that subsequent calls generate different states
	state2, err := provider.State()
	if err != nil {
		t.Errorf("Expected no error generating state, got %v", err)
	}

	if state1 == state2 {
		t.Error("Expected different states on subsequent calls")
	}

	/// Test that state is base64 URL encoded (no + or / characters, = padding is allowed)
	if strings.Contains(state1, "+") || strings.Contains(state1, "/") {
		t.Error("Expected URL-safe base64 encoding (no + or / characters)")
	}
}

func TestAddClientAuthentication(t *testing.T) {
	provider := NewInMemoryOAuthClientProvider("", auth.OAuthClientMetadata{}, nil)

	// Test with no client information
	headers := make(http.Header)
	params := make(url.Values)
	err := provider.AddClientAuthentication(headers, params, "https://token.example.com")
	if err != nil {
		t.Errorf("Expected no error with no client info, got %v", err)
	}

	// Should not add any parameters when no client info
	if params.Get("client_id") != "" {
		t.Error("Expected no client_id when no client info is saved")
	}

	// Test with client information
	clientInfo := auth.OAuthClientInformationFull{
		OAuthClientInformation: auth.OAuthClientInformation{
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
		},
	}
	err = provider.SaveClientInformation(clientInfo)
	if err != nil {
		t.Errorf("Expected no error saving client info, got %v", err)
	}

	params = make(url.Values)
	err = provider.AddClientAuthentication(headers, params, "https://token.example.com")
	if err != nil {
		t.Errorf("Expected no error with client info, got %v", err)
	}

	if params.Get("client_id") != clientInfo.ClientID {
		t.Errorf("Expected client_id %s, got %s", clientInfo.ClientID, params.Get("client_id"))
	}

	if params.Get("client_secret") != clientInfo.ClientSecret {
		t.Errorf("Expected client_secret %s, got %s", clientInfo.ClientSecret, params.Get("client_secret"))
	}

	// Test with only client ID (no secret)
	clientInfoNoSecret := auth.OAuthClientInformationFull{
		OAuthClientInformation: auth.OAuthClientInformation{
			ClientID: "test-client-id-only",
		},
	}
	err = provider.SaveClientInformation(clientInfoNoSecret)
	if err != nil {
		t.Errorf("Expected no error saving client info, got %v", err)
	}

	params = make(url.Values)
	err = provider.AddClientAuthentication(headers, params, "https://token.example.com")
	if err != nil {
		t.Errorf("Expected no error with client info, got %v", err)
	}

	if params.Get("client_id") != clientInfoNoSecret.ClientID {
		t.Errorf("Expected client_id %s, got %s", clientInfoNoSecret.ClientID, params.Get("client_id"))
	}

	if params.Get("client_secret") != "" {
		t.Error("Expected no client_secret when not provided")
	}
}

func TestValidateResourceURL(t *testing.T) {
	provider := NewInMemoryOAuthClientProvider("", auth.OAuthClientMetadata{}, nil)
	serverURL, _ := url.Parse("https://api.example.com")

	// Test with nil resource metadata
	result, err := provider.ValidateResourceURL(serverURL, nil)
	if err != nil {
		t.Errorf("Expected no error with nil metadata, got %v", err)
	}
	if result != nil {
		t.Error("Expected nil result with nil metadata")
	}

	// Test with valid resource URL (same origin)
	resourceMetadata := &auth.OAuthProtectedResourceMetadata{
		Resource: "https://api.example.com/data",
	}

	result, err = provider.ValidateResourceURL(serverURL, resourceMetadata)
	if err != nil {
		t.Errorf("Expected no error with valid resource URL, got %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if result.String() != resourceMetadata.Resource {
		t.Errorf("Expected resource URL %s, got %s", resourceMetadata.Resource, result.String())
	}

	// Test with valid resource URL (subpath)
	resourceMetadata2 := &auth.OAuthProtectedResourceMetadata{
		Resource: "https://api.example.com/v1/users",
	}

	result, err = provider.ValidateResourceURL(serverURL, resourceMetadata2)
	if err != nil {
		t.Errorf("Expected no error with valid subpath resource URL, got %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Test with different scheme
	resourceMetadata3 := &auth.OAuthProtectedResourceMetadata{
		Resource: "http://api.example.com/data",
	}

	result, err = provider.ValidateResourceURL(serverURL, resourceMetadata3)
	if err == nil {
		t.Error("Expected error with different scheme")
	}
	if result != nil {
		t.Error("Expected nil result with invalid URL")
	}

	// Test with different host
	resourceMetadata4 := &auth.OAuthProtectedResourceMetadata{
		Resource: "https://other.example.com/data",
	}

	result, err = provider.ValidateResourceURL(serverURL, resourceMetadata4)
	if err == nil {
		t.Error("Expected error with different host")
	}
	if result != nil {
		t.Error("Expected nil result with invalid URL")
	}

	// Test with invalid URL
	resourceMetadata5 := &auth.OAuthProtectedResourceMetadata{
		Resource: "://invalid-url",
	}

	result, err = provider.ValidateResourceURL(serverURL, resourceMetadata5)
	if err == nil {
		t.Error("Expected error with invalid URL")
	}
	if result != nil {
		t.Error("Expected nil result with invalid URL")
	}
}

func TestInvalidateCredentials(t *testing.T) {
	provider := NewInMemoryOAuthClientProvider("", auth.OAuthClientMetadata{}, nil)

	// Set up some data first
	clientInfo := auth.OAuthClientInformationFull{
		OAuthClientInformation: auth.OAuthClientInformation{
			ClientID:     "test-client",
			ClientSecret: "test-secret",
		},
	}
	provider.SaveClientInformation(clientInfo)

	tokens := auth.OAuthTokens{
		AccessToken: "test-access-token",
	}
	provider.SaveTokens(tokens)

	provider.SaveCodeVerifier("test-verifier")

	// Test invalidating all credentials
	err := provider.InvalidateCredentials("all")
	if err != nil {
		t.Errorf("Expected no error invalidating all credentials, got %v", err)
	}

	if provider.ClientInformation() != nil {
		t.Error("Expected client information to be cleared")
	}

	savedTokens, _ := provider.Tokens()
	if savedTokens != nil {
		t.Error("Expected tokens to be cleared")
	}

	_, err = provider.CodeVerifier()
	if err == nil {
		t.Error("Expected error getting code verifier after clearing")
	}

	// Set up data again for individual tests
	provider.SaveClientInformation(clientInfo)
	provider.SaveTokens(tokens)
	provider.SaveCodeVerifier("test-verifier")

	// Test invalidating only client info
	err = provider.InvalidateCredentials("client")
	if err != nil {
		t.Errorf("Expected no error invalidating client credentials, got %v", err)
	}

	if provider.ClientInformation() != nil {
		t.Error("Expected client information to be cleared")
	}

	savedTokens, _ = provider.Tokens()
	if savedTokens == nil {
		t.Error("Expected tokens to remain")
	}

	// Test invalidating only tokens
	provider.SaveClientInformation(clientInfo) // Restore client info
	err = provider.InvalidateCredentials("tokens")
	if err != nil {
		t.Errorf("Expected no error invalidating tokens, got %v", err)
	}

	savedTokens, _ = provider.Tokens()
	if savedTokens != nil {
		t.Error("Expected tokens to be cleared")
	}

	if provider.ClientInformation() == nil {
		t.Error("Expected client information to remain")
	}

	// Test invalidating only code verifier
	provider.SaveTokens(tokens)                // Restore tokens
	provider.SaveCodeVerifier("test-verifier") // Restore verifier

	err = provider.InvalidateCredentials("verifier")
	if err != nil {
		t.Errorf("Expected no error invalidating verifier, got %v", err)
	}

	_, err = provider.CodeVerifier()
	if err == nil {
		t.Error("Expected error getting code verifier after clearing")
	}

	// Other data should remain
	if provider.ClientInformation() == nil {
		t.Error("Expected client information to remain")
	}

	savedTokens, _ = provider.Tokens()
	if savedTokens == nil {
		t.Error("Expected tokens to remain")
	}

	// Test invalid scope
	err = provider.InvalidateCredentials("invalid-scope")
	if err == nil {
		t.Error("Expected error with invalid scope")
	}

	if !strings.Contains(err.Error(), "unknown invalidation scope") {
		t.Errorf("Expected 'unknown invalidation scope' error, got %v", err)
	}
}

func TestConcurrentAccess(t *testing.T) {
	provider := NewInMemoryOAuthClientProvider("", auth.OAuthClientMetadata{}, nil)

	// Test concurrent access to ensure thread safety
	var wg sync.WaitGroup
	numGoroutines := 10

	// Test concurrent writes
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			clientInfo := auth.OAuthClientInformationFull{
				OAuthClientInformation: auth.OAuthClientInformation{
					ClientID:     fmt.Sprintf("client-%d", id),
					ClientSecret: fmt.Sprintf("secret-%d", id),
				},
			}
			provider.SaveClientInformation(clientInfo)

			tokens := auth.OAuthTokens{
				AccessToken: fmt.Sprintf("token-%d", id),
			}
			provider.SaveTokens(tokens)

			provider.SaveCodeVerifier(fmt.Sprintf("verifier-%d", id))
		}(i)
	}

	wg.Wait()

	// Test concurrent reads
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()

			provider.ClientInformation()
			provider.Tokens()
			provider.CodeVerifier()
		}()
	}

	wg.Wait()

	// Test concurrent invalidation
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			scopes := []string{"client", "tokens", "verifier", "all"}
			scope := scopes[id%len(scopes)]
			provider.InvalidateCredentials(scope)
		}(i)
	}

	wg.Wait()
}

func TestStateRandomness(t *testing.T) {
	provider := NewInMemoryOAuthClientProvider("", auth.OAuthClientMetadata{}, nil)

	// Generate multiple states and ensure they're different
	states := make(map[string]bool)
	for i := 0; i < 10; i++ {
		state, err := provider.State()
		if err != nil {
			t.Errorf("Expected no error generating state, got %v", err)
		}

		if states[state] {
			t.Errorf("Generated duplicate state: %s", state)
		}
		states[state] = true

		// Verify length (32 bytes base64 encoded should be ~43 characters)
		if len(state) < 40 {
			t.Errorf("Expected state length >= 40, got %d", len(state))
		}
	}
}

func TestRedirectToAuthorizationError(t *testing.T) {
	expectedErr := fmt.Errorf("redirect failed")
	onRedirect := func(u *url.URL) error {
		return expectedErr
	}

	provider := NewInMemoryOAuthClientProvider("", auth.OAuthClientMetadata{}, onRedirect)

	testURL, _ := url.Parse("https://auth.example.com/authorize")
	err := provider.RedirectToAuthorization(testURL)

	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
}

func TestMutexProtection(t *testing.T) {
	provider := NewInMemoryOAuthClientProvider("", auth.OAuthClientMetadata{}, nil)

	// Test that mutex properly protects against race conditions
	// This test runs multiple operations concurrently and ensures no data races
	var wg sync.WaitGroup
	numOperations := 100

	wg.Add(numOperations)
	for i := 0; i < numOperations; i++ {
		go func(id int) {
			defer wg.Done()

			// Mix of read and write operations
			switch id % 4 {
			case 0:
				clientInfo := auth.OAuthClientInformationFull{
					OAuthClientInformation: auth.OAuthClientInformation{
						ClientID: fmt.Sprintf("client-%d", id),
					},
				}
				provider.SaveClientInformation(clientInfo)
			case 1:
				provider.ClientInformation()
			case 2:
				tokens := auth.OAuthTokens{
					AccessToken: fmt.Sprintf("token-%d", id),
				}
				provider.SaveTokens(tokens)
			case 3:
				provider.Tokens()
			}
		}(i)
	}

	wg.Wait()

	// If we reach here without data races, the mutex protection is working
}

// Benchmark tests
func BenchmarkState(b *testing.B) {
	provider := NewInMemoryOAuthClientProvider("", auth.OAuthClientMetadata{}, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := provider.State()
		if err != nil {
			b.Fatalf("Error generating state: %v", err)
		}
	}
}

func BenchmarkConcurrentAccess(b *testing.B) {
	provider := NewInMemoryOAuthClientProvider("", auth.OAuthClientMetadata{}, nil)

	// Setup initial data
	clientName := "bench-client"
	clientInfo := auth.OAuthClientInformationFull{
		OAuthClientMetadata: auth.OAuthClientMetadata{
			ClientName: &clientName,
		},
		OAuthClientInformation: auth.OAuthClientInformation{
			ClientID:     "bench-client-id",
			ClientSecret: "bench-secret",
		},
	}
	provider.SaveClientInformation(clientInfo)

	tokens := auth.OAuthTokens{
		AccessToken: "bench-token",
	}
	provider.SaveTokens(tokens)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Mix of read operations
			provider.ClientInformation()
			provider.Tokens()
		}
	})
}
