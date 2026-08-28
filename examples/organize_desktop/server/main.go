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
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"encoding/json"
	"sync"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

// 分类方式枚举
var organizeModes = []string{"type", "ctime", "project"}

var (
	resourceOnce       sync.Once
	resourceRegistered bool
	resourceURI        string
)

func main() {
	log.Printf("Starting organize_desktop_files MCP server...")

	mcpServer := mcp.NewServer(
		"Organize-Desktop-Server",
		"0.1.0",
		mcp.WithServerAddress(":3001"),
		mcp.WithServerPath("/mcp"),
		mcp.WithServerLogger(mcp.GetDefaultLogger()),
	)

	organizeTool := mcp.NewTool("organize_desktop_files",
		mcp.WithDescription("自动分析桌面文件并归类整理，支持 SSE 进度与资源报告下载。"),
		mcp.WithString("dir_path", mcp.Description("要整理的桌面目录路径。")),
		mcp.WithString("mode", mcp.Description("归类方式：type/ctime/project。"), mcp.Enum(organizeModes...)),
	)

	mcpServer.RegisterTool(organizeTool, handleOrganizeDesktopFiles)
	log.Printf("Registered tool: organize_desktop_files")

	// 注册资源（首次注册时）
	resourceOnce.Do(func() {
		resource := &mcp.Resource{
			URI:         "resource://organize_desktop/report.json",
			Name:        "desktop-organize-report",
			Description: "桌面整理 JSON 总结报告",
			MimeType:    "application/json",
		}
		mcpServer.RegisterResource(resource, func(ctx context.Context, req *mcp.ReadResourceRequest) (mcp.ResourceContents, error) {
			// 读取最新报告内容
			data, err := os.ReadFile("desktop_organize_report.json")
			if err != nil {
				return mcp.TextResourceContents{
					URI:      resource.URI,
					MIMEType: resource.MimeType,
					Text:     "报告文件不存在或读取失败。",
				}, nil
			}
			return mcp.TextResourceContents{
				URI:      resource.URI,
				MIMEType: resource.MimeType,
				Text:     string(data),
			}, nil
		})
		resourceRegistered = true
		resourceURI = resource.URI
	})

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		log.Printf("MCP server started, listening on port 3001, path /mcp")
		if err := mcpServer.Start(); err != nil {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()
	<-stop
	log.Printf("Shutting down server...")
}

// handleOrganizeDesktopFiles 是 organize_desktop_files 工具的处理函数
func handleOrganizeDesktopFiles(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dirPath, _ := req.Params.Arguments["dir_path"].(string)
	mode, _ := req.Params.Arguments["mode"].(string)
	if dirPath == "" {
		dirPath = os.Getenv("USERPROFILE") + "\\Desktop" // Windows 桌面默认路径
	}
	if mode == "" {
		mode = "type"
	}

	// 获取 SSE 通知 sender
	notificationSender, hasSender := mcp.GetNotificationSender(ctx)
	if !hasSender {
		return mcp.NewTextResult("Error: 无法获取 SSE 通知 sender，无法推送进度。"), fmt.Errorf("no notification sender")
	}

	// 模拟扫描文件阶段
	notificationSender.SendProgress(0.05, "开始扫描桌面文件...")
	time.Sleep(300 * time.Millisecond)

	files := []string{}
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过无法访问的文件
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		notificationSender.SendLogMessage("error", "扫描文件失败: "+err.Error())
		return mcp.NewTextResult("扫描文件失败: " + err.Error()), err
	}
	notificationSender.SendProgress(0.15, fmt.Sprintf("共发现 %d 个文件，准备归类...", len(files)))
	time.Sleep(300 * time.Millisecond)

	// 模拟归类阶段
	total := len(files)
	if total == 0 {
		notificationSender.SendProgress(1.0, "桌面无可整理文件。")
		return mcp.NewTextResult("桌面无可整理文件。"), nil
	}

	// 统计结果结构
	type FileInfo struct {
		Path  string `json:"path"`
		Type  string `json:"type"`
		CTime string `json:"ctime"`
	}
	result := map[string][]FileInfo{}

	for i, f := range files {
		info, err := os.Stat(f)
		if err != nil {
			continue
		}
		fileType := filepath.Ext(f)
		ctime := info.ModTime().Format("2006-01-02 15:04:05")
		item := FileInfo{Path: f, Type: fileType, CTime: ctime}
		var key string
		switch mode {
		case "type":
			key = fileType
		case "ctime":
			key = info.ModTime().Format("2006-01")
		default:
			key = "other"
		}
		result[key] = append(result[key], item)
		if i%10 == 0 || i == total-1 {
			progress := 0.2 + 0.7*float64(i+1)/float64(total)
			msg := fmt.Sprintf("已归类 %d/%d 个文件...", i+1, total)
			notificationSender.SendProgress(progress, msg)
			time.Sleep(10 * time.Millisecond)
		}
	}

	notificationSender.SendProgress(1.0, "整理完成，生成报告...")
	time.Sleep(200 * time.Millisecond)

	// 生成 JSON 报告
	reportBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		notificationSender.SendLogMessage("error", "生成 JSON 报告失败: "+err.Error())
		return mcp.NewTextResult("生成 JSON 报告失败: " + err.Error()), err
	}
	reportPath := "desktop_organize_report.json"
	err = os.WriteFile(reportPath, reportBytes, 0644)
	if err != nil {
		notificationSender.SendLogMessage("error", "写入报告文件失败: "+err.Error())
		return mcp.NewTextResult("写入报告文件失败: " + err.Error()), err
	}

	// 注册资源（如果未注册）
	if !resourceRegistered {
		resourceOnce.Do(func() {}) // 确保 main 中的注册已执行
	}

	notificationSender.SendProgress(1.0, "报告已生成，可下载。")
	return mcp.NewTextResult(fmt.Sprintf("整理完成，共 %d 个文件。报告资源 URI: %s", total, resourceURI)), nil
}
