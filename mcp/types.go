package mcp

// ClientRequest represents all client requests.
type ClientRequest interface{}

// ClientNotification represents all client notifications.
type ClientNotification interface{}

// ClientResult represents all client results.
type ClientResult interface{}

// ServerRequest represents all server requests.
type ServerRequest interface{}

// ServerNotification represents all server notifications.
type ServerNotification interface{}

// ServerResult represents all server results.
type ServerResult interface{}

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
