package e2e

import (
	"flag"
	"fmt"
	"os"
	"testing"
	"time"
)

// 全局标志
var (
	// useRealServer 标志用于控制是否使用真实 HTTP 服务器
	useRealServer = flag.Bool("real", false, "使用真实 HTTP 服务器而不是 httptest")

	// serverAddr 指定真实服务器地址
	serverAddr = flag.String("addr", "localhost:3456", "真实服务器地址")

	// verboseLogging 控制详细日志
	verboseLogging = flag.Bool("verbose", false, "启用详细日志")

	// TestGetSSEEnabled 标志用于控制是否启用 GET SSE 测试
	TestGetSSEEnabled = flag.Bool("getsse", true, "启用 GET SSE 测试")
)

// 全局超时设置
const (
	// 默认测试超时
	defaultTestTimeout = 5 * time.Second

	// 长测试超时
	longTestTimeout = 10 * time.Second
)

// TestMain 设置整个测试包的环境
func TestMain(m *testing.M) {
	// 解析命令行标志
	flag.Parse()

	// 设置日志前缀
	if *verboseLogging {
		fmt.Println("启用详细日志模式")
	}

	// 运行测试
	exitCode := m.Run()

	// 退出
	os.Exit(exitCode)
}

// 辅助函数：基于短名称获取完整测试名称
func getTestName(t *testing.T, shortName string) string {
	return fmt.Sprintf("%s/%s", t.Name(), shortName)
}
