// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth"
)

// mockDynClientStore is a test double implementation of a dynamic client store
// It captures inputs, simulates errors, and returns controlled responses for testing
type mockDynClientStore struct {
	wantErr        error                            // optional error to return on RegisterClient
	lastRegistered *auth.OAuthClientInformationFull // last registered client captured during call
	returnedClient *auth.OAuthClientInformationFull // client to return instead of echoing input
	callCount      int                              // number of times RegisterClient was invoked
}

// RegisterClient mocks the dynamic client registration behavior
func (m *mockDynClientStore) RegisterClient(in auth.OAuthClientInformationFull) (*auth.OAuthClientInformationFull, error) {
	m.callCount++
	// capture input
	tmp := in
	m.lastRegistered = &tmp

	// if configured, return a forced error
	if m.wantErr != nil {
		return nil, m.wantErr
	}

	// if configured, return a pre-set client
	if m.returnedClient != nil {
		return m.returnedClient, nil
	}

	// by default echo back
	return &in, nil
}

// postJSONBody is a helper that sends an HTTP POST request with a JSON body
func postJSONBody(t *testing.T, h http.Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()

	var buf bytes.Buffer

	// encode request body as JSON if provided
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}

	// build POST request
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", "application/json")

	// record the response
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestClientRegistration_NotImplemented_WhenNoStore(t *testing.T) {
	// ClientsStore==nil -> 501 Not Implemented (Explicitly returns that dynamic registration is not supported)
	h := ClientRegistrationHandler(ClientRegistrationHandlerOptions{})

	rr := postJSONBody(t, h, "/register", map[string]any{
		"token_endpoint_auth_method": "client_secret_post",
		"redirect_uris":              []string{"https://cb"},
	})
	assert.Equal(t, http.StatusNotImplemented, rr.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "unsupported_grant_type", resp["error"])
}

func TestClientRegistration_MethodNotAllowed_Get405(t *testing.T) {
	// Only POST is allowed, other methods will return 405
	store := &mockDynClientStore{}
	h := ClientRegistrationHandler(ClientRegistrationHandlerOptions{
		ClientsStore: store,
	})

	req := httptest.NewRequest(http.MethodGet, "/register", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestClientRegistration_InvalidJSON_400(t *testing.T) {
	// JSON parsing failed -> 400 + invalid_client_metadata (determined within core logic)
	store := &mockDynClientStore{}
	h := ClientRegistrationHandler(ClientRegistrationHandlerOptions{ClientsStore: store})

	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBufferString("{bad json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "invalid_client_metadata", resp["error"])
}

func TestClientRegistration_MetadataValidation_400(t *testing.T) {
	// Missing required fields -> validateClientMetadata returns an error -> 400 (token_endpoint_auth_method and redirect_uris required)
	store := &mockDynClientStore{}
	h := ClientRegistrationHandler(ClientRegistrationHandlerOptions{ClientsStore: store})

	rr := postJSONBody(t, h, "/register", map[string]any{
		"redirect_uris": []string{}, // invalid
	})
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var resp map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.Equal(t, "invalid_client_metadata", resp["error"])
}

func TestClientRegistration_PublicClient_NoSecret(t *testing.T) {
	// token_endpoint_auth_method == "none" -> Public client: does not generate client_secret and does not set the expiration field
	store := &mockDynClientStore{}
	h := ClientRegistrationHandler(ClientRegistrationHandlerOptions{ClientsStore: store})

	rr := postJSONBody(t, h, "/register", map[string]any{
		"token_endpoint_auth_method": "none",
		"redirect_uris":              []string{"https://cb"},
	})
	assert.Equal(t, http.StatusCreated, rr.Code)

	var resp auth.OAuthClientInformationFull
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))

	// By default, the server generates client_id / client_id_issued_at (ClientIdGeneration is true by default)
	assert.NotEmpty(t, resp.OAuthClientInformation.ClientID)
	assert.NotNil(t, resp.OAuthClientInformation.ClientIDIssuedAt)

	// Public clients have no secrets and no expiration
	assert.Equal(t, "", resp.OAuthClientInformation.ClientSecret)
	assert.Nil(t, resp.OAuthClientInformation.ClientSecretExpiresAt)
}

func TestClientRegistration_ConfidentialClient_GenerateSecret_And_Expiry(t *testing.T) {
	// Non-public clients generate a 32-byte hex secret (length 64) and set an expiration time (default 30 days)
	store := &mockDynClientStore{}
	h := ClientRegistrationHandler(ClientRegistrationHandlerOptions{ClientsStore: store})

	start := time.Now().Unix()
	rr := postJSONBody(t, h, "/register", map[string]any{
		"token_endpoint_auth_method": "client_secret_post",
		"redirect_uris":              []string{"https://cb"},
	})
	assert.Equal(t, http.StatusCreated, rr.Code)

	var resp auth.OAuthClientInformationFull
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))

	secret := resp.OAuthClientInformation.ClientSecret
	if assert.NotEmpty(t, secret) {
		assert.Len(t, secret, 64) // 32 bytes hex -> 64 chars
	}
	if assert.NotNil(t, resp.OAuthClientInformation.ClientSecretExpiresAt) {
		exp := *resp.OAuthClientInformation.ClientSecretExpiresAt
		assert.Greater(t, exp, start)
	}
}

func TestClientRegistration_ConfidentialClient_NoExpiryWhenZeroConfig(t *testing.T) {
	// ClientSecretExpirySeconds==0 -> Does not expire (value is 0)
	store := &mockDynClientStore{}
	zero := 0
	h := ClientRegistrationHandler(ClientRegistrationHandlerOptions{
		ClientsStore:              store,
		ClientSecretExpirySeconds: &zero,
	})

	rr := postJSONBody(t, h, "/register", map[string]any{
		"token_endpoint_auth_method": "client_secret_basic",
		"redirect_uris":              []string{"https://cb"},
	})
	assert.Equal(t, http.StatusCreated, rr.Code)

	var resp auth.OAuthClientInformationFull
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	if assert.NotNil(t, resp.OAuthClientInformation.ClientSecretExpiresAt) {
		assert.Equal(t, int64(0), *resp.OAuthClientInformation.ClientSecretExpiresAt)
	}
}

func TestClientRegistration_RegisterError_500(t *testing.T) {
	// RegisterClient error -> 500 server_error
	store := &mockDynClientStore{wantErr: assert.AnError}
	h := ClientRegistrationHandler(ClientRegistrationHandlerOptions{ClientsStore: store})

	rr := postJSONBody(t, h, "/register", map[string]any{
		"token_endpoint_auth_method": "client_secret_post",
		"redirect_uris":              []string{"https://cb"},
	})
	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "server_error", resp["error"])
}

func TestClientRegistration_RateLimit_429_WhenEnabled(t *testing.T) {
	// Explicitly enable current limiting configuration: Max=1, the second request in the same window will result in a 429 error
	store := &mockDynClientStore{}
	h := ClientRegistrationHandler(ClientRegistrationHandlerOptions{
		ClientsStore: store,
		RateLimit: &RegisterRateLimitConfig{
			WindowMs: 60_000,
			Max:      1,
			Message:  "too many",
		},
	})

	// First request OK
	rr1 := postJSONBody(t, h, "/register", map[string]any{
		"token_endpoint_auth_method": "client_secret_post",
		"redirect_uris":              []string{"https://cb"},
	})
	assert.Equal(t, http.StatusCreated, rr1.Code)

	// Second immediate request -> 429
	rr2 := postJSONBody(t, h, "/register", map[string]any{
		"token_endpoint_auth_method": "client_secret_post",
		"redirect_uris":              []string{"https://cb"},
	})
	assert.Equal(t, http.StatusTooManyRequests, rr2.Code)
}

// Extra: Verified that rate limiting is not applied when RateLimit=nil to avoid interfering with other tests.
func TestClientRegistration_NoRateLimitByDefault(t *testing.T) {
	store := &mockDynClientStore{}
	h := ClientRegistrationHandler(ClientRegistrationHandlerOptions{
		ClientsStore: store,
		// RateLimit omitted -> unlimited flow (create limiter and wrap middleware only when it is not nil)
	})

	for i := 0; i < 3; i++ {
		rr := postJSONBody(t, h, "/register", map[string]any{
			"token_endpoint_auth_method": "client_secret_post",
			"redirect_uris":              []string{"https://cb"},
		})
		assert.Equal(t, http.StatusCreated, rr.Code)
	}
}

// When a custom current limiting configuration is in effect,
// the window parameter conversion logic using rate.Limiter will not panic
func TestClientRegistration_CustomLimiter_DoesNotPanic(t *testing.T) {
	store := &mockDynClientStore{}
	cfg := &RegisterRateLimitConfig{WindowMs: 1000, Max: 2}
	h := ClientRegistrationHandler(ClientRegistrationHandlerOptions{
		ClientsStore: store,
		RateLimit:    cfg,
	})

	// Manually construct 2 requests, the third one should be 429
	rr1 := postJSONBody(t, h, "/register", map[string]any{
		"token_endpoint_auth_method": "client_secret_post",
		"redirect_uris":              []string{"https://cb"},
	})
	assert.Equal(t, http.StatusCreated, rr1.Code)

	rr2 := postJSONBody(t, h, "/register", map[string]any{
		"token_endpoint_auth_method": "client_secret_post",
		"redirect_uris":              []string{"https://cb"},
	})
	assert.Equal(t, http.StatusCreated, rr2.Code)

	rr3 := postJSONBody(t, h, "/register", map[string]any{
		"token_endpoint_auth_method": "client_secret_post",
		"redirect_uris":              []string{"https://cb"},
	})
	assert.Equal(t, http.StatusTooManyRequests, rr3.Code)

	// Extra sanity check
	_ = rate.NewLimiter(rate.Every(time.Second/time.Duration(cfg.Max)), cfg.Max)
}
