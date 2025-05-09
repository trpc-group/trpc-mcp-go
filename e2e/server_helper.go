package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/streamable-mcp/schema"
	"github.com/modelcontextprotocol/streamable-mcp/server"
	"github.com/modelcontextprotocol/streamable-mcp/transport"
)

// ServerOption 定义服务器选项函数
type ServerOption func(*server.Server)

// WithTestTools 选项：注册测试工具
func WithTestTools() ServerOption {
	return func(s *server.Server) {
		// 注册测试工具
		RegisterTestTools(s)
	}
}

// WithCustomPathPrefix 选项：设置自定义路径前缀
func WithCustomPathPrefix(prefix string) ServerOption {
	return func(s *server.Server) {
		// 在实例化时已应用，这里只是占位
	}
}

// WithServerOptions 选项：设置多个服务器选项
func WithServerOptions(opts ...server.ServerOption) ServerOption {
	return func(s *server.Server) {
		for _, opt := range opts {
			opt(s)
		}
	}
}

// StartTestServer 启动测试服务器，返回其 URL 和清理函数
func StartTestServer(t *testing.T, opts ...ServerOption) (string, func()) {
	t.Helper()

	// 创建基本的服务器配置
	pathPrefix := "/mcp"

	// 实例化服务器
	s := server.NewServer("", schema.Implementation{
		Name:    "E2E-Test-Server",
		Version: "1.0.0",
	}, server.WithPathPrefix(pathPrefix))

	// 应用选项
	for _, opt := range opts {
		opt(s)
	}

	// 创建 HTTP 测试服务器
	httpServer := httptest.NewServer(s.HTTPHandler())
	serverURL := httpServer.URL + pathPrefix

	t.Logf("启动测试服务器 URL: %s", serverURL)

	// 返回清理函数
	cleanup := func() {
		t.Log("关闭测试服务器")
		httpServer.Close()
	}

	return serverURL, cleanup
}

// StartSSETestServer 启动支持 SSE 模式的测试服务器，返回其 URL 和清理函数
func StartSSETestServer(t *testing.T, opts ...ServerOption) (string, func()) {
	t.Helper()

	// 创建基本的服务器配置
	pathPrefix := "/mcp"

	// 创建会话管理器
	sessionManager := transport.NewSessionManager(3600)

	// 实例化服务器并启用 SSE
	s := server.NewServer("", schema.Implementation{
		Name:    "E2E-SSE-Test-Server",
		Version: "1.0.0",
	},
		server.WithPathPrefix(pathPrefix),
		server.WithSessionManager(sessionManager),
		server.WithSSEEnabled(true),           // 启用 SSE
		server.WithGetSSEEnabled(true),        // 启用 GET SSE
		server.WithDefaultResponseMode("sse"), // 设置默认响应模式为 SSE
	)

	// 应用选项
	for _, opt := range opts {
		opt(s)
	}

	// 创建 HTTP 处理器
	httpHandler := transport.NewHTTPServerHandler(
		s.MCPHandler(),
		transport.WithSessionManager(sessionManager),
		transport.WithServerSSEEnabled(true),
		transport.WithGetSSEEnabled(true),
		transport.WithServerDefaultResponseMode("sse"),
	)

	// 创建自定义的 Mux 来处理测试路由
	mux := http.NewServeMux()

	// 注册 MCP 路径
	mux.Handle(pathPrefix, httpHandler)

	// 注册测试通知路径
	mux.HandleFunc(pathPrefix+"/test/notify", func(w http.ResponseWriter, r *http.Request) {
		handleTestNotify(w, r, httpHandler)
	})

	// 创建 HTTP 测试服务器
	httpServer := httptest.NewServer(mux)
	serverURL := httpServer.URL + pathPrefix

	t.Logf("启动 SSE 测试服务器 URL: %s", serverURL)

	// 返回清理函数
	cleanup := func() {
		t.Log("关闭 SSE 测试服务器")
		httpServer.Close()
	}

	return serverURL, cleanup
}

// handleTestNotify 处理发送通知的测试端点
func handleTestNotify(w http.ResponseWriter, r *http.Request, httpHandler *transport.HTTPServerHandler) {
	// 获取目标会话 ID
	sessionID := r.URL.Query().Get("sessionId")
	if sessionID == "" {
		http.Error(w, "缺少 sessionId 参数", http.StatusBadRequest)
		return
	}

	// 解析通知内容
	var notification schema.Notification
	if err := json.NewDecoder(r.Body).Decode(&notification); err != nil {
		http.Error(w, fmt.Sprintf("无效的通知格式: %v", err), http.StatusBadRequest)
		return
	}

	// 直接使用 HTTPServerHandler 发送通知
	if err := httpHandler.SendNotification(sessionID, &notification); err != nil {
		http.Error(w, fmt.Sprintf("发送通知失败: %v", err), http.StatusInternalServerError)
		return
	}

	// 返回成功
	w.WriteHeader(http.StatusAccepted)
}

// StartLocalTestServer 启动在实际端口上的服务器
// 适用于需要真实网络通信的测试
func StartLocalTestServer(t *testing.T, addr string, opts ...ServerOption) (*server.Server, func()) {
	t.Helper()

	// 创建服务器
	s := server.NewServer(addr, schema.Implementation{
		Name:    "E2E-Test-Server",
		Version: "1.0.0",
	}, server.WithPathPrefix("/mcp"))

	// 应用选项
	for _, opt := range opts {
		opt(s)
	}

	// 启动服务器
	go func() {
		if err := s.Start(); err != nil {
			log.Printf("服务器关闭: %v", err)
		}
	}()

	t.Logf("启动本地测试服务器 地址: %s", addr)

	// 返回清理函数
	cleanup := func() {
		t.Log("关闭本地测试服务器")
		// 服务器没有提供 Close 方法，但 Start 会在服务器关闭时返回
		// 此处我们不需要显式关闭，因为 httptest.Server 的 Close 会处理
	}

	return s, cleanup
}

// ServerNotifier 测试用的服务器通知发送器
type ServerNotifier interface {
	// 发送通知
	SendNotification(sessionID string, notification *schema.Notification) error
}

// GetServerNotifier 获取测试服务器的通知发送器
func GetServerNotifier(t *testing.T, serverURL string) ServerNotifier {
	t.Helper()

	// 解析 URL 并创建一个指向同一服务器的新客户端
	// 这个客户端仅用于调用通知 API，不会建立真正的连接
	return NewTestServerNotifier(t, serverURL)
}

// TestServerNotifier 实现 ServerNotifier 接口，用于测试
type TestServerNotifier struct {
	t         *testing.T
	serverURL string
	client    *http.Client
}

// NewTestServerNotifier 创建新的测试服务器通知发送器
func NewTestServerNotifier(t *testing.T, serverURL string) *TestServerNotifier {
	return &TestServerNotifier{
		t:         t,
		serverURL: serverURL,
		client:    &http.Client{},
	}
}

// SendNotification 发送通知
func (n *TestServerNotifier) SendNotification(sessionID string, notification *schema.Notification) error {
	n.t.Helper()

	// 创建 POST 请求向服务器发送通知
	notificationBytes, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("序列化通知失败: %v", err)
	}

	// 构建请求 URL
	reqURL := fmt.Sprintf("%s/test/notify?sessionId=%s", n.serverURL, sessionID)

	// 创建请求
	req, err := http.NewRequest(http.MethodPost, reqURL, bytes.NewReader(notificationBytes))
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 检查状态码
	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("服务器返回非200状态码: %d, 响应体: %s", resp.StatusCode, string(body))
	}

	return nil
}
