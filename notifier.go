package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"

	"trpc.group/trpc-go/trpc-mcp-go/internal/sseutil"
)

// SSENotificationSender implements the SSE notification sender
type SSENotificationSender struct {
	// Response writer
	writer http.ResponseWriter

	// Flusher
	flusher http.Flusher

	// Session ID
	sessionID string

	// SSE utility writer
	sseWriter *sseutil.Writer
}

// NewSSENotificationSender creates an SSE notification sender
func NewSSENotificationSender(w http.ResponseWriter, f http.Flusher, sessionID string) *SSENotificationSender {
	return &SSENotificationSender{
		writer:    w,
		flusher:   f,
		sessionID: sessionID,
		sseWriter: sseutil.NewWriter(),
	}
}

// SendLogMessage sends a log message notification
func (s *SSENotificationSender) SendLogMessage(level string, message string) error {
	return s.SendCustomNotification(NotificationMethodMessage, map[string]interface{}{
		"level": level,
		"data": map[string]interface{}{
			"type":    "log_message",
			"message": message,
		},
	})
}

// SendProgress sends a progress update notification
func (s *SSENotificationSender) SendProgress(progress float64, message string) error {
	return s.SendCustomNotification(NotificationMethodProgress, map[string]interface{}{
		"progress": progress,
		"message":  message,
		"data": map[string]interface{}{
			"type":     "process_progress",
			"progress": progress,
			"message":  message,
		},
	})
}

// SendCustomNotification sends a custom notification
func (s *SSENotificationSender) SendCustomNotification(method string, params map[string]interface{}) error {
	// Create NotificationParams
	notificationParams := NotificationParams{
		AdditionalFields: params,
	}

	// Handle _meta if present
	if meta, ok := params["_meta"]; ok {
		if metaMap, isMap := meta.(map[string]interface{}); isMap {
			notificationParams.Meta = metaMap
			delete(params, "_meta") // Remove from AdditionalFields to avoid duplication
		}
	}

	notification := Notification{
		Method: method,
		Params: notificationParams,
	}

	// Create jsonNotification
	jsonNotification := NewJSONRPCNotification(notification)

	// Serialize jsonNotification
	data, err := json.Marshal(jsonNotification)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNotificationSerialization, err)
	}

	// Send SSE event using sseutil.Writer instead of direct fmt.Fprintf
	eventID := s.sseWriter.GenerateEventID()
	return s.sseWriter.WriteEvent(s.writer, s.flusher, sseutil.Event{
		ID:   eventID,
		Data: data,
	})
}

// SendNotification sends a custom notification
func (s *SSENotificationSender) SendNotification(notification *Notification) error {
	// Create notification
	jsonNotification := NewJSONRPCNotification(*notification)

	// Serialize notifications
	data, err := json.Marshal(jsonNotification)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNotificationSerialization, err)
	}

	// Send SSE event using sseutil.Writer instead of direct fmt.Fprintf
	eventID := s.sseWriter.GenerateEventID()
	return s.sseWriter.WriteEvent(s.writer, s.flusher, sseutil.Event{
		ID:   eventID,
		Data: data,
	})
}

// NoopNotificationSender implements a no-operation notification sender
type NoopNotificationSender struct{}

// SendLogMessage no-op implementation
func (n *NoopNotificationSender) SendLogMessage(level string, message string) error {
	return nil
}

// SendProgress no-op implementation
func (n *NoopNotificationSender) SendProgress(progress float64, message string) error {
	return nil
}

// SendCustomNotification no-op implementation
func (n *NoopNotificationSender) SendCustomNotification(method string, params map[string]interface{}) error {
	return nil
}

func (NoopNotificationSender) SendNotification(notification *Notification) error {
	return nil
}
