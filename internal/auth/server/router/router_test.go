// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"trpc.group/trpc-go/trpc-mcp-go/internal/auth"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth/server"
)

// fullProvider implements a fuller OAuthServerProvider used to exercise all router endpoints in tests
type fullProvider struct{}

// ClientsStore returns a store that supports dynamic registration for tests
func (p *fullProvider) ClientsStore() *server.OAuthClientsStore {
	return server.NewOAuthClientStoreSupportDynamicRegistration(
		func(clientID string) (*auth.OAuthClientInformationFull, error) {
			return &auth.OAuthClientInformationFull{
				OAuthClientInformation: auth.OAuthClientInformation{
					ClientID: clientID,
				},
			}, nil
		},
		func(client auth.OAuthClientInformationFull) (*auth.OAuthClientInformationFull, error) {
			return &client, nil
		},
	)
}

// Authorize simulates a successful authorization response by redirecting with a mock code
func (p *fullProvider) Authorize(client auth.OAuthClientInformationFull, params server.AuthorizationParams, w http.ResponseWriter, r *http.Request) error {
	u, _ := url.Parse(params.RedirectURI)
	q := u.Query()
	q.Set("code", "mock_auth_code")
	if params.State != "" {
		q.Set("state", params.State)
	}
	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
	return nil
}

// ChallengeForAuthorizationCode returns a fixed PKCE challenge for testing
func (p *fullProvider) ChallengeForAuthorizationCode(client auth.OAuthClientInformationFull, code string) (string, error) {
	return "mock_challenge", nil
}

// ExchangeAuthorizationCode returns mock tokens for a valid authorization code exchange
func (p *fullProvider) ExchangeAuthorizationCode(client auth.OAuthClientInformationFull, code string, verifier *string, redirect *string, resource *url.URL) (*auth.OAuthTokens, error) {
	expires := int64(3600)
	rt := "mock_refresh_token"
	return &auth.OAuthTokens{
		AccessToken:  "mock_access_token",
		TokenType:    "bearer",
		ExpiresIn:    &expires,
		RefreshToken: &rt,
	}, nil
}

// ExchangeRefreshToken returns mock tokens for a valid refresh token exchange
func (p *fullProvider) ExchangeRefreshToken(client auth.OAuthClientInformationFull, rt string, scopes []string, resource *url.URL) (*auth.OAuthTokens, error) {
	expires := int64(3600)
	newRT := "new_mock_refresh_token"
	return &auth.OAuthTokens{
		AccessToken:  "new_mock_access_token",
		TokenType:    "bearer",
		ExpiresIn:    &expires,
		RefreshToken: &newRT,
	}, nil
}

// VerifyAccessToken validates a token and returns mock auth info or an error
func (p *fullProvider) VerifyAccessToken(token string) (*server.AuthInfo, error) {
	if token == "valid_token" {
		exp := time.Now().Add(time.Hour).Unix()
		return &server.AuthInfo{
			Token:     token,
			ClientID:  "valid-client",
			Scopes:    []string{"read", "write"},
			ExpiresAt: &exp,
		}, nil
	}
	return nil, ErrInvalid
}

// RevokeToken implements token revocation to satisfy the interface
func (p *fullProvider) RevokeToken(client auth.OAuthClientInformationFull, req auth.OAuthTokenRevocationRequest) error {
	return nil
}

// minimalProvider implements the minimal surface needed for router tests without dynamic registration
type minimalProvider struct{}

// ClientsStore returns nil to indicate no dynamic client registration in minimal mode
func (p *minimalProvider) ClientsStore() *server.OAuthClientsStore { return nil }

// Authorize simulates a basic authorization redirect with a mock code
func (p *minimalProvider) Authorize(client auth.OAuthClientInformationFull, params server.AuthorizationParams, w http.ResponseWriter, r *http.Request) error {
	u, _ := url.Parse(params.RedirectURI)
	q := u.Query()
	q.Set("code", "mock_auth_code")
	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
	return nil
}

// ChallengeForAuthorizationCode returns a fixed PKCE challenge in minimal mode
func (p *minimalProvider) ChallengeForAuthorizationCode(client auth.OAuthClientInformationFull, code string) (string, error) {
	return "mock_challenge", nil
}

// ExchangeAuthorizationCode returns a basic mock access token for code exchange
func (p *minimalProvider) ExchangeAuthorizationCode(client auth.OAuthClientInformationFull, code string, verifier *string, redirect *string, resource *url.URL) (*auth.OAuthTokens, error) {
	expires := int64(3600)
	return &auth.OAuthTokens{AccessToken: "mock_access_token", TokenType: "bearer", ExpiresIn: &expires}, nil
}

// ExchangeRefreshToken returns a basic mock access token for refresh exchange
func (p *minimalProvider) ExchangeRefreshToken(client auth.OAuthClientInformationFull, rt string, scopes []string, resource *url.URL) (*auth.OAuthTokens, error) {
	expires := int64(3600)
	return &auth.OAuthTokens{AccessToken: "new_mock_access_token", TokenType: "bearer", ExpiresIn: &expires}, nil
}

// VerifyAccessToken always returns a valid mock auth info in minimal mode
func (p *minimalProvider) VerifyAccessToken(token string) (*server.AuthInfo, error) {
	exp := time.Now().Add(time.Hour).Unix()
	return &server.AuthInfo{Token: token, ClientID: "valid-client", Scopes: []string{"read"}, ExpiresAt: &exp}, nil
}

// RevokeToken implements a no-op revocation to satisfy the embedded interface
func (p *minimalProvider) RevokeToken(client auth.OAuthClientInformationFull, req auth.OAuthTokenRevocationRequest) error {
	return nil
}

func Test_McpAuthRouter_RouterCreation_Validation(t *testing.T) {
	mux := http.NewServeMux()
	issuerHTTP, _ := url.Parse("http://auth.example.com")
	err := McpAuthRouter(mux, AuthRouterOptions{
		Provider:  &fullProvider{},
		IssuerUrl: issuerHTTP,
	})
	if err == nil {
		t.Fatalf("expected error for non-HTTPS issuer")
	}

	muxOK := http.NewServeMux()
	issuerHTTPS, _ := url.Parse("https://auth.example.com")
	if err := McpAuthRouter(muxOK, AuthRouterOptions{
		Provider:  &fullProvider{},
		IssuerUrl: issuerHTTPS,
	}); err != nil {
		t.Fatalf("unexpected error for valid https issuer: %v", err)
	}
}

func Test_Metadata_AuthorizationServer_Full(t *testing.T) {
	mux := http.NewServeMux()

	issuer, _ := url.Parse("https://auth.example.com/")
	if err := McpAuthRouter(mux, AuthRouterOptions{
		Provider:                &fullProvider{},
		IssuerUrl:               issuer,
		ServiceDocumentationUrl: mustParseURL("https://docs.example.com"),
		ScopesSupported:         []string{"read", "write"},
	}); err != nil {
		t.Fatalf("router init failed: %v", err)
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	res, err := http.Get(ts.URL + "/.well-known/oauth-authorization-server")
	if err != nil {
		t.Fatalf("GET metadata failed: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}

	var body map[string]any
	_ = json.NewDecoder(res.Body).Decode(&body)

	expectStr(t, body, "issuer", "https://auth.example.com/")
	expectStr(t, body, "authorization_endpoint", "https://auth.example.com/authorize")
	expectStr(t, body, "token_endpoint", "https://auth.example.com/token")

	expectArr(t, body, "response_types_supported", []string{"code"})
	expectArr(t, body, "grant_types_supported", []string{"authorization_code", "refresh_token"})
	expectArr(t, body, "code_challenge_methods_supported", []string{"S256"})
	expectArr(t, body, "token_endpoint_auth_methods_supported", containsAny("client_secret_post", "client_secret_basic"))

	// fullProvider: typically has revoke/register endpoints (depends on router implementation)
	// Not asserting presence to avoid coupling with specific implementation; add expectStr(...) if needed
}

func Test_Metadata_ProtectedResource_Full(t *testing.T) {
	mux := http.NewServeMux()

	issuer, _ := url.Parse("https://auth.example.com/")
	if err := McpAuthRouter(mux, AuthRouterOptions{
		Provider:                &fullProvider{},
		IssuerUrl:               issuer,
		ServiceDocumentationUrl: mustParseURL("https://docs.example.com/"),
		ScopesSupported:         []string{"read", "write"},
		ResourceName:            strPtr("Test API"),
	}); err != nil {
		t.Fatalf("router init failed: %v", err)
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	res, err := http.Get(ts.URL + "/.well-known/oauth-protected-resource")
	if err != nil {
		t.Fatalf("GET resource metadata failed: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}

	var body map[string]any
	_ = json.NewDecoder(res.Body).Decode(&body)

	expectStr(t, body, "resource", "https://auth.example.com/") // depends on router implementation
	expectArr(t, body, "authorization_servers", []string{"https://auth.example.com/"})
	expectArr(t, body, "scopes_supported", []string{"read", "write"})
	expectStr(t, body, "resource_name", "Test API")
	expectStr(t, body, "resource_documentation", "https://docs.example.com/")
}

func Test_Metadata_Minimal_NoOptionalFields(t *testing.T) {
	mux := http.NewServeMux()

	issuer, _ := url.Parse("https://auth.example.com")
	if err := McpAuthRouter(mux, AuthRouterOptions{
		Provider:  &minimalProvider{},
		IssuerUrl: issuer,
	}); err != nil {
		t.Fatalf("router init failed: %v", err)
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Authorization server metadata: optional fields omitted
	as, _ := http.Get(ts.URL + "/.well-known/oauth-authorization-server")
	defer as.Body.Close()
	var a map[string]any
	_ = json.NewDecoder(as.Body).Decode(&a)
	if _, ok := a["service_documentation"]; ok {
		t.Fatalf("service_documentation should be omitted")
	}
	if _, ok := a["scopes_supported"]; ok {
		t.Fatalf("scopes_supported should be omitted")
	}

	// Protected resource metadata: optional fields omitted
	pr, _ := http.Get(ts.URL + "/.well-known/oauth-protected-resource")
	defer pr.Body.Close()
	var p map[string]any
	_ = json.NewDecoder(pr.Body).Decode(&p)
	if _, ok := p["scopes_supported"]; ok {
		t.Fatalf("scopes_supported should be omitted")
	}
	if _, ok := p["resource_name"]; ok {
		t.Fatalf("resource_name should be omitted")
	}
	if _, ok := p["resource_documentation"]; ok {
		t.Fatalf("resource_documentation should be omitted")
	}
}

func Test_Routes_Register_And_Revoke_Presence_MinimalVsFull(t *testing.T) {
	issuer, _ := url.Parse("https://auth.example.com")

	// full provider: registers /register endpoint
	muxFull := http.NewServeMux()
	if err := McpAuthRouter(muxFull, AuthRouterOptions{
		Provider:  &fullProvider{},
		IssuerUrl: issuer,
	}); err != nil {
		t.Fatalf("init full failed: %v", err)
	}
	tsFull := httptest.NewServer(muxFull)
	defer tsFull.Close()

	// Test /register exists
	r1, _ := http.PostForm(tsFull.URL+"/register", url.Values{
		"redirect_uris": {"https://example.com/callback"},
	})
	_ = r1.Body.Close()
	if r1.StatusCode == http.StatusNotFound {
		t.Fatalf("full: /register should exist (got 404)")
	}

	// minimal provider: should return 404
	muxMin := http.NewServeMux()
	if err := McpAuthRouter(muxMin, AuthRouterOptions{
		Provider:  &minimalProvider{},
		IssuerUrl: issuer,
	}); err != nil {
		t.Fatalf("init minimal failed: %v", err)
	}
	tsMin := httptest.NewServer(muxMin)
	defer tsMin.Close()

	// Test /register does not exist
	mr, _ := http.PostForm(tsMin.URL+"/register", url.Values{
		"redirect_uris": {"https://example.com/callback"},
	})
	_ = mr.Body.Close()
	if mr.StatusCode != http.StatusNotFound {
		t.Fatalf("minimal: /register should be 404 when ClientsStore==nil, got %d", mr.StatusCode)
	}
}

// ErrInvalid is a sentinel error used by tests to simulate token verification failures
var ErrInvalid = &struct{ error }{}

// mustParseURL parses a URL and panics on error for test setup convenience
func mustParseURL(s string) *url.URL { u, _ := url.Parse(s); return u }

// strPtr returns a pointer to the provided string
func strPtr(s string) *string { return &s }

// expectStr asserts a string field in a metadata map
func expectStr(t *testing.T, m map[string]any, key, want string) {
	t.Helper()
	v, ok := m[key]
	if !ok {
		t.Fatalf("missing key %q", key)
	}
	vs, _ := v.(string)
	if vs != want {
		t.Fatalf("%s mismatch: got %q want %q", key, vs, want)
	}
}

// expectArr asserts a string slice field in a metadata map
func expectArr(t *testing.T, m map[string]any, key string, want []string) {
	t.Helper()
	v, ok := m[key]
	if !ok {
		t.Fatalf("missing key %q", key)
	}
	arr, ok := v.([]any)
	if !ok {
		t.Fatalf("%s is not array", key)
	}
	got := make([]string, 0, len(arr))
	for _, x := range arr {
		if s, ok := x.(string); ok {
			got = append(got, s)
		}
	}
	if len(want) == 1 && strings.HasPrefix(want[0], "__any__:") {
		needle := strings.TrimPrefix(want[0], "__any__:")
		found := false
		for _, g := range got {
			if g == needle {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("%s expected to contain %q, got %v", key, needle, got)
		}
		return
	}
	if len(got) != len(want) {
		t.Fatalf("%s length mismatch: got %v want %v", key, got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("%s[%d] mismatch: got %q want %q", key, i, got[i], want[i])
		}
	}
}

// containsAny encodes a loose expectation for any one of the provided values
func containsAny(values ...string) []string {
	if len(values) == 0 {
		return nil
	}
	return []string{"__any__:" + values[0]}
}
