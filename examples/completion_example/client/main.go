package main

import (
	"context"
	"log"
	"time"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

func main() {
	// 创建上下文
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 创建客户端信息
	clientInfo := mcp.Implementation{
		Name:    "Completion-Example-Client",
		Version: "1.0.0",
	}

	// 创建客户端
	mcpClient, err := mcp.NewClient(
		"http://localhost:3000/mcp",
		clientInfo,
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer mcpClient.Close()

	// 初始化客户端
	log.Printf("Initializing client...")
	initResp, err := mcpClient.Initialize(ctx, &mcp.InitializeRequest{})
	if err != nil {
		log.Fatalf("Initialization failed: %v", err)
	}

	log.Printf("Initialization succeeded: Server=%s %s, Protocol=%s",
		initResp.ServerInfo.Name, initResp.ServerInfo.Version, initResp.ProtocolVersion)
	log.Printf("Server capabilities: %+v", initResp.Capabilities)

	// 检查服务器是否支持 completions
	if initResp.Capabilities.Completions == nil {
		log.Println("Warning: Server does not support completions")
		return
	}

	log.Println("Server supports completions!")

	// 列出所有 prompts
	log.Printf("Listing prompts...")
	promptsResp, err := mcpClient.ListPrompts(ctx, &mcp.ListPromptsRequest{})
	if err != nil {
		log.Fatalf("Failed to list prompts: %v", err)
	}

	log.Printf("Server provides %d prompts", len(promptsResp.Prompts))
	for _, prompt := range promptsResp.Prompts {
		log.Printf("- Prompt: %s (%s)", prompt.Name, prompt.Description)
	}

	// 测试语言参数的 completion
	log.Println("\n=== Testing language completion ===")

	// 测试 "py" 前缀
	testCompletion(ctx, mcpClient, "code_review", "language", "py")

	// 测试 "ja" 前缀
	testCompletion(ctx, mcpClient, "code_review", "language", "ja")

	// 测试 "go" 前缀
	testCompletion(ctx, mcpClient, "code_review", "language", "go")

	// 测试框架参数的 completion
	log.Println("\n=== Testing framework completion ===")

	// 测试 Python 框架
	testCompletion(ctx, mcpClient, "code_review", "framework", "fl")

	// 测试 JavaScript 框架
	testCompletion(ctx, mcpClient, "code_review", "framework", "re")

	// 测试获取 prompt
	log.Println("\n=== Testing prompt retrieval ===")
	promptResp, err := mcpClient.GetPrompt(ctx, &mcp.GetPromptRequest{
		Params: struct {
			Name      string            `json:"name"`
			Arguments map[string]string `json:"arguments,omitempty"`
		}{
			Name: "code_review",
			Arguments: map[string]string{
				"language":  "python",
				"framework": "django",
			},
		},
	})
	if err != nil {
		log.Fatalf("Failed to get prompt: %v", err)
	}

	log.Printf("Prompt description: %s", promptResp.Description)
	log.Printf("Prompt messages: %d", len(promptResp.Messages))
	for i, msg := range promptResp.Messages {
		if textContent, ok := msg.Content.(mcp.TextContent); ok {
			log.Printf("Message %d: %s", i+1, textContent.Text)
		}
	}

	log.Printf("Client example finished, exiting in 3 seconds...")
	time.Sleep(3 * time.Second)
}

// testCompletion 测试基本的 completion 功能
func testCompletion(ctx context.Context, client *mcp.Client, promptName, argumentName, argumentValue string) {
	log.Printf("Testing completion for %s.%s = '%s'", promptName, argumentName, argumentValue)

	// 构建 completion 请求
	req := &mcp.CompletionCompleteRequest{
		Params: struct {
			Ref      map[string]string `json:"ref"`
			Argument map[string]string `json:"argument"`
		}{
			Ref: map[string]string{
				"type": "ref/prompt",
				"name": promptName,
			},
			Argument: map[string]string{
				"name":  argumentName,
				"value": argumentValue,
			},
		},
	}

	// 发送请求
	resp, err := client.CompletionComplete(ctx, req)
	if err != nil {
		log.Printf("Completion failed: %v", err)
		return
	}

	log.Printf("Completion results:")
	log.Printf("  Values: %v", resp.Completion.Values)
	log.Printf("  Total: %d", resp.Completion.Total)
	log.Printf("  HasMore: %t", resp.Completion.HasMore)
}
