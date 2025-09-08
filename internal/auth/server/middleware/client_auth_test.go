// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"trpc.group/trpc-go/trpc-mcp-go/internal/auth"
	srv "trpc.group/trpc-go/trpc-mcp-go/internal/auth/server"
	oauth "trpc.group/trpc-go/trpc-mcp-go/internal/errors"
)

// mockClientsStore is a test double for OAuthClientsStoreInterface that lets tests
// control lookups via a provided function
type mockClientsStore struct {
	get func(clientID string) (*auth.OAuthClientInformationFull, error)
}

// GetClient returns the mocked client info for the given client ID
func (m *mockClientsStore) GetClient(clientID string) (*auth.OAuthClientInformationFull, error) {
	return m.get(clientID)
}

// RegisterClient indicates dynamic client registration is not supported in this mock
func (m *mockClientsStore) RegisterClient(client auth.OAuthClientInformationFull) (*auth.OAuthClientInformationFull, error) {
	return nil, fmt.Errorf("dynamic client registration is not supported")
}

// runClientAuth executes a request through the client-authentication middleware and
// returns the recorder and whether the next handler was called
func runClientAuth(t *testing.T, store srv.OAuthClientsStoreInterface, body interface{}, contentType string) (rec *httptest.ResponseRecorder, nextCalled bool) {
	t.Helper()

	handlerCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		cli, ok := GetAuthenticatedClient(r)
		if !ok {
			t.Fatalf("authenticated client not found in context")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(struct {
			Success bool                             `json:"success"`
			Client  *auth.OAuthClientInformationFull `json:"client"`
		}{Success: true, Client: cli})
	})

	middleware := AuthenticateClient(ClientAuthenticationMiddlewareOptions{ClientsStore: store})
	h := middleware(next)

	var b []byte
	switch v := body.(type) {
	case []byte:
		b = v
	default:
		var err error
		b, err = json.Marshal(v)
		if err != nil {
			t.Fatalf("failed to marshal body: %v", err)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/protected", bytes.NewReader(b))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	} else {
		req.Header.Set("Content-Type", "application/json")
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec, handlerCalled
}

// decodeOAuthError parses an OAuthErrorResponse from the test recorder body
func decodeOAuthError(t *testing.T, rec *httptest.ResponseRecorder) *oauth.OAuthErrorResponse {
	t.Helper()
	var v oauth.OAuthErrorResponse
	_ = json.NewDecoder(rec.Body).Decode(&v)
	return &v
}

func TestAuthenticateClient_ValidCredentials(t *testing.T) {
	store := &mockClientsStore{get: func(clientID string) (*auth.OAuthClientInformationFull, error) {
		if clientID == "valid-client" {
			return &auth.OAuthClientInformationFull{OAuthClientMetadata: auth.OAuthClientMetadata{RedirectURIs: []string{"https://example.com/callback"}}, OAuthClientInformation: auth.OAuthClientInformation{ClientID: "valid-client", ClientSecret: "valid-secret"}}, nil
		}
		return nil, nil
	}}

	rec, nextCalled := runClientAuth(t, store, map[string]interface{}{"client_id": "valid-client", "client_secret": "valid-secret"}, "application/json")
	if rec.Code != http.StatusOK || !nextCalled {
		t.Fatalf("expected 200 and next called, got %d next=%v", rec.Code, nextCalled)
	}

	var body struct {
		Success bool `json:"success"`
		Client  struct {
			ClientID string `json:"client_id"`
		} `json:"client"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if !body.Success || body.Client.ClientID != "valid-client" {
		t.Fatalf("unexpected body: %+v", body)
	}
}

func TestAuthenticateClient_InvalidClientID(t *testing.T) {
	store := &mockClientsStore{get: func(clientID string) (*auth.OAuthClientInformationFull, error) { return nil, nil }}
	rec, nextCalled := runClientAuth(t, store, map[string]interface{}{"client_id": "non-existent-client", "client_secret": "some-secret"}, "application/json")
	if nextCalled || rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without next, got %d next=%v", rec.Code, nextCalled)
	}
	body := decodeOAuthError(t, rec)
	if body.Error != "invalid_client" || body.ErrorDescription != "invalid client credentials" {
		t.Fatalf("unexpected body: %+v", body)
	}
}

func TestAuthenticateClient_InvalidClientSecret(t *testing.T) {
	store := &mockClientsStore{get: func(clientID string) (*auth.OAuthClientInformationFull, error) {
		if clientID == "valid-client" {
			return &auth.OAuthClientInformationFull{OAuthClientInformation: auth.OAuthClientInformation{ClientID: "valid-client", ClientSecret: "valid-secret"}}, nil
		}
		return nil, nil
	}}
	rec, nextCalled := runClientAuth(t, store, map[string]interface{}{"client_id": "valid-client", "client_secret": "wrong-secret"}, "application/json")
	if nextCalled || rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without next, got %d next=%v", rec.Code, nextCalled)
	}
	body := decodeOAuthError(t, rec)
	if body.Error != "invalid_client" || body.ErrorDescription != "Invalid client_secret" {
		t.Fatalf("unexpected body: %+v", body)
	}
}

func TestAuthenticateClient_MissingClientID(t *testing.T) {
	store := &mockClientsStore{get: func(clientID string) (*auth.OAuthClientInformationFull, error) { return nil, nil }}
	rec, nextCalled := runClientAuth(t, store, map[string]interface{}{"client_secret": "valid-secret"}, "application/json")
	if nextCalled || rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 without next, got %d next=%v", rec.Code, nextCalled)
	}
	body := decodeOAuthError(t, rec)
	if body.Error != "invalid_request" {
		t.Fatalf("unexpected body: %+v", body)
	}
}

func TestAuthenticateClient_AllowsMissingSecretIfClientHasNone(t *testing.T) {
	store := &mockClientsStore{get: func(clientID string) (*auth.OAuthClientInformationFull, error) {
		if clientID == "expired-client" {
			return &auth.OAuthClientInformationFull{OAuthClientInformation: auth.OAuthClientInformation{ClientID: "expired-client"}}, nil
		}
		return nil, nil
	}}
	rec, nextCalled := runClientAuth(t, store, map[string]interface{}{"client_id": "expired-client"}, "application/json")
	if rec.Code != http.StatusOK || !nextCalled {
		t.Fatalf("expected 200 and next called, got %d next=%v", rec.Code, nextCalled)
	}
}

func TestAuthenticateClient_RejectsExpiredSecret(t *testing.T) {
	past := time.Now().Add(-1 * time.Hour).Unix()
	store := &mockClientsStore{get: func(clientID string) (*auth.OAuthClientInformationFull, error) {
		if clientID == "client-with-expired-secret" {
			return &auth.OAuthClientInformationFull{OAuthClientInformation: auth.OAuthClientInformation{ClientID: "client-with-expired-secret", ClientSecret: "expired-secret", ClientSecretExpiresAt: &past}}, nil
		}
		return nil, nil
	}}
	rec, nextCalled := runClientAuth(t, store, map[string]interface{}{"client_id": "client-with-expired-secret", "client_secret": "expired-secret"}, "application/json")
	if nextCalled || rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without next, got %d next=%v", rec.Code, nextCalled)
	}
	body := decodeOAuthError(t, rec)
	if body.Error != "invalid_client" || body.ErrorDescription != "Client secret has expired" {
		t.Fatalf("unexpected body: %+v", body)
	}
}

func TestAuthenticateClient_MalformedRequestBody(t *testing.T) {
	store := &mockClientsStore{get: func(clientID string) (*auth.OAuthClientInformationFull, error) { return nil, nil }}
	rec, nextCalled := runClientAuth(t, store, []byte("not-json-format"), "application/json")
	if nextCalled || rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 without next, got %d next=%v", rec.Code, nextCalled)
	}
}

func TestAuthenticateClient_IgnoresExtraFields(t *testing.T) {
	store := &mockClientsStore{get: func(clientID string) (*auth.OAuthClientInformationFull, error) {
		if clientID == "valid-client" {
			return &auth.OAuthClientInformationFull{OAuthClientInformation: auth.OAuthClientInformation{ClientID: "valid-client", ClientSecret: "valid-secret"}}, nil
		}
		return nil, nil
	}}
	rec, nextCalled := runClientAuth(t, store, map[string]interface{}{"client_id": "valid-client", "client_secret": "valid-secret", "extra_field": "ignored"}, "application/json")
	if rec.Code != http.StatusOK || !nextCalled {
		t.Fatalf("expected 200 and next called, got %d next=%v", rec.Code, nextCalled)
	}
}
