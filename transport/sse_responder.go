package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/modelcontextprotocol/streamable-mcp/schema"
)

// SSEResponder implements the SSE response handler
type SSEResponder struct {
	// Current event ID
	eventID string

	// Whether to use stateless mode
	isStateless bool

	// Event ID counter
	eventCounter uint64
}

// NewSSEResponder creates a new SSE response handler
func NewSSEResponder(options ...func(*SSEResponder)) *SSEResponder {
	responder := &SSEResponder{
		eventCounter: 0,
		isStateless:  false, // Default to stateful mode
	}

	// Apply options
	for _, option := range options {
		option(responder)
	}

	return responder
}

// WithSSEStatelessMode sets whether to use stateless mode
func WithSSEStatelessMode(isStateless bool) func(*SSEResponder) {
	return func(r *SSEResponder) {
		r.isStateless = isStateless
	}
}

// WithEventID sets the event ID
func WithEventID(eventID string) func(*SSEResponder) {
	return func(r *SSEResponder) {
		if eventID != "" {
			r.eventID = eventID
		}
	}
}

// Respond sends an SSE response
func (r *SSEResponder) Respond(ctx context.Context, w http.ResponseWriter, req *http.Request, resp interface{}, session *Session) error {
	// Check if streaming is supported
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming is not supported by the underlying HTTP implementation")
	}

	// Set SSE response headers
	w.Header().Set(ContentTypeHeader, ContentTypeSSE)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Set session ID only in stateful mode
	if !r.isStateless && session != nil {
		w.Header().Set(SessionIDHeader, session.ID)
	}

	// If response is nil (notification response), return 202 Accepted
	if resp == nil {
		w.WriteHeader(http.StatusAccepted)
		return nil
	}

	// Serialize response
	respBytes, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("failed to serialize response: %w", err)
	}

	// Send SSE event
	eventID := r.nextEventID()
	data := string(respBytes)

	// Write event
	fmt.Fprintf(w, "id: %s\n", eventID)
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()

	return nil
}

// SupportsContentType checks if the specified content type is supported
func (r *SSEResponder) SupportsContentType(accepts []string) bool {
	return containsContentType(accepts, ContentTypeSSE)
}

// ContainsRequest determines if the request might contain a request (not a notification)
func (r *SSEResponder) ContainsRequest(body []byte) bool {
	// When SSE is supported, we can handle any request containing an "id" field
	return true
}

// Note: SendResponse method has been removed, its functionality is integrated into the Respond method

// SendNotification sends a notification event
func (r *SSEResponder) SendNotification(w http.ResponseWriter, flusher http.Flusher, notification interface{}) (string, error) {
	// Check if it's a response type, which should be sent using the Respond method
	if _, ok := notification.(*schema.Response); ok {
		return "", fmt.Errorf("response types should be sent using the Respond method")
	}

	// Generate event ID
	eventID := r.nextEventID()

	// Ensure notification object is a core.Notification type with correct jsonrpc field
	var notifObj *schema.Notification
	var notifBytes []byte
	var err error

	// Try to convert to core.Notification to validate format
	if n, ok := notification.(*schema.Notification); ok {
		notifObj = n
		// Ensure jsonrpc field is set correctly
		if notifObj.JSONRPC == "" {
			notifObj.JSONRPC = schema.JSONRPCVersion
		}
		// Serialize notification
		notifBytes, err = json.Marshal(notifObj)
	} else {
		// If not a core.Notification type, try to serialize directly
		notifBytes, err = json.Marshal(notification)
	}

	if err != nil {
		return "", fmt.Errorf("failed to serialize notification: %w", err)
	}

	// Send event
	fmt.Fprintf(w, "id: %s\n", eventID)
	fmt.Fprintf(w, "data: %s\n\n", notifBytes)
	flusher.Flush()

	return eventID, nil
}

// Generate the next event ID
func (r *SSEResponder) nextEventID() string {
	timestamp := time.Now().UnixNano() / 1000000 // Millisecond timestamp
	counter := atomic.AddUint64(&r.eventCounter, 1)
	return fmt.Sprintf("evt-%d-%d", timestamp, counter)
}
