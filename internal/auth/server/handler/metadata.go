// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package handler

import (
	"encoding/json"
	"net/http"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth/server/middleware"
)

// MetadataHandler creates a handler for metadata endpoints
// This matches the TypeScript implementation using middleware composition
func MetadataHandler(metadata interface{}) http.HandlerFunc {
	// Core handler that just serves JSON - no CORS or method validation
	coreHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(metadata)
	})

	middlewareHandler := middleware.CorsMiddleware(
		middleware.AllowedMethods([]string{"GET"})(coreHandler),
	)

	// Convert http.Handler to http.HandlerFunc
	return func(w http.ResponseWriter, r *http.Request) {
		middlewareHandler.ServeHTTP(w, r)
	}
}
