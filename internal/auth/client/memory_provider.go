// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package client

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"trpc.group/trpc-go/trpc-mcp-go/internal/auth"
)

// InMemoryOAuthClientProvider provides an in-memory implementation of OAuthClientProvider
// It stores client information, tokens, and PKCE code verifiers with thread safety
//
// NOTE: This is a demo implementation for testing and development purposes only.
// It is NOT recommended for production use as it stores sensitive data in memory
// without persistence or proper security measures.
type InMemoryOAuthClientProvider struct {
	redirectURL    string
	clientMetadata auth.OAuthClientMetadata
	clientInfo     *auth.OAuthClientInformation
	tokens         *auth.OAuthTokens
	codeVerifier   string
	onRedirect     func(*url.URL) error
	mutex          sync.RWMutex
}

// NewInMemoryOAuthClientProvider creates a new in-memory OAuth client provider
func NewInMemoryOAuthClientProvider(
	redirectURL string,
	clientMetadata auth.OAuthClientMetadata,
	onRedirect func(*url.URL) error) *InMemoryOAuthClientProvider {
	if onRedirect == nil {
		onRedirect = func(u *url.URL) error {
			return nil
		}
	}
	return &InMemoryOAuthClientProvider{
		redirectURL:    redirectURL,
		clientMetadata: clientMetadata,
		onRedirect:     onRedirect,
	}
}

// RedirectURL returns the registered redirect URL
func (p *InMemoryOAuthClientProvider) RedirectURL() string {
	return p.redirectURL
}

// ClientMetadata returns the client metadata
func (p *InMemoryOAuthClientProvider) ClientMetadata() auth.OAuthClientMetadata {
	return p.clientMetadata
}

// ClientInformation returns stored client credentials if available
func (p *InMemoryOAuthClientProvider) ClientInformation() *auth.OAuthClientInformation {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.clientInfo
}

// SaveClientInformation saves client credentials into memory
func (p *InMemoryOAuthClientProvider) SaveClientInformation(clientInformation auth.OAuthClientInformationFull) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.clientInfo = &auth.OAuthClientInformation{
		ClientID:              clientInformation.ClientID,
		ClientSecret:          clientInformation.ClientSecret,
		ClientIDIssuedAt:      clientInformation.ClientIDIssuedAt,
		ClientSecretExpiresAt: clientInformation.ClientSecretExpiresAt,
	}
	return nil
}

// Tokens returns stored tokens if available
func (p *InMemoryOAuthClientProvider) Tokens() (*auth.OAuthTokens, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.tokens, nil
}

// SaveTokens saves OAuth tokens into memory
func (p *InMemoryOAuthClientProvider) SaveTokens(tokens auth.OAuthTokens) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.tokens = &tokens
	return nil
}

// RedirectToAuthorization executes the redirect callback with authorization URL
func (p *InMemoryOAuthClientProvider) RedirectToAuthorization(authorizationUrl *url.URL) error {
	return p.onRedirect(authorizationUrl)
}

// CodeVerifier retrieves the stored PKCE code verifier
func (p *InMemoryOAuthClientProvider) CodeVerifier() (string, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	if p.codeVerifier == "" {
		return "", fmt.Errorf("no code verifier saved")
	}
	return p.codeVerifier, nil
}

// SaveCodeVerifier saves the PKCE code verifier into memory
func (p *InMemoryOAuthClientProvider) SaveCodeVerifier(codeVerifier string) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.codeVerifier = codeVerifier
	return nil
}

// State generates a random state string for CSRF protection
func (p *InMemoryOAuthClientProvider) State() (string, error) {
	// Generate a random state parameter for CSRF protection
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random state: %w", err)
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// AddClientAuthentication adds client authentication parameters using client_secret_post
func (p *InMemoryOAuthClientProvider) AddClientAuthentication(headers http.Header, params url.Values, tokenUrl string) error {
	// Add client authentication using client_secret_post method
	p.mutex.RLock()
	clientInfo := p.clientInfo
	p.mutex.RUnlock()

	if clientInfo != nil && clientInfo.ClientID != "" {
		params.Set("client_id", clientInfo.ClientID)
		if clientInfo.ClientSecret != "" {
			params.Set("client_secret", clientInfo.ClientSecret)
		}
	}
	return nil
}

// ValidateResourceURL validates the resource URL from metadata against the server URL
func (p *InMemoryOAuthClientProvider) ValidateResourceURL(serverUrl *url.URL, resourceMetadata *auth.OAuthProtectedResourceMetadata) (*url.URL, error) {
	// If no resource metadata provided, return nil (no resource parameter needed)
	if resourceMetadata == nil {
		return nil, nil
	}

	// Parse the resource URL from metadata
	resourceURL, err := url.Parse(resourceMetadata.Resource)
	if err != nil {
		return nil, fmt.Errorf("invalid resource URL in metadata: %w", err)
	}

	// Basic validation: ensure the resource URL has the same origin as server URL
	if resourceURL.Scheme != serverUrl.Scheme || resourceURL.Host != serverUrl.Host {
		// Allow if resource URL is a more specific path under the same origin
		if !strings.HasPrefix(resourceURL.String(), serverUrl.Scheme+"://"+serverUrl.Host) {
			return nil, fmt.Errorf("resource URL %s does not match server origin %s://%s",
				resourceURL.String(), serverUrl.Scheme, serverUrl.Host)
		}
	}

	return resourceURL, nil
}

// InvalidateCredentials clears stored credentials based on scope
func (p *InMemoryOAuthClientProvider) InvalidateCredentials(scope string) error {
	// Clear the corresponding credentials
	// according to the scope and use a mutex to protect them
	p.mutex.Lock()
	defer p.mutex.Unlock()

	switch scope {
	case "all":
		p.clientInfo = nil
		p.tokens = nil
		p.codeVerifier = ""
	case "client":
		p.clientInfo = nil
	case "tokens":
		p.tokens = nil
	case "verifier":
		p.codeVerifier = ""
	default:
		return fmt.Errorf("unknown invalidation scope: %s", scope)
	}
	return nil
}
