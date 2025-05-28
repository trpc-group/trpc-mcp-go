// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 THL A29 Limited, a Tencent company.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

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

// notificationSender defines the notification sender interface
type notificationSender interface {
	// SendLogMessage sends a log message notification
	SendLogMessage(level string, message string) error

	// SendProgress sends a progress update notification
	SendProgress(progress float64, message string) error

	// SendCustomNotification sends a custom notification
	SendCustomNotification(method string, params map[string]interface{}) error

	SendNotification(notification *Notification) error
}

// withNotificationSender adds a notification sender to the context
func withNotificationSender(ctx context.Context, sender notificationSender) context.Context {
	return context.WithValue(ctx, notificationSenderKey, sender)
}

// GetNotificationSender retrieves the notification sender from the context
func GetNotificationSender(ctx context.Context) (notificationSender, bool) {
	sender, ok := ctx.Value(notificationSenderKey).(notificationSender)
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
