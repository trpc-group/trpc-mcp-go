package log_test

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/streamable-mcp/log"
)

// testLogger 实现 Logger 接口用于测试
type testLogger struct {
	buffer *bytes.Buffer
	level  log.Level
}

// 确保 testLogger 实现了 Logger 接口
var _ log.Logger = (*testLogger)(nil)

// 创建新的测试日志器
func newTestLogger() *testLogger {
	return &testLogger{
		buffer: &bytes.Buffer{},
		level:  log.InfoLevel,
	}
}

// 实现 Logger 接口的所有方法
func (t *testLogger) Debug(args ...interface{}) { t.write("DEBUG", args...) }
func (t *testLogger) Debugf(format string, args ...interface{}) {
	t.write("DEBUG", fmt.Sprintf(format, args...))
}
func (t *testLogger) Info(args ...interface{}) { t.write("INFO", args...) }
func (t *testLogger) Infof(format string, args ...interface{}) {
	t.write("INFO", fmt.Sprintf(format, args...))
}
func (t *testLogger) Notice(args ...interface{}) { t.write("NOTICE", args...) }
func (t *testLogger) Noticef(format string, args ...interface{}) {
	t.write("NOTICE", fmt.Sprintf(format, args...))
}
func (t *testLogger) Warning(args ...interface{}) { t.write("WARNING", args...) }
func (t *testLogger) Warningf(format string, args ...interface{}) {
	t.write("WARNING", fmt.Sprintf(format, args...))
}
func (t *testLogger) Error(args ...interface{}) { t.write("ERROR", args...) }
func (t *testLogger) Errorf(format string, args ...interface{}) {
	t.write("ERROR", fmt.Sprintf(format, args...))
}
func (t *testLogger) Fatal(args ...interface{}) { t.write("FATAL", args...) }
func (t *testLogger) Fatalf(format string, args ...interface{}) {
	t.write("FATAL", fmt.Sprintf(format, args...))
}

// 将消息写入缓冲区
func (t *testLogger) write(level string, args ...interface{}) {
	msg := fmt.Sprint(args...)
	t.buffer.WriteString(level + ": " + msg + "\n")
}

// 实现 WithFields 方法
func (t *testLogger) WithFields(fields map[string]interface{}) log.Logger {
	newLogger := &testLogger{
		buffer: t.buffer,
		level:  t.level,
	}

	fieldStr := ""
	for k, v := range fields {
		fieldStr += fmt.Sprintf("%s=%v ", k, v)
	}

	if fieldStr != "" {
		newLogger.buffer.WriteString("FIELDS: " + fieldStr + "\n")
	}

	return newLogger
}

// 实现设置和获取级别的方法
func (t *testLogger) SetLevel(level log.Level) {
	t.level = level
}

func (t *testLogger) GetLevel() log.Level {
	return t.level
}

// 获取缓冲区内容
func (t *testLogger) String() string {
	return t.buffer.String()
}

// 测试 NullLogger
func TestNullLogger(t *testing.T) {
	logger := log.NewNullLogger()

	// 这些调用不应该导致任何错误
	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warning("warning message")
	logger.Error("error message")
	logger.Fatal("fatal message")

	// 测试级别设置和获取
	logger.SetLevel(log.WarningLevel)
	if logger.GetLevel() != log.WarningLevel {
		t.Errorf("Expected warning level, got %v", logger.GetLevel())
	}

	// 测试 WithFields
	fieldLogger := logger.WithFields(map[string]interface{}{
		"key": "value",
	})

	// 应该返回实现 Logger 接口的对象
	if _, ok := fieldLogger.(log.Logger); !ok {
		t.Error("WithFields did not return a Logger")
	}
}

// 测试日志级别 String 方法
func TestLevelString(t *testing.T) {
	testCases := []struct {
		level    log.Level
		expected string
	}{
		{log.DebugLevel, "debug"},
		{log.InfoLevel, "info"},
		{log.NoticeLevel, "notice"},
		{log.WarningLevel, "warning"},
		{log.ErrorLevel, "error"},
		{log.FatalLevel, "fatal"},
		{log.Level(999), "unknown"},
	}

	for _, tc := range testCases {
		if tc.level.String() != tc.expected {
			t.Errorf("Expected %s for level %d, got %s", tc.expected, tc.level, tc.level.String())
		}
	}
}

// 测试全局日志函数
func TestGlobalLogFunctions(t *testing.T) {
	// 保存原始 logger 以便测试后恢复
	originalLogger := log.GetLogger()
	defer log.SetLogger(originalLogger)

	// 设置一个测试 logger
	testLog := newTestLogger()
	log.SetLogger(testLog)

	// 测试全局函数
	log.Debug("test debug message")
	log.Info("test info message")
	log.Warning("test warning message")
	log.Error("test error message")
	log.Notice("test notice message")
	log.Fatal("test fatal message")

	output := testLog.String()

	if !strings.Contains(output, "DEBUG: test debug message") {
		t.Error("Global Debug function failed")
	}
	if !strings.Contains(output, "INFO: test info message") {
		t.Error("Global Info function failed")
	}
	if !strings.Contains(output, "WARNING: test warning message") {
		t.Error("Global Warning function failed")
	}
	if !strings.Contains(output, "ERROR: test error message") {
		t.Error("Global Error function failed")
	}
	if !strings.Contains(output, "NOTICE: test notice message") {
		t.Error("Global Notice function failed")
	}
	if !strings.Contains(output, "FATAL: test fatal message") {
		t.Error("Global Fatal function failed")
	}

	// 测试格式化函数
	log.Debugf("formatted %s", "debug")
	log.Infof("formatted %s", "info")
	log.Warningf("formatted %s", "warning")
	log.Errorf("formatted %s", "error")
	log.Fatalf("formatted %s", "fatal")

	output = testLog.String()
	if !strings.Contains(output, "DEBUG: formatted debug") {
		t.Error("Global Debugf function failed")
	}
	if !strings.Contains(output, "INFO: formatted info") {
		t.Error("Global Infof function failed")
	}
	if !strings.Contains(output, "WARNING: formatted warning") {
		t.Error("Global Warningf function failed")
	}
	if !strings.Contains(output, "ERROR: formatted error") {
		t.Error("Global Errorf function failed")
	}
	if !strings.Contains(output, "FATAL: formatted fatal") {
		t.Error("Global Fatalf function failed")
	}

	// 测试全局级别设置和获取
	log.SetLevel(log.ErrorLevel)
	if log.GetLevel() != log.ErrorLevel {
		t.Errorf("Global level not set correctly, expected %v, got %v", log.ErrorLevel, log.GetLevel())
	}

	// 测试全局 WithFields
	fieldLogger := log.WithFields(map[string]interface{}{
		"request_id": "123",
		"user":       "admin",
	})

	fieldLogger.Info("test with fields")

	if !strings.Contains(testLog.String(), "FIELDS: ") ||
		!strings.Contains(testLog.String(), "request_id=123") ||
		!strings.Contains(testLog.String(), "user=admin") {
		t.Error("Global WithFields function failed")
	}
}

// 测试 ZapLogger 的工厂方法
func TestNewZapLogger(t *testing.T) {
	// 测试通过环境变量设置日志级别
	os.Setenv("LOG_LEVEL", "error")
	defer os.Unsetenv("LOG_LEVEL")

	logger := log.NewZapLogger()
	if logger.GetLevel() != log.ErrorLevel {
		t.Errorf("Expected ZapLogger level to be ErrorLevel, got %v", logger.GetLevel())
	}
}
