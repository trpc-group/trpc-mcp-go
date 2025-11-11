package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

var outputMutex sync.Mutex

func main() {
	log.Println("Starting SSE Client...")
	ctx := context.Background()

	// 创建支持SSE的客户端
	client, err := mcp.NewClient(
		"http://localhost:3000/sse",
		mcp.Implementation{Name: "SSE-Client", Version: "1.0.0"},
		mcp.WithClientGetSSEEnabled(true),
	)
	if err != nil {
		log.Fatalf("Client creation failed: %v", err)
	}
	defer client.Close()

	// 初始化连接
	if _, err := client.Initialize(ctx, &mcp.InitializeRequest{}); err != nil {
		log.Fatalf("Initialization failed: %v", err)
	}

	// 创建通知处理器
	processor := &NotificationProcessor{}
	client.RegisterNotificationHandler("notifications/progress", processor.HandleProgress)
	client.RegisterNotificationHandler("notifications/message", processor.HandleLog)
	client.RegisterNotificationHandler("stocks/update", processor.HandleStockUpdate)
	client.RegisterNotificationHandler("stocks/summary", processor.HandleStockSummary)

	// 执行三种不同的SSE场景
	go runProgressScenario(ctx, client)
	go runLogScenario(ctx, client)
	go runStockScenario(ctx, client)

	// 保持客户端运行
	log.Println("Client running. Press Ctrl+C to exit.")
	select {}
}

// 通知处理器
type NotificationProcessor struct{}

func (p *NotificationProcessor) HandleProgress(n *mcp.JSONRPCNotification) error {
	progress, _ := n.Params.AdditionalFields["progress"].(float64)
	message, _ := n.Params.AdditionalFields["message"].(string)
	
	outputMutex.Lock()
	defer outputMutex.Unlock()
	
	fmt.Printf("\r[PROGRESS] %s (%.0f%%)", message, progress*100)
	if progress >= 1.0 {
		fmt.Println("\nTask completed!")
	}
	return nil
}

func (p *NotificationProcessor) HandleLog(n *mcp.JSONRPCNotification) error {
	level, _ := n.Params.AdditionalFields["level"].(string)
	
	message, _ := n.Params.AdditionalFields["message"].(string)
	
	outputMutex.Lock()
	defer outputMutex.Unlock()
	
	fmt.Printf("[%s] %s\n", strings.ToUpper(level), message)
	return nil
}

func (p *NotificationProcessor) HandleStockUpdate(n *mcp.JSONRPCNotification) error {
	outputMutex.Lock()
	defer outputMutex.Unlock()
	
	
	fmt.Print("\n[STOCKS] ")
	for symbol, price := range n.Params.AdditionalFields {
		if p, ok := price.(float64); ok {
			fmt.Printf("%s: $%.2f  ", symbol, p)
		}
	}
	return nil
}

func (p *NotificationProcessor) HandleStockSummary(n *mcp.JSONRPCNotification) error {
	outputMutex.Lock()
	defer outputMutex.Unlock()
	
	
	fmt.Println("\n\n=== STOCK SUMMARY ===")
	for symbol, data := range n.Params.AdditionalFields {
		if info, ok := data.(map[string]interface{}); ok {
			price := info["price"].(float64)
			change := info["change"].(float64)
			percent := info["percent"].(float64) * 100
			
			changeSign := "+"
			if change < 0 {
				changeSign = ""
			}
			
			fmt.Printf("%s: $%.2f (%s%.2f, %s%.2f%%)\n",
				symbol, price, changeSign, change, changeSign, percent)
		}
	}
	fmt.Println("=====================")
	return nil
}

// 场景1: 进度通知
func runProgressScenario(ctx context.Context, client *mcp.Client) {
	time.Sleep(1 * time.Second)
	log.Println("Starting progress task...")
	
	_, err := client.CallTool(ctx, &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "start-progress",
			Arguments: map[string]interface{}{
				"total": 8,
			},
		},
	})
	
	if err != nil {
		log.Printf("Progress task failed: %v", err)
	}
}

// 场景2: 日志流
func runLogScenario(ctx context.Context, client *mcp.Client) {
	time.Sleep(3 * time.Second)
	log.Println("Starting log stream...")
	
	_, err := client.CallTool(ctx, &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "start-logs",
			Arguments: map[string]interface{}{
				"count": 20,
			},
		},
	})
	
	if err != nil {
		log.Printf("Log stream failed: %v", err)
	}
}

// 场景3: 股票数据
func runStockScenario(ctx context.Context, client *mcp.Client) {
	time.Sleep(5 * time.Second)
	log.Println("Starting stock monitoring...")
	
	_, err := client.CallTool(ctx, &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "monitor-stocks",
			Arguments: map[string]interface{}{
				"symbols": "AAPL,MSFT,GOOG,AMZN",
			},
		},
	})
	
	if err != nil {
		log.Printf("Stock monitoring failed: %v", err)
	}
}
