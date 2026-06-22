// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package main

import (
	"context"
	"fmt"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

func main() {
	server := mcp.NewStdioServer("stdio-child-server", "1.0.0")

	server.RegisterTool(
		mcp.NewTool("echo",
			mcp.WithDescription("Echo text through the stdio child server."),
			mcp.WithString("text", mcp.Description("Text to echo.")),
		),
		func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			text, _ := req.Params.Arguments["text"].(string)
			if text == "" {
				return mcp.NewErrorResult("missing text argument"), nil
			}
			return mcp.NewTextResult("stdio child received: " + text), nil
		},
	)

	server.RegisterTool(
		mcp.NewTool("add",
			mcp.WithDescription("Add two numbers through the stdio child server."),
			mcp.WithNumber("a", mcp.Description("First number.")),
			mcp.WithNumber("b", mcp.Description("Second number.")),
		),
		func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			a, okA := numberArg(req.Params.Arguments["a"])
			b, okB := numberArg(req.Params.Arguments["b"])
			if !okA || !okB {
				return mcp.NewErrorResult("arguments a and b must be numbers"), nil
			}
			return mcp.NewTextResult(fmt.Sprintf("%.0f", a+b)), nil
		},
	)

	if err := server.Start(); err != nil {
		panic(err)
	}
}

func numberArg(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case int32:
		return float64(v), true
	default:
		return 0, false
	}
}
