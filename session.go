package mcp

import (
	"context"
	"time"

	"trpc.group/trpc-go/trpc-mcp-go/internal/session"
)

// Session defines the session interface
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

// SessionManager defines the session manager interface
type SessionManager interface {
	// CreateSession creates a new session
	CreateSession() Session

	// GetSession gets a session
	GetSession(id string) (Session, bool)

	// GetActiveSessions gets all active session IDs
	GetActiveSessions() []string

	// TerminateSession terminates a session
	TerminateSession(id string) bool
}

// NewSession creates a new session
func NewSession() Session {
	return session.NewSession()
}

// Session manager adapter that implements the SessionManager interface
type sessionManagerAdapter struct {
	manager *session.SessionManager
}

// CreateSession creates a new session
func (a *sessionManagerAdapter) CreateSession() Session {
	return a.manager.CreateSession()
}

// GetSession gets a session
func (a *sessionManagerAdapter) GetSession(id string) (Session, bool) {
	return a.manager.GetSession(id)
}

// GetActiveSessions gets all active session IDs
func (a *sessionManagerAdapter) GetActiveSessions() []string {
	return a.manager.GetActiveSessions()
}

// TerminateSession terminates a session
func (a *sessionManagerAdapter) TerminateSession(id string) bool {
	return a.manager.TerminateSession(id)
}

// NewSessionManager creates a session manager
func NewSessionManager(expirySeconds int) SessionManager {
	return &sessionManagerAdapter{
		manager: session.NewSessionManager(expirySeconds),
	}
}

// Session context key
type sessionContextKey struct{}

// Server context key
type serverContextKey struct{}

// SetSessionToContext adds a session to the context
func SetSessionToContext(ctx context.Context, session Session) context.Context {
	return context.WithValue(ctx, sessionContextKey{}, session)
}

// GetSessionFromContext gets a session from the context
func GetSessionFromContext(ctx context.Context) (Session, bool) {
	session, ok := ctx.Value(sessionContextKey{}).(Session)
	return session, ok
}

// SetServerToContext adds a server instance to the context
func SetServerToContext(ctx context.Context, server interface{}) context.Context {
	return context.WithValue(ctx, serverContextKey{}, server)
}

// GetServerFromContext gets a server instance from the context
func GetServerFromContext(ctx context.Context) interface{} {
	return ctx.Value(serverContextKey{})
}
