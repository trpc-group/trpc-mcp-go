// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package e2e

import (
	"context"
	"errors"
	"fmt"
	"time"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

// handleBasicGreet handles the basic greeting tool.
func handleBasicGreet(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Check if context is cancelled.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		// Continue execution.
	}

	// Extract name parameter.
	name := "World"
	if nameArg, ok := req.Params.Arguments["name"]; ok {
		if nameStr, ok := nameArg.(string); ok && nameStr != "" {
			name = nameStr
		}
	}

	// Create greeting message.
	greeting := fmt.Sprintf("Hello, %s", name)

	// Create tool result.
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(greeting),
		},
	}, nil
}

// handleStreamingGreet handles the streaming greeting tool.
func handleStreamingGreet(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract parameters.
	name := "World"
	if nameArg, ok := req.Params.Arguments["name"]; ok {
		if nameStr, ok := nameArg.(string); ok && nameStr != "" {
			name = nameStr
		}
	}

	count := 3
	if countArg, ok := req.Params.Arguments["count"]; ok {
		if countInt, ok := countArg.(float64); ok && countInt > 0 {
			count = int(countInt)
		}
	}

	// Create multiple messages.
	content := make([]mcp.Content, 0, count)
	for i := 1; i <= count; i++ {
		select {
		case <-ctx.Done():
			return &mcp.CallToolResult{Content: content}, ctx.Err()
		default:
			// Continue execution.
		}

		// Create greeting message.
		greeting := fmt.Sprintf("Streaming Message %d/%d: Hello, %s!", i, count, name)
		content = append(content, mcp.NewTextContent(greeting))

		// Add a simple delay to simulate streaming.
		time.Sleep(100 * time.Millisecond)
	}

	return &mcp.CallToolResult{Content: content}, nil
}

// handleError handles the error tool.
func handleError(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract error message.
	errorMsg := "This is an intentional error"
	if msgArg, ok := req.Params.Arguments["error_message"]; ok {
		if msgStr, ok := msgArg.(string); ok && msgStr != "" {
			errorMsg = msgStr
		}
	}

	// Directly return error.
	return nil, errors.New(errorMsg)
}

// handleDelay handles the delay tool.
func handleDelay(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract delay time.
	delayMs := 1000
	if delayArg, ok := req.Params.Arguments["delay_ms"]; ok {
		if delayInt, ok := delayArg.(float64); ok && delayInt > 0 {
			delayMs = int(delayInt)
		}
	}

	// Extract message.
	message := "Delay finished"
	if msgArg, ok := req.Params.Arguments["message"]; ok {
		if msgStr, ok := msgArg.(string); ok && msgStr != "" {
			message = msgStr
		}
	}

	// Create timer.
	timer := time.NewTimer(time.Duration(delayMs) * time.Millisecond)
	defer timer.Stop()

	// Wait for timer or context cancellation.
	select {
	case <-timer.C:
		// Timer expired, return result.
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(fmt.Sprintf("%s (delay %dms)", message, delayMs)),
			},
		}, nil
	case <-ctx.Done():
		// Context cancelled, return error.
		return nil, ctx.Err()
	}
}

// handleSSEProgress handles the SSE progress tool.
func handleSSEProgress(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract parameters.
	steps := 5
	if stepsArg, ok := req.Params.Arguments["steps"]; ok {
		if stepsFloat, ok := stepsArg.(float64); ok && stepsFloat > 0 {
			steps = int(stepsFloat)
		}
	}

	delayMs := 100
	if delayArg, ok := req.Params.Arguments["delay_ms"]; ok {
		if delayFloat, ok := delayArg.(float64); ok && delayFloat > 0 {
			delayMs = int(delayFloat)
		}
	}

	message := "SSE progress test completed"
	if msgArg, ok := req.Params.Arguments["message"]; ok {
		if msgStr, ok := msgArg.(string); ok && msgStr != "" {
			message = msgStr
		}
	}

	// Send progress notifications.
	for i := 1; i <= steps; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			// Calculate progress percentage.
			progress := float64(i) / float64(steps)
			// Send progress notification.
			if sender, ok := mcp.GetNotificationSender(ctx); ok {
				err := sender.SendProgress(progress, fmt.Sprintf("Step %d/%d", i, steps))
				if err != nil {
					return nil, fmt.Errorf("Failed to send progress notification: %v", err)
				}
				// Send log message.
				sender.SendLogMessage("info", fmt.Sprintf("Finished step %d", i))
			}
			// Wait for the specified delay.
			time.Sleep(time.Duration(delayMs) * time.Millisecond)
		}
	}

	// Return the final result with the message
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: message,
			},
		},
	}, nil
}

// RegisterTestTools registers all test tools to the server.
func RegisterTestTools(s *mcp.Server) {
	// Register the basic greeting tool.
	s.RegisterTool(NewBasicTool(), handleBasicGreet)

	// Register the streaming tool.
	s.RegisterTool(NewStreamingTool(), handleStreamingGreet)

	// Register the error tool.
	s.RegisterTool(NewErrorTool(), handleError)

	// Register the delay tool.
	s.RegisterTool(NewDelayTool(), handleDelay)

	// Register the SSE progress tool.
	s.RegisterTool(NewSSEProgressTool(), handleSSEProgress)
}

// NewBasicTool creates a simple greeting tool.
func NewBasicTool() *mcp.Tool {
	return mcp.NewTool("basic-greet",
		mcp.WithDescription("A simple greeting tool that returns a greeting message."),
		mcp.WithString("name",
			mcp.Description("The name to greet."),
		),
	)
}

// NewStreamingTool creates a streaming tool that generates multiple messages.
func NewStreamingTool() *mcp.Tool {
	return mcp.NewTool("streaming-greet",
		mcp.WithDescription("A streaming greeting tool that returns multiple greeting messages."),
		mcp.WithString("name",
			mcp.Description("The name to greet."),
		),
		mcp.WithNumber("count",
			mcp.Description("The number of messages to generate."),
			mcp.Default(3),
		),
	)
}

// NewErrorTool creates a tool that always returns an error.
func NewErrorTool() *mcp.Tool {
	return mcp.NewTool("error-tool",
		mcp.WithDescription("A tool that always returns an error."),
		mcp.WithString("error_message",
			mcp.Description("The error message to return."),
			mcp.Default("This is an intentional error"),
		),
	)
}

// NewDelayTool creates a tool that delays for a specified time.
func NewDelayTool() *mcp.Tool {
	return mcp.NewTool("delay-tool",
		mcp.WithDescription("A tool that delays for a specified time."),
		mcp.WithNumber("delay_ms",
			mcp.Description("Delay time in milliseconds."),
			mcp.Default(1000),
		),
		mcp.WithString("message",
			mcp.Description("Message to return after delay."),
			mcp.Default("Delay finished"),
		),
	)
}

// NewSSEProgressTool creates an SSE test tool that supports sending progress notifications.
func NewSSEProgressTool() *mcp.Tool {
	return mcp.NewTool("sse-progress-tool",
		mcp.WithDescription("An SSE progress tool that sends progress notifications using SSE."),
		mcp.WithNumber("steps",
			mcp.Description("Number of progress steps."),
			mcp.Default(5),
		),
		mcp.WithNumber("delay_ms",
			mcp.Description("Delay between each step in milliseconds."),
			mcp.Default(100),
		),
		mcp.WithString("message",
			mcp.Description("Response message."),
			mcp.Default("SSE progress test completed"),
		),
	)
}

// HandleCustomNotification handles custom notifications.
func (nc *NotificationCollector) HandleCustomNotification(notification *mcp.JSONRPCNotification) error {
	nc.addNotification(notification)
	return nil
}

// HandleNotification handles standard notifications.
func (nc *NotificationCollector) HandleNotification(notification *mcp.JSONRPCNotification) error {
	nc.addNotification(notification)
	return nil
}
