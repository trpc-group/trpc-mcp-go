// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

// package mcptest 包含所有可被项目内其他包共享的、用于测试的公共辅助函数。
package mcptest

import (
	"context"
	"testing"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

// RunMiddlewareTest 是一个底层的测试辅助函数，用于对单个中间件进行单元测试。
func RunMiddlewareTest(
	t *testing.T,
	middlewareToTest mcp.MiddlewareFunc,
	req *mcp.JSONRPCRequest,
	finalHandler mcp.HandleFunc,
) (mcp.JSONRPCMessage, error) {
	t.Helper() 

	if finalHandler == nil {
		finalHandler = func(ctx context.Context, req *mcp.JSONRPCRequest, session mcp.Session) (mcp.JSONRPCMessage, error) {
			return &mcp.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  "default mock response",
			}, nil
		}
	}
	return middlewareToTest(context.Background(), req, nil, finalHandler)
}

// CheckMiddlewareFunc 是一个高层次的测试辅助函数，用于快速检查中间件是否符合基本约定。
func CheckMiddlewareFunc(t *testing.T, middlewareToTest mcp.MiddlewareFunc) {
	t.Helper()

	const requestID = "check-middleware-123"
	mockReq := &mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      requestID,
		Request: mcp.Request{
			Method: "test/method",
		},
		Params: map[string]interface{}{},
	}

	finalHandlerCalled := false
	mockFinalHandler := func(ctx context.Context, req *mcp.JSONRPCRequest, session mcp.Session) (mcp.JSONRPCMessage, error) {
		finalHandlerCalled = true
		return &mcp.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  "final handler result",
		}, nil
	}

	resp, err := RunMiddlewareTest(t, middlewareToTest, mockReq, mockFinalHandler)

	if err != nil {
		t.Errorf("CheckMiddlewareFunc: 中间件不应返回错误, 但收到了: %v", err)
	}
	if !finalHandlerCalled {
		t.Errorf("CheckMiddlewareFunc: 中间件必须调用 next 函数, 但它没有被调用")
	}

	response, ok := resp.(*mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("CheckMiddlewareFunc: 预期返回类型为 *JSONRPCResponse, 但收到了 %T", resp)
	}

	if response.ID != requestID {
		t.Errorf("CheckMiddlewareFunc: 中间件必须返回 next 函数的响应, 但响应 ID 不匹配。预期 %q, 收到 %q", requestID, response.ID)
	}
}