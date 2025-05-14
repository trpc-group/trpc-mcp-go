package sseutil

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

const (
	ContentTypeEventStream = "text/event-stream"
)

// Event represents a Server-Sent Event.
type Event struct {
	ID   string
	Data []byte // Pre-serialized data
	// Event string // Optional: for named events, not used by MCP currently
}

// Writer provides basic SSE writing capabilities.
type Writer struct {
	eventCounter uint64
}

// NewWriter creates a new SSE writer.
func NewWriter() *Writer {
	return &Writer{}
}

// GenerateEventID creates a unique event ID.
// This logic is moved from mcp.SSEResponder.nextEventID()
func (sw *Writer) GenerateEventID() string {
	timestamp := time.Now().UnixNano() / 1000000 // Millisecond timestamp
	counter := atomic.AddUint64(&sw.eventCounter, 1)
	return fmt.Sprintf("evt-%d-%d", timestamp, counter)
}

// WriteEvent writes a single SSE event to the http.ResponseWriter.
// It assumes standard SSE headers have been set appropriately by the caller.
// It requires data to be pre-serialized.
func (sw *Writer) WriteEvent(w http.ResponseWriter, flusher http.Flusher, event Event) error {
	if event.ID == "" {
		// Consider returning a predefined error from this package if this becomes common
		return fmt.Errorf("SSE event ID cannot be empty")
	}

	// Note: MCP generally sends data. A generic writer might allow empty data for comments/keep-alives.
	// For now, this writer expects data to be typically non-nil based on MCP usage.

	_, err := fmt.Fprintf(w, "id: %s\n", event.ID)
	if err != nil {
		return fmt.Errorf("failed to write SSE event ID: %w", err)
	}
	_, err = fmt.Fprintf(w, "data: %s\n\n", string(event.Data)) // event.Data is []byte
	if err != nil {
		return fmt.Errorf("failed to write SSE event data: %w", err)
	}

	if flusher != nil {
		flusher.Flush()
	}
	return nil
}

// SetStandardHeaders sets typical SSE headers.
func SetStandardHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", ContentTypeEventStream)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
}
