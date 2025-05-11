package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"trpc.group/trpc-go/trpc-mcp-go/log"
	"trpc.group/trpc-go/trpc-mcp-go/mcp"
	"trpc.group/trpc-go/trpc-mcp-go/server"
	"trpc.group/trpc-go/trpc-mcp-go/transport"
)

// Simple greeting tool handler function.
func handleGreet(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get session from context (if any).
	session, ok := transport.GetSessionFromContext(ctx)

	// Extract name from parameters.
	name := "World"
	if nameArg, ok := request.Params.Arguments["name"]; ok {
		if nameStr, ok := nameArg.(string); ok && nameStr != "" {
			name = nameStr
		}
	}

	// Create response content, customize message if session exists.
	content := []mcp.Content{}

	if ok && session != nil {
		content = append(content, mcp.NewTextContent(fmt.Sprintf(
			"Hello, %s! This is a greeting from the stateful JSON+GET SSE server. Your session ID is: %s",
			name, session.ID[:8]+"...")))
	} else {
		content = append(content, mcp.NewTextContent(fmt.Sprintf(
			"Hello, %s! This is a greeting from the stateful JSON+GET SSE server, but session info could not be obtained.",
			name)))
	}

	return &mcp.CallToolResult{Content: content}, nil
}

// Counter tool, used to demonstrate session state management.
func handleCounter(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get session.
	session, ok := transport.GetSessionFromContext(ctx)
	if !ok || session == nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent("Error: Could not get session info. This tool requires a stateful session."),
			},
		}, fmt.Errorf("Failed to get session from context")
	}

	// Get counter from session data.
	var count int
	if data, exists := session.GetData("counter"); exists {
		count, _ = data.(int)
	}

	// Increase counter.
	increment := 1
	if inc, ok := request.Params.Arguments["increment"].(float64); ok {
		increment = int(inc)
	}

	count += increment

	// Save back to session.
	session.SetData("counter", count)

	// Return result.
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Counter current value: %d (Session ID: %s)",
				count, session.ID[:8]+"...")),
		},
	}, nil
}

// Notification demo tool, used to send asynchronous notifications.
func handleNotification(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get session.
	session, ok := transport.GetSessionFromContext(ctx)
	if !ok || session == nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent("Error: Could not get session info. This tool requires a stateful session."),
			},
		}, fmt.Errorf("Failed to get session from context")
	}

	// Get message and delay from parameters.
	message := "This is a test notification message"
	if msgArg, ok := request.Params.Arguments["message"]; ok {
		if msgStr, ok := msgArg.(string); ok && msgStr != "" {
			message = msgStr
		}
	}

	delaySeconds := 2
	if delayArg, ok := request.Params.Arguments["delay"]; ok {
		if delay, ok := delayArg.(float64); ok {
			delaySeconds = int(delay)
		}
	}

	// Immediately return confirmation message.
	result := &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf(
				"Notification will be sent after %d seconds. Please make sure to subscribe to notifications with GET SSE connection. (Session ID: %s)",
				delaySeconds, session.ID[:8]+"...")),
		},
	}

	// Start a goroutine in the background to send delayed notification.
	go func() {
		time.Sleep(time.Duration(delaySeconds) * time.Second)

		// Get server instance from context.
		serverInstance := transport.GetServerFromContext(ctx)
		if serverInstance == nil {
			log.Infof("Failed to send notification: could not get server instance from context")
			return
		}

		// Type assertion.
		mcpServer, ok := serverInstance.(*server.Server)
		if !ok {
			log.Infof("Failed to send notification: server instance type error in context")
			return
		}

		// Use server's notification API to send notification directly.
		err := mcpServer.SendNotification(session.ID, "notifications/message", map[string]interface{}{
			"level": "info",
			"data": map[string]interface{}{
				"type":      "test_notification",
				"message":   message,
				"timestamp": time.Now().Format(time.RFC3339),
				"sessionId": session.ID,
			},
		})

		if err != nil {
			log.Infof("Failed to send notification: %v", err)
		} else {
			log.Infof("Notification sent to session %s", session.ID)
		}
	}()

	return result, nil
}

func main() {
	// Set log level.
	log.SetLevel(log.InfoLevel)
	log.Info("Starting Stateful JSON Yes GET SSE mode MCP server...")

	// Create server info.
	serverInfo := mcp.Implementation{
		Name:    "Stateful-JSON-Yes-GETSSE-Server",
		Version: "1.0.0",
	}

	// Create session manager (valid for 1 hour).
	sessionManager := transport.NewSessionManager(3600)

	// Create MCP server, configured as:
	// 1. Stateful mode (using SessionManager)
	// 2. Only return JSON responses (do not use SSE)
	// 3. Support GET SSE
	mcpServer := server.NewServer(
		":3004", // Server address and port
		serverInfo,
		server.WithPathPrefix("/mcp"), // Set API path
		server.WithSessionManager(sessionManager), // Use session manager (stateful)
		server.WithSSEEnabled(false),              // Disable SSE
		server.WithGetSSEEnabled(true),            // Enable GET SSE
		server.WithDefaultResponseMode("json"),    // Set default response mode to JSON
	)

	// Register a greeting tool.
	greetTool := mcp.NewTool("greet", handleGreet,
		mcp.WithDescription("A simple greeting tool"),
		mcp.WithString("name", mcp.Description("Name to greet")))

	if err := mcpServer.RegisterTool(greetTool); err != nil {
		log.Fatalf("Failed to register tool: %v", err)
	}
	log.Infof("Registered greeting tool: greet")

	// Register counter tool.
	counterTool := mcp.NewTool("counter", handleCounter,
		mcp.WithDescription("A session counter tool to demonstrate stateful sessions"),
		mcp.WithNumber("increment",
			mcp.Description("计数增量"),
			mcp.Default(1)))

	if err := mcpServer.RegisterTool(counterTool); err != nil {
		log.Fatalf("注册计数器工具失败: %v", err)
	}
	log.Infof("已注册计数器工具：counter")

	// 注册通知演示工具
	notifyTool := mcp.NewTool("sendNotification", handleNotification,
		mcp.WithDescription("一个通知演示工具，发送异步通知消息"),
		mcp.WithString("message",
			mcp.Description("要发送的通知消息"),
			mcp.Default("这是一条测试通知消息")),
		mcp.WithNumber("delay",
			mcp.Description("发送通知前的延迟秒数"),
			mcp.Default(2)))

	if err := mcpServer.RegisterTool(notifyTool); err != nil {
		log.Fatalf("注册通知工具失败: %v", err)
	}
	log.Infof("已注册通知工具：sendNotification")

	// 示例：定期广播系统状态通知
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// 广播系统状态通知给所有会话
				failedCount, err := mcpServer.BroadcastNotification(
					"notifications/message",
					map[string]interface{}{
						"level": "info",
						"data": map[string]interface{}{
							"type":      "system_status",
							"memory":    fmt.Sprintf("%.1f%%", float64(50+time.Now().Second()%30)),
							"cpu":       fmt.Sprintf("%.1f%%", float64(30+time.Now().Second()%40)),
							"timestamp": time.Now().Format(time.RFC3339),
							"message":   "系统正常运行中",
						},
					},
				)

				if err != nil {
					log.Infof("广播系统状态通知失败: %v (失败会话数: %d)", err, failedCount)
				} else {
					log.Infof("已广播系统状态通知给所有会话")
				}
			}
		}
	}()

	// 设置一个简单的健康检查路由
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("服务器运行正常"))
	})

	// 注册会话管理路由，允许查看活动会话
	http.HandleFunc("/sessions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			// 使用新的 API 获取活动会话列表
			sessions := mcpServer.HTTPHandler().(*transport.HTTPServerHandler).GetActiveSessions()

			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			fmt.Fprintf(w, "会话管理器状态：活动\n")
			fmt.Fprintf(w, "会话过期时间：%d秒\n", 3600)
			fmt.Fprintf(w, "GET SSE 支持：启用\n")
			fmt.Fprintf(w, "活动会话数：%d\n\n", len(sessions))

			// 显示所有活动会话
			for i, sessionID := range sessions {
				fmt.Fprintf(w, "%d) %s\n", i+1, sessionID)
			}
		} else {
			w.WriteHeader(http.StatusMethodNotAllowed)
			fmt.Fprintf(w, "不支持的方法: %s", r.Method)
		}
	})

	// 处理优雅退出
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Infof("收到信号 %v，正在退出...", sig)
		os.Exit(0)
	}()

	// 启动服务器
	log.Infof("MCP 服务器启动于 :3004，访问路径为 /mcp")
	log.Infof("这是一个有状态、纯 JSON 响应的服务器 - 会分配会话 ID，不使用 SSE 响应但支持 GET SSE")
	log.Infof("可以通过 http://localhost:3004/sessions 查看会话管理器状态")
	if err := mcpServer.Start(); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
