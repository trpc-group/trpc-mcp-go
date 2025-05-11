package log

import (
	"trpc.group/trpc-go/trpc-mcp-go/mcp"
)

// ToMCPLevel converts internal log level to MCP protocol log level.
func ToMCPLevel(level Level) mcp.LoggingLevel {
	switch level {
	case DebugLevel:
		return mcp.LoggingLevelDebug
	case InfoLevel:
		return mcp.LoggingLevelInfo
	case NoticeLevel:
		return mcp.LoggingLevelNotice
	case WarningLevel:
		return mcp.LoggingLevelWarning
	case ErrorLevel:
		return mcp.LoggingLevelError
	case FatalLevel:
		return mcp.LoggingLevelCritical // Using Critical as Fatal mapping for MCP protocol compatibility
	default:
		return mcp.LoggingLevelInfo
	}
}

// FromMCPLevel converts MCP protocol log level to internal log level.
func FromMCPLevel(level mcp.LoggingLevel) Level {
	switch level {
	case mcp.LoggingLevelDebug:
		return DebugLevel
	case mcp.LoggingLevelInfo:
		return InfoLevel
	case mcp.LoggingLevelNotice:
		return NoticeLevel
	case mcp.LoggingLevelWarning:
		return WarningLevel
	case mcp.LoggingLevelError:
		return ErrorLevel
	case mcp.LoggingLevelCritical, mcp.LoggingLevelAlert, mcp.LoggingLevelEmergency:
		return FatalLevel // All high-level logs map to Fatal
	default:
		return InfoLevel
	}
}

// CreateLogNotification creates an MCP log notification.
func CreateLogNotification(level Level, data interface{}, logger string) mcp.LoggingMessageNotification {
	return mcp.LoggingMessageNotification{
		Method: "notifications/message",
		Params: mcp.LoggingMessageParams{
			Level:  ToMCPLevel(level),
			Data:   data,
			Logger: logger,
		},
	}
}
