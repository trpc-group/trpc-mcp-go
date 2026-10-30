// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent. All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStatelessHeaderSchemas(t *testing.T) {
	for _, tc := range []struct {
		name, schema string
		valid        bool
		count        int
	}{
		{"nested", `{"properties":{"scope":{"properties":{"tenant":{"type":"string","x-mcp-header":"Tenant"}}}}}`, true, 1},
		{"none", `{"type":"object"}`, true, 0},
		{"root", `{"type":"string","x-mcp-header":"Bad"}`, false, 0},
		{"empty", `{"properties":{"x":{"type":"string","x-mcp-header":""}}}`, false, 0},
		{"injection", `{"properties":{"x":{"type":"string","x-mcp-header":"X\r\nY"}}}`, false, 0},
		{"number", `{"properties":{"x":{"type":"number","x-mcp-header":"X"}}}`, false, 0},
		{"collision", `{"properties":{"a":{"type":"string","x-mcp-header":"X"},"b":{"type":"integer","x-mcp-header":"x"}}}`, false, 0},
		{"items", `{"properties":{"a":{"type":"array","items":{"properties":{"b":{"type":"string","x-mcp-header":"X"}}}}}}`, false, 0},
		{"composition", `{"oneOf":[{"properties":{"b":{"type":"string","x-mcp-header":"X"}}}]}`, false, 0},
		{"ref", `{"properties":{"x":{"$ref":"#/defs/x","type":"string","x-mcp-header":"X"}}}`, false, 0},
		{"dependency-schema", `{"$schema":"http://json-schema.org/draft-07/schema#","type":"object","dependencies":{"a":{"properties":{"b":{"type":"string","x-mcp-header":"B"}}}}}`, false, 0},
		{"dependency-properties", `{"dependencies":{"a":["b"]},"properties":{"b":{"type":"string","x-mcp-header":"B"}}}`, true, 1},
		{"example-data", `{"examples":[{"x-mcp-header":"user data"}]}`, true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fields, err := toolHeaderFields(json.RawMessage(tc.schema))
			if !tc.valid {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Len(t, fields, tc.count)
		})
	}
}

func TestStatelessHeaderValues(t *testing.T) {
	for _, tc := range []struct {
		value      interface{}
		kind, want string
		valid      bool
	}{
		{json.Number("9007199254740991"), "integer", "9007199254740991", true},
		{json.Number("-9007199254740991"), "integer", "-9007199254740991", true},
		{json.Number("9007199254740992"), "integer", "", false},
		{json.Number("9007199254740991.1"), "integer", "", false},
		{json.Number("42.0"), "integer", "42", true},
		{json.Number("4.2e1"), "integer", "42", true},
		{json.Number("1e999999999"), "integer", "", false},
		{json.Number("1e-999999999"), "integer", "", false},
		{json.Number("0e999999999"), "integer", "0", true},
		{json.Number("-0e-999999999"), "integer", "0", true},
		{json.Number("1.2"), "integer", "", false},
		{false, "boolean", "false", true},
		{"false", "boolean", "", false},
		{"a", "string", "a", true},
	} {
		value, err := toolHeaderValue(tc.value, tc.kind)
		if !tc.valid {
			require.Error(t, err)
			continue
		}
		require.NoError(t, err)
		require.Equal(t, tc.want, value)
	}
	for _, value := range []string{"你好", " spaced ", "line\nbreak", "=?base64?literal?="} {
		require.Contains(t, encodeMCPHeader(value), "=?base64?")
	}
	require.Equal(t, "ascii", encodeMCPHeader("ascii"))
}

func TestStatelessInvalidToolDefinitionIsExcluded(t *testing.T) {
	result := &ListToolsResult{Tools: []Tool{
		{Name: "valid", RawInputSchema: json.RawMessage(`{"type":"object"}`)},
		{Name: "invalid", RawInputSchema: json.RawMessage(`{"properties":{"x":{"type":"object","x-mcp-header":"X"}}}`)},
		{Name: "invalid-dependency", RawInputSchema: json.RawMessage(`{"dependencies":{"a":{"properties":{"b":{"type":"string","x-mcp-header":"B"}}}}}`)},
	}}
	filterToolHeaders(result)
	require.Len(t, result.Tools, 1)
	require.Equal(t, "valid", result.Tools[0].Name)
}
