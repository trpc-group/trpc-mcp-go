// Package log provides logging functionality for streamable-mcp.
package log

import "sync"

var (
	// global is the default global logger instance.
	global Logger
	// mutex protects concurrent access to the global logger instance.
	mutex sync.RWMutex

	initOnce sync.Once
)

func init() {
	initOnce.Do(func() {
		// Create and set default logger.
		SetLogger(NewZapLogger())
	})
}

// Logger defines the core interface for the logging system.
type Logger interface {
	// Basic logging methods.
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Notice(args ...interface{})
	Noticef(format string, args ...interface{})
	Warning(args ...interface{})
	Warningf(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})

	// Structured logging with fields.
	WithFields(fields map[string]interface{}) Logger

	// Control.
	SetLevel(level Level)
	GetLevel() Level
}

// Level represents logging level.
type Level int

// Logging level constants, from low to high.
const (
	DebugLevel Level = iota
	InfoLevel
	NoticeLevel
	WarningLevel
	ErrorLevel
	FatalLevel
)

// String returns the string representation of log level.
func (l Level) String() string {
	switch l {
	case DebugLevel:
		return "debug"
	case InfoLevel:
		return "info"
	case NoticeLevel:
		return "notice"
	case WarningLevel:
		return "warning"
	case ErrorLevel:
		return "error"
	case FatalLevel:
		return "fatal"
	default:
		return "unknown"
	}
}

// SetLogger sets the global logger instance.
func SetLogger(logger Logger) {
	mutex.Lock()
	defer mutex.Unlock()
	global = logger
}

// GetLogger returns the global logger instance.
func GetLogger() Logger {
	mutex.RLock()
	defer mutex.RUnlock()
	return global
}

// Global convenience methods.
func Debug(args ...interface{}) {
	GetLogger().Debug(args...)
}

func Debugf(format string, args ...interface{}) {
	GetLogger().Debugf(format, args...)
}

func Info(args ...interface{}) {
	GetLogger().Info(args...)
}

func Infof(format string, args ...interface{}) {
	GetLogger().Infof(format, args...)
}

func Notice(args ...interface{}) {
	GetLogger().Notice(args...)
}

func Noticef(format string, args ...interface{}) {
	GetLogger().Noticef(format, args...)
}

func Warning(args ...interface{}) {
	GetLogger().Warning(args...)
}

func Warningf(format string, args ...interface{}) {
	GetLogger().Warningf(format, args...)
}

func Error(args ...interface{}) {
	GetLogger().Error(args...)
}

func Errorf(format string, args ...interface{}) {
	GetLogger().Errorf(format, args...)
}

func Fatal(args ...interface{}) {
	GetLogger().Fatal(args...)
}

func Fatalf(format string, args ...interface{}) {
	GetLogger().Fatalf(format, args...)
}

func WithFields(fields map[string]interface{}) Logger {
	return GetLogger().WithFields(fields)
}

func SetLevel(level Level) {
	GetLogger().SetLevel(level)
}

func GetLevel() Level {
	return GetLogger().GetLevel()
}

// NullLogger is a logger that doesn't perform any operations, used for testing.
type NullLogger struct {
	level Level
}

// Ensure NullLogger implements Logger interface.
var _ Logger = (*NullLogger)(nil)

// NewNullLogger creates a new null logger.
func NewNullLogger() *NullLogger {
	return &NullLogger{
		level: InfoLevel,
	}
}

// Debug implements Logger interface.
func (n *NullLogger) Debug(args ...interface{}) {}

func (n *NullLogger) Debugf(format string, args ...interface{}) {}

func (n *NullLogger) Info(args ...interface{}) {}

func (n *NullLogger) Infof(format string, args ...interface{}) {}

func (n *NullLogger) Notice(args ...interface{}) {}

func (n *NullLogger) Noticef(format string, args ...interface{}) {}

func (n *NullLogger) Warning(args ...interface{}) {}

func (n *NullLogger) Warningf(format string, args ...interface{}) {}

func (n *NullLogger) Error(args ...interface{}) {}

func (n *NullLogger) Errorf(format string, args ...interface{}) {}

func (n *NullLogger) Fatal(args ...interface{}) {}

func (n *NullLogger) Fatalf(format string, args ...interface{}) {}

func (n *NullLogger) WithFields(fields map[string]interface{}) Logger {
	return n
}

func (n *NullLogger) SetLevel(level Level) {
	n.level = level
}

func (n *NullLogger) GetLevel() Level {
	return n.level
}
