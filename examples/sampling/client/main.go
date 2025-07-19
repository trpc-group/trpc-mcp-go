// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
	"trpc.group/trpc-go/trpc-mcp-go/sampling"
)

// InteractiveApprovalWrapper Simple interactive audit wrapper
type InteractiveApprovalWrapper struct {
	baseHandler mcp.SamplingHandler
	reader      *bufio.Reader
}

// NewInteractiveApprovalWrapper Creating a wrapper with interactive auditing
func NewInteractiveApprovalWrapper(baseHandler mcp.SamplingHandler) *InteractiveApprovalWrapper {
	return &InteractiveApprovalWrapper{
		baseHandler: baseHandler,
		reader:      bufio.NewReader(os.Stdin),
	}
}

// HandleSamplingRequest Processing sampling requests (with review)
func (w *InteractiveApprovalWrapper) HandleSamplingRequest(ctx context.Context, req *sampling.SamplingCreateMessageRequest) (*mcp.SamplingCreateMessageResult, error) {
	// 1.Content security check
	if w.hasBlockedContent(req) {
		fmt.Println("The request contains prohibited content and is automatically rejected")
		return nil, fmt.Errorf("request denied by security filter")
	}

	// 2.Display request information
	w.displayRequest(req)

	// 3.Get user confirmation
	approved, err := w.getUserConfirmation()
	if err != nil {
		return nil, fmt.Errorf("failed to obtain user confirmation: %w", err)
	}

	if !approved {
		fmt.Println("Request denied by user...")
		return nil, fmt.Errorf("request denied by user")
	}

	// 4.Calling the original handler
	return w.baseHandler.HandleSamplingRequest(ctx, req)
}

// hasBlockedContent Check for prohibited content
func (w *InteractiveApprovalWrapper) hasBlockedContent(req *sampling.SamplingCreateMessageRequest) bool {
	blockedKeywords := []string{
		"hack", "exploit", "malware", "virus", "attack",
	}

	//Check the message content
	for _, msg := range req.Params.Messages {
		if textContent, ok := msg.Content.(sampling.TextContent); ok {
			text := strings.ToLower(textContent.Text)
			for _, keyword := range blockedKeywords {
				if strings.Contains(text, strings.ToLower(keyword)) {
					return true
				}
			}
		}
	}

	//Check system prompts
	if req.Params.SystemPrompt != nil {
		text := strings.ToLower(*req.Params.SystemPrompt)
		for _, keyword := range blockedKeywords {
			if strings.Contains(text, strings.ToLower(keyword)) {
				return true
			}
		}
	}

	return false
}

// displayRequest Display request information
func (w *InteractiveApprovalWrapper) displayRequest(req *sampling.SamplingCreateMessageRequest) {
	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("🔍 MCP Sampling Request a review")
	fmt.Println(strings.Repeat("=", 50))

	fmt.Printf("Request ID: %v\n", req.ID)
	fmt.Printf("Method: %s\n", req.Method)
	fmt.Printf("Number of messages: %d\n", len(req.Params.Messages))

	for i, msg := range req.Params.Messages {
		fmt.Printf("  Message %d [%s]: %s\n", i+1, msg.Role, w.getContentPreview(msg.Content))
	}

	if prefs := req.Params.ModelPreferences; prefs != nil {
		fmt.Println("Model Preferences:")
		if len(prefs.Hints) > 0 {
			fmt.Printf("  Prompt: %v\n", prefs.Hints)
		}
		if prefs.IntelligencePriority != nil {
			fmt.Printf("  Intelligence Priority: %.2f\n", *prefs.IntelligencePriority)
		}
	}

	if req.Params.MaxTokens != nil {
		fmt.Printf("Max Token: %d\n", *req.Params.MaxTokens)
	}

	fmt.Println(strings.Repeat("=", 50))
}

// getContentPreview Get content preview
func (w *InteractiveApprovalWrapper) getContentPreview(content sampling.Content) string {
	switch c := content.(type) {
	case sampling.TextContent:
		text := c.Text
		if len(text) > 100 {
			return text[:100] + "..."
		}
		return text
	case sampling.ImageContent:
		return fmt.Sprintf("[图像内容 - %s]", c.MimeType)
	case sampling.AudioContent:
		return fmt.Sprintf("[音频内容 - %s]", c.MimeType)
	default:
		return fmt.Sprintf("[%s 内容]", content.GetType())
	}
}

// getUserConfirmation Get user confirmation
func (w *InteractiveApprovalWrapper) getUserConfirmation() (bool, error) {
	fmt.Print("Do you want to approve this Sampling request? (y/n): ")

	input, err := w.reader.ReadString('\n')
	if err != nil {
		return false, err
	}

	input = strings.ToLower(strings.TrimSpace(input))

	switch input {
	case "y", "yes":
		fmt.Println("Request approved")
		return true, nil
	case "n", "no":
		fmt.Println("Request denied")
		return false, nil
	default:
		fmt.Println("Invalid input, please enter y(yes) or n(no)")
		return w.getUserConfirmation()
	}
}

// Used to inject SamplingHandler into the HTTP request context
type samplingHandlerContextKey struct{}

func SetSamplingHandlerToContext(ctx context.Context, handler mcp.SamplingHandler) context.Context {
	return context.WithValue(ctx, samplingHandlerContextKey{}, handler)
}

func GetSamplingHandlerFromContext(ctx context.Context) mcp.SamplingHandler {
	if handler, ok := ctx.Value(samplingHandlerContextKey{}).(mcp.SamplingHandler); ok {
		return handler
	}
	return nil
}

// RegisterSamplingHandler Register SamplingHandler to the global map so that it can be cleaned up during Cleanup
func RegisterSamplingHandler(client *mcp.Client, handler mcp.SamplingHandler) {
	support, exists := mcp.ClientSamplingMap[client]
	if !exists {
		support = &mcp.ClientSamplingSupport{SamplingEnabled: true}
		mcp.ClientSamplingMap[client] = support
	}
	support.SamplingHandler = handler
	support.SamplingEnabled = true
}

// Check Ollama Local Services
func testOllamaConnection(baseURL string) bool {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(baseURL + "/models")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// HTTP 回调入口
func samplingCreateMessageHandler(w http.ResponseWriter, r *http.Request) {
	handler := GetSamplingHandlerFromContext(r.Context())
	if handler == nil {
		http.Error(w, "Sampling handler not found", http.StatusInternalServerError)
		return
	}

	var req sampling.SamplingCreateMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Failed to decode request: "+err.Error(), http.StatusBadRequest)
		return
	}

	resp, err := handler.HandleSamplingRequest(r.Context(), &req)
	if err != nil {
		http.Error(w, "Sampling handler error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "Failed to encode response: "+err.Error(), http.StatusInternalServerError)
	}
}

func main() {
	log.Println("Start the Sampling client with interactive auditing support...")

	// 1.Read environment variables, giving priority to using real LLM
	openaiAPIKey := os.Getenv("OPENAI_API_KEY")
	claudeAPIKey := os.Getenv("CLAUDE_API_KEY")

	//Checks whether interactive auditing is enabled (controllable via environment variables)
	enableInteractiveApproval := os.Getenv("INTERACTIVE_APPROVAL") != "false" // 默认启用

	if openaiAPIKey == "" && claudeAPIKey == "" {
		log.Println("WARNING: OPENAI_API_KEY or CLAUDE_API_KEY not set, trying local or simulated models")
	}

	if enableInteractiveApproval {
		log.Println("Interactive auditing enabled...")
	} else {
		log.Println("Interactive auditing disabled...")
	}

	// 2. Construct general SamplingClientConfig
	config := &mcp.SamplingClientConfig{
		DefaultModel:        "gpt-3.5-turbo",
		AutoApprove:         false,
		MaxTokensPerRequest: 4000,
		ModelMappings: map[string]string{
			"claude-3-sonnet": "claude-3-sonnet-20240229",
			"claude-3-haiku":  "claude-3-haiku-20240307",
			"gpt-4":           "gpt-4",
			"gpt-3.5-turbo":   "gpt-3.5-turbo",
			"gpt-4-turbo":     "gpt-4-turbo-preview",
		},
		TimeoutSeconds: 120,
	}

	// 3.Select the base SamplingHandler based on environment variables or local Ollama
	var baseSamplingHandler mcp.SamplingHandler

	if openaiAPIKey != "" || claudeAPIKey != "" {
		// Real OpenAI/Claude handler
		baseSamplingHandler = mcp.NewRealLLMHandler(openaiAPIKey, claudeAPIKey, config)
		log.Println("使用真实 LLM 处理器 (OpenAI/Claude)")
	} else {
		// Try local model Ollama
		ollamaConfig := &mcp.SamplingClientConfig{
			DefaultModel:        "llama2",
			MaxTokensPerRequest: 4000,
			ModelMappings: map[string]string{
				"claude-3-sonnet": "llama2:13b",
				"gpt-4":           "llama2:13b",
				"gpt-3.5-turbo":   "llama2:7b",
			},
			TimeoutSeconds: 180,
		}
		ollamaHandler := mcp.NewOllamaHandler("http://localhost:11434", ollamaConfig)
		if testOllamaConnection("http://localhost:11434") {
			baseSamplingHandler = ollamaHandler
			log.Println("Use Ollama local model handler")
		} else {
			//Falling back to the framework's own simulation handler
			baseSamplingHandler = mcp.NewDefaultSamplingHandler(config)
			log.Println("Using an analog processor")
		}
	}

	// 4.Package interactive review feature (critical modification)
	var finalSamplingHandler mcp.SamplingHandler
	if enableInteractiveApproval {
		finalSamplingHandler = NewInteractiveApprovalWrapper(baseSamplingHandler)
		log.Println("Interactive audit wrapper enabled...")
	} else {
		finalSamplingHandler = baseSamplingHandler
		log.Println("Use direct processor (no audit)")
	}

	// 5.Create an MCP client (using the wrapped handler)
	mcpClient, err := mcp.NewClient(
		"http://localhost:3000/mcp",
		mcp.Implementation{Name: "Interactive-Sampling-Client", Version: "1.0.0"},
		mcp.WithClientLogger(mcp.GetDefaultLogger()),

		mcp.WithSamplingHandler(finalSamplingHandler), // 使用包装后的 handler
		mcp.WithSamplingConfig(config),
	)
	if err != nil {
		log.Fatalf("Client creation failed: %v", err)
	}
	defer func() {
		mcpClient.Close()
		mcp.CleanupClientSampling(mcpClient)
	}()

	// 6.Register Handler & route and inject into HTTP context
	RegisterSamplingHandler(mcpClient, finalSamplingHandler)
	http.Handle("/mcp/sampling/createMessage", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := SetSamplingHandlerToContext(r.Context(), finalSamplingHandler)
		samplingCreateMessageHandler(w, r.WithContext(ctx))
	}))

	// 7.Start HTTP service to receive callbacks
	go func() {
		log.Println("HTTP Service Startup，Listening :3001")
		if err := http.ListenAndServe(":3001", nil); err != nil {
			log.Fatalf("HTTP service startup failed: %v", err)
		}
	}()

	// 8.Initializing the connection
	log.Println("Initializing client connection...")
	ctx := context.Background()
	initResp, err := mcpClient.Initialize(ctx, &mcp.InitializeRequest{})
	if err != nil {
		log.Fatalf("Initialization failed: %v", err)
	}
	log.Printf("Connection successful! Server: %s %s，Protocol: %s",
		initResp.ServerInfo.Name,
		initResp.ServerInfo.Version,
		initResp.ProtocolVersion,
	)

	// 9.List available tools
	toolsResp, err := mcpClient.ListTools(ctx, &mcp.ListToolsRequest{})
	if err != nil {
		log.Fatalf("Failed to obtain tool list: %v", err)
	}
	log.Printf("Found %d tools：", len(toolsResp.Tools))
	for _, t := range toolsResp.Tools {
		log.Printf("  - %s: %s", t.Name, t.Description)
	}

	// 10.Demonstrates calling the "intelligent-analysis" tool (which triggers an interactive audit)
	log.Println("Test intelligent analysis tools...")
	log.Println("Tip: The review interface will appear next, please follow the prompts")

	analysisResult, err := mcpClient.CallTool(ctx, &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "intelligent-analysis",
			Arguments: map[string]interface{}{
				"data": "Sales data in 2024：Q1: 150w yuan，Q2: 180w yuan，Q3: 165w yuan，Q4: 220w yuan。Employee Satisfaction：85%。Customer complaint rate：2.3%。",
				"type": "sales",
			},
		},
	})
	if err != nil {
		log.Printf("Failed to call the intelligent analysis tool: %v", err)
	} else {
		log.Println("Intelligent analysis results：")
		for _, item := range analysisResult.Content {
			if text, ok := item.(mcp.TextContent); ok {
				log.Println(text.Text)
			}
		}
	}

	log.Println("\nThe client runs successfully！")
	log.Println("Now when the server initiates a Sampling request, an interactive review interface will appear")
	log.Println("Setting Environment Variables 'INTERACTIVE_APPROVAL=false' auditing can be disabled")

	//Block the main coroutine to keep the service running
	select {}
}
