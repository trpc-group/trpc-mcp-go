// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 THL A29 Limited, a Tencent company.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package httputil

import (
	"strings"
)

// ParseAcceptHeader parses HTTP Accept header, returns a list of content types
// For example: "application/json, text/plain;q=0.9, */*;q=0.8" will return
// ["application/json", "text/plain", "*/*"]
func ParseAcceptHeader(acceptHeader string) []string {
	if acceptHeader == "" {
		return []string{}
	}

	// Split and process each value
	accepts := []string{}
	for _, accept := range strings.Split(acceptHeader, ",") {
		// Remove potential parameters (like q values)
		mediaType := strings.Split(strings.TrimSpace(accept), ";")[0]
		if mediaType != "" {
			accepts = append(accepts, mediaType)
		}
	}

	return accepts
}

// ContainsContentType checks if Accept header contains the specified content type
// Returns true if the list contains the type or wildcard "*/*"
func ContainsContentType(accepts []string, contentType string) bool {
	for _, accept := range accepts {
		if accept == contentType || accept == "*/*" {
			return true
		}
	}
	return false
}
