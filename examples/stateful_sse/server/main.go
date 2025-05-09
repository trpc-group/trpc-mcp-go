package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/streamable-mcp/log"
	"github.com/modelcontextprotocol/streamable-mcp/schema"
	"github.com/modelcontextprotocol/streamable-mcp/server"
	"github.com/modelcontextprotocol/streamable-mcp/transport"
)

// Callback function for handling the greet tool.
func handleGreet(ctx context.Context, req *schema.CallToolRequest) (*schema.CallToolResult, error) {
	// Get session.
	session, ok := transport.GetSessionFromContext(ctx)
	if !ok || session == nil {
		return &schema.CallToolResult{
			Content: []schema.ToolContent{
				schema.NewTextContent("Warning: Session info not found, but you may continue."),
			},
		}, nil
	}

	// Extract name parameter.
	name := "Client user"
	if nameArg, ok := req.Params.Arguments["name"]; ok {
		if nameStr, ok := nameArg.(string); ok && nameStr != "" {
			name = nameStr
		}
	}

	// Return greeting message.
	return &schema.CallToolResult{
		Content: []schema.ToolContent{
			schema.NewTextContent(fmt.Sprintf("Hello, %s! (Session ID: %s)",
				name, session.ID[:8]+"...")),
		},
	}, nil
}

// Counter tool, used to demonstrate session state keeping.
func handleCounter(ctx context.Context, req *schema.CallToolRequest) (*schema.CallToolResult, error) {
	// Get session.
	session, ok := transport.GetSessionFromContext(ctx)
	if !ok || session == nil {
		return &schema.CallToolResult{
			Content: []schema.ToolContent{
				schema.NewTextContent("Error: Could not get session info. This tool requires a stateful session."),
			},
		}, fmt.Errorf("failed to get session from context")
	}

	// Get current count from session.
	count := 0
	if countVal, exists := session.GetData("counter"); exists && countVal != nil {
		count, _ = countVal.(int)
	}

	// Extract increment parameter.
	increment := 1
	if incArg, ok := req.Params.Arguments["increment"]; ok {
		if incFloat, ok := incArg.(float64); ok {
			increment = int(incFloat)
		}
	}

	count += increment

	// Save back to session.
	session.SetData("counter", count)

	// Return result.
	return &schema.CallToolResult{
		Content: []schema.ToolContent{
			schema.NewTextContent(fmt.Sprintf("Counter current value: %d (Session ID: %s)",
				count, session.ID[:8]+"...")),
		},
	}, nil
}

// Delayed response tool, demonstrates the advantage of SSE streaming response.
func handleDelayedResponse(ctx context.Context, req *schema.CallToolRequest) (*schema.CallToolResult, error) {
	// Get session.
	session, ok := transport.GetSessionFromContext(ctx)
	if !ok || session == nil {
		return &schema.CallToolResult{
			Content: []schema.ToolContent{
				schema.NewTextContent("Error: Could not get session info. This tool requires a stateful session."),
			},
		}, fmt.Errorf("failed to get session from context")
	}

	// Get notification sender.
	notifSender, ok := schema.GetNotificationSender(ctx)
	if !ok {
		return &schema.CallToolResult{
			Content: []schema.ToolContent{
				schema.NewTextContent("Error: Could not get notification sender. This feature requires SSE streaming response support."),
			},
		}, fmt.Errorf("failed to get notification sender from context")
	}

	// Get steps and delay per step from parameters.
	steps := 5
	if s, ok := req.Params.Arguments["steps"].(float64); ok && s > 0 {
		steps = int(s)
	}

	delayMs := 500
	if d, ok := req.Params.Arguments["delayMs"].(float64); ok && d > 0 {
		delayMs = int(d)
	}

	// Send initial response.
	initialNotification := schema.NewNotification("notifications/message", map[string]interface{}{
		"level": "info",
		"data": map[string]interface{}{
			"type":    "process_started",
			"message": "Start processing request...",
			"steps":   steps,
			"delayMs": delayMs,
		},
	})
	if err := notifSender.SendCustomNotification(initialNotification.Method, initialNotification.Params); err != nil {
		log.Infof("Failed to send initial notification: %v", err)
	}

	// Send progress notifications.
	for i := 1; i <= steps; i++ {
		// Delay for a while.
		time.Sleep(time.Duration(delayMs) * time.Millisecond)

		// Send progress notification.
		progressNotification := schema.NewNotification("notifications/message", map[string]interface{}{
			"level": "info",
			"data": map[string]interface{}{
				"type":     "process_progress",
				"step":     i,
				"total":    steps,
				"progress": float64(i) / float64(steps) * 100,
				"message":  fmt.Sprintf("Processing step %d/%d...", i, steps),
			},
		})
		if err := notifSender.SendCustomNotification(progressNotification.Method, progressNotification.Params); err != nil {
			log.Infof("Failed to send progress notification: %v", err)
		}
	}

	// Final return result.
	return &schema.CallToolResult{
		Content: []schema.ToolContent{
			schema.NewTextContent(fmt.Sprintf("Processing complete! %d steps executed, %d ms delay per step. (Session ID: %s)",
				steps, delayMs, session.ID[:8]+"...")),
		},
	}, nil
}

func main() {
	// Set log level.
	log.SetLevel(log.InfoLevel)
	log.Info("Starting Stateful SSE No GET SSE mode MCP server...")

	// Create server info.
	serverInfo := schema.Implementation{
		Name:    "Stateful-SSE-No-GETSSE-Server",
		Version: "1.0.0",
	}

	// Create session manager (valid for 1 hour).
	sessionManager := transport.NewSessionManager(3600)

	// Create MCP server, configured as:
	// 1. Stateful mode (using SessionManager)
	// 2. Use SSE response (streaming)
	// 3. Do not support independent GET SSE
	mcpServer := server.NewServer(
		":3005", // Server address and port
		serverInfo,
		server.WithPathPrefix("/mcp"), // Set API path
		server.WithSessionManager(sessionManager), // Use session manager (stateful)
		server.WithSSEEnabled(true),               // Enable SSE
		server.WithGetSSEEnabled(false),           // Disable GET SSE
		server.WithDefaultResponseMode("sse"),     // Set default response mode to SSE
	)

	// Register a greeting tool.
	greetTool := schema.NewTool("greet", handleGreet,
		schema.WithDescription("A simple greeting tool"),
		schema.WithString("name", schema.Description("Name to greet")))

	if err := mcpServer.RegisterTool(greetTool); err != nil {
		log.Fatalf("Failed to register tool: %v", err)
	}
	log.Info("Registered greeting tool: greet")

	// 注册计数器工具
	counterTool := schema.NewTool("counter", handleCounter,
		schema.WithDescription("一个会话计数器工具，演示有状态会话"),
		schema.WithNumber("increment",
			schema.Description("计数增量"),
			schema.Default(1)))

	if err := mcpServer.RegisterTool(counterTool); err != nil {
		log.Fatalf("注册计数器工具失败: %v", err)
	}
	log.Info("已注册计数器工具：counter")

	// 注册延迟响应工具
	delayedTool := schema.NewTool("delayedResponse", handleDelayedResponse,
		schema.WithDescription("一个延迟响应工具，展示 SSE 流式响应的优势"),
		schema.WithNumber("steps",
			schema.Description("处理步骤数"),
			schema.Default(5)),
		schema.WithNumber("delayMs",
			schema.Description("每步延迟的毫秒数"),
			schema.Default(500)))

	if err := mcpServer.RegisterTool(delayedTool); err != nil {
		log.Fatalf("注册延迟响应工具失败: %v", err)
	}
	log.Info("已注册延迟响应工具：delayedResponse")

	// 设置一个简单的健康检查路由
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("服务器运行正常"))
	})

	// 注册会话管理路由，允许查看活动会话
	http.HandleFunc("/sessions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			// 在这里我们无法直接获取所有活动会话，因为 SessionManager 没有提供这样的方法
			// 但我们可以提供一个会话监控页面
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			fmt.Fprintf(w, "会话管理器状态：活动\n")
			fmt.Fprintf(w, "会话过期时间：%d秒\n", 3600)
			fmt.Fprintf(w, "SSE 模式：启用\n")
			fmt.Fprintf(w, "GET SSE 支持：禁用\n")
			fmt.Fprintf(w, "注意：会话管理器不提供列出所有活动会话的功能。\n")
			fmt.Fprintf(w, "在真实服务器中，建议实现会话监控功能。\n")
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
	log.Infof("MCP 服务器启动于 :3005，访问路径为 /mcp")
	log.Infof("这是一个有状态、SSE 流式响应的服务器 - 会分配会话 ID，使用 SSE，不支持 GET SSE")
	log.Infof("可以通过 http://localhost:3005/sessions 查看会话管理器状态")
	if err := mcpServer.Start(); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
