package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"

	"trpc.group/trpc-go/trpc-mcp-go/internal/sseutil"
)

// sseNotificationSender implements the SSE notification sender
type sseNotificationSender struct {
	// Response writer
	writer http.ResponseWriter

	// Flusher
	flusher http.Flusher

	// Session ID
	sessionID string

	// SSE utility writer
	sseWriter *sseutil.Writer
}

// newSSENotificationSender creates an SSE notification sender
func newSSENotificationSender(w http.ResponseWriter, f http.Flusher, sessionID string) *sseNotificationSender {
	return &sseNotificationSender{
		writer:    w,
		flusher:   f,
		sessionID: sessionID,
		sseWriter: sseutil.NewWriter(),
	}
}

// SendLogMessage sends a log message notification
func (s *sseNotificationSender) SendLogMessage(level string, message string) error {
	return s.SendCustomNotification(NotificationMethodMessage, map[string]interface{}{
		"level": level,
		"data": map[string]interface{}{
			"type":    "log_message",
			"message": message,
		},
	})
}

// SendProgress sends a progress update notification
func (s *sseNotificationSender) SendProgress(progress float64, message string) error {
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
func (s *sseNotificationSender) SendCustomNotification(method string, params map[string]interface{}) error {
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
	jsonNotification := newJSONRPCNotification(notification)

	// Serialize jsonNotification
	data, err := json.Marshal(jsonNotification)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNotificationSerialization, err)
	}

	// Send SSE event using sseutil.Writer instead of direct fmt.Fprintf
	eventID := s.sseWriter.GenerateEventID()
	return s.sseWriter.WriteEvent(s.writer, sseutil.Event{
		ID:   eventID,
		Data: data,
	})
}

// SendNotification sends a custom notification
func (s *sseNotificationSender) SendNotification(notification *Notification) error {
	// Create notification
	jsonNotification := newJSONRPCNotification(*notification)

	// Serialize notifications
	data, err := json.Marshal(jsonNotification)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNotificationSerialization, err)
	}

	// Send SSE event using sseutil.Writer instead of direct fmt.Fprintf
	eventID := s.sseWriter.GenerateEventID()
	return s.sseWriter.WriteEvent(s.writer, sseutil.Event{
		ID:   eventID,
		Data: data,
	})
}

// noopNotificationSender implements a no-operation notification sender
type noopNotificationSender struct{}

// SendLogMessage no-op implementation
func (n *noopNotificationSender) SendLogMessage(level string, message string) error {
	return nil
}

// SendProgress no-op implementation
func (n *noopNotificationSender) SendProgress(progress float64, message string) error {
	return nil
}

// SendCustomNotification no-op implementation
func (n *noopNotificationSender) SendCustomNotification(method string, params map[string]interface{}) error {
	return nil
}

func (n *noopNotificationSender) SendNotification(notification *Notification) error {
	return nil
}
