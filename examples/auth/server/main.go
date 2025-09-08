// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/golang-jwt/jwt/v4"
	mcp "trpc.group/trpc-go/trpc-mcp-go"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth/server"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth/server/providers"
)

const hmacSecret = "demo-shared-secret"

// strPtr returns a pointer to the given string
func strPtr(s string) *string {
	return &s
}

// mustURL parses the given string as a URL and panics if invalid
func mustURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

func main() {
	log.Println("🚀 Starting OAuth Authentication Server...")
	log.Println("   Mock OAuth Server: http://localhost:3030")
	log.Println("   MCP Server: http://localhost:3000/mcp")
	log.Println()

	// Start the mock OAuth server first
	go startMockOAuthServer()
	time.Sleep(2 * time.Second)

	// Test the mock server
	resp, err := http.Get("http://localhost:3030/authorize?test=1")
	if err != nil {
		log.Fatalf("Mock OAuth server not ready: %v", err)
	}
	resp.Body.Close()
	log.Println("✅ OAuth infrastructure ready")

	// Create OAuth Provider
	provider := providers.NewProxyOAuthServerProvider(providers.ProxyOptions{
		Endpoints: providers.ProxyEndpoints{
			AuthorizationURL: "http://localhost:3030/authorize",
			TokenURL:         "http://localhost:3030/token",
			RevocationURL:    "http://localhost:3030/revoke",
			RegistrationURL:  "http://localhost:3030/register",
		},

		VerifyAccessToken: func(token string) (*server.AuthInfo, error) {
			ai, err := mockVerifyJWT(token)
			if err != nil {
				fmt.Printf("❌ Token verification failed: %v\n", err)
				return nil, err
			}
			return &ai, nil
		},

		GetClient: func(clientID string) (*auth.OAuthClientInformationFull, error) {
			return &auth.OAuthClientInformationFull{
				OAuthClientMetadata: auth.OAuthClientMetadata{
					RedirectURIs:  []string{"http://localhost:5173/callback"},
					ResponseTypes: []string{"code"},
					GrantTypes:    []string{"authorization_code", "refresh_token"},
					ClientName:    strPtr("demo-client"),
					Scope:         strPtr("mcp.read mcp.write"),
				},
				OAuthClientInformation: auth.OAuthClientInformation{
					ClientID:     clientID,
					ClientSecret: "", // Public client, no secret
				},
			}, nil
		},
	})

	// Create and start the MCP server
	// Build a TokenVerifier (use introspection for the demo)
	ctx := context.Background()
	v, err := server.NewTokenVerifier(ctx, server.TokenVerifierConfig{
		Introspection: &server.IntrospectionConfig{
			Endpoint:         "http://localhost:3030/introspect",
			Timeout:          5 * time.Second,
			CacheTTL:         30 * time.Second,
			NegativeCacheTTL: 10 * time.Second,
			UseOnJWTFail:     true,
		},
	})
	if err != nil {
		log.Fatalf("failed to create TokenVerifier: %v", err)
	}

	mcpServer := mcp.NewServer(
		"Auth-Example-Server",
		"1.0.0",
		mcp.WithServerAddress(":3000"),
		mcp.WithServerPath("/mcp"),
		mcp.WithOAuthRoutes(mcp.OAuthRoutesConfig{
			Provider:        provider,
			IssuerURL:       mustURL("http://localhost:3030"),
			BaseURL:         mustURL("http://localhost:3000"),
			ScopesSupported: []string{"mcp.read", "mcp.write"},
		}),
		mcp.WithOAuthMetadata(mcp.OAuthMetadataConfig{
			ResourceServerURL: mustURL("http://localhost:3000"),
			ScopesSupported:   []string{"mcp.read", "mcp.write"},
			ResourceName:      strPtr("MCP Server"),
		}),
		mcp.WithBearerAuth(&mcp.BearerAuthConfig{
			Enabled:        true,
			RequiredScopes: []string{"mcp.read", "mcp.write"},
			Verifier:       v, // directly use TokenVerifier implementation
		}),
		mcp.WithAudit(&mcp.AuditConfig{
			Enabled:             true,
			Level:               "basic", // Reduced from "detailed"
			HashSensitiveData:   true,
			IncludeRequestBody:  false, // Disabled to reduce noise
			IncludeResponseBody: false, // Disabled to reduce noise
			EndpointPatterns:    []string{"/mcp/", "/authorize", "/token"},
			ExcludePatterns:     []string{"/healthz"},
		}),
	)

	// Set up a graceful shutdown.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start server (run in goroutine).
	go func() {
		log.Println("🔐 MCP Auth Server started successfully")
		log.Println("   Waiting for authentication requests...")
		fmt.Println()
		if err := mcpServer.Start(); err != nil {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()
	// Wait for termination signal.
	<-stop
	log.Println("🛑 Shutting down server...")
}

// startMockOAuthServer starts a simple mock OAuth server on port 3030
func startMockOAuthServer() {
	mux := http.NewServeMux()

	// Store the authorization code
	var authCode = "mock_auth_code_12345"

	// Authorize endpoint
	mux.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		// Handle test requests silently
		if r.URL.Query().Get("test") != "" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
			return
		}

		redirectURI := r.URL.Query().Get("redirect_uri")
		state := r.URL.Query().Get("state")
		clientID := r.URL.Query().Get("client_id")

		log.Printf("🔐 OAuth Authorization Request\n")
		log.Printf("   Client ID: %s\n", clientID)
		log.Printf("   Scopes: %s\n", r.URL.Query().Get("scope"))

		if redirectURI == "" {
			http.Error(w, "Missing redirect_uri", http.StatusBadRequest)
			return
		}

		// Construct redirect URLs
		redirectURL := redirectURI + "?code=" + authCode
		if state != "" {
			redirectURL += "&state=" + state
		}

		log.Printf("   Redirecting to client callback\n\n")
		http.Redirect(w, r, redirectURL, http.StatusFound)
	})

	// Token endpoint: supports authorization_code and refresh_token, and issues HS256 JWT
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Unified parsing form
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form", http.StatusBadRequest)
			return
		}

		grantType := r.FormValue("grant_type")

		// Issuing HS256 JWT
		signJWT := func(claims jwt.MapClaims) (string, error) {
			tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
			signed, err := tok.SignedString([]byte(hmacSecret))
			if err != nil {
				return "", err
			}
			return signed, nil
		}

		switch grantType {
		case "authorization_code":
			clientID := r.FormValue("client_id")
			code := r.FormValue("code")

			log.Printf("🎫 Token Exchange (Authorization Code)\n")
			log.Printf("   Client ID: %s\n", clientID)
			log.Printf("   Code: %s\n", code)

			// Basic parameter verification
			if clientID == "" || code == "" {
				http.Error(w, "Missing required parameters", http.StatusBadRequest)
				return
			}

			now := time.Now()
			// Issue access_token
			accessToken, err := signJWT(jwt.MapClaims{
				"iss":       "http://localhost:3030",
				"aud":       "http://localhost:3000",
				"iat":       now.Unix(),
				"exp":       now.Add(1 * time.Hour).Unix(),
				"client_id": clientID,
				"sub":       clientID,
				"scope":     "mcp.read mcp.write",
			})
			if err != nil {
				http.Error(w, "failed to sign access token", http.StatusInternalServerError)
				return
			}

			// Issue refresh token
			refreshToken, err := signJWT(jwt.MapClaims{
				"iss":       "http://localhost:3030",
				"aud":       "http://localhost:3000",
				"iat":       now.Unix(),
				"exp":       now.Add(24 * time.Hour).Unix(),
				"client_id": clientID,
				"sub":       clientID,
				"typ":       "refresh",
			})
			if err != nil {
				http.Error(w, "failed to sign refresh token", http.StatusInternalServerError)
				return
			}

			log.Printf("   ✅ Tokens issued successfully\n\n")

			resp := map[string]any{
				"access_token":  accessToken,
				"token_type":    "Bearer",
				"expires_in":    3600,
				"scope":         "mcp.read mcp.write",
				"refresh_token": refreshToken,
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)

		case "refresh_token":
			rt := r.FormValue("refresh_token")
			if rt == "" {
				http.Error(w, "Missing refresh_token", http.StatusBadRequest)
				return
			}

			log.Printf("🔄 Token Refresh Request\n")

			// Parse and verify RT (HS256)
			parsed, err := jwt.Parse(rt, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
				}
				return []byte(hmacSecret), nil
			})
			if err != nil || !parsed.Valid {
				log.Printf("   ❌ Invalid refresh token\n\n")
				http.Error(w, "invalid refresh_token", http.StatusUnauthorized)
				return
			}
			claims, ok := parsed.Claims.(jwt.MapClaims)
			if !ok {
				http.Error(w, "invalid refresh_token claims", http.StatusUnauthorized)
				return
			}

			// Extract client_id from RT claims
			clientID, _ := claims["client_id"].(string)
			if clientID == "" {
				clientID = "public-client"
			}

			fmt.Printf("   Client ID: %s\n", clientID)

			now := time.Now()
			// New access_token
			newAT, err := signJWT(jwt.MapClaims{
				"iss":       "http://localhost:3030",
				"aud":       "http://localhost:3000",
				"iat":       now.Unix(),
				"exp":       now.Add(1 * time.Hour).Unix(),
				"client_id": clientID,
				"sub":       clientID,
				"scope":     "mcp.read mcp.write",
			})
			if err != nil {
				http.Error(w, "failed to sign access token", http.StatusInternalServerError)
				return
			}

			// New refresh_token
			newRT, err := signJWT(jwt.MapClaims{
				"iss":       "http://localhost:3030",
				"aud":       "http://localhost:3000",
				"iat":       now.Unix(),
				"exp":       now.Add(24 * time.Hour).Unix(),
				"client_id": clientID,
				"sub":       clientID,
				"typ":       "refresh",
			})
			if err != nil {
				http.Error(w, "failed to sign refresh token", http.StatusInternalServerError)
				return
			}

			fmt.Printf("   ✅ New tokens issued\n\n")

			resp := map[string]any{
				"access_token":  newAT,
				"token_type":    "Bearer",
				"expires_in":    3600,
				"scope":         "mcp.read mcp.write",
				"refresh_token": newRT,
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)

		default:
			http.Error(w, "unsupported_grant_type", http.StatusBadRequest)
			return
		}
	})

	// Revocation endpoint (optional) - silent
	mux.HandleFunc("/revoke", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Registration endpoint (optional)
	mux.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("📝 Client Registration Request\n")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"client_id":      "test-client-id",
			"client_secret":  "test-secret",
			"client_name":    "demo-client",
			"scope":          "mcp.read mcp.write",
			"redirect_uris":  []string{"http://localhost:5173/callback"},
			"grant_types":    []string{"authorization_code", "refresh_token"},
			"response_types": []string{"code"},
		})
		log.Printf("   ✅ Client registered: test-client-id\n\n")
	})

	// Introspection endpoint (RFC7662 simplified for demo)
	mux.HandleFunc("/introspect", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		token := r.FormValue("token")
		resp := map[string]any{"active": false}
		if token != "" {
			parsed, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
				}
				return []byte(hmacSecret), nil
			})
			if err == nil && parsed != nil && parsed.Valid {
				if claims, ok := parsed.Claims.(jwt.MapClaims); ok {
					var exp int64
					if v, ok := claims["exp"].(float64); ok {
						exp = int64(v)
					}
					scope, _ := claims["scope"].(string)
					clientID, _ := claims["client_id"].(string)
					if clientID == "" {
						if sub, _ := claims["sub"].(string); sub != "" {
							clientID = sub
						}
					}
					resp = map[string]any{
						"active":    true,
						"exp":       exp,
						"scope":     scope,
						"client_id": clientID,
						"aud":       "http://localhost:3000",
						"iss":       "http://localhost:3030",
					}
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	// Authorization Server Metadata (RFC 8414) - silent
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		meta := map[string]any{
			"issuer":                                "http://localhost:3030",
			"authorization_endpoint":                "http://localhost:3030/authorize",
			"token_endpoint":                        "http://localhost:3030/token",
			"registration_endpoint":                 "http://localhost:3030/register",
			"revocation_endpoint":                   "http://localhost:3030/revoke",
			"response_types_supported":              []string{"code"},
			"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
			"code_challenge_methods_supported":      []string{"S256"},
			"token_endpoint_auth_methods_supported": []string{"client_secret_post"},
			"scopes_supported":                      []string{"mcp.read", "mcp.write"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(meta)
	})

	// Compatible with OIDC discovery - silent
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		cfg := map[string]any{
			"issuer":                                "http://localhost:3030",
			"authorization_endpoint":                "http://localhost:3030/authorize",
			"token_endpoint":                        "http://localhost:3030/token",
			"registration_endpoint":                 "http://localhost:3030/register",
			"revocation_endpoint":                   "http://localhost:3030/revoke",
			"response_types_supported":              []string{"code"},
			"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
			"code_challenge_methods_supported":      []string{"S256"},
			"token_endpoint_auth_methods_supported": []string{"client_secret_post"},
			"scopes_supported":                      []string{"mcp.read", "mcp.write"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(cfg)
	})

	server := &http.Server{
		Addr:    ":3030",
		Handler: mux,
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("Mock OAuth server error: %v", err)
	}
}

// mockVerifyJWT verifies a JWT using HMAC and extracts AuthInfo
func mockVerifyJWT(token string) (server.AuthInfo, error) {
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(hmacSecret), nil
	})
	if err != nil || !parsed.Valid {
		return server.AuthInfo{}, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return server.AuthInfo{}, fmt.Errorf("invalid claims")
	}

	// Parse client_id or sub
	var clientID string
	if cid, _ := claims["client_id"].(string); cid != "" {
		clientID = cid
	}
	if sub, _ := claims["sub"].(string); sub != "" {
		clientID = sub
	}

	// Parse scope
	scopeStr, _ := claims["scope"].(string)
	var scopes []string
	if scopeStr != "" {
		scopes = strings.Split(scopeStr, " ")
	}

	// Parse exp
	var expPtr *int64
	if v, ok := claims["exp"].(float64); ok {
		vv := int64(v)
		expPtr = &vv
	}

	// Make sure that Extra contains sub + client_id
	if claims["client_id"] == nil && clientID != "" {
		claims["client_id"] = clientID
	}
	if claims["sub"] == nil && clientID != "" {
		claims["sub"] = clientID
	}

	return server.AuthInfo{
		Token:     token,
		ClientID:  clientID,
		Scopes:    scopes,
		ExpiresAt: expPtr,
		Extra: map[string]any{
			"client_id": clientID,
			"sub":       clientID,
			"scope":     strings.Join(scopes, " "),
			"exp":       expPtr,
		},
	}, nil
}
