package mcp

import (
	"context"
)

// Notification type constants
const (
	// NotificationMethodMessage for log notification method
	NotificationMethodMessage = "notifications/message"

	// NotificationMethodProgress for progress notification method
	NotificationMethodProgress = "notifications/progress"
)

// Context key type to avoid key collisions
type contextKey string

// Notification sender context key
const notificationSenderKey contextKey = "notificationSender"

// NotificationSender defines the notification sender interface
type NotificationSender interface {
	// SendLogMessage sends a log message notification
	SendLogMessage(level string, message string) error

	// SendProgress sends a progress update notification
	SendProgress(progress float64, message string) error

	// SendCustomNotification sends a custom notification
	SendCustomNotification(method string, params map[string]interface{}) error

	SendNotification(notification *Notification) error
}

// WithNotificationSender adds a notification sender to the context
func WithNotificationSender(ctx context.Context, sender NotificationSender) context.Context {
	return context.WithValue(ctx, notificationSenderKey, sender)
}

// GetNotificationSender retrieves the notification sender from the context
func GetNotificationSender(ctx context.Context) (NotificationSender, bool) {
	sender, ok := ctx.Value(notificationSenderKey).(NotificationSender)
	return sender, ok
}

func NewNotification(method string, params map[string]interface{}) *Notification {
	notificationParams := NotificationParams{
		AdditionalFields: make(map[string]interface{}),
	}

	// Extract meta-field if present
	if meta, ok := params["_meta"]; ok {
		if metaMap, ok := meta.(map[string]interface{}); ok {
			notificationParams.Meta = metaMap
		}
		delete(params, "_meta")
	}

	// Add remaining fields to AdditionalFields
	for k, v := range params {
		notificationParams.AdditionalFields[k] = v
	}

	return &Notification{
		Method: method,
		Params: notificationParams,
	}
}
