package transport

import (
	"context"
	"encoding/json"

	"trpc.group/trpc-go/trpc-mcp-go/mcp"
)

// HTTP Header constants
const (
	ContentTypeHeader = "Content-Type"
	AcceptHeader      = "Accept"
	SessionIDHeader   = "Mcp-Session-Id"
	LastEventIDHeader = "Last-Event-ID"

	ContentTypeJSON = "application/json"
	ContentTypeSSE  = "text/event-stream"
)

// Transport represents the interface for the communication transport layer
type Transport interface {
	// Send a request and wait for a response
	SendRequest(ctx context.Context, req *mcp.JSONRPCRequest) (*json.RawMessage, error)

	// Send a notification (no response expected)
	SendNotification(ctx context.Context, notification *mcp.JSONRPCNotification) error

	// Send a response
	SendResponse(ctx context.Context, resp *mcp.JSONRPCResponse) error

	// Close the transport
	Close() error
}

// HTTPTransport represents the interface for HTTP transport
type HTTPTransport interface {
	Transport

	// Get the session ID
	GetSessionID() string

	// Set the session ID
	SetSessionID(sessionID string)

	// Terminate the session
	TerminateSession(ctx context.Context) error
}
