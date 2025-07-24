// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
	sampling "trpc.group/trpc-go/trpc-mcp-go/sampling"
)

func handleIntelligentAnalysis(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get the sampling sender - this should be obtained from the server itself
	samplingSender, hasSampling := mcp.GetSamplingSender(ctx)
	if !hasSampling {
		return mcp.NewErrorResult("Sampling功能不可用"),
			fmt.Errorf("sampling not supported")
	}

	data, ok := req.Params.Arguments["data"].(string)
	if !ok {
		return mcp.NewErrorResult("Missing data parameter"),
			fmt.Errorf("data parameter required")
	}

	analysisType, _ := req.Params.Arguments["type"].(string)
	if analysisType == "" {
		analysisType = "general"
	}

	//Creating a Sampling Request
	samplingReq := &sampling.SamplingCreateMessageRequest{
		JSONRPC: "2.0",
		ID:      mcp.GenerateRequestID(),
		Method:  "sampling/createMessage",
		Params: sampling.SamplingCreateMessageParams{
			Messages: []sampling.SamplingMessage{
				{
					Role: "user",
					Content: mcp.SamplingTextContent{
						Type: "text",
						Text: fmt.Sprintf("请分析以下%s数据：%s", analysisType, data),
					},
				},
			},
			ModelPreferences: (*sampling.ModelPreferences)(&mcp.SamplingModelPreferences{
				Hints:                []string{"claude-3-sonnet", "gpt-4"},
				IntelligencePriority: mcp.FloatPtr(0.9),
				SpeedPriority:        mcp.FloatPtr(0.3),
				CostPriority:         mcp.FloatPtr(0.2),
			}),
			SystemPrompt: mcp.StringPtr("You are a professional data analyst, please provide accurate and insightful analysis."),
			MaxTokens:    mcp.IntPtr(1500),
			Temperature:  mcp.FloatPtr(0.7),
		},
	}

	//Send request to client's LLM
	result, err := samplingSender.SendSamplingRequest(ctx, samplingReq)
	if err != nil {
		return mcp.NewErrorResult("AI分析失败"), err
	}

	//Handling the Response
	if textContent, ok := result.Content.(mcp.SamplingTextContent); ok {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(fmt.Sprintf(
					"Intelligent analysis results (Model: %s)\n\n%s\n\n---\nstop reason: %s",
					result.Model,
					textContent.Text,
					result.StopReason,
				)),
			},
		}, nil
	}

	return mcp.NewErrorResult("Response format error"),
		fmt.Errorf("unexpected content type")
}

func main() {
	log.Printf("Start the intelligent analysis server that supports sampling...")

	//Creating a Sampling-enabled Server
	mcpServer := mcp.NewServer(
		"Intelligent-Analysis-Server",
		"1.0.0",
		mcp.WithServerAddress(":3000"),
		mcp.WithServerPath("/mcp"),
		mcp.WithServerLogger(mcp.GetDefaultLogger()),
		// 启用Sampling功能
		mcp.WithSamplingEnabled(true),
		mcp.WithSamplingConfigServer(&mcp.SamplingServerConfig{
			MaxTokensLimit:      2000,
			RateLimitPerMinute:  20,
			AllowedContentTypes: []string{"text", "image"},
			RequireApproval:     true,
		}),
	)

	mcpServer.RegisterSamplingHandler(mcp.NewDefaultSamplingHandler(&mcp.SamplingClientConfig{
		MaxTokensPerRequest: 4096,
		TimeoutSeconds:      60,
		ModelMappings: map[string]string{
			"claude-3-sonnet": "gpt-4",
			"claude-3-haiku":  "gpt-3.5-turbo",
		},
		AutoApprove:  false,
		DefaultModel: "gpt-3.5-turbo",
	}))

	// Register smart analysis tool - use closure to pass server instance
	intelligentAnalysisTool := mcp.NewTool("intelligent-analysis",
		mcp.WithDescription("Using AI for smart data analysis"),
		mcp.WithString("data", mcp.Description("Data to be analyzed")),
		mcp.WithString("type",
			mcp.Description("Analysis type: financial, sales, marketing, general"),
			mcp.Default("general"),
		))

	//Modify the tool processing function to ensure that there is a SamplingSender in the context
	mcpServer.RegisterTool(intelligentAnalysisTool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		//Make sure there is a SamplingSender in the context
		ctx = mcp.SetSamplingSender(ctx, mcpServer)
		return handleIntelligentAnalysis(ctx, req)
	})

	log.Printf("Registered Tools:")
	log.Printf("- intelligent-analysis -")

	//HTTP endpoint that handles Sampling requests
	http.HandleFunc("/mcp/sampling/createMessage", func(w http.ResponseWriter, r *http.Request) {
		// 设置 SamplingSender（服务端）
		ctx := mcp.SetSamplingSender(r.Context(), mcpServer)

		//Get the Sampling support structure
		samplingSupport := mcp.ServerSamplingMap[mcpServer]
		if samplingSupport == nil || !samplingSupport.SamplingEnabled {
			http.Error(w, "sampling not enabled", http.StatusInternalServerError)
			return
		}

		if samplingSupport.SamplingHandler == nil {
			http.Error(w, "sampling handler not configured", http.StatusInternalServerError)
			return
		}

		//Parsing Requests
		var req sampling.SamplingCreateMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "failed to decode request", http.StatusBadRequest)
			return
		}

		//Calling the Sampling Handler
		resp, err := samplingSupport.SamplingHandler.HandleSamplingRequest(ctx, &req)
		if err != nil {
			http.Error(w, "sampling handler error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		//Return Response
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, "failed to encode response", http.StatusInternalServerError)
		}
	})

	//Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	//Start the server
	go func() {
		log.Printf("The MCP server has been started, listening on port 3000, path /mcp")
		log.Printf("Sampling function is enabled, proxy behavior is supported")
		if err := mcpServer.Start(); err != nil {
			log.Fatalf("Server startup failed: %v", err)
		}
	}()

	//Waiting for shutdown signal
	<-stop
	log.Printf("Shut down server...")
	mcp.CleanupServerSampling(mcpServer)
}
