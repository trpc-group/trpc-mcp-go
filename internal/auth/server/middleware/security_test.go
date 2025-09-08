// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/time/rate"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth/server"
)

// okHandler returns a simple HTTP handler that always responds with status 200 OK and the body "ok"
func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`ok`))
	})
}

// do executes the given handler with a constructed HTTP request
func do(handler http.Handler, method, path string, body io.Reader, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, body)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func TestCorsMiddleware_NoOrigin_PassThrough(t *testing.T) {
	h := CorsMiddleware(okHandler())
	rr := do(h, http.MethodGet, "/", nil, nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if v := rr.Header().Get("Access-Control-Allow-Origin"); v != "" {
		t.Fatalf("expected no CORS header, got %q", v)
	}
}

func TestCorsMiddleware_WithOrigin_SetsHeaders(t *testing.T) {
	h := CorsMiddleware(okHandler())
	rr := do(h, http.MethodGet, "/", nil, map[string]string{
		"Origin": "http://example.com",
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if v := rr.Header().Get("Access-Control-Allow-Origin"); v != "*" {
		t.Fatalf("expected '*', got %q", v)
	}
	if v := rr.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(v, "GET") {
		t.Fatalf("expected allow methods set, got %q", v)
	}
}

func TestCorsMiddleware_Options_Preflight(t *testing.T) {
	h := CorsMiddleware(okHandler())
	rr := do(h, http.MethodOptions, "/", nil, map[string]string{
		"Origin": "http://example.com",
	})

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
	if v := rr.Header().Get("Content-Length"); v != "0" {
		t.Fatalf("expected Content-Length 0, got %q", v)
	}
}

func TestRateLimitMiddleware_Allow(t *testing.T) {
	lim := rate.NewLimiter(rate.Inf, 0) // always allow
	h := RateLimitMiddleware(lim)(okHandler())

	rr := do(h, http.MethodPost, "/token", nil, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestRateLimitMiddleware_Reject(t *testing.T) {
	lim := rate.NewLimiter(0, 0) // always reject
	h := RateLimitMiddleware(lim)(okHandler())

	rr := do(h, http.MethodPost, "/token", nil, nil)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rr.Code)
	}
	if !strings.Contains(strings.ToLower(rr.Body.String()), "too_many_requests") {
		t.Fatalf("expected body to contain too_many_requests, got %q", rr.Body.String())
	}
}

func TestContentTypeValidation_MissingHeader(t *testing.T) {
	h := ContentTypeValidationMiddleware([]string{"application/x-www-form-urlencoded"}, false)(okHandler())

	rr := do(h, http.MethodPost, "/token", bytes.NewBufferString("a=b"), nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	if !strings.Contains(strings.ToLower(rr.Body.String()), "invalid_request") {
		t.Fatalf("expected invalid_request, got %q", rr.Body.String())
	}
}

func TestContentTypeValidation_Allowed(t *testing.T) {
	h := ContentTypeValidationMiddleware([]string{"application/x-www-form-urlencoded"}, false)(okHandler())

	rr := do(h, http.MethodPost, "/token", bytes.NewBufferString("a=b"), map[string]string{
		"Content-Type": "application/x-www-form-urlencoded; charset=utf-8",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestContentTypeValidation_NotAllowed_NoFallback(t *testing.T) {
	h := ContentTypeValidationMiddleware([]string{"application/x-www-form-urlencoded"}, false)(okHandler())

	rr := do(h, http.MethodPost, "/token", bytes.NewBufferString(`{"a":"b"}`), map[string]string{
		"Content-Type": "application/json",
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestContentTypeValidation_JSONFallback(t *testing.T) {
	h := ContentTypeValidationMiddleware([]string{"application/x-www-form-urlencoded"}, true)(okHandler())

	rr := do(h, http.MethodPost, "/token", bytes.NewBufferString(`{"a":"b"}`), map[string]string{
		"Content-Type": "application/json; charset=utf-8",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 with JSON fallback, got %d", rr.Code)
	}
}

func TestURLEncodedValidationMiddleware(t *testing.T) {
	h := URLEncodedValidationMiddleware(false)(okHandler())

	rr := do(h, http.MethodPost, "/revoke", bytes.NewBufferString("a=b"), map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestJSONValidationMiddleware(t *testing.T) {
	h := JSONValidationMiddleware()(okHandler())

	rr := do(h, http.MethodPost, "/register", bytes.NewBufferString(`{"a":"b"}`), map[string]string{
		"Content-Type": "application/json",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

type denyAllAuthorizer struct{}

func (denyAllAuthorizer) Authorize(authInfo server.AuthInfo, resource, action string) error {
	// not used in this test since no auth info will be present
	return nil
}

func TestAuthorizationMiddleware_NoAuthInfo_Returns401(t *testing.T) {
	h := AuthorizationMiddleware(denyAllAuthorizer{}, "urn:mcp:resource", "read")(okHandler())

	rr := do(h, http.MethodGet, "/protected", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
	if v := rr.Header().Get("WWW-Authenticate"); !strings.Contains(v, "invalid_token") {
		t.Fatalf("expected WWW-Authenticate with invalid_token, got %q", v)
	}
}

func TestPolicyAuthorizer_Authorize_Allowed(t *testing.T) {
	mapper := &DefaultScopeMapper{
		Mapping: map[string][]string{
			"workspace.read": {"urn:mcp:workspace:xyz:read"},
		},
	}
	a := &PolicyAuthorizer{ScopeMapper: mapper}

	authInfo := server.AuthInfo{Scopes: []string{"workspace.read"}}
	if err := a.Authorize(authInfo, "urn:mcp:workspace:xyz", "read"); err != nil {
		t.Fatalf("expected authorize success, got %v", err)
	}
}

func TestPolicyAuthorizer_Authorize_Denied(t *testing.T) {
	mapper := &DefaultScopeMapper{
		Mapping: map[string][]string{
			"workspace.read": {"urn:mcp:workspace:xyz:read"},
		},
	}
	a := &PolicyAuthorizer{ScopeMapper: mapper}

	authInfo := server.AuthInfo{Scopes: []string{"workspace.read"}}
	if err := a.Authorize(authInfo, "urn:mcp:workspace:xyz", "write"); err == nil {
		t.Fatalf("expected authorize deny, got nil error")
	}
}

func TestRateLimitMiddleware_Reject_ReturnsJSON(t *testing.T) {
	lim := rate.NewLimiter(0, 0)
	h := RateLimitMiddleware(lim)(okHandler())

	rr := do(h, http.MethodPost, "/token", nil, nil)
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %q", ct)
	}
	var parsed map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("expected JSON body, got err: %v; body=%q", err, rr.Body.String())
	}
}
