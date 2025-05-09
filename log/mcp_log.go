package log

import (
	"github.com/modelcontextprotocol/streamable-mcp/schema"
)

// ToMCPLevel converts internal log level to MCP protocol log level.
func ToMCPLevel(level Level) schema.LoggingLevel {
	switch level {
	case DebugLevel:
		return schema.LoggingLevelDebug
	case InfoLevel:
		return schema.LoggingLevelInfo
	case NoticeLevel:
		return schema.LoggingLevelNotice
	case WarningLevel:
		return schema.LoggingLevelWarning
	case ErrorLevel:
		return schema.LoggingLevelError
	case FatalLevel:
		return schema.LoggingLevelCritical // Using Critical as Fatal mapping for MCP protocol compatibility
	default:
		return schema.LoggingLevelInfo
	}
}

// FromMCPLevel converts MCP protocol log level to internal log level.
func FromMCPLevel(level schema.LoggingLevel) Level {
	switch level {
	case schema.LoggingLevelDebug:
		return DebugLevel
	case schema.LoggingLevelInfo:
		return InfoLevel
	case schema.LoggingLevelNotice:
		return NoticeLevel
	case schema.LoggingLevelWarning:
		return WarningLevel
	case schema.LoggingLevelError:
		return ErrorLevel
	case schema.LoggingLevelCritical, schema.LoggingLevelAlert, schema.LoggingLevelEmergency:
		return FatalLevel // All high-level logs map to Fatal
	default:
		return InfoLevel
	}
}

// CreateLogNotification creates an MCP log notification.
func CreateLogNotification(level Level, data interface{}, logger string) schema.LoggingMessageNotification {
	return schema.LoggingMessageNotification{
		Method: "notifications/message",
		Params: schema.LoggingMessageParams{
			Level:  ToMCPLevel(level),
			Data:   data,
			Logger: logger,
		},
	}
}
