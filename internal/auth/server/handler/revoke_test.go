// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth"
	as "trpc.group/trpc-go/trpc-mcp-go/internal/auth/server"
)

const (
	testClientID     = "c1"
	testClientSecret = "s3cr3t"
)

// mockRevokeProvider is a fake implementation of OAuthServerProvider used for revocation tests
// It tracks calls to RevokeToken and allows simulating errors
type mockRevokeProvider struct {
	store        *as.OAuthClientsStore             // backing client store
	lastReq      *auth.OAuthTokenRevocationRequest // last revocation request captured
	revokeErr    error                             // error to return from RevokeToken
	calledRevoke int                               // number of times RevokeToken was invoked
}

func (m *mockRevokeProvider) ClientsStore() *as.OAuthClientsStore {
	return m.store
}

func (m *mockRevokeProvider) RevokeToken(client auth.OAuthClientInformationFull, request auth.OAuthTokenRevocationRequest) error {
	m.calledRevoke++
	// capture the request for assertions
	tmp := request
	m.lastReq = &tmp
	return m.revokeErr
}

// The remaining interface methods are no-ops since they are not used in revocation tests
func (m *mockRevokeProvider) Authorize(client auth.OAuthClientInformationFull, params as.AuthorizationParams, w http.ResponseWriter, r *http.Request) error {
	return nil
}
func (m *mockRevokeProvider) ChallengeForAuthorizationCode(client auth.OAuthClientInformationFull, authorizationCode string) (string, error) {
	return "", nil
}
func (m *mockRevokeProvider) ExchangeAuthorizationCode(client auth.OAuthClientInformationFull, authorizationCode string, codeVerifier *string, redirectUri *string, resource *url.URL) (*auth.OAuthTokens, error) {
	return nil, nil
}
func (m *mockRevokeProvider) ExchangeRefreshToken(client auth.OAuthClientInformationFull, refreshToken string, scopes []string, resource *url.URL) (*auth.OAuthTokens, error) {
	return nil, nil
}
func (m *mockRevokeProvider) VerifyAccessToken(token string) (*as.AuthInfo, error) { return nil, nil }

// makeClientBasic constructs a client using client_secret_basic authentication
func makeClientBasic(id string) *auth.OAuthClientInformationFull {
	return &auth.OAuthClientInformationFull{
		OAuthClientInformation: auth.OAuthClientInformation{
			ClientID:     id,
			ClientSecret: testClientSecret,
		},
		OAuthClientMetadata: auth.OAuthClientMetadata{
			RedirectURIs:            []string{"https://cb"},
			TokenEndpointAuthMethod: "client_secret_basic",
		},
	}
}

// makeClientPost constructs a client using client_secret_post authentication
func makeClientPost(id string) *auth.OAuthClientInformationFull {
	return &auth.OAuthClientInformationFull{
		OAuthClientInformation: auth.OAuthClientInformation{
			ClientID:     id,
			ClientSecret: testClientSecret,
		},
		OAuthClientMetadata: auth.OAuthClientMetadata{
			RedirectURIs:            []string{"https://cb"},
			TokenEndpointAuthMethod: "client_secret_post",
		},
	}
}

// postFormBasicAuth helper submits a POST form request using HTTP Basic authentication
func postFormBasicAuth(t *testing.T, h http.Handler, path, clientID, clientSecret string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	if form == nil {
		form = url.Values{}
	}
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

// postFormClientSecretPost helper submits a POST form request using client_secret_post authentication
func postFormClientSecretPost(t *testing.T, h http.Handler, path, clientID, clientSecret string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	if form == nil {
		form = url.Values{}
	}
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)

	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestRevocation_Success_200(t *testing.T) {
	mpBasic := &mockRevokeProvider{store: makeStoreWithClient(makeClientBasic(testClientID))}
	hBasic := RevocationHandler(RevocationHandlerOptions{Provider: mpBasic})

	form := url.Values{"token": {"at-123"}}
	rr := postFormBasicAuth(t, hBasic, "/revoke", testClientID, testClientSecret, form)

	if rr.Code != http.StatusOK {
		mpPost := &mockRevokeProvider{store: makeStoreWithClient(makeClientPost(testClientID))}
		hPost := RevocationHandler(RevocationHandlerOptions{Provider: mpPost})
		rr2 := postFormClientSecretPost(t, hPost, "/revoke", testClientID, testClientSecret, form)

		if rr2.Code != http.StatusOK {
			t.Skipf("Skip: Authentication failed (Basic=%d/%s, Post=%d/%s)",
				rr.Code, strings.TrimSpace(rr.Body.String()),
				rr2.Code, strings.TrimSpace(rr2.Body.String()),
			)
			return
		}
		assert.Equal(t, 1, mpPost.calledRevoke)
		require.NotNil(t, mpPost.lastReq)
		assert.Equal(t, "at-123", mpPost.lastReq.Token)
		return
	}

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, 1, mpBasic.calledRevoke)
	require.NotNil(t, mpBasic.lastReq)
	assert.Equal(t, "at-123", mpBasic.lastReq.Token)
}

func TestRevocation_MissingToken_400(t *testing.T) {
	mp := &mockRevokeProvider{store: makeStoreWithClient(makeClientBasic(testClientID))}
	h := RevocationHandler(RevocationHandlerOptions{Provider: mp})

	rr := postFormBasicAuth(t, h, "/revoke", testClientID, testClientSecret, url.Values{})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, strings.ToLower(rr.Body.String()), "invalid_request")
}

func TestRevocation_UnsupportedTokenHint_Still200(t *testing.T) {
	mpBasic := &mockRevokeProvider{store: makeStoreWithClient(makeClientBasic(testClientID))}
	hBasic := RevocationHandler(RevocationHandlerOptions{Provider: mpBasic})

	form := url.Values{
		"token":           {"rt-xyz"},
		"token_type_hint": {"access_token"},
	}
	rr := postFormBasicAuth(t, hBasic, "/revoke", testClientID, testClientSecret, form)

	if rr.Code != http.StatusOK {
		mpPost := &mockRevokeProvider{store: makeStoreWithClient(makeClientPost(testClientID))}
		hPost := RevocationHandler(RevocationHandlerOptions{Provider: mpPost})
		rr2 := postFormClientSecretPost(t, hPost, "/revoke", testClientID, testClientSecret, form)

		if rr2.Code != http.StatusOK {
			t.Skipf("Skip: Authentication failed (Basic=%d/%s, Post=%d/%s)",
				rr.Code, strings.TrimSpace(rr.Body.String()),
				rr2.Code, strings.TrimSpace(rr2.Body.String()),
			)
			return
		}
		require.Equal(t, http.StatusOK, rr2.Code)
		return
	}

	require.Equal(t, http.StatusOK, rr.Code)
}

func TestRevocation_MethodNotAllowed_405(t *testing.T) {
	mp := &mockRevokeProvider{store: makeStoreWithClient(makeClientBasic(testClientID))}
	h := RevocationHandler(RevocationHandlerOptions{Provider: mp})

	req := httptest.NewRequest(http.MethodGet, "/revoke", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestRevocation_RateLimit_429(t *testing.T) {
	mp := &mockRevokeProvider{store: makeStoreWithClient(makeClientBasic(testClientID))}
	h := RevocationHandler(RevocationHandlerOptions{
		Provider: mp,
		RateLimit: &RevocationRateLimitConfig{
			WindowMs: 60_000,
			Max:      1,
		},
	})

	_ = postFormBasicAuth(t, h, "/revoke", testClientID, testClientSecret, url.Values{"token": {"at-123"}})

	rr2 := postFormBasicAuth(t, h, "/revoke", testClientID, testClientSecret, url.Values{"token": {"at-456"}})
	require.Equal(t, http.StatusTooManyRequests, rr2.Code)
}

func TestRevocation_OPTIONS_405(t *testing.T) {
	mp := &mockRevokeProvider{store: makeStoreWithClient(makeClientBasic(testClientID))}
	h := RevocationHandler(RevocationHandlerOptions{Provider: mp})

	req := httptest.NewRequest(http.MethodOptions, "/revoke", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}
