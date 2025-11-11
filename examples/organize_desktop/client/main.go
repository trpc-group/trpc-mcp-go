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

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

func main() {
	log.Println("启动 organize_desktop_files 工具调用示例客户端...")

	ctx := context.Background()
	serverURL := "http://localhost:3001/mcp"
	client, err := mcp.NewClient(
		serverURL,
		mcp.Implementation{
			Name:    "Organize-Desktop-Client",
			Version: "1.0.0",
		},
		mcp.WithClientLogger(mcp.GetDefaultLogger()),
	)
	if err != nil {
		log.Fatalf("创建 MCP 客户端失败: %v", err)
	}
	defer client.Close()

	_, err = client.Initialize(ctx, &mcp.InitializeRequest{})
	if err != nil {
		log.Fatalf("初始化失败: %v", err)
	}
	client.RegisterNotificationHandler("notifications/progress", func(n *mcp.JSONRPCNotification) error {
		progress, _ := n.Params.AdditionalFields["progress"].(float64)
		message, _ := n.Params.AdditionalFields["message"].(string)
		fmt.Printf("[进度] %.0f%% - %s\n", progress*100, message)
		return nil
	})

	// 调用 organize_desktop_files 工具
	callReq := &mcp.CallToolRequest{}
	callReq.Params.Name = "organize_desktop_files"
	callReq.Params.Arguments = map[string]interface{}{
		// "dir_path": "C:\\Users\\你的用户名\\Desktop", // 可省略，默认桌面
		"dir_path": "D:\\Desktop",
		"mode": "type", // 可选 type/ctime/project
	}
	log.Println("调用 organize_desktop_files 工具...")
	resp, err := client.CallTool(ctx, callReq)
	if err != nil {
		log.Fatalf("工具调用失败: %v", err)
	}

	log.Println("工具调用结果:")
	var reportURI string
	for _, item := range resp.Content {
		if text, ok := item.(mcp.TextContent); ok {
			fmt.Println(text.Text)
			// 自动提取报告 URI
			if idx := findReportURI(text.Text); idx != "" {
				reportURI = idx
			}
		} else {
			fmt.Printf("[其他类型内容] %+v\n", item)
		}
	}

	// 自动读取并打印报告内容
	if reportURI != "" {
		log.Printf("\n读取报告资源: %s ...", reportURI)
		readReq := &mcp.ReadResourceRequest{}
		readReq.Params.URI = reportURI
		resourceContent, err := client.ReadResource(ctx, readReq)
		if err != nil {
			log.Fatalf("读取资源失败: %v", err)
		}
		for _, content := range resourceContent.Contents {
			if text, ok := content.(mcp.TextResourceContents); ok {
				fmt.Println("\n报告内容：")
				fmt.Println(text.Text)
			}
		}
	}
	log.Println("客户端示例结束.")
}

// findReportURI 从文本中提取 resource://organize_desktop/report.json URI
func findReportURI(s string) string {
	// 简单正则或字符串查找
	const prefix = "resource://organize_desktop/report.json"
	if idx := len(s) - len(prefix); idx >= 0 && s[idx:] == prefix {
		return prefix
	}
	if i := findIndex(s, prefix); i >= 0 {
		return prefix
	}
	return ""
}

func findIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
