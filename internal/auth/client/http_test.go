// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package client

import (
	"context"
	"errors"
	"testing"
	"time"

	"trpc.group/trpc-go/trpc-mcp-go/internal/auth"
)

func TestWithAuthInfo(t *testing.T) {
	ctx := context.Background()

	// Test with nil auth info
	newCtx := WithAuthInfo(ctx, nil)
	if newCtx != ctx {
		t.Error("Expected same context when auth info is nil")
	}

	// Test with valid auth info
	authInfo := &ClientAuthInfo{
		AccessToken: "test-token",
		Scopes:      []string{"read", "write"},
	}

	newCtx = WithAuthInfo(ctx, authInfo)
	if newCtx == ctx {
		t.Error("Expected different context when auth info is provided")
	}

	// Verify the auth info was stored
	stored, ok := GetAuthInfo(newCtx)
	if !ok {
		t.Error("Expected to retrieve auth info from context")
	}
	if stored != authInfo {
		t.Error("Expected stored auth info to match original")
	}
}

func TestGetAuthInfo(t *testing.T) {
	ctx := context.Background()

	// Test with empty context
	authInfo, ok := GetAuthInfo(ctx)
	if ok {
		t.Error("Expected no auth info in empty context")
	}
	if authInfo != nil {
		t.Error("Expected nil auth info from empty context")
	}

	// Test with auth info in context
	originalAuthInfo := &ClientAuthInfo{
		AccessToken:  "test-access-token",
		RefreshToken: stringPtr("test-refresh-token"),
		Scopes:       []string{"read", "write"},
		Extra:        map[string]interface{}{"custom": "value"},
	}

	ctx = WithAuthInfo(ctx, originalAuthInfo)
	authInfo, ok = GetAuthInfo(ctx)

	if !ok {
		t.Error("Expected to find auth info in context")
	}
	if authInfo == nil {
		t.Fatal("Expected non-nil auth info")
	}
	if authInfo.AccessToken != originalAuthInfo.AccessToken {
		t.Errorf("Expected AccessToken %s, got %s", originalAuthInfo.AccessToken, authInfo.AccessToken)
	}
	if *authInfo.RefreshToken != *originalAuthInfo.RefreshToken {
		t.Errorf("Expected RefreshToken %s, got %s", *originalAuthInfo.RefreshToken, *authInfo.RefreshToken)
	}

	// Test with nil auth info stored in context
	ctx = context.WithValue(context.Background(), ctxKeyClientAuthInfo, (*ClientAuthInfo)(nil))
	authInfo, ok = GetAuthInfo(ctx)
	if ok {
		t.Error("Expected no auth info when nil is stored")
	}
	if authInfo != nil {
		t.Error("Expected nil auth info when nil is stored")
	}

	// Test with wrong type in context
	ctx = context.WithValue(context.Background(), ctxKeyClientAuthInfo, "not-auth-info")
	authInfo, ok = GetAuthInfo(ctx)
	if ok {
		t.Error("Expected no auth info when wrong type is stored")
	}
	if authInfo != nil {
		t.Error("Expected nil auth info when wrong type is stored")
	}
}

func TestWithAuthErr(t *testing.T) {
	ctx := context.Background()

	// Test with nil error
	newCtx := WithAuthErr(ctx, nil)
	if newCtx != ctx {
		t.Error("Expected same context when error is nil")
	}

	// Test with valid error
	testErr := errors.New("auth error")
	newCtx = WithAuthErr(ctx, testErr)
	if newCtx == ctx {
		t.Error("Expected different context when error is provided")
	}

	// Verify the error was stored
	stored := newCtx.Value(ctxKeyClientAuthErr)
	if stored == nil {
		t.Error("Expected error to be stored in context")
	}
	if stored != testErr {
		t.Errorf("Expected stored error %v, got %v", testErr, stored)
	}
}

func TestConvertTokensToAuthInfo(t *testing.T) {
	// Test with nil tokens
	authInfo := ConvertTokensToAuthInfo(nil)
	if authInfo != nil {
		t.Error("Expected nil auth info when tokens is nil")
	}

	// Test with empty access token
	emptyTokens := &auth.OAuthTokens{
		AccessToken: "",
	}
	authInfo = ConvertTokensToAuthInfo(emptyTokens)
	if authInfo != nil {
		t.Error("Expected nil auth info when access token is empty")
	}

	// Test with valid tokens (minimal)
	validTokens := &auth.OAuthTokens{
		AccessToken: "test-access-token",
		TokenType:   "Bearer",
	}
	authInfo = ConvertTokensToAuthInfo(validTokens)
	if authInfo == nil {
		t.Fatal("Expected non-nil auth info")
	}
	if authInfo.AccessToken != validTokens.AccessToken {
		t.Errorf("Expected AccessToken %s, got %s", validTokens.AccessToken, authInfo.AccessToken)
	}
	if authInfo.RefreshToken != nil {
		t.Error("Expected nil RefreshToken when not provided")
	}
	if authInfo.ExpiresAt != nil {
		t.Error("Expected nil ExpiresAt when ExpiresIn not provided")
	}
	if authInfo.Extra == nil {
		t.Error("Expected Extra map to be initialized")
	}

	// Test with full tokens
	refreshToken := "test-refresh-token"
	expiresIn := int64(3600)
	scope := "read write admin"
	fullTokens := &auth.OAuthTokens{
		AccessToken:  "test-access-token",
		RefreshToken: &refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    &expiresIn,
		Scope:        &scope,
	}

	before := time.Now()
	authInfo = ConvertTokensToAuthInfo(fullTokens)
	after := time.Now()

	if authInfo == nil {
		t.Fatal("Expected non-nil auth info")
	}
	if authInfo.AccessToken != fullTokens.AccessToken {
		t.Errorf("Expected AccessToken %s, got %s", fullTokens.AccessToken, authInfo.AccessToken)
	}
	if authInfo.RefreshToken == nil {
		t.Fatal("Expected RefreshToken to be set")
	}
	if *authInfo.RefreshToken != *fullTokens.RefreshToken {
		t.Errorf("Expected RefreshToken %s, got %s", *fullTokens.RefreshToken, *authInfo.RefreshToken)
	}

	// Test ExpiresAt calculation
	if authInfo.ExpiresAt == nil {
		t.Fatal("Expected ExpiresAt to be set")
	}
	expectedExpiry := before.Add(time.Duration(expiresIn) * time.Second)
	actualExpiry := *authInfo.ExpiresAt
	if actualExpiry.Before(expectedExpiry) || actualExpiry.After(after.Add(time.Duration(expiresIn)*time.Second)) {
		t.Errorf("Expected ExpiresAt around %v, got %v", expectedExpiry, actualExpiry)
	}

	// Test scopes parsing (currently returns empty slice)
	if len(authInfo.Scopes) != 0 {
		t.Errorf("Expected empty scopes (parseTokenScopes returns empty), got %v", authInfo.Scopes)
	}
}

func TestIsTokenExpired(t *testing.T) {
	// Test with nil auth info
	if !IsTokenExpired(nil) {
		t.Error("Expected nil auth info to be considered expired")
	}

	// Test with no expiration time
	authInfoNoExpiry := &ClientAuthInfo{
		AccessToken: "test-token",
	}
	if IsTokenExpired(authInfoNoExpiry) {
		t.Error("Expected auth info without expiry to not be expired")
	}

	// Test with future expiration (not expired)
	futureExpiry := time.Now().Add(time.Hour)
	authInfoFuture := &ClientAuthInfo{
		AccessToken: "test-token",
		ExpiresAt:   &futureExpiry,
	}
	if IsTokenExpired(authInfoFuture) {
		t.Error("Expected auth info with future expiry to not be expired")
	}

	// Test with past expiration (expired)
	pastExpiry := time.Now().Add(-time.Hour)
	authInfoPast := &ClientAuthInfo{
		AccessToken: "test-token",
		ExpiresAt:   &pastExpiry,
	}
	if !IsTokenExpired(authInfoPast) {
		t.Error("Expected auth info with past expiry to be expired")
	}

	// Test with expiration within 30 seconds (considered expired due to buffer)
	soonExpiry := time.Now().Add(15 * time.Second)
	authInfoSoon := &ClientAuthInfo{
		AccessToken: "test-token",
		ExpiresAt:   &soonExpiry,
	}
	if !IsTokenExpired(authInfoSoon) {
		t.Error("Expected auth info expiring within 30 seconds to be considered expired")
	}

	// Test with expiration just outside 30 second buffer
	laterExpiry := time.Now().Add(35 * time.Second)
	authInfoLater := &ClientAuthInfo{
		AccessToken: "test-token",
		ExpiresAt:   &laterExpiry,
	}
	if IsTokenExpired(authInfoLater) {
		t.Error("Expected auth info expiring outside 30 second buffer to not be expired")
	}
}

func TestParseTokenScopes(t *testing.T) {
	// Test with nil tokens
	scopes := parseTokenScopes(nil)
	if len(scopes) != 0 {
		t.Errorf("Expected empty scopes for nil tokens, got %v", scopes)
	}

	// Test with tokens without scope
	tokens := &auth.OAuthTokens{
		AccessToken: "test-token",
	}
	scopes = parseTokenScopes(tokens)
	if len(scopes) != 0 {
		t.Errorf("Expected empty scopes when no scope in tokens, got %v", scopes)
	}

	// Test with tokens with scope
	scope := "read write admin"
	tokensWithScope := &auth.OAuthTokens{
		AccessToken: "test-token",
		Scope:       &scope,
	}
	scopes = parseTokenScopes(tokensWithScope)
	// Currently returns empty slice, but testing the current behavior
	if len(scopes) != 0 {
		t.Errorf("Expected empty scopes (current implementation), got %v", scopes)
	}
}

func TestClientAuthInfoComplete(t *testing.T) {
	// Test complete ClientAuthInfo structure
	refreshToken := "refresh-123"
	expiresAt := time.Now().Add(time.Hour)
	authInfo := &ClientAuthInfo{
		AccessToken:  "access-123",
		RefreshToken: &refreshToken,
		ExpiresAt:    &expiresAt,
		Scopes:       []string{"read", "write", "admin"},
		Extra: map[string]interface{}{
			"user_id":  "12345",
			"username": "testuser",
			"custom":   true,
		},
	}

	// Test all fields are preserved
	if authInfo.AccessToken != "access-123" {
		t.Errorf("Expected AccessToken access-123, got %s", authInfo.AccessToken)
	}
	if authInfo.RefreshToken == nil || *authInfo.RefreshToken != refreshToken {
		t.Errorf("Expected RefreshToken %s, got %v", refreshToken, authInfo.RefreshToken)
	}
	if authInfo.ExpiresAt == nil || !authInfo.ExpiresAt.Equal(expiresAt) {
		t.Errorf("Expected ExpiresAt %v, got %v", expiresAt, authInfo.ExpiresAt)
	}
	if len(authInfo.Scopes) != 3 {
		t.Errorf("Expected 3 scopes, got %d", len(authInfo.Scopes))
	}
	if authInfo.Extra["user_id"] != "12345" {
		t.Errorf("Expected Extra user_id 12345, got %v", authInfo.Extra["user_id"])
	}
}

func TestContextKeyUniqueness(t *testing.T) {
	// Test that context keys are unique
	if ctxKeyClientAuthInfo == ctxKeyClientAuthErr {
		t.Error("Expected context keys to be unique")
	}

	// Test that different values can be stored with different keys
	ctx := context.Background()
	authInfo := &ClientAuthInfo{AccessToken: "test"}
	authErr := errors.New("test error")

	ctx = WithAuthInfo(ctx, authInfo)
	ctx = WithAuthErr(ctx, authErr)

	// Both should be retrievable
	storedAuthInfo, ok := GetAuthInfo(ctx)
	if !ok || storedAuthInfo != authInfo {
		t.Error("Expected to retrieve stored auth info")
	}

	storedAuthErr := ctx.Value(ctxKeyClientAuthErr)
	if storedAuthErr != authErr {
		t.Error("Expected to retrieve stored auth error")
	}
}

func TestAuthInfoEdgeCases(t *testing.T) {
	// Test with zero values
	authInfo := &ClientAuthInfo{}
	if authInfo.AccessToken != "" {
		t.Error("Expected empty AccessToken by default")
	}
	if authInfo.RefreshToken != nil {
		t.Error("Expected nil RefreshToken by default")
	}
	if authInfo.ExpiresAt != nil {
		t.Error("Expected nil ExpiresAt by default")
	}
	if authInfo.Scopes != nil {
		t.Error("Expected nil Scopes by default")
	}
	if authInfo.Extra != nil {
		t.Error("Expected nil Extra by default")
	}

	// Test IsTokenExpired with zero value auth info
	if IsTokenExpired(authInfo) {
		t.Error("Expected zero value auth info (no expiry) to not be expired")
	}
}

func TestTokenExpirationBoundaryConditions(t *testing.T) {
	// Test exactly at 30 second boundary
	exactBoundary := time.Now().Add(30 * time.Second)
	authInfoBoundary := &ClientAuthInfo{
		AccessToken: "test-token",
		ExpiresAt:   &exactBoundary,
	}

	// Due to the -30 second buffer, this should be considered expired
	if !IsTokenExpired(authInfoBoundary) {
		t.Error("Expected token expiring at exactly 30 seconds to be considered expired")
	}

	// Test just before the boundary
	justBefore := time.Now().Add(29 * time.Second)
	authInfoJustBefore := &ClientAuthInfo{
		AccessToken: "test-token",
		ExpiresAt:   &justBefore,
	}
	if !IsTokenExpired(authInfoJustBefore) {
		t.Error("Expected token expiring before 30 second buffer to be expired")
	}

	// Test just after the boundary
	justAfter := time.Now().Add(31 * time.Second)
	authInfoJustAfter := &ClientAuthInfo{
		AccessToken: "test-token",
		ExpiresAt:   &justAfter,
	}
	if IsTokenExpired(authInfoJustAfter) {
		t.Error("Expected token expiring after 30 second buffer to not be expired")
	}
}

func TestConvertTokensToAuthInfoEdgeCases(t *testing.T) {
	// Test with tokens having only access token
	minimalTokens := &auth.OAuthTokens{
		AccessToken: "minimal-token",
		TokenType:   "Bearer",
	}
	authInfo := ConvertTokensToAuthInfo(minimalTokens)
	if authInfo == nil {
		t.Fatal("Expected non-nil auth info for minimal valid tokens")
	}
	if authInfo.AccessToken != "minimal-token" {
		t.Errorf("Expected AccessToken minimal-token, got %s", authInfo.AccessToken)
	}
	if authInfo.RefreshToken != nil {
		t.Error("Expected nil RefreshToken when not provided")
	}
	if authInfo.ExpiresAt != nil {
		t.Error("Expected nil ExpiresAt when ExpiresIn not provided")
	}

	// Test with zero ExpiresIn
	zeroExpiresIn := int64(0)
	tokensZeroExpiry := &auth.OAuthTokens{
		AccessToken: "test-token",
		ExpiresIn:   &zeroExpiresIn,
	}
	authInfo = ConvertTokensToAuthInfo(tokensZeroExpiry)
	if authInfo == nil {
		t.Fatal("Expected non-nil auth info")
	}
	if authInfo.ExpiresAt == nil {
		t.Fatal("Expected ExpiresAt to be set even with zero ExpiresIn")
	}
	// Should be approximately now (since ExpiresIn is 0)
	if time.Since(*authInfo.ExpiresAt) > time.Second {
		t.Error("Expected ExpiresAt to be approximately now when ExpiresIn is 0")
	}

	// Test with negative ExpiresIn
	negativeExpiresIn := int64(-3600)
	tokensNegativeExpiry := &auth.OAuthTokens{
		AccessToken: "test-token",
		ExpiresIn:   &negativeExpiresIn,
	}
	authInfo = ConvertTokensToAuthInfo(tokensNegativeExpiry)
	if authInfo == nil {
		t.Fatal("Expected non-nil auth info")
	}
	if authInfo.ExpiresAt == nil {
		t.Fatal("Expected ExpiresAt to be set even with negative ExpiresIn")
	}
	// Should be in the past
	if authInfo.ExpiresAt.After(time.Now()) {
		t.Error("Expected ExpiresAt to be in the past when ExpiresIn is negative")
	}
}

func TestConcurrentContextOperations(t *testing.T) {
	// Test concurrent access to context operations
	ctx := context.Background()

	// Set up initial context with auth info
	authInfo := &ClientAuthInfo{
		AccessToken: "concurrent-test-token",
		Scopes:      []string{"read"},
	}
	ctx = WithAuthInfo(ctx, authInfo)

	// Test concurrent reads
	numGoroutines := 10
	results := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			storedInfo, ok := GetAuthInfo(ctx)
			results <- ok && storedInfo != nil && storedInfo.AccessToken == "concurrent-test-token"
		}()
	}

	// Collect results
	for i := 0; i < numGoroutines; i++ {
		if !<-results {
			t.Error("Expected successful concurrent read of auth info")
		}
	}
}

func TestAuthInfoDeepCopy(t *testing.T) {
	// Test that modifications to returned auth info don't affect stored version
	refreshToken := "original-refresh"
	expiresAt := time.Now().Add(time.Hour)
	originalAuthInfo := &ClientAuthInfo{
		AccessToken:  "original-access",
		RefreshToken: &refreshToken,
		ExpiresAt:    &expiresAt,
		Scopes:       []string{"read", "write"},
		Extra:        map[string]interface{}{"key": "value"},
	}

	ctx := WithAuthInfo(context.Background(), originalAuthInfo)
	retrievedAuthInfo, ok := GetAuthInfo(ctx)
	if !ok {
		t.Fatal("Expected to retrieve auth info")
	}

	// Modify the retrieved auth info
	retrievedAuthInfo.AccessToken = "modified-access"
	*retrievedAuthInfo.RefreshToken = "modified-refresh"
	retrievedAuthInfo.Scopes[0] = "modified"
	retrievedAuthInfo.Extra["key"] = "modified"

	// Check that original is also modified (since we're returning the same pointer)
	// This test documents the current behavior - no deep copy is performed
	if originalAuthInfo.AccessToken != "modified-access" {
		t.Error("Auth info appears to be deep copied (might be unexpected)")
	}
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}

// Benchmark tests
func BenchmarkWithAuthInfo(b *testing.B) {
	ctx := context.Background()
	authInfo := &ClientAuthInfo{
		AccessToken: "benchmark-token",
		Scopes:      []string{"read", "write"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		WithAuthInfo(ctx, authInfo)
	}
}

func BenchmarkGetAuthInfo(b *testing.B) {
	authInfo := &ClientAuthInfo{
		AccessToken: "benchmark-token",
		Scopes:      []string{"read", "write"},
	}
	ctx := WithAuthInfo(context.Background(), authInfo)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GetAuthInfo(ctx)
	}
}

func BenchmarkConvertTokensToAuthInfo(b *testing.B) {
	refreshToken := "bench-refresh-token"
	expiresIn := int64(3600)
	tokens := &auth.OAuthTokens{
		AccessToken:  "bench-access-token",
		RefreshToken: &refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    &expiresIn,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ConvertTokensToAuthInfo(tokens)
	}
}

func BenchmarkIsTokenExpired(b *testing.B) {
	expiresAt := time.Now().Add(time.Hour)
	authInfo := &ClientAuthInfo{
		AccessToken: "bench-token",
		ExpiresAt:   &expiresAt,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsTokenExpired(authInfo)
	}
}
