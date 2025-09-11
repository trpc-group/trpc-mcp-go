package middlewares

import (
	"context"
	"errors"
	"testing"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
	"trpc.group/trpc-go/trpc-mcp-go/mcptest"
)

func TestRecoveryMiddleware_Conformance(t *testing.T) {
	mcptest.CheckMiddlewareFunc(t, Recovery())
}

func TestRecoveryMiddleware(t *testing.T) {
	// 准备模拟请求
	mockReq := &mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "request-1",
		Request: mcp.Request{
			Method: "tools/call",
		},
		Params: map[string]interface{}{
			"name":      "test-tool",
			"arguments": map[string]interface{}{"param1": "value1"},
		},
	}

	// 测试1: 测试正常流程(无panic)
	t.Run("normal flow", func(t *testing.T) {
		finalHandlerCalled := false
		mockFinalHandler := func(ctx context.Context, req *mcp.JSONRPCRequest, session mcp.Session) (mcp.JSONRPCMessage, error) {
			finalHandlerCalled = true
			return &mcp.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  "normal result",
			}, nil
		}

		_, err := mcptest.RunMiddlewareTest(t, Recovery(), mockReq, mockFinalHandler)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !finalHandlerCalled {
			t.Error("final handler should be called")
		}
	})

	// 测试2: 测试panic恢复
	t.Run("panic recovery", func(t *testing.T) {
		panicErr := errors.New("test panic")
		mockFinalHandler := func(ctx context.Context, req *mcp.JSONRPCRequest, session mcp.Session) (mcp.JSONRPCMessage, error) {
			panic(panicErr)
		}

		resp, err := mcptest.RunMiddlewareTest(t, Recovery(), mockReq, mockFinalHandler)
		if err != nil {
			t.Errorf("middleware should recover from panic, got error: %v", err)
		}
		if resp == nil {
			t.Error("middleware should return error response")
		}
	})

	// 测试3: 测试panic过滤
	t.Run("panic filtering", func(t *testing.T) {
		// 配置只处理runtime errors
		middleware := RecoveryWithOptions(WithPanicFilter(OnlyHandleRuntimeErrors()))

		// 测试1: 测试字符串panic(应该被过滤掉)
		t.Run("string panic should be filtered", func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Error("string panic should not be caught by middleware")
				}
			}()

			mockFinalHandler := func(ctx context.Context, req *mcp.JSONRPCRequest, session mcp.Session) (mcp.JSONRPCMessage, error) {
				panic("test string panic")
			}

			mcptest.RunMiddlewareTest(t, middleware, mockReq, mockFinalHandler)
		})

		// 测试2: 测试runtime panic(应该被处理)
		t.Run("runtime error should be handled", func(t *testing.T) {
			mockFinalHandler := func(ctx context.Context, req *mcp.JSONRPCRequest, session mcp.Session) (mcp.JSONRPCMessage, error) {
				var s []int
				_ = s[1] // 故意触发index out of range panic
				return nil, nil
			}

			resp, err := mcptest.RunMiddlewareTest(t, middleware, mockReq, mockFinalHandler)
			if err != nil {
				t.Errorf("middleware should recover from runtime panic, got error: %v", err)
			}
			if resp == nil {
				t.Error("middleware should return error response for runtime panic")
			}
		})
	})

	// 测试4: 测试自定义错误响应
	t.Run("custom error response", func(t *testing.T) {
		customMsg := "custom error message"
		middleware := RecoveryWithOptions(WithCustomErrorResponse(
			func(ctx context.Context, req *mcp.JSONRPCRequest, panicErr interface{}) mcp.JSONRPCMessage {
				return mcp.NewJSONRPCErrorResponse(
					req.ID,
					mcp.ErrCodeInternal,
					customMsg,
					nil,
				)
			},
		))

		mockFinalHandler := func(ctx context.Context, req *mcp.JSONRPCRequest, session mcp.Session) (mcp.JSONRPCMessage, error) {
			panic("test panic")
		}

		resp, err := mcptest.RunMiddlewareTest(t, middleware, mockReq, mockFinalHandler)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if resp == nil {
			t.Error("middleware should return custom error response")
		}
		if errResp, ok := resp.(*mcp.JSONRPCError); ok {
			if errResp.Error.Message != customMsg {
				t.Errorf("expected custom error message %q, got %q", customMsg, errResp.Error.Message)
			}
		} else {
			t.Error("response should be JSONRPCErrorResponse")
		}
	})
}
