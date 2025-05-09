package e2e

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/streamable-mcp/client"
	"github.com/modelcontextprotocol/streamable-mcp/schema"
	"github.com/modelcontextprotocol/streamable-mcp/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ClientOption 定义客户端选项函数
type ClientOption func(*client.Client)

// WithProtocolVersion 选项：设置协议版本
func WithProtocolVersion(version string) ClientOption {
	return func(c *client.Client) {
		// 在客户端创建时已应用，这里只是占位
	}
}

// WithGetSSEEnabled 选项：启用 GET SSE 连接
func WithGetSSEEnabled() ClientOption {
	return func(c *client.Client) {
		// 使用 client 包中已有的 WithGetSSEEnabled
		client.WithGetSSEEnabled(true)(c)
	}
}

// WithLastEventID 选项：设置 Last-Event-ID 用于流恢复
// 注意：这只是在测试辅助函数中保存 eventID，实际使用时需要在调用时通过 StreamOptions 传递
func WithLastEventID(eventID string) ClientOption {
	return func(c *client.Client) {
		// 不需要在这里设置，将在 ExecuteSSETestTool 等方法中使用
	}
}

// CreateTestClient 创建连接到给定 URL 的测试客户端
func CreateTestClient(t *testing.T, url string, opts ...ClientOption) *client.Client {
	t.Helper()

	// 创建客户端
	c, err := client.NewClient(url, schema.Implementation{
		Name:    "E2E-Test-Client",
		Version: "1.0.0",
	})
	require.NoError(t, err, "创建客户端失败")
	require.NotNil(t, c, "客户端不应为 nil")

	// 应用选项
	for _, opt := range opts {
		opt(c)
	}

	t.Logf("创建测试客户端 URL: %s", url)

	return c
}

// InitializeClient 初始化客户端并验证成功
func InitializeClient(t *testing.T, c *client.Client) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 初始化客户端
	resp, err := c.Initialize(ctx)
	require.NoError(t, err, "初始化客户端失败")
	require.NotNil(t, resp, "初始化响应不应为 nil")

	// 验证关键字段
	assert.NotEmpty(t, resp.ServerInfo.Name, "服务器名称不应为空")
	assert.NotEmpty(t, resp.ServerInfo.Version, "服务器版本不应为空")
	assert.NotEmpty(t, resp.ProtocolVersion, "协议版本不应为空")

	// 验证会话 ID
	sessionID := c.GetSessionID()
	assert.NotEmpty(t, sessionID, "会话 ID 不应为空")

	t.Logf("客户端初始化成功，服务器: %s %s, 会话ID: %s",
		resp.ServerInfo.Name, resp.ServerInfo.Version, sessionID)
}

// ExecuteTestTool 执行测试工具并验证结果
func ExecuteTestTool(t *testing.T, c *client.Client, toolName string, args map[string]interface{}) []schema.ToolContent {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 调用工具
	content, err := c.CallTool(ctx, toolName, args)
	require.NoError(t, err, "调用工具 %s 失败", toolName)

	t.Logf("工具 %s 调用成功，结果内容数量: %d", toolName, len(content))

	return content
}

// CleanupClient 清理客户端资源
func CleanupClient(t *testing.T, c *client.Client) {
	t.Helper()

	if c == nil {
		return
	}

	// 尝试终止会话
	sessionID := c.GetSessionID()
	if sessionID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := c.TerminateSession(ctx)
		if err != nil {
			t.Logf("终止会话 %s 失败: %v", sessionID, err)
		} else {
			t.Logf("会话 %s 已终止", sessionID)
		}
	}

	// 关闭客户端
	c.Close()
	t.Log("客户端资源已清理")
}

// CreateSSETestClient 创建配置为使用 SSE 的测试客户端
func CreateSSETestClient(t *testing.T, url string, opts ...ClientOption) *client.Client {
	t.Helper()

	// 创建客户端
	c, err := client.NewClient(url, schema.Implementation{
		Name:    "E2E-SSE-Test-Client",
		Version: "1.0.0",
	})
	require.NoError(t, err, "创建 SSE 客户端失败")
	require.NotNil(t, c, "SSE 客户端不应为 nil")

	// 应用选项
	for _, opt := range opts {
		opt(c)
	}

	t.Logf("创建 SSE 测试客户端 URL: %s", url)

	return c
}

// ExecuteSSETestTool 执行测试工具并支持收集通知
func ExecuteSSETestTool(t *testing.T, c *client.Client, toolName string, args map[string]interface{}, collector *NotificationCollector) []schema.ToolContent {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 创建 streamOptions
	streamOpts := &transport.StreamOptions{
		NotificationHandlers: collector.GetHandlers(),
	}

	// 调用工具，使用带流的方法
	content, err := c.CallToolWithStream(ctx, toolName, args, streamOpts)
	require.NoError(t, err, "调用工具 %s 失败", toolName)

	t.Logf("工具 %s 调用成功，结果内容数量: %d，收到通知数量: %d",
		toolName, len(content), collector.Count())

	return content
}

// NotificationCollector 用于收集和验证通知
type NotificationCollector struct {
	// 通知收集通道
	notifications chan *schema.Notification
	// 锁，保护计数器
	mu sync.Mutex
	// 通知计数器
	count int
	// 通知映射
	notificationsByMethod map[string][]*schema.Notification
}

// NewNotificationCollector 创建新的通知收集器
func NewNotificationCollector() *NotificationCollector {
	return &NotificationCollector{
		notifications:         make(chan *schema.Notification, 50),
		notificationsByMethod: make(map[string][]*schema.Notification),
	}
}

// GetHandlers 返回通知处理器映射
func (nc *NotificationCollector) GetHandlers() map[string]transport.NotificationHandler {
	// 创建处理器映射
	handlers := make(map[string]transport.NotificationHandler)

	// 进度通知处理器
	handlers["notifications/progress"] = func(n *schema.Notification) error {
		nc.addNotification(n)
		return nil
	}

	// 日志通知处理器
	handlers["notifications/message"] = func(n *schema.Notification) error {
		nc.addNotification(n)
		return nil
	}

	return handlers
}

// addNotification 添加通知
func (nc *NotificationCollector) addNotification(n *schema.Notification) {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	// 增加计数
	nc.count++

	// 添加到通道
	select {
	case nc.notifications <- n:
		// 通知已发送到通道
	default:
		// 通道已满，跳过
	}

	// 按方法分类
	nc.notificationsByMethod[n.Method] = append(nc.notificationsByMethod[n.Method], n)
}

// Count 返回收到的通知总数
func (nc *NotificationCollector) Count() int {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	return nc.count
}

// GetNotifications 返回指定方法的通知列表
func (nc *NotificationCollector) GetNotifications(method string) []*schema.Notification {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	return nc.notificationsByMethod[method]
}

// GetProgressNotifications 返回进度通知列表
func (nc *NotificationCollector) GetProgressNotifications() []*schema.Notification {
	return nc.GetNotifications("notifications/progress")
}

// GetLogNotifications 返回日志通知列表
func (nc *NotificationCollector) GetLogNotifications() []*schema.Notification {
	return nc.GetNotifications("notifications/message")
}

// AssertNotificationCount 断言指定方法的通知数量
func (nc *NotificationCollector) AssertNotificationCount(t *testing.T, method string, expectedCount int) {
	notifications := nc.GetNotifications(method)
	assert.Equal(t, expectedCount, len(notifications),
		"方法 %s 的通知数量应为 %d，实际为 %d", method, expectedCount, len(notifications))
}

// GetUnderlyingTransport 从客户端获取底层传输对象
// 注意：这是测试环境的简化实现，无法直接访问客户端的底层传输对象
// 返回 nil 并报告失败
func GetUnderlyingTransport(c *client.Client) (interface{}, bool) {
	// 客户端没有公开访问底层传输的方法
	return nil, false
}

// CreateClient 创建客户端连接
func CreateClient(url string, enableGetSSE bool) (*client.Client, error) {
	// 创建客户端
	c, err := client.NewClient(url, schema.Implementation{
		Name:    "Test-Client",
		Version: "1.0.0",
	}, client.WithGetSSEEnabled(enableGetSSE))
	if err != nil {
		return nil, fmt.Errorf("创建 MCP 客户端失败: %v", err)
	}

	return c, nil
}

// CreateClientWithRequestMode 创建带有请求模式的客户端连接
func CreateClientWithRequestMode(url string, mode string, enableGetSSE bool) (*client.Client, error) {
	// 创建客户端
	c, err := client.NewClient(url, schema.Implementation{
		Name:    "Test-Client",
		Version: "1.0.0",
	}, client.WithGetSSEEnabled(enableGetSSE))
	if err != nil {
		return nil, fmt.Errorf("创建 MCP 客户端失败: %v", err)
	}

	return c, nil
}
