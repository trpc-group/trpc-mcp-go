// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetaMarshalling(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		meta    *Meta
		expMeta *Meta
	}{
		{
			name: "nil meta",
			json: "null",
			meta: nil,
		},
		{
			name: "empty meta",
			json: "{}",
			meta: &Meta{},
		},
		{
			name: "only progressToken",
			json: `{"progressToken":123}`,
			meta: &Meta{
				ProgressToken: 123,
			},
			expMeta: &Meta{
				ProgressToken:    123,
				AdditionalFields: map[string]interface{}{},
			},
		},
		{
			name: "progressToken string",
			json: `{"progressToken":"abc-123"}`,
			meta: &Meta{
				ProgressToken: "abc-123",
			},
			expMeta: &Meta{
				ProgressToken:    "abc-123",
				AdditionalFields: map[string]interface{}{},
			},
		},
		{
			name: "only additional fields",
			json: `{"customKey":"customValue","nested":{"field":"value"}}`,
			meta: &Meta{
				AdditionalFields: map[string]interface{}{
					"customKey": "customValue",
					"nested": map[string]interface{}{
						"field": "value",
					},
				},
			},
		},
		{
			name: "progressToken and additional fields",
			json: `{"progressToken":456,"platform.auth/token":"eyJhbGci...","platform.auth/tenant":"tenant-abc"}`,
			meta: &Meta{
				ProgressToken: 456,
				AdditionalFields: map[string]interface{}{
					"platform.auth/token":  "eyJhbGci...",
					"platform.auth/tenant": "tenant-abc",
				},
			},
		},
		{
			name: "complex additional fields",
			json: `{"progressToken":789,"custom.domain/array":["item1","item2"],"custom.domain/number":42.5}`,
			meta: &Meta{
				ProgressToken: 789,
				AdditionalFields: map[string]interface{}{
					"custom.domain/array":  []interface{}{"item1", "item2"},
					"custom.domain/number": 42.5,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+" marshal", func(t *testing.T) {
			data, err := json.Marshal(tt.meta)
			require.NoError(t, err)

			// Verify JSON structure matches expected
			var got, expected map[string]interface{}
			if tt.json != "null" {
				require.NoError(t, json.Unmarshal([]byte(tt.json), &expected))
				require.NoError(t, json.Unmarshal(data, &got))
				assert.Equal(t, expected, got, "marshalled JSON should match expected")
			}
		})

		t.Run(tt.name+" unmarshal", func(t *testing.T) {
			var meta Meta
			err := json.Unmarshal([]byte(tt.json), &meta)
			require.NoError(t, err)

			// Use expMeta if provided, otherwise use original meta
			expected := tt.meta
			if tt.expMeta != nil {
				expected = tt.expMeta
			}

			if expected != nil {
				assert.Equal(t, expected.ProgressToken, meta.ProgressToken, "progressToken should match")
				assert.Equal(t, expected.AdditionalFields, meta.AdditionalFields, "additionalFields should match")
			}
		})

		t.Run(tt.name+" roundtrip", func(t *testing.T) {
			if tt.meta == nil {
				t.Skip("skipping roundtrip for nil meta")
			}

			// Marshal
			data, err := json.Marshal(tt.meta)
			require.NoError(t, err)

			// Unmarshal
			var meta Meta
			err = json.Unmarshal(data, &meta)
			require.NoError(t, err)

			// Verify roundtrip
			assert.Equal(t, tt.meta.ProgressToken, meta.ProgressToken, "progressToken should survive roundtrip")
			assert.Equal(t, tt.meta.AdditionalFields, meta.AdditionalFields, "additionalFields should survive roundtrip")
		})
	}
}

func TestMetaGetSet(t *testing.T) {
	meta := &Meta{}

	// Test Get on empty meta
	assert.Nil(t, meta.Get("nonexistent"))

	// Test Set
	meta.Set("key1", "value1")
	assert.Equal(t, "value1", meta.Get("key1"))

	meta.Set("key2", 123)
	assert.Equal(t, 123, meta.Get("key2"))

	meta.Set("key3", map[string]interface{}{"nested": "value"})
	assert.Equal(t, map[string]interface{}{"nested": "value"}, meta.Get("key3"))

	// Test overwriting
	meta.Set("key1", "value2")
	assert.Equal(t, "value2", meta.Get("key1"))

	// Test Get on nil meta
	var nilMeta *Meta
	assert.Nil(t, nilMeta.Get("key"))
}

func TestRequestWithMeta(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr bool
	}{
		{
			name: "request with progressToken only",
			json: `{
				"method": "tools/call",
				"params": {
					"_meta": {
						"progressToken": 123
					}
				}
			}`,
		},
		{
			name: "request with custom metadata",
			json: `{
				"method": "tools/call",
				"params": {
					"_meta": {
						"progressToken": 456,
						"platform.auth/token": "eyJhbGci...",
						"platform.auth/tenant": "tenant-abc"
					}
				}
			}`,
		},
		{
			name: "request without meta",
			json: `{
				"method": "tools/call",
				"params": {}
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req Request
			err := json.Unmarshal([]byte(tt.json), &req)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Marshal back
			data, err := json.Marshal(req)
			require.NoError(t, err)

			// Unmarshal again to verify roundtrip
			var req2 Request
			err = json.Unmarshal(data, &req2)
			require.NoError(t, err)

			// Verify meta preserved
			if req.Params.Meta != nil {
				require.NotNil(t, req2.Params.Meta)
				assert.Equal(t, req.Params.Meta.ProgressToken, req2.Params.Meta.ProgressToken)
				assert.Equal(t, req.Params.Meta.AdditionalFields, req2.Params.Meta.AdditionalFields)
			}
		})
	}
}

func TestCallToolParamsWithMeta(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr bool
	}{
		{
			name: "tool call with progressToken",
			json: `{
				"name": "getUserData",
				"arguments": {
					"userId": "12345"
				},
				"_meta": {
					"progressToken": 123
				}
			}`,
		},
		{
			name: "tool call with custom metadata",
			json: `{
				"name": "getUserData",
				"arguments": {
					"userId": "12345"
				},
				"_meta": {
					"progressToken": 456,
					"platform.auth/token": "eyJhbGci...",
					"platform.auth/tenant": "tenant-abc",
					"platform.auth/permissions": ["read", "write"]
				}
			}`,
		},
		{
			name: "tool call without meta",
			json: `{
				"name": "getUserData",
				"arguments": {
					"userId": "12345"
				}
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var params CallToolParams
			err := json.Unmarshal([]byte(tt.json), &params)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Marshal back
			data, err := json.Marshal(params)
			require.NoError(t, err)

			// Unmarshal again to verify roundtrip
			var params2 CallToolParams
			err = json.Unmarshal(data, &params2)
			require.NoError(t, err)

			// Verify basic fields
			assert.Equal(t, params.Name, params2.Name)
			assert.Equal(t, params.Arguments, params2.Arguments)

			// Verify meta preserved
			if params.Meta != nil {
				require.NotNil(t, params2.Meta)
				assert.Equal(t, params.Meta.ProgressToken, params2.Meta.ProgressToken)
				assert.Equal(t, params.Meta.AdditionalFields, params2.Meta.AdditionalFields)
			}
		})
	}
}

func TestMetaUseCaseFromFaustli(t *testing.T) {
	// This test demonstrates the use case from faustli's requirement:
	// Passing auth metadata from A2A protocol through MCP to business MCP server

	// Client sends request with auth metadata
	reqJSON := `{
		"method": "tools/call",
		"params": {
			"name": "getUserData",
			"arguments": {
				"userId": "12345"
			},
			"_meta": {
				"progressToken": "token-123",
				"platform.auth/token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
				"platform.auth/tenant": "tenant-abc",
				"platform.auth/permissions": ["read", "write"]
			}
		}
	}`

	var req CallToolRequest
	err := json.Unmarshal([]byte(reqJSON), &req)
	require.NoError(t, err)

	// Server extracts auth metadata
	assert.NotNil(t, req.Params.Meta)
	assert.Equal(t, "token-123", req.Params.Meta.ProgressToken)

	authToken := req.Params.Meta.Get("platform.auth/token")
	assert.Equal(t, "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...", authToken)

	tenant := req.Params.Meta.Get("platform.auth/tenant")
	assert.Equal(t, "tenant-abc", tenant)

	permissions := req.Params.Meta.Get("platform.auth/permissions")
	assert.Equal(t, []interface{}{"read", "write"}, permissions)

	// Verify metadata is not exposed to LLM (not in arguments)
	_, hasAuthInArgs := req.Params.Arguments["platform.auth/token"]
	assert.False(t, hasAuthInArgs, "auth metadata should not be in arguments")
}
