// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 THL A29 Limited, a Tencent company.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

// Package utils provides utility functions used in the MCP framework
package utils

import (
	"encoding/json"
	"fmt"
)

// ParseJSONObject parses a JSON message into a map structure
func ParseJSONObject(rawMessage *json.RawMessage) (map[string]interface{}, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(*rawMessage, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON object: %v", err)
	}
	return data, nil
}

// ExtractString extracts a string value from a map
func ExtractString(data map[string]interface{}, key string) string {
	if value, ok := data[key].(string); ok {
		return value
	}
	return ""
}

// ExtractMap extracts a sub-map structure from a map
func ExtractMap(data map[string]interface{}, key string) map[string]interface{} {
	if value, ok := data[key].(map[string]interface{}); ok {
		return value
	}
	return nil
}

// ExtractArray extracts an array structure from a map
func ExtractArray(data map[string]interface{}, key string) []interface{} {
	if value, ok := data[key].([]interface{}); ok {
		return value
	}
	return nil
}

// ParseResourceContent parses resource content from a map structure
func ParseResourceContent(contentMap map[string]interface{}) (string, string, string, bool) {
	uri := ExtractString(contentMap, "uri")
	mimeType := ExtractString(contentMap, "mimeType")

	// Check if it's a text resource
	if text, ok := contentMap["text"].(string); ok {
		return uri, mimeType, text, true
	}

	// Check if it's a binary resource
	if blob, ok := contentMap["blob"].(string); ok {
		return uri, mimeType, blob, false
	}

	return uri, mimeType, "", true // Default to treating as text
}

// ParseToolItem parses a tool item from a map structure
func ParseToolItem(toolMap map[string]interface{}) (string, string, json.RawMessage, error) {
	// Parse tool name (required)
	name := ExtractString(toolMap, "name")
	if name == "" {
		return "", "", nil, fmt.Errorf("tool missing name")
	}

	// Parse optional description
	description := ExtractString(toolMap, "description")

	// Parse input schema if present
	var rawSchema json.RawMessage
	if schema, ok := toolMap["inputSchema"].(map[string]interface{}); ok {
		// Convert to JSON then parse to Schema
		schemaBytes, err := json.Marshal(schema)
		if err != nil {
			return name, description, nil, err
		}
		rawSchema = schemaBytes
	}

	return name, description, rawSchema, nil
}
