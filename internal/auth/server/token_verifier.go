// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	oauthErrors "trpc.group/trpc-go/trpc-mcp-go/internal/errors"

	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jws"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

// standardClaims are filtered out of Extra because they are either mapped to
// AuthInfo first-class fields or are not useful as extra metadata.
// Mapped fields: client_id -> ClientID, sub -> Subject, scope -> Scopes,
// exp -> ExpiresAt, aud -> Resource (via extractResource)
var standardClaims = map[string]bool{
	"client_id": true,
	"sub":       true,
	"scope":     true,
	"exp":       true,
	"aud":       true,
	// Other common standard/housekeeping claims not needed in Extra
	"iss": true,
	"iat": true,
	"jti": true,
	"kid": true,
}

type TokenVerifierInterface interface {
	VerifyAccessToken(ctx context.Context, token string) (AuthInfo, error)
}

// LocalJWKSConfig local JWKS configuration
type LocalJWKSConfig struct {
	JWKS string // Local JWKS JSON string
	File string // Local JWKS file path
}

// RemoteJWKSConfig remote JWKS configuration
type RemoteJWKSConfig struct {
	URLs            []string          // Remote JWKS URLs
	IssuerToURL     map[string]string // Mapping from issuer (iss) to remote URL
	RefreshInterval time.Duration     // Refresh interval
}

// TokenVerifierConfig configuration for TokenVerifier
type TokenVerifierConfig struct {
	Local         *LocalJWKSConfig     // Local JWKS configuration
	Remote        *RemoteJWKSConfig    // Remote JWKS configuration
	Introspection *IntrospectionConfig // Remote introspection configuration (RFC7662)
}

// IntrospectionCredentials client credentials for introspection
type IntrospectionCredentials struct {
	ClientID     string
	ClientSecret string
}

// IntrospectionConfig remote introspection configuration
type IntrospectionConfig struct {
	// Default introspection endpoint (optional). Used when no issuer-bound endpoint is found.
	Endpoint string
	// Endpoint selection by issuer (multi-tenant).
	IssuerToEndpoint map[string]string

	// Default credentials and per-issuer credentials (optional).
	DefaultCredentials *IntrospectionCredentials
	IssuerCredentials  map[string]IntrospectionCredentials

	// HTTP timeout
	Timeout time.Duration

	// Cache TTL (positive) and negative cache TTL (for inactive tokens or 4xx/401, etc.).
	CacheTTL         time.Duration
	NegativeCacheTTL time.Duration

	// Whether to fall back to introspection on JWT verification failure.
	UseOnJWTFail bool
}

// TokenVerifier holds verification configuration and helpers
type TokenVerifier struct {
	localKeySet jwk.Set           // Local JWKS key set
	cache       *jwk.Cache        // Remote JWKS cache (jwx v2)
	issuerToURL map[string]string // Mapping from issuer (iss) to remote URL
	isRemote    bool              // Whether remote JWKS mode is enabled

	// RFC7662 introspection
	introspectionEnabled   bool
	httpClient             *http.Client
	defaultIntrospectEP    string
	issuerToIntrospectEP   map[string]string
	defaultCreds           *IntrospectionCredentials
	issuerCreds            map[string]IntrospectionCredentials
	useIntrospectionOnFail bool

	// Simple in-memory cache
	introspectCache   map[string]introspectionCacheEntry
	introspectCacheMu sync.RWMutex
	cacheTTL          time.Duration
	negativeCacheTTL  time.Duration
}

type TokenVerifierFunc func(ctx context.Context, token string) (AuthInfo, error)

func (f TokenVerifierFunc) VerifyAccessToken(ctx context.Context, token string) (AuthInfo, error) {
	return f(ctx, token)
}

// NewLocalTokenVerifier creates a TokenVerifier that uses only local JWKS
func newLocalTokenVerifier(ctx context.Context, cfg LocalJWKSConfig) (*TokenVerifier, error) {
	verifier := &TokenVerifier{}

	defaultSet := jwk.NewSet()

	// Load JWKS from string
	if cfg.JWKS != "" {
		set, err := jwk.Parse([]byte(cfg.JWKS))
		if err != nil {
			return nil, fmt.Errorf("failed to parse local JWKS: %w", err)
		}
		for i := 0; i < set.Len(); i++ {
			key, _ := set.Key(i)
			_ = defaultSet.AddKey(key)
		}
	}

	// Load JWKS from file
	if cfg.File != "" {
		set, err := jwk.ReadFile(cfg.File)
		if err != nil {
			return nil, fmt.Errorf("failed to parse local JWKS file: %w", err)
		}
		for i := 0; i < set.Len(); i++ {
			key, _ := set.Key(i)
			_ = defaultSet.AddKey(key)
		}
	}

	if defaultSet.Len() == 0 {
		return nil, fmt.Errorf("must provide JWKS or File")
	}

	verifier.localKeySet = defaultSet
	return verifier, nil
}

// NewRemoteTokenVerifier creates a TokenVerifier that uses only remote JWKS
func newRemoteTokenVerifier(ctx context.Context, cfg RemoteJWKSConfig) (*TokenVerifier, error) {
	if len(cfg.URLs) == 0 {
		return nil, fmt.Errorf("must provide at least one RemoteURL")
	}

	// jwx v2 cache
	cache := jwk.NewCache(ctx)
	for _, url_ := range cfg.URLs {
		_ = cache.Register(url_)
	}

	return &TokenVerifier{
		cache: cache,
		issuerToURL: func() map[string]string {
			if cfg.IssuerToURL == nil {
				return nil
			}
			m := make(map[string]string, len(cfg.IssuerToURL))
			for k, v := range cfg.IssuerToURL {
				m[k] = v
			}
			return m
		}(),
		isRemote: true,
	}, nil
}

// NewIntrospectionTokenVerifier creates a TokenVerifier that uses only RFC7662 introspection
func newIntrospectionTokenVerifier(ctx context.Context, cfg IntrospectionConfig) (*TokenVerifier, error) {
	verifier := &TokenVerifier{}

	to := cfg.Timeout
	if to <= 0 {
		to = 5 * time.Second
	}
	verifier.httpClient = &http.Client{Timeout: to}
	verifier.defaultIntrospectEP = cfg.Endpoint
	// Copy IssuerToEndpoint
	if cfg.IssuerToEndpoint != nil {
		verifier.issuerToIntrospectEP = make(map[string]string, len(cfg.IssuerToEndpoint))
		for k, v := range cfg.IssuerToEndpoint {
			verifier.issuerToIntrospectEP[k] = v
		}
	}
	// Copy DefaultCredentials
	if cfg.DefaultCredentials != nil {
		dc := *cfg.DefaultCredentials
		verifier.defaultCreds = &dc
	}
	// Copy IssuerCredentials
	if cfg.IssuerCredentials != nil {
		verifier.issuerCreds = make(map[string]IntrospectionCredentials, len(cfg.IssuerCredentials))
		for k, v := range cfg.IssuerCredentials {
			verifier.issuerCreds[k] = v
		}
	}
	verifier.useIntrospectionOnFail = cfg.UseOnJWTFail
	verifier.cacheTTL = cfg.CacheTTL
	if verifier.cacheTTL <= 0 {
		verifier.cacheTTL = 60 * time.Second
	}
	verifier.negativeCacheTTL = cfg.NegativeCacheTTL
	if verifier.negativeCacheTTL <= 0 {
		verifier.negativeCacheTTL = 15 * time.Second
	}
	verifier.introspectionEnabled = true
	verifier.introspectCache = make(map[string]introspectionCacheEntry)
	return verifier, nil
}

// NewTokenVerifier creates a comprehensive TokenVerifier.
// Provide any one or more configurations. The SDK will automatically prefer Local → Remote → Introspection (if enabled).
func NewTokenVerifier(ctx context.Context, cfg TokenVerifierConfig) (*TokenVerifier, error) {
	var verifier *TokenVerifier
	var err error

	if cfg.Remote != nil && len(cfg.Remote.URLs) > 0 {
		verifier, err = newRemoteTokenVerifier(ctx, *cfg.Remote)
		if err != nil {
			return nil, err
		}
	}

	if cfg.Local != nil && (cfg.Local.JWKS != "" || cfg.Local.File != "") {
		localVerifier, err := newLocalTokenVerifier(ctx, *cfg.Local)
		if err != nil {
			return nil, err
		}

		if verifier != nil {
			verifier.localKeySet = localVerifier.localKeySet
		} else {
			verifier = localVerifier
		}
	}

	if verifier == nil {
		// If no JWKS is provided, allow constructing an introspection-only mode
		if cfg.Introspection != nil {
			return newIntrospectionTokenVerifier(ctx, *cfg.Introspection)
		}
		return nil, errors.New("no verification method configured: configure Local JWKS (Local), or Remote JWKS (Remote), or Introspection")
	}

	// Initialize introspection (optional)
	if cfg.Introspection != nil {
		to := cfg.Introspection.Timeout
		if to <= 0 {
			to = 5 * time.Second
		}
		verifier.httpClient = &http.Client{Timeout: to}
		verifier.defaultIntrospectEP = cfg.Introspection.Endpoint
		// Copy IssuerToEndpoint to avoid external mutations
		if cfg.Introspection.IssuerToEndpoint != nil {
			verifier.issuerToIntrospectEP = make(map[string]string, len(cfg.Introspection.IssuerToEndpoint))
			for k, v := range cfg.Introspection.IssuerToEndpoint {
				verifier.issuerToIntrospectEP[k] = v
			}
		}
		// Copy DefaultCredentials
		if cfg.Introspection.DefaultCredentials != nil {
			dc := *cfg.Introspection.DefaultCredentials
			verifier.defaultCreds = &dc
		}
		// Copy IssuerCredentials
		if cfg.Introspection.IssuerCredentials != nil {
			verifier.issuerCreds = make(map[string]IntrospectionCredentials, len(cfg.Introspection.IssuerCredentials))
			for k, v := range cfg.Introspection.IssuerCredentials {
				verifier.issuerCreds[k] = v
			}
		}
		verifier.useIntrospectionOnFail = cfg.Introspection.UseOnJWTFail
		verifier.cacheTTL = cfg.Introspection.CacheTTL
		if verifier.cacheTTL <= 0 {
			verifier.cacheTTL = 60 * time.Second
		}
		verifier.negativeCacheTTL = cfg.Introspection.NegativeCacheTTL
		if verifier.negativeCacheTTL <= 0 {
			verifier.negativeCacheTTL = 15 * time.Second
		}
		verifier.introspectionEnabled = true
		verifier.introspectCache = make(map[string]introspectionCacheEntry)
	}

	// Do not set an explicit "mode"; choose dynamically during Verify based on configuration

	return verifier, nil
}

// VerifyAccessToken verifies an access token and returns AuthInfo or error
func (v *TokenVerifier) VerifyAccessToken(ctx context.Context, tokenStr string) (AuthInfo, error) {
	// If no JWKS configured and introspection is enabled: directly use introspection (works for opaque/JWT)
	if v.localKeySet == nil && !v.isRemote && v.introspectionEnabled {
		ai, err := v.introspectAccessToken(ctx, tokenStr, "")
		if err != nil {
			return AuthInfo{}, oauthErrors.NewOAuthError(oauthErrors.ErrInvalidToken, "failed to verify token", "")
		}
		return ai, nil
	}

	// Parse token first (without verifying signature) to obtain iss; if parsing fails and introspection is enabled, try introspection directly (supports opaque tokens).
	unverifiedToken, err := jwt.ParseInsecure([]byte(tokenStr))
	if err != nil {
		if v.introspectionEnabled {
			if ai, ierr := v.introspectAccessToken(ctx, tokenStr, ""); ierr == nil {
				return ai, nil
			}
		}
		return AuthInfo{}, oauthErrors.NewOAuthError(oauthErrors.ErrInvalidToken, "malformed token: cannot parse header/payload; if you are using opaque tokens, enable Introspection", "")
	}

	// Extract issuer (iss)
	iss := unverifiedToken.Issuer()
	if iss == "" {
		return AuthInfo{}, oauthErrors.NewOAuthError(oauthErrors.ErrInvalidToken, "missing issuer (iss) in token", "")
	}

	// Extract kid from JWS header
	kid, err := extractKIDFromHeader(tokenStr)
	if err != nil || kid == "" {
		// Try introspection fallback
		if v.introspectionEnabled && v.useIntrospectionOnFail {
			if ai, ierr := v.introspectAccessToken(ctx, tokenStr, iss); ierr == nil {
				return ai, nil
			}
		}
		return AuthInfo{}, oauthErrors.NewOAuthError(oauthErrors.ErrInvalidToken, "missing key id (kid) in JWS header; if your tokens omit kid, ensure the JWKS only contains one key or use Introspection fallback", "")
	}

	// Try to obtain target keySet
	keySet, err := v.getTargetKeySet(ctx, iss, kid)
	if err != nil {
		return AuthInfo{}, err
	}

	// Validate token with key set and basic claims, allowing time skew
	token, err := jwt.Parse([]byte(tokenStr),
		jwt.WithKeySet(keySet),
		jwt.WithValidate(true),
		jwt.WithAcceptableSkew(30*time.Second),
		// RFC 9068: exp and iat are validated automatically; here we only require presence for other claims
		jwt.WithRequiredClaim("exp"),
		jwt.WithRequiredClaim("aud"),
		jwt.WithRequiredClaim("sub"),
		jwt.WithRequiredClaim("iat"),
	)
	if err != nil || token == nil {
		// On JWT signature/claims validation failure, optionally fall back to introspection
		if v.introspectionEnabled && v.useIntrospectionOnFail {
			if ai, ierr := v.introspectAccessToken(ctx, tokenStr, iss); ierr == nil {
				return ai, nil
			}
		}
		return AuthInfo{}, oauthErrors.NewOAuthError(oauthErrors.ErrInvalidToken, "signature validation failed or claims invalid; ensure JWKS is configured for issuer or enable Introspection fallback", "")
	}

	// Ensure non-empty subject
	if sub := token.Subject(); sub == "" {
		return AuthInfo{}, oauthErrors.NewOAuthError(oauthErrors.ErrInvalidToken, "missing required 'sub' claim", "")
	}

	authInfo, err := v.convertJWTToAuthInfo(token, tokenStr)
	if err != nil {
		return AuthInfo{}, err
	}
	return authInfo, nil
}

func (v *TokenVerifier) getTargetKeySet(ctx context.Context, iss, kid string) (jwk.Set, error) {
	// Prefer local JWKS
	if v.localKeySet != nil {
		if _, ok := v.localKeySet.LookupKeyID(kid); ok {
			return v.localKeySet, nil
		}
	}

	// If remote mode enabled, try remote JWKS
	if v.isRemote {
		if url_, ok := v.issuerToURL[iss]; ok {
			if v.cache != nil {
				if keySet, err := v.cache.Get(ctx, url_); err == nil {
					if _, ok := keySet.LookupKeyID(kid); !ok {
						if refreshed, ferr := jwk.Fetch(ctx, url_); ferr == nil {
							if _, ok2 := refreshed.LookupKeyID(kid); ok2 {
								return refreshed, nil
							}
						}
					}
					return keySet, nil
				}
			}
			keySet, err := jwk.Fetch(ctx, url_)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch remote JWKS for issuer %s (url=%s): %w", iss, url_, err)
			}
			return keySet, nil
		}
		return nil, fmt.Errorf("no remote JWKS URL found for issuer %s: provide Remote.IssuerToURL mapping", iss)
	}

	return nil, fmt.Errorf("no JWKS found for issuer %s: neither Local nor Remote key set available", iss)
}

// ---- RFC7662 introspection implementation ----

type introspectionCacheEntry struct {
	authInfo  AuthInfo
	inactive  bool
	expiresAt time.Time
}

func (v *TokenVerifier) resolveIntrospectionEndpoint(issuer string) (string, *IntrospectionCredentials) {
	ep := ""
	if issuer != "" && v.issuerToIntrospectEP != nil {
		if e, ok := v.issuerToIntrospectEP[issuer]; ok {
			ep = e
		}
	}
	if ep == "" {
		ep = v.defaultIntrospectEP
	}
	var creds *IntrospectionCredentials
	if issuer != "" && v.issuerCreds != nil {
		if c, ok := v.issuerCreds[issuer]; ok {
			cc := c
			creds = &cc
		}
	}
	if creds == nil {
		creds = v.defaultCreds
	}
	return ep, creds
}

func (v *TokenVerifier) introspectionCacheKey(endpoint, token string) string {
	return endpoint + "|" + token
}

func (v *TokenVerifier) loadFromIntrospectionCache(key string) (introspectionCacheEntry, bool) {
	v.introspectCacheMu.RLock()
	defer v.introspectCacheMu.RUnlock()
	entry, ok := v.introspectCache[key]
	if !ok {
		return introspectionCacheEntry{}, false
	}
	if time.Now().After(entry.expiresAt) {
		return introspectionCacheEntry{}, false
	}
	return entry, true
}

func (v *TokenVerifier) storeToIntrospectionCache(key string, entry introspectionCacheEntry) {
	v.introspectCacheMu.Lock()
	v.introspectCache[key] = entry
	v.introspectCacheMu.Unlock()
}

func (v *TokenVerifier) introspectAccessToken(ctx context.Context, tokenStr, issuer string) (AuthInfo, error) {
	if !v.introspectionEnabled {
		return AuthInfo{}, errors.New("introspection not enabled")
	}
	endpoint, creds := v.resolveIntrospectionEndpoint(issuer)
	if endpoint == "" {
		return AuthInfo{}, errors.New("no introspection endpoint configured: set Introspection.Endpoint or IssuerToEndpoint for the issuer")
	}

	key := v.introspectionCacheKey(endpoint, tokenStr)
	if entry, ok := v.loadFromIntrospectionCache(key); ok {
		if entry.inactive {
			return AuthInfo{}, oauthErrors.NewOAuthError(oauthErrors.ErrInvalidToken, "inactive token", "")
		}
		return entry.authInfo, nil
	}

	form := url.Values{}
	form.Set("token", tokenStr)
	form.Set("token_type_hint", "access_token")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return AuthInfo{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if creds != nil && creds.ClientID != "" {
		basic := creds.ClientID + ":" + creds.ClientSecret
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(basic)))
	}

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return AuthInfo{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		v.storeToIntrospectionCache(key, introspectionCacheEntry{inactive: true, expiresAt: time.Now().Add(v.negativeCacheTTL)})
		return AuthInfo{}, oauthErrors.NewOAuthError(oauthErrors.ErrInvalidToken, "introspection request failed", "")
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return AuthInfo{}, err
	}
	active, _ := payload["active"].(bool)
	if !active {
		v.storeToIntrospectionCache(key, introspectionCacheEntry{inactive: true, expiresAt: time.Now().Add(v.negativeCacheTTL)})
		return AuthInfo{}, oauthErrors.NewOAuthError(oauthErrors.ErrInvalidToken, "inactive token", "")
	}

	ai, err := v.convertIntrospectionToAuthInfo(payload, tokenStr)
	if err != nil {
		return AuthInfo{}, err
	}

	ttl := v.cacheTTL
	if expV, ok := payload["exp"]; ok {
		switch t := expV.(type) {
		case float64:
			expTs := time.Unix(int64(t), 0)
			if expTs.After(time.Now()) {
				rem := time.Until(expTs)
				if rem < ttl {
					ttl = rem
				}
			}
		case json.Number:
			if v, err2 := t.Int64(); err2 == nil {
				expTs := time.Unix(v, 0)
				if expTs.After(time.Now()) {
					rem := time.Until(expTs)
					if rem < ttl {
						ttl = rem
					}
				}
			}
		}
	}
	v.storeToIntrospectionCache(key, introspectionCacheEntry{authInfo: ai, expiresAt: time.Now().Add(ttl)})
	return ai, nil
}

func (v *TokenVerifier) convertIntrospectionToAuthInfo(payload map[string]interface{}, tokenStr string) (AuthInfo, error) {
	var ai AuthInfo
	ai.Token = tokenStr
	if cid, _ := payload["client_id"].(string); cid != "" {
		ai.ClientID = cid
	}
	if sc, ok := payload["scope"]; ok {
		ai.Scopes = parseScopesFromRaw(sc)
	}
	switch exp := payload["exp"].(type) {
	case float64:
		ts := int64(exp)
		ai.ExpiresAt = &ts
	case json.Number:
		if v, err := exp.Int64(); err == nil {
			ai.ExpiresAt = &v
		}
	}
	if r, err := extractResourceFromIntrospection(payload["aud"]); err == nil {
		ai.Resource = r
	}

	extra := make(map[string]interface{})
	for k, v := range payload {
		if standardClaims[k] {
			continue
		}
		switch k {
		case "active", "username", "token_type", "token_type_hint":
			continue
		case "client_id", "scope", "exp", "aud", "iss", "sub", "iat", "jti":
			continue
		default:
			extra[k] = v
		}
	}
	if len(extra) > 0 {
		ai.Extra = extra
	}
	return ai, nil
}

func parseScopesFromRaw(raw interface{}) []string {
	switch s := raw.(type) {
	case string:
		if s == "" {
			return nil
		}
		return strings.Split(s, " ")
	case []interface{}:
		var scopes []string
		for _, v := range s {
			if str, ok := v.(string); ok && str != "" {
				scopes = append(scopes, str)
			}
		}
		if len(scopes) == 0 {
			return nil
		}
		return scopes
	default:
		return nil
	}
}

func extractResourceFromIntrospection(audRaw interface{}) (*url.URL, error) {
	if audRaw == nil {
		return nil, nil
	}
	var candidates []string
	switch v := audRaw.(type) {
	case string:
		if v != "" {
			candidates = []string{v}
		}
	case []interface{}:
		for _, it := range v {
			if s, ok := it.(string); ok && s != "" {
				candidates = append(candidates, s)
			}
		}
	case []string:
		candidates = v
	}
	for _, c := range candidates {
		looksLikeURL := strings.HasPrefix(c, "http://") || strings.HasPrefix(c, "https://") || strings.Contains(c, "://")
		if !looksLikeURL {
			continue
		}
		u, err := url.Parse(c)
		if err != nil || u == nil || u.Scheme == "" || u.Host == "" {
			continue
		}
		u.Fragment = "" // Remove fragment (per RFC 8707)
		return u, nil
	}
	return nil, nil
}

// extractKIDFromHeader 从 JWS Header 提取 kid
func extractKIDFromHeader(tokenStr string) (string, error) {
	msg, err := jws.Parse([]byte(tokenStr))
	if err != nil {
		return "", fmt.Errorf("failed to parse JWS: %w", err)
	}
	sigs := msg.Signatures()
	if len(sigs) == 0 {
		return "", errors.New("no signatures found in JWS")
	}

	// Prefer protected headers
	if ph := sigs[0].ProtectedHeaders(); ph != nil {
		if v, ok := ph.Get(jws.KeyIDKey); ok {
			if kid, ok2 := v.(string); ok2 && kid != "" {
				return kid, nil
			}
		}
	}
	return "", errors.New("missing kid in JWS header")
}

// convertJWTToAuthInfo converts jwt.Token to AuthInfo structure.
func (v *TokenVerifier) convertJWTToAuthInfo(token jwt.Token, tokenStr string) (AuthInfo, error) {
	authInfo := AuthInfo{Token: tokenStr}

	// Write exp -> ExpiresAt (must be done first)
	if exp := token.Expiration(); !exp.IsZero() {
		ts := exp.Unix()
		authInfo.ExpiresAt = &ts
	} else {
		// This case should not be reached because WithRequiredClaim("exp") was used in Parse
		// But for robustness, return invalid_token for clarity
		return AuthInfo{}, oauthErrors.NewOAuthError(oauthErrors.ErrInvalidToken, "missing exp claim", "")
	}

	// Extract OAuth-related fields
	var err error
	if authInfo.ClientID, err = extractClientID(token); err != nil {
		return AuthInfo{}, err
	}
	if authInfo.Resource, err = extractResource(token); err != nil {
		return AuthInfo{}, err
	}
	if authInfo.Scopes, err = extractScopes(token); err != nil {
		return AuthInfo{}, err
	}

	// Write subject -> AuthInfo.Subject
	if s := token.Subject(); s != "" {
		authInfo.Subject = s
	}

	// Other custom claims
	authInfo.Extra = extractExtra(token)
	return authInfo, nil
}

// extractClientID extracts client ID (optional)
func extractClientID(token jwt.Token) (string, error) {
	// client_id is not mandatory for access tokens (RFC9068)
	if v, ok := token.Get("client_id"); ok {
		if s, ok2 := v.(string); ok2 && s != "" {
			return s, nil
		}
	}
	// Fallback to azp (often used in OIDC)
	if v, ok := token.Get("azp"); ok {
		if s, ok2 := v.(string); ok2 && s != "" {
			return s, nil
		}
	}
	// Missing client identifier is acceptable
	return "", nil
}

// extractScopes extracts scopes from various claim formats (scope/scp). Missing is acceptable.
func extractScopes(token jwt.Token) ([]string, error) {
	var raw interface{}
	// Prefer RFC6749 style "scope" (space-delimited string or array)
	if v, ok := token.Get("scope"); ok {
		raw = v
	} else {
		// Fallback to "scp" (array of strings used by some providers)
		if v2, ok2 := token.Get("scp"); ok2 {
			raw = v2
		} else {
			// No scopes present → treat as empty without error
			return nil, nil
		}
	}

	switch s := raw.(type) {
	case string:
		if s == "" {
			return nil, nil
		}
		return strings.Split(s, " "), nil
	case []string:
		if len(s) == 0 {
			return nil, nil
		}
		return s, nil
	case []interface{}:
		if len(s) == 0 {
			return nil, nil
		}
		var scopes []string
		for _, v := range s {
			if str, ok := v.(string); ok {
				scopes = append(scopes, str)
			}
		}
		if len(scopes) == 0 {
			return nil, nil
		}
		return scopes, nil
	default:
		// Unknown format → ignore rather than failing hard for compatibility
		return nil, nil
	}
}

// extractResource extracts resource information
func extractResource(token jwt.Token) (*url.URL, error) {
	aud := token.Audience()
	if len(aud) == 0 {
		return nil, fmt.Errorf("missing required 'aud' claim")
	}

	// Iterate to find the first value that looks like a URL and is parseable as HTTP(S);
	// if none are URLs, return nil to indicate no resource indicator provided.
	for _, candidate := range aud {
		if candidate == "" {
			continue
		}
		looksLikeURL := strings.HasPrefix(candidate, "http://") || strings.HasPrefix(candidate, "https://") || strings.Contains(candidate, "://")
		if !looksLikeURL {
			continue
		}
		resourceURL, err := url.Parse(candidate)
		if err != nil || resourceURL == nil {
			continue
		}
		if resourceURL.Scheme == "" || resourceURL.Host == "" {
			continue
		}
		resourceURL.Fragment = "" // Remove fragment (per RFC 8707)
		return resourceURL, nil
	}
	return nil, nil
}

// extractExtra extracts custom claims to Extra map
func extractExtra(token jwt.Token) map[string]interface{} {
	all, _ := token.AsMap(context.Background())
	if len(all) == 0 {
		return nil
	}

	extra := make(map[string]interface{})
	for key, value := range all {
		if standardClaims[key] {
			continue
		}
		switch key {
		case "active", "username", "token_type", "token_type_hint":
			// Known noise in introspection/JWT contexts; exclude from Extra
			continue
		default:
			extra[key] = value
		}
	}
	if len(extra) == 0 {
		return nil
	}
	return extra
}

// Note: TokenVerifier is statically configured. After initialization, it does not support dynamically adding issuer mappings or clearing the local KeySet.
