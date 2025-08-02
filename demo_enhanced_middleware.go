// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"fmt"
	"log"
	"time"
)

func main() {
	log.Println("=== tRPC-MCP-Go 增强中间件系统演示 ===")
	
	// 1. 演示错误处理和分类
	demonstrateErrorHandling()
	
	// 2. 演示线程安全的限流中间件
	demonstrateRateLimit()
	
	// 3. 演示熔断器中间件
	demonstrateCircuitBreaker()
	
	// 4. 演示监控和指标收集
	demonstrateMonitoring()
	
	log.Println("=== 演示完成 ===")
}

func demonstrateErrorHandling() {
	log.Println("--- 1. 错误处理和分类演示 ---")
	
	// 创建一个认证中间件（故意使用空API密钥）
	authMiddleware := AuthMiddleware("")
	
	// 模拟处理器
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "success", nil
	}
	
	ctx := context.Background()
	req := &CallToolRequest{
		Request: Request{
			Method: "tools/call",
		},
		Params: CallToolParams{Name: "test_tool"},
	}
	
	// 执行中间件
	resp, err := authMiddleware(ctx, req, handler)
	
	if err != nil {
		if middlewareErr, ok := err.(*MiddlewareError); ok {
			log.Printf("捕获到中间件错误:")
			log.Printf("  错误码: %s", middlewareErr.Code)
			log.Printf("  错误消息: %s", middlewareErr.Message)
			log.Printf("  错误时间: %v", middlewareErr.Timestamp.Format("2006-01-02 15:04:05"))
			log.Printf("  错误上下文: %+v", middlewareErr.Context)
			log.Printf("  调用堆栈长度: %d", len(middlewareErr.Trace))
		}
	} else {
		log.Printf("响应: %v", resp)
	}
	log.Println()
}

func demonstrateRateLimit() {
	log.Println("--- 2. 线程安全限流中间件演示 ---")
	
	// 创建限流中间件：每秒最多2个请求
	rateLimitMiddleware := RateLimitingMiddleware(2, time.Second)
	
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "success", nil
	}
	
	ctx := context.WithValue(context.Background(), "user_id", "demo-user")
	req := &CallToolRequest{
		Request: Request{
			Method: "tools/call",
		},
		Params: CallToolParams{Name: "test_tool"},
	}
	
	// 快速发送5个请求
	for i := 1; i <= 5; i++ {
		resp, err := rateLimitMiddleware(ctx, req, handler)
		if err != nil {
			if middlewareErr, ok := err.(*MiddlewareError); ok {
				log.Printf("请求 %d: 被限流 - %s", i, middlewareErr.Message)
				log.Printf("  当前请求数: %v/%v", 
					middlewareErr.Context["current_requests"], 
					middlewareErr.Context["max_requests"])
			}
		} else {
			log.Printf("请求 %d: 成功 - %v", i, resp)
		}
		time.Sleep(300 * time.Millisecond) // 模拟请求间隔
	}
	log.Println()
}

func demonstrateCircuitBreaker() {
	log.Println("--- 3. 熔断器中间件演示 ---")
	
	// 创建熔断器中间件：失败阈值为2，超时1秒
	circuitBreakerMiddleware := CircuitBreakerMiddleware(2, time.Second)
	
	// 会失败的处理器
	failingHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, fmt.Errorf("模拟业务失败")
	}
	
	// 成功的处理器
	successHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "success", nil
	}
	
	ctx := context.Background()
	req := &CallToolRequest{
		Request: Request{
			Method: "tools/call",
		},
		Params: CallToolParams{Name: "test_tool"},
	}
	
	// 1. 发送失败请求直到熔断器打开
	log.Println("发送失败请求:")
	for i := 1; i <= 4; i++ {
		resp, err := circuitBreakerMiddleware(ctx, req, failingHandler)
		if err != nil {
			if middlewareErr, ok := err.(*MiddlewareError); ok && middlewareErr.Code == ErrCodeCircuitBreaker {
				log.Printf("请求 %d: 熔断器已打开 - %s", i, middlewareErr.Message)
				log.Printf("  失败次数: %v/%v", 
					middlewareErr.Context["failure_count"],
					middlewareErr.Context["failure_threshold"])
			} else {
				log.Printf("请求 %d: 业务失败 - %v", i, err)
			}
		} else {
			log.Printf("请求 %d: 成功 - %v", i, resp)
		}
	}
	
	// 2. 等待熔断器恢复
	log.Println("等待熔断器恢复...")
	time.Sleep(1100 * time.Millisecond)
	
	// 3. 发送成功请求
	log.Println("发送成功请求:")
	resp, err := circuitBreakerMiddleware(ctx, req, successHandler)
	if err != nil {
		log.Printf("请求失败: %v", err)
	} else {
		log.Printf("请求成功: %v", resp)
	}
	log.Println()
}

func demonstrateMonitoring() {
	log.Println("--- 4. 监控和指标收集演示 ---")
	
	// 获取全局监控器
	monitor := GetGlobalMonitor()
	
	// 创建监控中间件
	monitoringMiddleware := MonitoringMiddleware("demo_middleware")
	
	// 创建一个有时成功有时失败的处理器
	var requestCount int
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		requestCount++
		if requestCount%3 == 0 {
			return nil, fmt.Errorf("模拟失败")
		}
		time.Sleep(time.Duration(requestCount*10) * time.Millisecond) // 模拟不同的响应时间
		return fmt.Sprintf("success_%d", requestCount), nil
	}
	
	ctx := context.Background()
	req := &CallToolRequest{
		Request: Request{
			Method: "tools/call",
		},
		Params: CallToolParams{Name: "test_tool"},
	}
	
	// 发送多个请求
	log.Println("发送监控请求:")
	for i := 1; i <= 6; i++ {
		resp, err := monitoringMiddleware(ctx, req, handler)
		if err != nil {
			log.Printf("请求 %d: 失败 - %v", i, err)
		} else {
			log.Printf("请求 %d: 成功 - %v", i, resp)
		}
		time.Sleep(50 * time.Millisecond)
	}
	
	// 显示监控指标
	log.Println("监控指标:")
	metrics := monitor.GetMetrics("demo_middleware")
	if metrics != nil {
		log.Printf("  请求总数: %d", metrics.RequestCount)
		log.Printf("  错误总数: %d", metrics.ErrorCount)
		log.Printf("  成功率: %.2f%%", float64(metrics.RequestCount-metrics.ErrorCount)/float64(metrics.RequestCount)*100)
		log.Printf("  平均响应时间: %v", metrics.AverageDuration)
		log.Printf("  最大响应时间: %v", metrics.MaxDuration)
		log.Printf("  最小响应时间: %v", metrics.MinDuration)
	}
	
	// 打印完整报告
	log.Println("完整性能报告:")
	monitor.PrintReport()
	
	// 导出JSON格式的指标
	jsonMetrics, err := monitor.ToJSON()
	if err == nil {
		log.Printf("JSON格式指标:\n%s", jsonMetrics)
	}
	log.Println()
}