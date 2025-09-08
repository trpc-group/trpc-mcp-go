// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package server

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthInfoFields(t *testing.T) {
	// Setting authInfo data
	token := "mock-token"
	clientID := "client-123"
	scopes := []string{"read", "write"}
	expiresAt := int64(1609459200) // 2021-01-01 00:00:00
	resource, err := url.Parse("https://example.com/resource")
	require.NoError(t, err)
	extra := map[string]interface{}{"key1": "value1"}

	authInfo := &AuthInfo{
		Token:     token,
		ClientID:  clientID,
		Scopes:    scopes,
		ExpiresAt: &expiresAt,
		Resource:  resource,
		Extra:     extra,
	}

	// Verify that the stored data is correct
	assert.Equal(t, token, authInfo.Token)
	assert.Equal(t, clientID, authInfo.ClientID)
	assert.ElementsMatch(t, scopes, authInfo.Scopes)
	assert.Equal(t, expiresAt, *authInfo.ExpiresAt)
	assert.Equal(t, resource, authInfo.Resource)
	assert.Equal(t, extra, authInfo.Extra)
}

func TestAuthInfoWithNilExpiresAt(t *testing.T) {
	authInfo := &AuthInfo{
		Token:     "mock-token",
		ClientID:  "client-123",
		Scopes:    []string{"read", "write"},
		ExpiresAt: nil, // nil expiresAt
	}

	// Verify that ExpiresAt is nil
	assert.Nil(t, authInfo.ExpiresAt)
}

func TestAuthInfoResourceValidation(t *testing.T) {
	validURL, err := url.Parse("https://example.com/resource")
	require.NoError(t, err)
	invalidURL, err := url.Parse("https://example.com/invalid-resource")
	require.NoError(t, err)

	authInfo := &AuthInfo{
		Token:    "mock-token",
		ClientID: "client-123",
		Scopes:   []string{"read"},
		Resource: validURL,
	}

	// Verify that resource match
	assert.Equal(t, validURL.String(), authInfo.Resource.String())

	// Setting Resource to an invalid URL
	authInfo.Resource = invalidURL
	assert.Equal(t, invalidURL.String(), authInfo.Resource.String())
}

func TestAuthInfoExtraData(t *testing.T) {
	extraData := map[string]interface{}{"key1": "value1", "key2": 1234}

	authInfo := &AuthInfo{
		Token:    "mock-token",
		ClientID: "client-123",
		Scopes:   []string{"read"},
		Extra:    extraData,
	}

	// Verify that Extra data is stored correctly
	assert.Equal(t, extraData, authInfo.Extra)
}
