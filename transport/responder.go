package transport

import (
	"context"
	"net/http"
	"strings"
)

// Responder defines the interface for different response handlers
type Responder interface {
	// Respond to a request
	Respond(ctx context.Context, w http.ResponseWriter, r *http.Request, resp interface{}, session *Session) error

	// Check if the specified content type is supported
	SupportsContentType(accepts []string) bool

	// Determine if the request potentially contains a request (non-notification)
	ContainsRequest(body []byte) bool
}

// ResponderOption represents an option for a responder
type ResponderOption func(Responder)

// Parse Accept header
func parseAcceptHeader(acceptHeader string) []string {
	if acceptHeader == "" {
		return []string{}
	}

	// Split and process each value
	accepts := []string{}
	for _, accept := range strings.Split(acceptHeader, ",") {
		// Remove potential parameters (like q values)
		mediaType := strings.Split(strings.TrimSpace(accept), ";")[0]
		if mediaType != "" {
			accepts = append(accepts, mediaType)
		}
	}

	return accepts
}

// Check if Accept header contains the specified content type
func containsContentType(accepts []string, contentType string) bool {
	for _, accept := range accepts {
		if accept == contentType || accept == "*/*" {
			return true
		}
	}
	return false
}
