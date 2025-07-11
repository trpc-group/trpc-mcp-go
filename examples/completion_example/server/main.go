package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

func main() {
	// 创建 MCP 服务器
	server := mcp.NewServer("Completion Example Server", "1.0.0")

	// 定义编程语言列表
	languages := []string{"python", "javascript", "go", "java", "c++", "rust", "typescript", "php", "ruby", "swift"}

	// 定义框架映射
	frameworks := map[string][]string{
		"python":     {"django", "flask", "fastapi", "pytorch", "tensorflow"},
		"javascript": {"react", "vue", "angular", "express", "next.js"},
		"go":         {"gin", "echo", "fiber", "chi", "gorilla"},
		"java":       {"spring", "hibernate", "struts", "play", "quarkus"},
		"c++":        {"qt", "boost", "stl", "opencv", "eigen"},
		"rust":       {"actix", "rocket", "axum", "tokio", "serde"},
		"typescript": {"react", "vue", "angular", "express", "next.js"},
		"php":        {"laravel", "symfony", "codeigniter", "yii", "slim"},
		"ruby":       {"rails", "sinatra", "hanami", "grape", "padrino"},
		"swift":      {"swiftui", "uikit", "combine", "alamofire", "kingfisher"},
	}

	// 注册带有 completion handler 的 prompt
	server.RegisterPromptWithCompletion(
		&mcp.Prompt{
			Name:        "code_review",
			Description: "Generate a code review for the specified language and framework",
			Arguments: []mcp.PromptArgument{
				{
					Name:        "language",
					Description: "Programming language",
					Required:    true,
				},
				{
					Name:        "framework",
					Description: "Framework for the language",
					Required:    false,
				},
			},
		},
		// Prompt handler
		func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			language := req.Params.Arguments["language"]
			framework := req.Params.Arguments["framework"]

			description := fmt.Sprintf("Code review for %s", language)
			if framework != "" {
				description += fmt.Sprintf(" with %s framework", framework)
			}

			messages := []mcp.PromptMessage{
				{
					Role: mcp.RoleUser,
					Content: mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Please provide a comprehensive code review for %s code%s. Focus on best practices, performance, security, and maintainability.",
							language,
							func() string {
								if framework != "" {
									return fmt.Sprintf(" using %s framework", framework)
								}
								return ""
							}()),
					},
				},
			}

			return &mcp.GetPromptResult{
				Description: description,
				Messages:    messages,
			}, nil
		},
		// Completion handler
		func(ctx context.Context, req *mcp.CompletionCompleteRequest) (*mcp.CompletionCompleteResult, error) {
			argumentName := req.Params.Argument["name"]
			argumentValue := req.Params.Argument["value"]

			var suggestions []string
			var total int
			var hasMore bool

			switch argumentName {
			case "language":
				// 为语言参数提供建议
				value := strings.ToLower(argumentValue)
				for _, lang := range languages {
					if strings.HasPrefix(strings.ToLower(lang), value) {
						suggestions = append(suggestions, lang)
					}
				}
				total = len(suggestions)
				hasMore = false

			case "framework":
				// 为框架参数提供建议（简化版本，不依赖上下文）
				value := strings.ToLower(argumentValue)
				// 为所有语言的框架提供建议
				for _, frameworksForLang := range frameworks {
					for _, framework := range frameworksForLang {
						if strings.HasPrefix(strings.ToLower(framework), value) {
							suggestions = append(suggestions, framework)
						}
					}
				}
				total = len(suggestions)
				hasMore = false
			}

			// 限制返回的建议数量
			if len(suggestions) > 100 {
				suggestions = suggestions[:100]
			}

			return &mcp.CompletionCompleteResult{
				Completion: mcp.Completion{
					Values:  suggestions,
					Total:   total,
					HasMore: hasMore,
				},
			}, nil
		},
	)

	// 启动服务器
	log.Println("Starting completion example server on localhost:3000")
	log.Println("Server supports the following capabilities:")
	log.Println("- Prompts with completion suggestions")
	log.Println("- Language and framework autocompletion")

	if err := server.Start(); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
