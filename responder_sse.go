package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"trpc.group/trpc-go/trpc-mcp-go/internal/httputil"
	"trpc.group/trpc-go/trpc-mcp-go/internal/sseutil"
)

// sseResponder implements the SSE response handler
type sseResponder struct {
	// Current event ID (Note: this field seems unused, consider removal if truly not needed for other logic)
	eventID string

	// Whether to use stateless mode
	isStateless bool

	// SSE utility writer
	sseWriter *sseutil.Writer
}

// newSSEResponder creates a new SSE response handler
func newSSEResponder(options ...func(*sseResponder)) *sseResponder {
	responder := &sseResponder{
		isStateless: false, // Default to stateful mode
		sseWriter:   sseutil.NewWriter(),
	}

	// Apply options
	for _, option := range options {
		option(responder)
	}

	return responder
}

// withSSEStatelessMode sets whether to use stateless mode
func withSSEStatelessMode(isStateless bool) func(*sseResponder) {
	return func(r *sseResponder) {
		r.isStateless = isStateless
	}
}

// withEventID sets the event ID
func withEventID(eventID string) func(*sseResponder) {
	return func(r *sseResponder) {
		if eventID != "" {
			r.eventID = eventID
		}
	}
}

// respond sends an SSE response
func (r *sseResponder) respond(ctx context.Context, w http.ResponseWriter, req *http.Request, resp interface{}, session Session) error {
	// Check if streaming is supported
	flusher, ok := w.(http.Flusher)
	if !ok {
		return ErrStreamingNotSupported
	}

	// Set standard SSE headers + MCP specific session header
	sseutil.SetStandardHeaders(w)
	if !r.isStateless && session != nil {
		w.Header().Set(httputil.SessionIDHeader, session.GetID())
	}

	// If response is nil (e.g. for a notification that expects 202), return 202 Accepted
	if resp == nil {
		w.WriteHeader(http.StatusAccepted)
		return nil
	}

	// Serialize response
	respBytes, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrResponseSerialization, err)
	}

	// Send SSE event
	eventID := r.sseWriter.GenerateEventID()

	return r.sseWriter.WriteEvent(w, flusher, sseutil.Event{ID: eventID, Data: respBytes})
}

// supportsContentType checks if the specified content type is supported
func (r *sseResponder) supportsContentType(accepts []string) bool {
	return httputil.ContainsContentType(accepts, httputil.ContentTypeSSE)
}

// containsRequest determines if the request might contain a request (not a notification)
func (r *sseResponder) containsRequest(body []byte) bool {
	// When SSE is supported, we can handle any request containing an "id" field
	return true
}

// sendNotification sends a notification event
// Note: Standard SSE headers should be set by the caller (e.g. handleGet in httpServerHandler) if this is used for GET SSE streams.
func (r *sseResponder) sendNotification(w http.ResponseWriter, flusher http.Flusher, notification interface{}) (string, error) {
	// Check if it's a response type, which should be sent using the respond method
	if _, ok := notification.(*JSONRPCResponse); ok {
		return "", ErrInvalidResponseType
	}

	// Generate event ID
	eventID := r.sseWriter.GenerateEventID()

	// Ensure notification object is a mcp.Notification type with correct jsonrpc field
	var notifBytes []byte
	var err error

	// Try to convert to mcp.Notification to validate format and set JSONRPCVersion
	if n, ok := notification.(*JSONRPCNotification); ok {
		// Ensure jsonrpc field is set correctly
		if n.JSONRPC == "" {
			n.JSONRPC = JSONRPCVersion // JSONRPCVersion is a const in mcp package
		}
		// Serialize notification
		notifBytes, err = json.Marshal(n)
	} else {
		notifBytes, err = json.Marshal(notification)
	}

	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrNotificationSerialization, err)
	}

	err = r.sseWriter.WriteEvent(w, flusher, sseutil.Event{ID: eventID, Data: notifBytes})
	if err != nil {
		return "", err
	}

	return eventID, nil
}

// Generate the next event ID - This method is now effectively a proxy to sseWriter.
func (r *sseResponder) nextEventID() string {
	return r.sseWriter.GenerateEventID()
}
