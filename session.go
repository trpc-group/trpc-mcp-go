// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 THL A29 Limited, a Tencent company.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"time"

	"trpc.group/trpc-go/trpc-mcp-go/internal/session"
)

// Session defines the session interface.
type Session interface {
	// GetID returns the session ID
	GetID() string

	// GetCreatedAt returns the session creation time
	GetCreatedAt() time.Time

	// GetLastActivity returns the last activity time
	GetLastActivity() time.Time

	// UpdateActivity updates the last activity time
	UpdateActivity()

	// GetData gets session data
	GetData(key string) (interface{}, bool)

	// SetData sets session data
	SetData(key string, value interface{})
}

// sessionManager defines the session manager interface
type sessionManager interface {
	// CreateSession creates a new session
	createSession() Session

	// GetSession gets a session
	getSession(id string) (Session, bool)

	// getActiveSessions gets all active session IDs
	getActiveSessions() []string

	// TerminateSession terminates a session
	terminateSession(id string) bool
}

// newSession creates a new session
func newSession() Session {
	return session.NewSession()
}

// Session manager adapter that implements the sessionManager interface
type sessionManagerAdapter struct {
	manager *session.SessionManager
}

// CreateSession creates a new session
func (a *sessionManagerAdapter) createSession() Session {
	return a.manager.CreateSession()
}

// GetSession gets a session
func (a *sessionManagerAdapter) getSession(id string) (Session, bool) {
	return a.manager.GetSession(id)
}

// getActiveSessions gets all active session IDs
func (a *sessionManagerAdapter) getActiveSessions() []string {
	return a.manager.GetActiveSessions()
}

// TerminateSession terminates a session
func (a *sessionManagerAdapter) terminateSession(id string) bool {
	return a.manager.TerminateSession(id)
}

// newSessionManager creates a session manager
func newSessionManager(expirySeconds int) sessionManager {
	return &sessionManagerAdapter{
		manager: session.NewSessionManager(expirySeconds),
	}
}

// Session context key
type sessionContextKey struct{}

// Server context key
type serverContextKey struct{}

// setSessionToContext adds a session to the context
func setSessionToContext(ctx context.Context, session Session) context.Context {
	return context.WithValue(ctx, sessionContextKey{}, session)
}

// GetSessionFromContext gets a session from the context
func GetSessionFromContext(ctx context.Context) (Session, bool) {
	session, ok := ctx.Value(sessionContextKey{}).(Session)
	return session, ok
}

// setServerToContext adds a server instance to the context
func setServerToContext(ctx context.Context, server interface{}) context.Context {
	return context.WithValue(ctx, serverContextKey{}, server)
}

// GetServerFromContext gets a server instance from the context
func GetServerFromContext(ctx context.Context) interface{} {
	return ctx.Value(serverContextKey{})
}
