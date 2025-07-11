// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

// Package sseutil provides utilities for Server-Sent Events (SSE).
package sseutil

import (
	"fmt"
	"net/http"
	"strings"
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
// The function will automatically flush the response if the http.ResponseWriter implements http.Flusher.
func (sw *Writer) WriteEvent(w http.ResponseWriter, event Event) error {
	if event.ID == "" {
		// Consider returning a predefined error from this package if this becomes common
		return fmt.Errorf("SSE event ID cannot be empty")
	}

	// Note: MCP generally sends data. A generic writer might allow empty data for comments/keep-alives.
	// For now, this writer expects data to be typically non-nil based on MCP usage.

	// Write event ID
	if _, err := fmt.Fprintf(w, "id: %s\n", event.ID); err != nil {
		return fmt.Errorf("failed to write SSE event ID: %w", err)
	}

	// Write event data with proper SSE formatting
	// Split data by newlines and prefix each line with 'data: '
	dataStr := string(event.Data)
	if dataStr != "" {
		lines := strings.Split(strings.TrimSuffix(dataStr, "\n"), "\n")
		for _, line := range lines {
			if _, err := fmt.Fprintf(w, "data: %s\n", line); err != nil {
				return fmt.Errorf("failed to write SSE event data line: %w", err)
			}
		}
	}
	// End of event (double newline)
	if _, err := fmt.Fprint(w, "\n"); err != nil {
		return fmt.Errorf("failed to write SSE event terminator: %w", err)
	}

	// Try to flush the response if the writer supports it
	if flusher, ok := w.(http.Flusher); ok {
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
