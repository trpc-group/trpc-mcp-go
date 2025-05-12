package log_test

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"trpc.group/trpc-go/trpc-mcp-go/log"
)

// testLogger implements the Logger interface for testing.
type testLogger struct {
	buffer *bytes.Buffer
	level  log.Level
}

// Ensure testLogger implements the Logger interface.
var _ log.Logger = (*testLogger)(nil)

// Create a new test logger.
func newTestLogger() *testLogger {
	return &testLogger{
		buffer: &bytes.Buffer{},
		level:  log.InfoLevel,
	}
}

// Implement all methods of the Logger interface.
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

// Write message to buffer.
func (t *testLogger) write(level string, args ...interface{}) {
	msg := fmt.Sprint(args...)
	t.buffer.WriteString(level + ": " + msg + "\n")
}

// Implement WithFields method.
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

// Implement methods for setting and getting levels.
func (t *testLogger) SetLevel(level log.Level) {
	t.level = level
}

func (t *testLogger) GetLevel() log.Level {
	return t.level
}

// Get buffer content.
func (t *testLogger) String() string {
	return t.buffer.String()
}

// Test NullLogger.
func TestNullLogger(t *testing.T) {
	logger := log.NewNullLogger()

	// These calls should not cause any errors.
	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warning("warning message")
	logger.Error("error message")
	logger.Fatal("fatal message")

	// Test level setting and getting.
	logger.SetLevel(log.WarningLevel)
	if logger.GetLevel() != log.WarningLevel {
		t.Errorf("Expected warning level, got %v", logger.GetLevel())
	}

	// Test WithFields.
	fieldLogger := logger.WithFields(map[string]interface{}{
		"key": "value",
	})

	// Should return an object that implements the Logger interface.
	if _, ok := fieldLogger.(log.Logger); !ok {
		t.Error("WithFields did not return a Logger")
	}
}

// Test log level String method.
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

// Test global log functions.
func TestGlobalLogFunctions(t *testing.T) {
	// Save original logger to restore after test.
	originalLogger := log.GetLogger()
	defer log.SetLogger(originalLogger)

	// Set a test logger.
	testLog := newTestLogger()
	log.SetLogger(testLog)

	// Test global functions.
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

	// Test formatting functions.
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

	// Test global level setting and getting.
	log.SetLevel(log.ErrorLevel)
	if log.GetLevel() != log.ErrorLevel {
		t.Errorf("Global level not set correctly, expected %v, got %v", log.ErrorLevel, log.GetLevel())
	}

	// Test global WithFields.
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

// Test ZapLogger's factory method.
func TestNewZapLogger(t *testing.T) {
	// Test setting log level via environment variable.
	os.Setenv("LOG_LEVEL", "error")
	defer os.Unsetenv("LOG_LEVEL")

	logger := log.NewZapLogger()
	if logger.GetLevel() != log.ErrorLevel {
		t.Errorf("Expected ZapLogger level to be ErrorLevel, got %v", logger.GetLevel())
	}
}
