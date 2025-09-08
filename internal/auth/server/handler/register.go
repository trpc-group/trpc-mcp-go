// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth/server/middleware"
	"trpc.group/trpc-go/trpc-mcp-go/internal/errors"

	"github.com/google/uuid"
	"golang.org/x/time/rate"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth/server"
)

const (
	DEFAULT_CLIENT_SECRET_EXPIRY_SECONDS = 30 * 24 * 60 * 60 // 30 days
	DEFAULT_RATE_LIMIT_WINDOW_MS         = 60 * 60 * 1000    // 1 hour
	DEFAULT_RATE_LIMIT_MAX               = 20                // 20 requests per hour
)

// ClientRegistrationHandlerOptions configuration for client registration handler
type ClientRegistrationHandlerOptions struct {
	// A store used to save information about dynamically registered OAuth clients.
	ClientsStore server.SupportDynamicClientRegistration

	// The number of seconds after which to expire issued client secrets, or 0 to prevent expiration of client secrets (not recommended).
	// If not set, defaults to 30 days.
	ClientSecretExpirySeconds *int

	// Rate limiting configuration for the client registration endpoint.
	// Set to nil to disable rate limiting for this endpoint.
	// Registration endpoints are particularly sensitive to abuse and should be rate limited.
	RateLimit *RegisterRateLimitConfig

	// Whether to generate a client ID before calling the client registration endpoint.
	// If not set, defaults to true.
	ClientIdGeneration *bool
}

type RegisterRateLimitConfig struct {
	WindowMs int    // Window duration in milliseconds
	Max      int    // Maximum requests per window
	Message  string // Customize over-limit prompt information
}

// ClientRegistrationHandler creates a handler for OAuth client registration
func ClientRegistrationHandler(options ClientRegistrationHandlerOptions) http.Handler {
	rateLimitConfig := options.RateLimit
	if rateLimitConfig == nil {
		rateLimitConfig = &RegisterRateLimitConfig{
			WindowMs: DEFAULT_RATE_LIMIT_WINDOW_MS,
			Max:      DEFAULT_RATE_LIMIT_MAX,
		}
	}

	if options.ClientsStore == nil {
		// Return a handler that always returns an error
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotImplemented)

			notImplError := errors.NewOAuthError(
				errors.ErrUnsupportedGrantType,
				"Dynamic client registration is not supported by this server",
				"https://datatracker.ietf.org/doc/html/rfc7591",
			)
			json.NewEncoder(w).Encode(notImplError.ToResponseStruct())
		})
	}

	clientSecretExpirySeconds := DEFAULT_CLIENT_SECRET_EXPIRY_SECONDS
	if options.ClientSecretExpirySeconds != nil {
		clientSecretExpirySeconds = *options.ClientSecretExpirySeconds
	}

	clientIdGeneration := true
	if options.ClientIdGeneration != nil {
		clientIdGeneration = *options.ClientIdGeneration
	}

	// Core handler logic
	coreHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")

		// Parse JSON request body
		var clientMetadata auth.OAuthClientMetadata
		if err := json.NewDecoder(r.Body).Decode(&clientMetadata); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)

			oauthErr := errors.NewOAuthError(
				errors.ErrInvalidClientMetadata,
				fmt.Sprintf("Invalid JSON in request body: %v", err),
				"",
			)
			json.NewEncoder(w).Encode(oauthErr.ToResponseStruct())
			return
		}

		// Validate client metadata
		if err := validateClientMetadata(&clientMetadata); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)

			oauthErr := errors.NewOAuthError(
				errors.ErrInvalidClientMetadata,
				err.Error(),
				"",
			)
			json.NewEncoder(w).Encode(oauthErr.ToResponseStruct())
			return
		}

		isPublicClient := clientMetadata.TokenEndpointAuthMethod == "none"

		// Generate client credentials
		var clientSecret string
		if !isPublicClient {
			secret, err := generateClientSecret()
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)

				oauthErr := errors.NewOAuthError(
					errors.ErrServerError,
					"Failed to generate client secret",
					"",
				)
				json.NewEncoder(w).Encode(oauthErr.ToResponseStruct())
				return
			}
			clientSecret = secret
		}

		clientIdIssuedAt := time.Now().Unix()

		// Calculate client secret expiry time
		clientsDoExpire := clientSecretExpirySeconds > 0
		var clientSecretExpiresAt *int64
		if !isPublicClient {
			if clientsDoExpire {
				expiryTime := clientIdIssuedAt + int64(clientSecretExpirySeconds)
				clientSecretExpiresAt = &expiryTime
			} else {
				zero := int64(0)
				clientSecretExpiresAt = &zero
			}
		}

		// Create client information
		clientInfo := auth.OAuthClientInformationFull{
			OAuthClientMetadata: clientMetadata,
			OAuthClientInformation: auth.OAuthClientInformation{
				ClientSecret:          clientSecret,
				ClientSecretExpiresAt: clientSecretExpiresAt,
			},
		}

		if clientIdGeneration {
			clientId := uuid.New().String()
			clientInfo.OAuthClientInformation.ClientID = clientId
			clientInfo.OAuthClientInformation.ClientIDIssuedAt = &clientIdIssuedAt
		}

		registeredClient, err := options.ClientsStore.RegisterClient(clientInfo)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)

			oauthErr := errors.NewOAuthError(
				errors.ErrServerError,
				"Failed to register client",
				"",
			)
			json.NewEncoder(w).Encode(oauthErr.ToResponseStruct())
			return
		}

		// Success response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(registeredClient)
	})

	var handler http.Handler = coreHandler

	if options.RateLimit != nil {
		windowDuration := time.Duration(rateLimitConfig.WindowMs) * time.Millisecond
		limit := rate.Every(windowDuration / time.Duration(rateLimitConfig.Max))
		limiter := rate.NewLimiter(limit, rateLimitConfig.Max)

		handler = middleware.RateLimitMiddleware(limiter)(handler)
	}

	handler = middleware.JSONValidationMiddleware()(handler)

	handler = middleware.AllowedMethods([]string{"POST"})(handler)

	handler = middleware.CorsMiddleware(handler)

	return handler
}

// generateClientSecret generates a random 32-byte hex string
func generateClientSecret() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// validateClientMetadata performs basic validation on client metadata
func validateClientMetadata(metadata *auth.OAuthClientMetadata) error {
	// Add validation logic as needed
	if metadata.TokenEndpointAuthMethod == "" {
		return fmt.Errorf("token_endpoint_auth_method is required")
	}

	switch metadata.TokenEndpointAuthMethod {
	case "client_secret_basic", "client_secret_post", "none":
	default:
		return fmt.Errorf("invalid token_endpoint_auth_method: %s", metadata.TokenEndpointAuthMethod)
	}

	if len(metadata.RedirectURIs) == 0 {
		return fmt.Errorf("redirect_uris is required")
	}

	return nil
}
