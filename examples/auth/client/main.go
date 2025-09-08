// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
	"trpc.group/trpc-go/trpc-mcp-go/internal/auth"
)

const (
	// Base origin of the MCP resource server
	serverURL = "http://localhost:3000"

	// Well-known OAuth protected resource metadata endpoint
	resourceMetadataURL = "http://localhost:3000/.well-known/oauth-protected-resource"

	// Local redirect URI that receives the authorization code
	redirectURL = "http://localhost:5173/callback"

	// Requested scopes for this demo
	scope = "mcp.read mcp.write"

	// HTTP listen address for the local callback server
	callbackListenAddr = ":5173"

	// MCP entry endpoint used by the SDK client
	mcpEndpoint = "http://localhost:3000/mcp/"
)

func main() {
	fmt.Println("🖥️  Starting OAuth Client Demo")
	fmt.Println("   Target Server: http://localhost:3000")
	fmt.Println("   Callback URL: http://localhost:5173/callback")
	fmt.Println("   Required Scopes: mcp.read mcp.write")
	fmt.Println()

	// Configure the auth flow used by the MCP SDK
	authFlow := mcp.AuthFlowConfig{
		ServerURL: serverURL,
		ClientMetadata: auth.OAuthClientMetadata{
			ClientName:              strPtr("demo-client"),
			GrantTypes:              []string{"authorization_code", "refresh_token"},
			TokenEndpointAuthMethod: "client_secret_post",
			RedirectURIs:            []string{redirectURL},
			Scope:                   strPtr(scope),
		},
		ResourceMetadataURL: strPtr(resourceMetadataURL),
		RedirectURL:         redirectURL,
		Scope:               strPtr(scope),
		OnRedirect: func(u *url.URL) error {
			fmt.Printf("🌐 Authorization Required\n")
			fmt.Printf("   Please open this URL in your browser:\n")
			fmt.Printf("   %s\n\n", u.String())
			fmt.Printf("   Waiting for authorization...\n")
			return nil
		},
	}

	// Create the MCP client with auth flow enabled
	client, err := mcp.NewClient(
		mcpEndpoint,
		mcp.Implementation{Name: "Auth-Example-Client", Version: "0.1.0"},
		mcp.WithAuthFlow(authFlow),
	)
	if err != nil {
		fmt.Printf("❌ Failed to create client: %v\n", err)
		return
	}

	// Start the local HTTP callback server to capture the authorization code
	authDone := make(chan struct{}, 1)
	cbServer := startCallbackServer(client, authDone)
	defer shutdownServer(cbServer)

	// First initialize will typically request user authorization
	fmt.Println("🔄 Step 1: Initializing client (triggering OAuth flow)...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, _ = client.Initialize(ctx, &mcp.InitializeRequest{})

	// Wait for the browser redirect to complete the code exchange
	select {
	case <-authDone:
		fmt.Println("✅ Step 2: Authorization flow completed successfully")
	case <-time.After(3 * time.Minute):
		fmt.Println("❌ Authorization timeout after 3 minutes")
		return
	}

	// Small delay to ensure token persistence
	time.Sleep(2 * time.Second)

	// Second initialize should succeed using the stored tokens
	fmt.Println("🔄 Step 3: Testing authenticated connection...")
	ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel2()

	initResp, err := client.Initialize(ctx2, &mcp.InitializeRequest{})
	if err != nil {
		fmt.Printf("❌ Authenticated connection failed: %v\n", err)
		return
	}

	fmt.Println("📋 OAuth Flow Summary:")
	fmt.Println("   1. ✅ Client registration")
	fmt.Println("   2. ✅ User authorization")
	fmt.Println("   3. ✅ Token exchange")
	fmt.Println("   4. ✅ Authenticated API access")
	fmt.Println()
	fmt.Printf("🎉 Success! Connected to MCP Server\n")
	fmt.Printf("   Server: %s v%s\n", initResp.ServerInfo.Name, initResp.ServerInfo.Version)
	fmt.Printf("   Authentication: OAuth 2.0 with Bearer Token\n")
}

// startCallbackServer runs an HTTP server that handles /callback and completes the OAuth flow via the SDK
func startCallbackServer(c *mcp.Client, done chan<- struct{}) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		state := r.URL.Query().Get("state")

		fmt.Printf("🔄 Callback received\n")
		fmt.Printf("   Authorization Code: %s\n", code[:20]+"...")
		if state != "" {
			fmt.Printf("   State: %s\n", state[:20]+"...")
		}

		if code == "" {
			fmt.Println("❌ Missing authorization code")
			http.Error(w, "missing code parameter", http.StatusBadRequest)
			return
		}

		fmt.Printf("🎫 Exchanging authorization code for tokens...\n")

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		if err := c.CompleteAuthFlow(ctx, code); err != nil {
			fmt.Printf("❌ Token exchange failed: %v\n", err)
			http.Error(w, fmt.Sprintf("Authorization failed: %v", err), http.StatusBadRequest)
			return
		}

		fmt.Println("✅ Token exchange successful")

		// Send a nice response page
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`Authorization Complete!`))

		// Notify the main goroutine
		select {
		case done <- struct{}{}:
		default:
		}
	})

	srv := &http.Server{
		Addr:    callbackListenAddr,
		Handler: mux,
	}
	go func() {
		fmt.Printf("🌐 Callback server listening on %s\n", callbackListenAddr)
		srv.ListenAndServe()
	}()
	return srv
}

// shutdownServer gracefully stops the HTTP server within a short timeout
func shutdownServer(srv *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

// strPtr returns a pointer to s
func strPtr(s string) *string {
	return &s
}
