// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package middleware

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"trpc.group/trpc-go/trpc-mcp-go/internal/auth"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth/server"
	"trpc.group/trpc-go/trpc-mcp-go/internal/errors"
)

// ClientAuthenticationMiddlewareOptions contains options for client authentication middleware
type ClientAuthenticationMiddlewareOptions struct {
	// ClientsStore is a store used to read information about registered OAuth clients
	ClientsStore server.OAuthClientsStoreInterface
	// Optional: When grant_type=refresh_token and client_id is not provided, try to parse/reverse-check client_id from refresh_token
	ResolveClientIDFromRefreshToken func(refreshToken string) (clientID string, ok bool)
}

// ClientAuthenticatedRequest represents the request schema for client authentication
type ClientAuthenticatedRequest struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret,omitempty"`
}

// clientInfoKeyType used to identify the context key storing OAuthClientInformationFull
type clientInfoKeyType struct{}

// validateClientRequest validates the client authentication request
func validateClientRequest(req *ClientAuthenticatedRequest) error {
	if req.ClientID == "" {
		return errors.NewOAuthError(errors.ErrInvalidRequest, "client_id is required", "")
	}
	return nil
}

// AuthenticateClient returns an HTTP middleware function for client authentication
func AuthenticateClient(options ClientAuthenticationMiddlewareOptions) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			setErrorResponse := func(w http.ResponseWriter, err errors.OAuthError, clientID string) {
				var statusCode int
				switch err.ErrorCode {
				case errors.ErrInvalidClient.Error():
					statusCode = http.StatusUnauthorized
				case errors.ErrInvalidRequest.Error():
					statusCode = http.StatusBadRequest
				case errors.ErrServerError.Error():
					statusCode = http.StatusInternalServerError
				default:
					statusCode = http.StatusBadRequest
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(statusCode)
				_ = json.NewEncoder(w).Encode(err.ToResponseStruct())
			}

			var reqData ClientAuthenticatedRequest
			var clientID string
			var bodyBytes []byte

			// Priority: Basic Auth first
			if authz := r.Header.Get("Authorization"); strings.HasPrefix(strings.ToLower(authz), "basic ") {
				enc := strings.TrimSpace(authz[len("Basic "):])
				raw, decErr := base64.StdEncoding.DecodeString(enc)
				if decErr != nil {
					setErrorResponse(w, errors.NewOAuthError(errors.ErrInvalidClient, "malformed basic credentials", ""), "")
					return
				}
				parts := strings.SplitN(string(raw), ":", 2)
				if len(parts) != 2 {
					setErrorResponse(w, errors.NewOAuthError(errors.ErrInvalidClient, "malformed basic credentials", ""), "")
					return
				}
				reqData.ClientID, reqData.ClientSecret = parts[0], parts[1]
				clientID = reqData.ClientID
			} else {
				// Non-Basic: buffer and restore Body, support form or JSON
				bodyBytes, _ = io.ReadAll(r.Body)
				_ = r.Body.Close()
				r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

				ct := strings.ToLower(r.Header.Get("Content-Type"))
				switch {
				case strings.HasPrefix(ct, "application/x-www-form-urlencoded"):
					formVals, _ := url.ParseQuery(string(bodyBytes))
					reqData.ClientID = formVals.Get("client_id")
					reqData.ClientSecret = formVals.Get("client_secret")
					clientID = reqData.ClientID
				case strings.HasPrefix(ct, "application/json"):
					if err := json.Unmarshal(bodyBytes, &reqData); err != nil {
						setErrorResponse(w, errors.NewOAuthError(errors.ErrInvalidRequest, "Invalid request body", ""), "")
						return
					}
					clientID = reqData.ClientID
				default:
					// Unknown type: maintain compatibility behavior, treat as JSON decode error
					setErrorResponse(w, errors.NewOAuthError(errors.ErrInvalidRequest, "Invalid request body", ""), "")
					return
				}
			}

			// Only try to fall back when client_id is not obtained, and it is a form or JSON, and grant_type=refresh_token
			if reqData.ClientID == "" {
				ct := strings.ToLower(r.Header.Get("Content-Type"))
				var grantType, refreshToken string

				switch {
				case strings.HasPrefix(ct, "application/x-www-form-urlencoded"):
					formVals, _ := url.ParseQuery(string(bodyBytes))
					grantType = formVals.Get("grant_type")
					refreshToken = formVals.Get("refresh_token")

				case strings.HasPrefix(ct, "application/json"):
					type raw struct {
						GrantType    string `json:"grant_type"`
						RefreshToken string `json:"refresh_token"`
					}
					var v raw
					_ = json.Unmarshal(bodyBytes, &v)
					grantType = v.GrantType
					refreshToken = v.RefreshToken
				}

				if strings.EqualFold(grantType, "refresh_token") && refreshToken != "" && options.ResolveClientIDFromRefreshToken != nil {
					if cid, ok := options.ResolveClientIDFromRefreshToken(refreshToken); ok && cid != "" {
						reqData.ClientID = cid
						clientID = cid
					}
				}
			}

			// Validate client_id
			if err := validateClientRequest(&reqData); err != nil {
				if oauthErr, ok := err.(errors.OAuthError); ok {
					setErrorResponse(w, oauthErr, clientID)
				} else {
					setErrorResponse(w, errors.NewOAuthError(errors.ErrInvalidRequest, "Invalid client_id", ""), clientID)
				}
				return
			}

			// Read client and validate secret/expiration
			client, err := options.ClientsStore.GetClient(reqData.ClientID)
			if err != nil {
				setErrorResponse(w, errors.NewOAuthError(errors.ErrInvalidClient, "invalid client credentials", ""), clientID)
				return
			}
			if client == nil {
				setErrorResponse(w, errors.NewOAuthError(errors.ErrInvalidClient, "invalid client credentials", ""), clientID)
				return
			}
			if client.ClientSecret != "" {
				if reqData.ClientSecret == "" {
					setErrorResponse(w, errors.NewOAuthError(errors.ErrInvalidClient, "Client secret is required", ""), clientID)
					return
				}
				if client.ClientSecret != reqData.ClientSecret {
					setErrorResponse(w, errors.NewOAuthError(errors.ErrInvalidClient, "Invalid client_secret", ""), clientID)
					return
				}
				if client.ClientSecretExpiresAt != nil {
					now := time.Now().Unix()
					if *client.ClientSecretExpiresAt != 0 && *client.ClientSecretExpiresAt < now {
						setErrorResponse(w, errors.NewOAuthError(errors.ErrInvalidClient, "Client secret has expired", ""), clientID)
						return
					}
				}
			}

			ctx := context.WithValue(r.Context(), clientInfoKeyType{}, client)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetAuthenticatedClient retrieves the authenticated client from HTTP request context
func GetAuthenticatedClient(r *http.Request) (*auth.OAuthClientInformationFull, bool) {
	client := r.Context().Value(clientInfoKeyType{})
	if client == nil {
		return nil, false
	}

	authenticatedClient, ok := client.(*auth.OAuthClientInformationFull)
	return authenticatedClient, ok
}
