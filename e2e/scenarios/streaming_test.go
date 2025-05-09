package scenarios

import (
	"context"
	"testing"
	"time"

	"github.com/modelcontextprotocol/streamable-mcp/e2e"
	"github.com/modelcontextprotocol/streamable-mcp/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStreamingContent 测试流式内容从服务器到客户端的传输
func TestStreamingContent(t *testing.T) {
	// 设置测试服务器
	serverURL, cleanup := e2e.StartTestServer(t, e2e.WithTestTools())
	defer cleanup()

	// 创建测试客户端
	client := e2e.CreateTestClient(t, serverURL)
	defer e2e.CleanupClient(t, client)

	// 初始化客户端
	e2e.InitializeClient(t, client)

	// 测试流式问候工具
	t.Run("StreamingGreet", func(t *testing.T) {
		// 设置消息数量
		messageCount := 5

		// 调用流式问候工具
		content := e2e.ExecuteTestTool(t, client, "streaming-greet", map[string]interface{}{
			"name":  "流式测试",
			"count": messageCount,
		})

		// 验证结果
		require.Len(t, content, messageCount, "应该有指定数量的内容")

		// 验证每条内容
		for _, item := range content {
			// 使用类型断言转换为 TextContent
			textContent, ok := item.(schema.TextContent)
			assert.True(t, ok, "内容应该是 TextContent 类型")
			assert.Equal(t, "text", textContent.Type, "内容类型应为文本")
			assert.Contains(t, textContent.Text, "流式消息", "内容应包含流式消息标记")
			assert.Contains(t, textContent.Text, "流式测试", "内容应包含用户名")
		}
	})
}

// TestDelayedResponse 测试延迟响应
func TestDelayedResponse(t *testing.T) {
	// 设置测试服务器
	serverURL, cleanup := e2e.StartTestServer(t, e2e.WithTestTools())
	defer cleanup()

	// 创建测试客户端
	client := e2e.CreateTestClient(t, serverURL)
	defer e2e.CleanupClient(t, client)

	// 初始化客户端
	e2e.InitializeClient(t, client)

	// 测试延迟工具
	t.Run("DelayTool", func(t *testing.T) {
		// 设置延迟时间
		delayMs := 1000
		message := "延迟测试消息"

		// 记录开始时间
		startTime := time.Now()

		// 调用延迟工具
		content := e2e.ExecuteTestTool(t, client, "delay-tool", map[string]interface{}{
			"delay_ms": delayMs,
			"message":  message,
		})

		// 计算经过的时间
		elapsed := time.Since(startTime)

		// 验证结果
		require.Len(t, content, 1, "应该只有一条内容")

		// 使用类型断言转换为 TextContent
		textContent, ok := content[0].(schema.TextContent)
		assert.True(t, ok, "内容应该是 TextContent 类型")
		assert.Equal(t, "text", textContent.Type, "内容类型应为文本")
		assert.Contains(t, textContent.Text, message, "内容应包含提供的消息")

		// 验证至少经过了指定的延迟时间
		assert.GreaterOrEqual(t, elapsed.Milliseconds(), int64(delayMs),
			"响应时间应至少为指定的延迟时间")
	})
}

// TestContextCancellation 测试上下文取消
func TestContextCancellation(t *testing.T) {
	// 设置测试服务器
	serverURL, cleanup := e2e.StartTestServer(t, e2e.WithTestTools())
	defer cleanup()

	// 创建测试客户端
	client := e2e.CreateTestClient(t, serverURL)
	defer e2e.CleanupClient(t, client)

	// 初始化客户端
	e2e.InitializeClient(t, client)

	// 测试上下文取消
	t.Run("CancelDelayTool", func(t *testing.T) {
		// 创建一个短超时的上下文
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		// 调用长延迟工具
		_, err := client.CallTool(ctx, "delay-tool", map[string]interface{}{
			"delay_ms": 2000, // 设置长于上下文超时的延迟
			"message":  "这条消息应该不会收到",
		})

		// 验证错误
		require.Error(t, err, "应该返回超时错误")
		assert.Contains(t, err.Error(), "context", "错误应与上下文取消有关")
	})
}
