// Package httputil defines HTTP related constants
package httputil

// HTTP Header constants - Standard HTTP headers and MCP protocol specific headers
const (
	// ContentTypeHeader is the HTTP Content-Type header
	ContentTypeHeader = "Content-Type"

	// AcceptHeader is the HTTP Accept header
	AcceptHeader = "Accept"

	// SessionIDHeader is the MCP session ID header
	SessionIDHeader = "Mcp-Session-Id"

	// LastEventIDHeader is the SSE Last-Event-ID header
	LastEventIDHeader = "Last-Event-ID"
)

// Content Type constants - Supported content types
const (
	// ContentTypeJSON is the JSON content type
	ContentTypeJSON = "application/json"

	// ContentTypeSSE is the Server-Sent Events (SSE) content type
	ContentTypeSSE = "text/event-stream"
)
