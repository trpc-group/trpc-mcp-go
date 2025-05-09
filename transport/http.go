package transport

import (
	"context"

	"github.com/modelcontextprotocol/streamable-mcp/schema"
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
	SendRequest(ctx context.Context, req *schema.Request) (*schema.Response, error)

	// Send a notification (no response expected)
	SendNotification(ctx context.Context, notification *schema.Notification) error

	// Send a response
	SendResponse(ctx context.Context, resp *schema.Response) error

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
