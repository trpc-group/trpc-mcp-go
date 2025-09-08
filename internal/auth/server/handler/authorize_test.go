// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth"
	as "trpc.group/trpc-go/trpc-mcp-go/internal/auth/server"
)

// validChallenge is a known good S256 PKCE code challenge used in tests
const validChallenge = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"

// oauthErrResp matches the JSON shape returned for OAuth error responses in tests
type oauthErrResp struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
	ErrorURI         string `json:"error_uri,omitempty"`
}

// mockProvider is a test double that satisfies the OAuthServerProvider interface
// it allows overriding Authorize via authorizeFunc for behavior-driven tests
type mockProvider struct {
	store         *as.OAuthClientsStore
	authorizeFunc func(client auth.OAuthClientInformationFull, params as.AuthorizationParams, w http.ResponseWriter, r *http.Request) error
}

// ClientsStore returns the in memory store used by the mock provider
func (m *mockProvider) ClientsStore() *as.OAuthClientsStore { return m.store }

// Authorize simulates the authorization endpoint behavior
// if authorizeFunc is set it delegates to it
// otherwise it redirects to redirect_uri with a fixed code and optional state
func (m *mockProvider) Authorize(client auth.OAuthClientInformationFull, params as.AuthorizationParams, w http.ResponseWriter, r *http.Request) error {
	// Delegate to custom behavior when provided
	if m.authorizeFunc != nil {
		return m.authorizeFunc(client, params, w, r)
	}
	// Compose redirect with code and optional state
	u, _ := url.Parse(params.RedirectURI)
	q := u.Query()
	q.Set("code", "abc123")
	if params.State != "" {
		q.Set("state", params.State)
	}
	u.RawQuery = q.Encode()
	// Issue HTTP 302 redirect
	http.Redirect(w, r, u.String(), http.StatusFound)
	return nil
}

// ChallengeForAuthorizationCode returns an empty string in this mock provider
// real providers would return the stored code_challenge for the code
func (m *mockProvider) ChallengeForAuthorizationCode(client auth.OAuthClientInformationFull, authorizationCode string) (string, error) {
	return "", nil
}

// ExchangeAuthorizationCode is a stub that returns nil values for the mock
func (m *mockProvider) ExchangeAuthorizationCode(client auth.OAuthClientInformationFull, authorizationCode string, codeVerifier *string, redirectUri *string, resource *url.URL) (*auth.OAuthTokens, error) {
	return nil, nil
}

// ExchangeRefreshToken is a stub that returns nil values for the mock
func (m *mockProvider) ExchangeRefreshToken(client auth.OAuthClientInformationFull, refreshToken string, scopes []string, resource *url.URL) (*auth.OAuthTokens, error) {
	return nil, nil
}

// VerifyAccessToken is a stub that returns nil in this test double
func (m *mockProvider) VerifyAccessToken(token string) (*as.AuthInfo, error) { return nil, nil }

// RevokeToken satisfies the optional SupportTokenRevocation interface with a no op
func (m *mockProvider) RevokeToken(client auth.OAuthClientInformationFull, request auth.OAuthTokenRevocationRequest) error {
	return nil
}

// makeStoreWithClient creates a store that returns the provided client when looked up by id
func makeStoreWithClient(c *auth.OAuthClientInformationFull) *as.OAuthClientsStore {
	return as.NewOAuthClientStore(func(id string) (*auth.OAuthClientInformationFull, error) {
		// Return client when ids match otherwise nil to simulate not found
		if c != nil && c.ClientID == id {
			return c, nil
		}
		return nil, nil
	})
}

// makeClient builds a client record with id redirect uris and optional default scope
func makeClient(id string, redirects []string, scope *string) *auth.OAuthClientInformationFull {
	return &auth.OAuthClientInformationFull{
		OAuthClientInformation: auth.OAuthClientInformation{
			ClientID: id,
		},
		OAuthClientMetadata: auth.OAuthClientMetadata{
			RedirectURIs: redirects,
			Scope:        scope,
		},
	}
}

// newGET constructs a GET request helper for tests
func newGET(urlStr string) *http.Request {
	return httptest.NewRequest(http.MethodGet, urlStr, nil)
}

// newPOST constructs a POST request with x www form urlencoded body for tests
func newPOST(urlStr string, form url.Values) *http.Request {
	req := httptest.NewRequest(http.MethodPost, urlStr, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func TestAuthorization_SuccessGET(t *testing.T) {
	// Prepare client with registered redirect and default scopes
	scope := "read write"
	client := makeClient("c1", []string{"https://app.example.com/cb"}, &scope)
	mp := &mockProvider{store: makeStoreWithClient(client)}

	// Build handler under test
	h := AuthorizationHandler(AuthorizationHandlerOptions{Provider: mp})

	// Compose query for a valid authorization request
	qs := url.Values{
		"client_id":             {"c1"},
		"redirect_uri":          {"https://app.example.com/cb"},
		"response_type":         {"code"},
		"code_challenge":        {validChallenge},
		"code_challenge_method": {"S256"},
		"state":                 {"st-123"},
		"scope":                 {"read"},
	}
	req := newGET("/authorize?" + qs.Encode())
	rr := httptest.NewRecorder()

	// Execute handler
	h.ServeHTTP(rr, req)

	// Assert redirect and parameters
	assert.Equal(t, http.StatusFound, rr.Code)
	loc := rr.Header().Get("Location")
	u, err := url.Parse(loc)
	require.NoError(t, err)
	q := u.Query()
	assert.Equal(t, "abc123", q.Get("code"))
	assert.Equal(t, "st-123", q.Get("state"))
}

func TestAuthorization_MissingClientID_JSON400(t *testing.T) {
	client := makeClient("c1", []string{"https://app.example.com/cb"}, nil)
	mp := &mockProvider{store: makeStoreWithClient(client)}
	h := AuthorizationHandler(AuthorizationHandlerOptions{Provider: mp})

	// Build request without client_id
	req := newGET("/authorize?redirect_uri=https://app.example.com/cb")
	rr := httptest.NewRecorder()

	// Execute handler
	h.ServeHTTP(rr, req)

	// Validate error payload
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	var resp oauthErrResp
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "invalid_request", resp.Error)
	assert.NotEmpty(t, resp.ErrorDescription)
}

func TestAuthorization_UnregisteredRedirect_JSON400(t *testing.T) {
	client := makeClient("c1", []string{"https://app.example.com/cb"}, nil)
	mp := &mockProvider{store: makeStoreWithClient(client)}
	h := AuthorizationHandler(AuthorizationHandlerOptions{Provider: mp})

	// Use an unregistered redirect_uri
	req := newGET("/authorize?client_id=c1&redirect_uri=https://evil.example.com/cb")
	rr := httptest.NewRecorder()

	// Execute and assert
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	var resp oauthErrResp
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "invalid_request", resp.Error)
	assert.Contains(t, strings.ToLower(resp.ErrorDescription), "redirect")
}

func TestAuthorization_MultipleRedirects_RequireExplicit_JSON400(t *testing.T) {
	client := makeClient("c1", []string{"https://a/cb", "https://b/cb"}, nil)
	mp := &mockProvider{store: makeStoreWithClient(client)}
	h := AuthorizationHandler(AuthorizationHandlerOptions{Provider: mp})

	// Missing redirect_uri should fail when multiple are registered
	req := newGET("/authorize?client_id=c1")
	rr := httptest.NewRecorder()

	// Execute and assert
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	var resp oauthErrResp
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "invalid_request", resp.Error)
	assert.Contains(t, strings.ToLower(resp.ErrorDescription), "redirect")
}

func TestAuthorization_InvalidScope_302_WithState(t *testing.T) {
	scope := "read write"
	client := makeClient("c1", []string{"https://app.example.com/cb"}, &scope)
	mp := &mockProvider{store: makeStoreWithClient(client)}
	h := AuthorizationHandler(AuthorizationHandlerOptions{Provider: mp})

	// Request includes a scope not in the client's allowed set
	qs := url.Values{
		"client_id":             {"c1"},
		"redirect_uri":          {"https://app.example.com/cb"},
		"response_type":         {"code"},
		"code_challenge":        {validChallenge},
		"code_challenge_method": {"S256"},
		"scope":                 {"delete"},
		"state":                 {"keep-me"},
	}
	req := newGET("/authorize?" + qs.Encode())
	rr := httptest.NewRecorder()

	// Execute and assert error redirect
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusFound, rr.Code)
	u, _ := url.Parse(rr.Header().Get("Location"))
	q := u.Query()
	assert.Equal(t, "invalid_scope", q.Get("error"))
	assert.Equal(t, "keep-me", q.Get("state"))
	assert.NotEmpty(t, q.Get("error_description"))
}

func TestAuthorization_InvalidResourceURL_302_ErrorRedirect(t *testing.T) {
	scope := "read"
	client := makeClient("c1", []string{"https://app.example.com/cb"}, &scope)
	mp := &mockProvider{store: makeStoreWithClient(client)}
	h := AuthorizationHandler(AuthorizationHandlerOptions{Provider: mp})

	// Provide a relative resource URL which is invalid
	qs := url.Values{
		"client_id":             {"c1"},
		"redirect_uri":          {"https://app.example.com/cb"},
		"response_type":         {"code"},
		"code_challenge":        {validChallenge},
		"code_challenge_method": {"S256"},
		"resource":              {"/relative"},
	}
	req := newGET("/authorize?" + qs.Encode())
	rr := httptest.NewRecorder()

	// Execute and assert error redirect
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusFound, rr.Code)
	u, _ := url.Parse(rr.Header().Get("Location"))
	q := u.Query()
	assert.Equal(t, "invalid_request", q.Get("error"))
	assert.NotEmpty(t, q.Get("error_description"))
}

func TestAuthorization_RateLimit_429_JSON(t *testing.T) {
	client := makeClient("c1", []string{"https://app.example.com/cb"}, nil)
	mp := &mockProvider{store: makeStoreWithClient(client)}
	// Limiter with zero rate to always deny
	limiter := rate.NewLimiter(0, 0)

	h := AuthorizationHandler(AuthorizationHandlerOptions{
		Provider:  mp,
		RateLimit: limiter,
	})

	// No params needed because limiter will block before validation
	req := newGET("/authorize")
	rr := httptest.NewRecorder()

	// Execute and assert
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	var resp oauthErrResp
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "too_many_requests", resp.Error)
}

func TestAllowedMethods_GET_and_POST(t *testing.T) {
	client := makeClient("c1", []string{"https://cb"}, nil)
	mp := &mockProvider{store: makeStoreWithClient(client)}
	h := AuthorizationHandler(AuthorizationHandlerOptions{Provider: mp})

	// GET should be allowed
	rr1 := httptest.NewRecorder()
	h.ServeHTTP(rr1, newGET("/authorize"))
	assert.NotEqual(t, http.StatusMethodNotAllowed, rr1.Code)

	// PUT should be rejected with 405
	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPut, "/authorize", nil)
	h.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusMethodNotAllowed, rr2.Code)
}

func TestHelpers_StateParsing_GET_and_POST(t *testing.T) {
	// GET pathway
	reqGet := newGET("/authorize?state=GETSTATE")
	assert.Equal(t, "GETSTATE", getStateFromRequest(reqGet))

	// POST pathway
	form := url.Values{"state": {"POSTSTATE"}}
	reqPost := newPOST("/authorize", form)
	assert.Equal(t, "POSTSTATE", getStateFromRequest(reqPost))
}

func TestHelpers_ParseParams_Parity(t *testing.T) {
	// ClientAuthorizationParams via GET
	qs := url.Values{"client_id": {"c1"}, "redirect_uri": {"https://a/cb"}}
	cp := parseClientAuthorizationParams(newGET("/authorize?" + qs.Encode()))
	assert.Equal(t, "c1", cp.ClientID)
	assert.Equal(t, "https://a/cb", cp.RedirectURI)

	// RequestAuthorizationParams via POST
	form := url.Values{
		"response_type":         {"code"},
		"code_challenge":        {"abc"},
		"code_challenge_method": {"S256"},
		"scope":                 {"read write"},
		"resource":              {"https://api.example.com"},
		"state":                 {"s1"},
	}
	rp := parseRequestAuthorizationParams(newPOST("/authorize", form))
	assert.Equal(t, "code", rp.ResponseType)
	assert.Equal(t, "abc", rp.CodeChallenge)
	assert.Equal(t, "S256", rp.CodeChallengeMethod)
	assert.Equal(t, "read write", rp.Scope)
	assert.Equal(t, "s1", rp.State)
	assert.Equal(t, "https://api.example.com", rp.Resource)
}

func TestCreateErrorRedirect_ComposesQuery(t *testing.T) {
	// Inline error type to mimic minimal shape used by createErrorRedirect
	type inlineErr struct {
		ErrorCode string
		Message   string
		ErrorURI  string
	}
	errObj := inlineErr{ErrorCode: "invalid request", Message: "oops"}

	// Serialize and rehydrate to assert structure not affected by json tags
	bs, _ := json.Marshal(errObj)
	var rehydrated struct {
		ErrorCode string
		Message   string
		ErrorURI  string
	}
	_ = json.Unmarshal(bs, &rehydrated)

	// Build redirect URL and assert query parameters
	loc := createErrorRedirect("https://app.example.com/cb", rehydrated, "st")
	u, _ := url.Parse(loc)
	q := u.Query()
	assert.Equal(t, "invalid request", q.Get("error"))
	assert.Equal(t, "oops", q.Get("error_description"))
	assert.Equal(t, "st", q.Get("state"))
}
