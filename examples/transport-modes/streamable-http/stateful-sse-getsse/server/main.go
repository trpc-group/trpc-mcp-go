// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

var mcpServer *mcp.Server

func main() {
	// Print server start message.
	log.Printf("Starting Stateful SSE+GET SSE mode MCP server...")

	// Create MCP server, configured as:
	// 1. Stateful mode (Stateful, using sessionManager)
	// 2. Use SSE response (streaming)
	// 3. Support independent GET SSE
	mcpServer = mcp.NewServer(
		"Stateful-SSE-GETSSE-Server",   // Server name
		"1.0.0",                        // Server version
		mcp.WithServerAddress(":3006"), // Server address and port
		mcp.WithServerPath("/mcp"),     // Set API path
		mcp.WithPostSSEEnabled(true),   // Enable SSE
		mcp.WithGetSSEEnabled(true),    // Enable GET SSE
	)

	// Register a greeting tool
	greetTool := mcp.NewTool("greet",
		mcp.WithDescription("A simple greeting tool"),
		mcp.WithString("name", mcp.Description("Name to greet")))

	mcpServer.RegisterTool(greetTool, handleGreet)
	log.Printf("Registered greeting tool: greet")

	// Register counter tool
	counterTool := mcp.NewTool("counter",
		mcp.WithDescription("A session counter tool, demonstrating stateful session"),
		mcp.WithNumber("increment",
			mcp.Description("Counter increment"),
			mcp.Default(1)))

	mcpServer.RegisterTool(counterTool, handleCounter)
	log.Printf("Registered counter tool: counter")

	// Register delayed response tool
	delayedTool := mcp.NewTool("delayedResponse",
		mcp.WithDescription("A delayed response tool, demonstrating SSE streaming response advantage"),
		mcp.WithNumber("steps",
			mcp.Description("Processing steps"),
			mcp.Default(5)),
		mcp.WithNumber("delayMs",
			mcp.Description("Milliseconds per step"),
			mcp.Default(500)))

	mcpServer.RegisterTool(delayedTool, handleDelayedResponse)
	log.Printf("Registered delayed response tool: delayedResponse")

	// Register notification demo tool
	notifyTool := mcp.NewTool("sendNotification",
		mcp.WithDescription("A notification demo tool, sending asynchronous notification message"),
		mcp.WithString("message",
			mcp.Description("Notification message to send"),
			mcp.Default("This is a test notification message")),
		mcp.WithNumber("delay",
			mcp.Description("Delay seconds before sending notification"),
			mcp.Default(2)))

	mcpServer.RegisterTool(notifyTool, handleNotification)
	log.Printf("Registered notification tool: sendNotification")

	// Register chat room tool
	chatJoinTool := mcp.NewTool("chatJoin",
		mcp.WithDescription("Join chat room"),
		mcp.WithString("userName",
			mcp.Description("Chat room username")))

	mcpServer.RegisterTool(chatJoinTool, handleChatJoin)
	log.Printf("Registered chat join tool: chatJoin")

	// Register send chat message tool
	chatSendTool := mcp.NewTool("chatSend",
		mcp.WithDescription("Send chat message"),
		mcp.WithString("message",
			mcp.Description("Chat message content")))

	mcpServer.RegisterTool(chatSendTool, handleChatSend)
	log.Printf("Registered chat send tool: chatSend")

	// Set a simple health check route
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Server is running normally"))
	})

	// Register session manager route, allowing to view active sessions
	http.HandleFunc("/sessions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			// Use new API to get active session list
			sessions, err := mcpServer.GetActiveSessions()
			if err != nil {
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				fmt.Fprintf(w, "Error getting active sessions: %v\n", err)
				return
			}

			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			fmt.Fprintf(w, "Session manager status: active\n")
			fmt.Fprintf(w, "Session expiration time: %d seconds\n", 3600)
			fmt.Fprintf(w, "SSE mode: enabled\n")
			fmt.Fprintf(w, "GET SSE support: enabled\n")
			fmt.Fprintf(w, "Chat room status: active (%d users)\n", globalChatRoom.GetUserCount())
			fmt.Fprintf(w, "Active session count: %d\n\n", len(sessions))

			// Display all active sessions
			for i, sessionID := range sessions {
				userName, ok := globalChatRoom.GetUserName(sessionID)
				if ok {
					fmt.Fprintf(w, "%d) %s (Username: %s)\n", i+1, sessionID, userName)
				} else {
					fmt.Fprintf(w, "%d) %s\n", i+1, sessionID)
				}
			}
		} else {
			w.WriteHeader(http.StatusMethodNotAllowed)
			fmt.Fprintf(w, "Unsupported method: %s", r.Method)
		}
	})

	// Handle graceful exit
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Printf("Received signal %v, exiting...", sig)
		os.Exit(0)
	}()

	// Start server
	log.Printf("MCP server started at :3006, access path is /mcp")
	log.Printf("This is a fully featured server - Stateful, SSE streaming response, supports GET SSE")
	log.Printf("You can view session manager status at http://localhost:3006/sessions")
	log.Printf("Chat room initialized, supports multi-user chat")
	if err := mcpServer.Start(); err != nil {
		log.Fatalf("Server start failed: %v", err)
	}
}

// ChatRoom represents a chat room.
type ChatRoom struct {
	// User mapping (session ID -> username)
	users     map[string]string
	usersLock sync.RWMutex

	// Message history
	messages     []ChatMessage
	messagesLock sync.RWMutex

	// Chat room name
	name string

	// Message history capacity
	capacity int
}

// ChatMessage represents a chat message.
type ChatMessage struct {
	UserName  string    `json:"userName"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// NewChatRoom creates a new chat room.
func NewChatRoom(name string, capacity int) *ChatRoom {
	return &ChatRoom{
		users:    make(map[string]string),
		messages: make([]ChatMessage, 0, capacity),
		name:     name,
		capacity: capacity,
	}
}

// AddUser adds a user to the chat room.
func (cr *ChatRoom) AddUser(sessionID, userName string) {
	cr.usersLock.Lock()
	defer cr.usersLock.Unlock()
	cr.users[sessionID] = userName
}

// RemoveUser removes a user from the chat room.
func (cr *ChatRoom) RemoveUser(sessionID string) {
	cr.usersLock.Lock()
	defer cr.usersLock.Unlock()
	delete(cr.users, sessionID)
}

// GetUserName retrieves a username by session ID.
func (cr *ChatRoom) GetUserName(sessionID string) (string, bool) {
	cr.usersLock.RLock()
	defer cr.usersLock.RUnlock()
	name, ok := cr.users[sessionID]
	return name, ok
}

// AddMessage adds a message to the chat history.
func (cr *ChatRoom) AddMessage(userName, message string) {
	cr.messagesLock.Lock()
	defer cr.messagesLock.Unlock()

	// Add new message
	msg := ChatMessage{
		UserName:  userName,
		Message:   message,
		Timestamp: time.Now(),
	}
	cr.messages = append(cr.messages, msg)

	// Remove old messages if exceeding capacity
	if len(cr.messages) > cr.capacity {
		cr.messages = cr.messages[1:]
	}
}

// GetRecentMessages retrieves recent messages from the chat history.
func (cr *ChatRoom) GetRecentMessages(count int) []ChatMessage {
	cr.messagesLock.RLock()
	defer cr.messagesLock.RUnlock()

	if count >= len(cr.messages) {
		// Copy all messages
		result := make([]ChatMessage, len(cr.messages))
		copy(result, cr.messages)
		return result
	}

	// Copy only the most recent messages
	startIdx := len(cr.messages) - count
	result := make([]ChatMessage, count)
	copy(result, cr.messages[startIdx:])
	return result
}

// GetUserCount returns the number of users in the chat room.
func (cr *ChatRoom) GetUserCount() int {
	cr.usersLock.RLock()
	defer cr.usersLock.RUnlock()
	return len(cr.users)
}

// Global chat room
var globalChatRoom = NewChatRoom("Global Chat Room", 100)

// handleGreet processes the greeting tool request.
func handleGreet(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get session from context
	session, ok := mcp.GetSessionFromContext(ctx)
	if !ok || session == nil {
		// Unable to get session, return a simple greeting
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(
					"Hello! This is a greeting from the complete MCP server " +
						"(but unable to get session information).",
				),
			},
		}, nil
	}

	// Extract name from parameters
	name := "Client User"
	if nameArg, ok := req.Params.Arguments["name"]; ok {
		if nameStr, ok := nameArg.(string); ok && nameStr != "" {
			name = nameStr
		}
	}

	// Build greeting message
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf(
				"Hello, %s! This is a greeting from the complete MCP server. Your session ID is: %s",
				name, session.GetID()[:8]+"...",
			)),
		},
	}, nil
}

// handleCounter processes the counter tool request.
func handleCounter(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get session from context
	session, ok := mcp.GetSessionFromContext(ctx)
	if !ok || session == nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent("Error: Unable to get session information. This tool requires a stateful session to work."),
			},
		}, fmt.Errorf("unable to get session from context")
	}

	// Get counter from session data
	var count int
	if data, exists := session.GetData("counter"); exists {
		count, _ = data.(int)
	}

	// Get increment from parameters
	increment := 1
	if incArg, ok := req.Params.Arguments["increment"]; ok {
		if incFloat, ok := incArg.(float64); ok {
			increment = int(incFloat)
		}
	}

	// Update counter
	count += increment
	session.SetData("counter", count)

	// Return result
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf(
				"Counter current value: %d (Session ID: %s)",
				count, session.GetID()[:8]+"...",
			)),
		},
	}, nil
}

// handleDelayedResponse processes the delayed response tool request.
func handleDelayedResponse(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get session from context
	session, ok := mcp.GetSessionFromContext(ctx)
	if !ok || session == nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent("Error: Unable to get session information. This tool requires a stateful session to work."),
			},
		}, fmt.Errorf("unable to get session from context")
	}

	// Get notification sender from context
	notificationSender, ok := mcp.GetNotificationSender(ctx)
	if !ok {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(
					"Error: Unable to get notification sender. " +
						"This feature requires SSE streaming response support.",
				),
			},
		}, fmt.Errorf("unable to get notification sender from context")
	}

	// Get steps and delay from parameters
	steps := 5
	if stepsArg, ok := req.Params.Arguments["steps"]; ok {
		if stepsFloat, ok := stepsArg.(float64); ok && stepsFloat > 0 {
			steps = int(stepsFloat)
		}
	}

	delayMs := 500
	if delayArg, ok := req.Params.Arguments["delayMs"]; ok {
		if delayFloat, ok := delayArg.(float64); ok && delayFloat > 0 {
			delayMs = int(delayFloat)
		}
	}

	// Send processing start notification
	err := notificationSender.SendLogMessage("info", fmt.Sprintf(
		"Start processing request, will execute %d steps, each step delay %d milliseconds",
		steps, delayMs))
	if err != nil {
		log.Printf("Send notification failed: %v", err)
	}

	// Send progress notification
	for i := 1; i <= steps; i++ {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent(fmt.Sprintf("Processing cancelled at step %d", i)),
				},
			}, ctx.Err()
		default:
			// Continue execution
		}

		// Send progress notification
		progress := float64(i) / float64(steps)
		err := notificationSender.SendProgress(progress, fmt.Sprintf("Step %d/%d", i, steps))
		if err != nil {
			log.Printf("Send progress notification failed: %v", err)
		}

		// Send detailed log
		err = notificationSender.SendLogMessage("info", fmt.Sprintf(
			"Executing step %d/%d (progress: %.0f%%)",
			i, steps, progress*100))
		if err != nil {
			log.Printf("Send log notification failed: %v", err)
		}

		// Delay for a while
		time.Sleep(time.Duration(delayMs) * time.Millisecond)
	}

	// Send completion notification
	err = notificationSender.SendLogMessage("info", "Processing completed")
	if err != nil {
		log.Printf("Send completion notification failed: %v", err)
	}

	// Safely get truncated session ID
	sessionID := session.GetID()
	truncatedID := sessionID
	if len(sessionID) > 8 {
		truncatedID = sessionID[:8] + "..."
	}

	// Return result
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf(
				"Processing completed! Executed %d steps, each step delay %d milliseconds. (Session ID: %s)",
				steps, delayMs, truncatedID,
			)),
		},
	}, nil
}

// handleNotification processes the notification tool request.
func handleNotification(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get session from context
	session, ok := mcp.GetSessionFromContext(ctx)
	if !ok || session == nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent("Error: Unable to get session information. This tool requires a stateful session to work."),
			},
		}, fmt.Errorf("unable to get session from context")
	}

	// Get message and delay time from parameters
	message := "This is a test notification message"
	if msgArg, ok := req.Params.Arguments["message"]; ok {
		if msgStr, ok := msgArg.(string); ok && msgStr != "" {
			message = msgStr
		}
	}

	delaySeconds := 2
	if delayArg, ok := req.Params.Arguments["delay"]; ok {
		if delayFloat, ok := delayArg.(float64); ok {
			delaySeconds = int(delayFloat)
		}
	}

	// Immediately return confirmation message
	result := &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf(
				"Notification will be sent in %d seconds. (Session ID: %s)",
				delaySeconds, session.GetID()[:8]+"...",
			)),
		},
	}

	// Start background goroutine to send notification after delay
	go func() {
		time.Sleep(time.Duration(delaySeconds) * time.Second)

		err := mcpServer.SendNotification(session.GetID(), "notifications/message", map[string]interface{}{
			"level": "info",
			"data": map[string]interface{}{
				"type":      "test_notification",
				"message":   message,
				"timestamp": time.Now().Format(time.RFC3339),
				"sessionId": session.GetID(),
			},
		})

		if err != nil {
			log.Printf("Send notification failed: %v", err)
		} else {
			log.Printf("Notification sent to session %s", session.GetID())
		}
	}()

	return result, nil
}

// handleChatJoin processes the chat join tool request.
func handleChatJoin(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get session from context
	session, ok := mcp.GetSessionFromContext(ctx)
	if !ok || session == nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent("Error: Unable to get session information. This tool requires a stateful session to work."),
			},
		}, fmt.Errorf("unable to get session from context")
	}

	// Get username from parameters
	userName := fmt.Sprintf("User_%d", time.Now().Unix()%1000)
	if userArg, ok := req.Params.Arguments["userName"]; ok {
		if userStr, ok := userArg.(string); ok && userStr != "" {
			userName = userStr
		}
	}

	// Add user to chat room
	globalChatRoom.AddUser(session.GetID(), userName)

	// Broadcast user join message
	broadcastSystemMessage(fmt.Sprintf("%s joined the chat room", userName))

	// Get recent messages
	recentMessages := globalChatRoom.GetRecentMessages(10)
	messageText := fmt.Sprintf("Successfully joined the chat room as %s.", userName)
	if len(recentMessages) > 0 {
		messageText += "\n\nRecent messages:"
		for i, msg := range recentMessages {
			messageText += fmt.Sprintf(
				"\n%d) [%s] %s: %s",
				i+1,
				msg.Timestamp.Format("15:04:05"),
				msg.UserName,
				msg.Message,
			)
		}
	} else {
		messageText += "\n\nNo chat history yet."
	}

	// Save username to session
	session.SetData("chatUserName", userName)

	// Return result
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(messageText),
		},
	}, nil
}

// handleChatSend processes the chat send tool request.
func handleChatSend(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get session from context
	session, ok := mcp.GetSessionFromContext(ctx)
	if !ok || session == nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent("Error: Unable to get session information. This tool requires a stateful session to work."),
			},
		}, fmt.Errorf("unable to get session from context")
	}

	// Get username
	userName, ok := globalChatRoom.GetUserName(session.GetID())
	if !ok {
		// Try to get username from session
		userNameData, exists := session.GetData("chatUserName")
		if exists {
			if userNameStr, ok := userNameData.(string); ok {
				userName = userNameStr
				// Re-add to chat room
				globalChatRoom.AddUser(session.GetID(), userName)
			}
		}
	}

	// If still no username, please join the chat room first
	if userName == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent("Error: Please use chatJoin tool to join the chat room first."),
			},
		}, nil
	}

	// Get message from parameters
	message := ""
	if msgArg, ok := req.Params.Arguments["message"]; ok {
		if msgStr, ok := msgArg.(string); ok {
			message = msgStr
		}
	}

	// Validate message is not empty
	if message == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent("Error: Message cannot be empty."),
			},
		}, nil
	}

	// Add message to chat history
	globalChatRoom.AddMessage(userName, message)

	// Broadcast message
	broadcastChatMessage(userName, message)

	// Return result
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Message sent: %s", message)),
		},
	}, nil
}

// Broadcast chat message to all opened GET SSE connections.
func broadcastChatMessage(userName, message string) {
	// Use BroadcastNotification API to broadcast message to all sessions
	failedCount, err := mcpServer.BroadcastNotification("notifications/message", map[string]interface{}{
		"level": "info",
		"data": map[string]interface{}{
			"type":      "chat_message",
			"userName":  userName,
			"message":   message,
			"timestamp": time.Now().Format(time.RFC3339),
		},
	})

	if err != nil {
		log.Printf(
			"Broadcast chat message failed (failed session count: %d): %v",
			failedCount, err,
		)
	} else {
		log.Printf(
			"Broadcast chat message: %s: %s",
			userName, message,
		)
	}
}

// Broadcast system message.
func broadcastSystemMessage(message string) {
	// Use BroadcastNotification API to broadcast system message to all sessions
	failedCount, err := mcpServer.BroadcastNotification("notifications/message", map[string]interface{}{
		"level": "info",
		"data": map[string]interface{}{
			"type":      "chat_system_message",
			"message":   message,
			"timestamp": time.Now().Format(time.RFC3339),
		},
	})

	if err != nil {
		log.Printf(
			"Broadcast system message failed (failed session count: %d): %v",
			failedCount, err,
		)
	} else {
		log.Printf(
			"Broadcast system message: %s",
			message,
		)
	}
}
