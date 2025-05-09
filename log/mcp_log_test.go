package log_test

import (
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/streamable-mcp/log"
	"github.com/modelcontextprotocol/streamable-mcp/schema"
)

func TestLogLevelConversion(t *testing.T) {
	testCases := []struct {
		internalLevel log.Level
		mcpLevel      schema.LoggingLevel
	}{
		{log.DebugLevel, schema.LoggingLevelDebug},
		{log.InfoLevel, schema.LoggingLevelInfo},
		{log.NoticeLevel, schema.LoggingLevelNotice},
		{log.WarningLevel, schema.LoggingLevelWarning},
		{log.ErrorLevel, schema.LoggingLevelError},
		{log.FatalLevel, schema.LoggingLevelCritical},
	}

	for _, tc := range testCases {
		if result := log.ToMCPLevel(tc.internalLevel); result != tc.mcpLevel {
			t.Errorf("ToMCPLevel(%v) = %v, want %v", tc.internalLevel, result, tc.mcpLevel)
		}
	}

	mcpTestCases := []struct {
		mcpLevel      schema.LoggingLevel
		internalLevel log.Level
	}{
		{schema.LoggingLevelDebug, log.DebugLevel},
		{schema.LoggingLevelInfo, log.InfoLevel},
		{schema.LoggingLevelNotice, log.NoticeLevel},
		{schema.LoggingLevelWarning, log.WarningLevel},
		{schema.LoggingLevelError, log.ErrorLevel},
		{schema.LoggingLevelCritical, log.FatalLevel},
		{schema.LoggingLevelAlert, log.FatalLevel},
		{schema.LoggingLevelEmergency, log.FatalLevel},
	}

	for _, tc := range mcpTestCases {
		if result := log.FromMCPLevel(tc.mcpLevel); result != tc.internalLevel {
			t.Errorf("FromMCPLevel(%v) = %v, want %v", tc.mcpLevel, result, tc.internalLevel)
		}
	}
}

func TestCreateLogNotification(t *testing.T) {
	notification := log.CreateLogNotification(log.InfoLevel, "test message", "test-logger")

	if notification.Method != "notifications/message" {
		t.Errorf("Expected method to be 'notifications/message', got %s", notification.Method)
	}

	if notification.Params.Level != schema.LoggingLevelInfo {
		t.Errorf("Expected level to be Info, got %s", notification.Params.Level)
	}

	if msg, ok := notification.Params.Data.(string); !ok || msg != "test message" {
		t.Errorf("Expected data to be 'test message', got %v", notification.Params.Data)
	}

	if notification.Params.Logger != "test-logger" {
		t.Errorf("Expected logger to be 'test-logger', got %s", notification.Params.Logger)
	}
}

func TestLogNotificationJSON(t *testing.T) {
	original := log.CreateLogNotification(log.ErrorLevel, "error occurred", "system")

	jsonData, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal log notification: %v", err)
	}

	var decoded schema.LoggingMessageNotification
	err = json.Unmarshal(jsonData, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal log notification: %v", err)
	}

	if decoded.Method != original.Method {
		t.Errorf("Method mismatch: expected %s, got %s", original.Method, decoded.Method)
	}

	if decoded.Params.Level != original.Params.Level {
		t.Errorf("Level mismatch: expected %s, got %s", original.Params.Level, decoded.Params.Level)
	}

	if decoded.Params.Logger != original.Params.Logger {
		t.Errorf("Logger mismatch: expected %s, got %s", original.Params.Logger, decoded.Params.Logger)
	}

	originalMsg, _ := original.Params.Data.(string)
	decodedMsg, ok := decoded.Params.Data.(string)
	if !ok || decodedMsg != originalMsg {
		t.Errorf("Data mismatch: expected %v, got %v", original.Params.Data, decoded.Params.Data)
	}
}
