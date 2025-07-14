package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

func main() {
	log.Println("Starting SSE Notification Server...")

	// 创建支持 SSE 的服务器
	server := mcp.NewServer(
		"SSE-Notification-Server",
		"1.0.0",
		mcp.WithServerAddress(":3000"),
		mcp.WithServerPath("/sse"),
		mcp.WithPostSSEEnabled(true),
		mcp.WithGetSSEEnabled(true),
		mcp.WithNotificationBufferSize(20),
	)

	// 修复1: 正确使用 PropertyOption 参数
	progressTool := mcp.NewTool("start-progress",
		mcp.WithDescription("Start a task with progress notifications"),
		mcp.WithNumber("total", 
			mcp.Description("Total progress steps"), 
			mcp.Default(10),
		),
	)
	server.RegisterTool(progressTool, handleProgressTask)
	log.Println("Registered tool: start-progress")

	// 修复2: 参数选项使用正确格式
	logTool := mcp.NewTool("start-logs",
		mcp.WithDescription("Generate real-time log stream"),
		mcp.WithNumber("count", 
			mcp.Description("Number of log entries"), 
			mcp.Default(15),
		),
	)
	server.RegisterTool(logTool, handleLogStream)
	log.Println("Registered tool: start-logs")

	// 修复3: 字符串参数选项
	stockTool := mcp.NewTool("monitor-stocks",
		mcp.WithDescription("Real-time stock price monitor"),
		mcp.WithString("symbols", 
			mcp.Description("Comma separated stock symbols"), 
			mcp.Default("AAPL,MSFT,GOOG"),
		),
	)
	server.RegisterTool(stockTool, handleStockPrices)
	log.Println("Registered tool: monitor-stocks")

	// 优雅关机设置
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Println("Server listening on :3000, path /sse")
		if err := server.Start(); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	<-stop
	log.Println("Shutting down server...")
}

// 处理进度任务 - 场景1: 进度条通知
func handleProgressTask(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	total := 10
	if steps, ok := req.Params.Arguments["total"].(float64); ok {
		total = int(steps)
	}

	notifier, ok := mcp.GetNotificationSender(ctx)
	if !ok {
		return mcp.NewErrorResult("Notification system unavailable"), nil
	}

	// 发送进度通知
	for i := 0; i <= total; i++ {
		select {
		case <-ctx.Done():
			return mcp.NewTextResult("Task canceled"), nil
		default:
			progress := float64(i) / float64(total)
			msg := fmt.Sprintf("Processing step %d/%d", i, total)
			
			// 发送进度通知
			if err := notifier.SendProgress(progress, msg); err != nil {
				log.Printf("Progress send error: %v", err)
			}
			
			// 随机发送日志通知
			if rand.Intn(3) == 0 {
				logMsg := fmt.Sprintf("LOG: Step %d completed", i)
				_ = notifier.SendLogMessage("info", logMsg)
			}
			
			time.Sleep(time.Second)
		}
	}

	return mcp.NewTextResult("Task completed successfully"), nil
}

// 处理日志流 - 场景2: 实时日志
func handleLogStream(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	count := 15
	if cnt, ok := req.Params.Arguments["count"].(float64); ok {
		count = int(cnt)
	}

	notifier, ok := mcp.GetNotificationSender(ctx)
	if !ok {
		return mcp.NewErrorResult("Notification system unavailable"), nil
	}

	// 生成实时日志
	logTypes := []string{"INFO", "DEBUG", "WARN", "ERROR"}
	for i := 0; i < count; i++ {
		select {
		case <-ctx.Done():
			return mcp.NewTextResult("Log stream stopped"), nil
		default:
			logType := logTypes[rand.Intn(len(logTypes))]
			msg := fmt.Sprintf("[%s] Log entry %d: System operation completed", logType, i+1)
			
			// 发送日志通知
			if err := notifier.SendLogMessage(strings.ToLower(logType), msg); err != nil {
				log.Printf("Log send error: %v", err)
			}
			
			time.Sleep(500 * time.Millisecond)
		}
	}

	return mcp.NewTextResult("Log generation completed"), nil
}

// 处理股票价格 - 场景3: 实时数据流
func handleStockPrices(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	symbols := "AAPL,MSFT,GOOG"
	if sym, ok := req.Params.Arguments["symbols"].(string); ok {
		symbols = sym
	}

	notifier, ok := mcp.GetNotificationSender(ctx)
	if !ok {
		return mcp.NewErrorResult("Notification system unavailable"), nil
	}

	stockList := strings.Split(symbols, ",")
	priceMap := make(map[string]float64)
	
	// 初始化价格
	for _, symbol := range stockList {
		priceMap[symbol] = 100 + rand.Float64()*300
	}

	// 实时更新股票价格
	for i := 0; ; i++ {
		select {
		case <-ctx.Done():
			return mcp.NewTextResult("Stock monitoring stopped"), nil
		default:
			// 更新所有股票价格
			update := make(map[string]interface{}) // 修复4: 使用正确的接口类型
			for symbol, price := range priceMap {
				change := (rand.Float64() - 0.5) * 10
				newPrice := price + change
				priceMap[symbol] = newPrice
				update[symbol] = newPrice // 直接赋值，float64 兼容 interface{}
			}

			// 发送股票更新通知
			if err := notifier.SendCustomNotification("stocks/update", update); err != nil {
				log.Printf("Stock update error: %v", err)
			}
			
			// 每5次发送一次摘要
			if i%5 == 0 {
				summary := make(map[string]interface{})
				for sym, price := range priceMap {
					summary[sym] = map[string]interface{}{
						"price":   price,
						"change":  price - 100,
						"percent": (price - 100) / 100,
					}
				}
				_ = notifier.SendCustomNotification("stocks/summary", summary)
			}
			
			time.Sleep(1 * time.Second)
		}
	}
}
