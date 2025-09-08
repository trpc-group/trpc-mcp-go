// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAllowedMethods(t *testing.T) {
	// Create a test handler
	createTestHandler := func() http.Handler {
		mux := http.NewServeMux()
		// Define /test route that only supports GET
		mux.Handle("/test", AllowedMethods([]string{"GET"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("GET success"))
		})))
		return mux
	}

	// Case 1 allows specified HTTP method
	t.Run("allows specified HTTP method", func(t *testing.T) {
		handler := createTestHandler()
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
		if body := rr.Body.String(); body != "GET success" {
			t.Errorf("expected body %q, got %q", "GET success", body)
		}
	})

	// Case 2 returns 405 for unsupported methods
	t.Run("returns 405 for unspecified HTTP methods", func(t *testing.T) {
		methods := []string{"POST", "PUT", "DELETE", "PATCH"}

		for _, method := range methods {
			t.Run(method, func(t *testing.T) {
				handler := createTestHandler()
				req := httptest.NewRequest(method, "/test", nil)
				rr := httptest.NewRecorder()
				handler.ServeHTTP(rr, req)

				if rr.Code != http.StatusMethodNotAllowed {
					t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rr.Code)
				}

				var response map[string]string
				if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}

				expected := map[string]string{
					"error":             "method not allowed",
					"error_description": "The method " + method + " is not allowed for this endpoint",
				}
				if response["error"] == expected["error"] && response["error_description"] == expected["error_description"] {
					t.Errorf("expected response %v, got %v", expected, response)
				}
			})
		}
	})

	// Case 3 checks Allow header
	t.Run("includes Allow header with specified methods", func(t *testing.T) {
		handler := createTestHandler()
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if allow := rr.Header().Get("Allow"); allow != "GET" {
			t.Errorf("expected Allow header %q, got %q", "GET", allow)
		}
	})

	// Case 4 supports multiple allowed methods
	t.Run("works with multiple allowed methods", func(t *testing.T) {
		// Define /multi route supporting GET and POST
		mux := http.NewServeMux()
		mux.Handle("/multi", AllowedMethods([]string{"GET", "POST"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				_, _ = w.Write([]byte("GET"))
			} else if r.Method == http.MethodPost {
				_, _ = w.Write([]byte("POST"))
			}
		})))

		// Allowed methods
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			t.Run(method, func(t *testing.T) {
				req := httptest.NewRequest(method, "/multi", nil)
				rr := httptest.NewRecorder()
				mux.ServeHTTP(rr, req)

				if rr.Code != http.StatusOK {
					t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
				}
				expectedBody := strings.ToUpper(method)
				if body := rr.Body.String(); body != expectedBody {
					t.Errorf("expected body %q, got %q", expectedBody, body)
				}
			})
		}

		// Unsupported method PUT
		t.Run("PUT", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/multi", nil)
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			if rr.Code != http.StatusMethodNotAllowed {
				t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rr.Code)
			}
			if allow := rr.Header().Get("Allow"); allow != "GET, POST" {
				t.Errorf("expected Allow header %q, got %q", "GET, POST", allow)
			}
		})
	})
}
