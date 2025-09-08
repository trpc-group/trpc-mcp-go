package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"trpc.group/trpc-go/trpc-mcp-go/internal/auth"
	"trpc.group/trpc-go/trpc-mcp-go/internal/errors"
)

// Helper function to create string pointer
var stringPtr = func(s string) *string {
	return &s
}

// TestMetadataHandler tests the MetadataHandler function
func TestMetadataHandler(t *testing.T) {

	// Example OAuthMetadata for testing
	exampleMetadata := auth.OAuthMetadata{
		Issuer:                            "https://auth.example.com",
		AuthorizationEndpoint:             "https://auth.example.com/authorize",
		TokenEndpoint:                     "https://auth.example.com/token",
		RegistrationEndpoint:              stringPtr("https://auth.example.com/register"),
		RevocationEndpoint:                stringPtr("https://auth.example.com/revoke"),
		ScopesSupported:                   []string{"profile", "email"},
		ResponseTypesSupported:            []string{"code"},
		GrantTypesSupported:               []string{"authorization_code", "refresh_token"},
		TokenEndpointAuthMethodsSupported: []string{"client_secret_basic"},
		CodeChallengeMethodsSupported:     []string{"S256"},
	}

	// Setup test cases
	t.Run("requires GET method", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/.well-known/oauth-authorization-server", nil)
		w := httptest.NewRecorder()
		handler := MetadataHandler(exampleMetadata)
		handler(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
		}

		if w.Header().Get("Allow") != "GET" {
			t.Errorf("Expected Allow header 'GET', got '%s'", w.Header().Get("Allow"))
		}

		var oauthErr errors.OAuthErrorResponse
		if err := json.NewDecoder(w.Body).Decode(&oauthErr); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		expectedErr := errors.OAuthErrorResponse{
			Error:            "method_not_allowed",
			ErrorDescription: "Only GET method is allowed",
		}
		if !reflect.DeepEqual(oauthErr, expectedErr) {
			t.Errorf("Expected error response %+v, got %+v", expectedErr, oauthErr)
		}
	})

	t.Run("returns the metadata object", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
		w := httptest.NewRecorder()
		handler := MetadataHandler(exampleMetadata)
		handler(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
		}

		var result auth.OAuthMetadata
		if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if !reflect.DeepEqual(result, exampleMetadata) {
			t.Errorf("Expected metadata %+v, got %+v", exampleMetadata, result)
		}
	})

	t.Run("includes CORS headers in response", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
		req.Header.Set("Origin", "https://example.com")
		w := httptest.NewRecorder()
		handler := MetadataHandler(exampleMetadata)
		handler(w, req)

		if w.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Errorf("Expected Access-Control-Allow-Origin '*', got '%s'", w.Header().Get("Access-Control-Allow-Origin"))
		}
	})

	t.Run("supports OPTIONS preflight requests", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/.well-known/oauth-authorization-server", nil)
		req.Header.Set("Origin", "https://example.com")
		req.Header.Set("Access-Control-Request-Method", "GET")
		w := httptest.NewRecorder()
		handler := MetadataHandler(exampleMetadata)
		handler(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
		}

		if w.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Errorf("Expected Access-Control-Allow-Origin '*', got '%s'", w.Header().Get("Access-Control-Allow-Origin"))
		}

		if w.Header().Get("Access-Control-Allow-Methods") != "GET, OPTIONS" {
			t.Errorf("Expected Access-Control-Allow-Methods 'GET, OPTIONS', got '%s'", w.Header().Get("Access-Control-Allow-Methods"))
		}
	})

	t.Run("works with minimal metadata", func(t *testing.T) {
		minimalMetadata := auth.OAuthMetadata{
			Issuer:                 "https://auth.example.com",
			AuthorizationEndpoint:  "https://auth.example.com/authorize",
			TokenEndpoint:          "https://auth.example.com/token",
			ResponseTypesSupported: []string{"code"},
		}

		req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
		w := httptest.NewRecorder()
		handler := MetadataHandler(minimalMetadata)
		handler(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
		}

		var result auth.OAuthMetadata
		if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if !reflect.DeepEqual(result, minimalMetadata) {
			t.Errorf("Expected metadata %+v, got %+v", minimalMetadata, result)
		}
	})
}

// TestAuthorizationServerMetadataHandler tests the AuthorizationServerMetadataHandler function
func TestAuthorizationServerMetadataHandler(t *testing.T) {

	// Setup test cases
	t.Run("includes registration endpoint with dynamic registration support", func(t *testing.T) {
		handler := AuthorizationServerMetadataHandler("https://auth.example.com", store)
		req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
		w := httptest.NewRecorder()
		handler(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
		}

		var result auth.OAuthMetadata
		if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		expected := auth.OAuthMetadata{
			Issuer:                            "https://auth.example.com",
			AuthorizationEndpoint:             "https://auth.example.com/authorize",
			TokenEndpoint:                     "https://auth.example.com/token",
			RegistrationEndpoint:              stringPtr("https://auth.example.com/register"),
			ResponseTypesSupported:            []string{"code"},
			GrantTypesSupported:               []string{"authorization_code", "refresh_token"},
			CodeChallengeMethodsSupported:     []string{"S256"},
			TokenEndpointAuthMethodsSupported: []string{"client_secret_basic", "client_secret_post", "none"},
		}

		if !reflect.DeepEqual(result, expected) {
			t.Errorf("Expected metadata %+v, got %+v", expected, result)
		}
	})

	t.Run("excludes registration endpoint without dynamic registration support", func(t *testing.T) {
		store := &mockStore{} // Not implementing SupportDynamicClientRegistration
		handler := AuthorizationServerMetadataHandler("https://auth.example.com", store)
		req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
		w := httptest.NewRecorder()
		handler(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
		}

		var result auth.OAuthMetadata
		if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if result.RegistrationEndpoint != nil {
			t.Errorf("Expected RegistrationEndpoint to be nil, got %v", result.RegistrationEndpoint)
		}
	})
}
