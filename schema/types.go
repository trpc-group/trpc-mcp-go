package schema

// Role represents the sender or recipient of a message
type Role string

const (
	// RoleUser represents the user role
	RoleUser Role = "user"

	// RoleAssistant represents the assistant role
	RoleAssistant Role = "assistant"
)

// Annotations represents optional client annotations
type Annotations struct {
	// Audience describes the target user for this object or data
	Audience []Role `json:"audience,omitempty"`

	// Priority describes the importance of this data to server operation
	// 1 means "most important", the data is actually required
	// 0 means "least important", the data is completely optional
	Priority float64 `json:"priority,omitempty"`
}

// LoggingLevel represents the severity of a log message
//
// These map to syslog message severities as defined in RFC-5424:
// https://datatracker.ietf.org/doc/html/rfc5424#section-6.2.1
type LoggingLevel string

// Logging level constants
const (
	LoggingLevelDebug     LoggingLevel = "debug"
	LoggingLevelInfo      LoggingLevel = "info"
	LoggingLevelNotice    LoggingLevel = "notice"
	LoggingLevelWarning   LoggingLevel = "warning"
	LoggingLevelError     LoggingLevel = "error"
	LoggingLevelCritical  LoggingLevel = "critical"
	LoggingLevelAlert     LoggingLevel = "alert"
	LoggingLevelEmergency LoggingLevel = "emergency"
)

// LoggingMessageParams defines parameters for a log message notification
type LoggingMessageParams struct {
	// Severity of this log message
	Level LoggingLevel `json:"level"`

	// Data to log, such as a string message or object
	// Any JSON-serializable type is allowed here
	Data interface{} `json:"data"`

	// Optional name of the logger that emitted this message
	Logger string `json:"logger,omitempty"`
}

// LoggingMessageNotification log message notification from server to client
type LoggingMessageNotification struct {
	Method string               `json:"method"`
	Params LoggingMessageParams `json:"params"`
}
