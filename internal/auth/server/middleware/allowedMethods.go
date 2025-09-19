// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"trpc.group/trpc-go/trpc-mcp-go/internal/errors"
)

// AllowedMethods returns a middleware that permits only the provided HTTP methods
// If the request method is not allowed it responds with 405 Method Not Allowed
// The response includes an Allow header and a JSON OAuth error body
func AllowedMethods(methods []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Allow request to proceed when method matches one of the allowed methods
			for _, method := range methods {
				if r.Method == method {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Build 405 response with Allow header listing permitted methods
			w.Header().Set("Allow", strings.Join(methods, ", "))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)

			// Create an OAuth-style error payload for method not allowed
			oauthErr := errors.NewOAuthError(
				errors.ErrMethodNotAllowed,
				fmt.Sprintf("The method %s is not allowed for this endpoint", r.Method),
				"", // Optional error URI
			)

			// Encode the error as JSON response body
			_ = json.NewEncoder(w).Encode(oauthErr.ToResponseStruct())
		})
	}
}
