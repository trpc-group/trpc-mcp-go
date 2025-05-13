package mcp

import (
	"trpc.group/trpc-go/trpc-mcp-go/internal/log"
)

// Logger defines the logging interface used throughout MCP framework.
type Logger interface {
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Warn(args ...interface{})
	Warnf(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
}

// NewZapLogger returns a Logger interface, hiding zap 细节。
func NewZapLogger() Logger {
	return log.NewZapLogger()
}

var (
	defaultLogger Logger = NewZapLogger()
)

// SetDefaultLogger sets the global default logger.
func SetDefaultLogger(l Logger) {
	defaultLogger = l
}

// GetDefaultLogger returns the global default logger.
func GetDefaultLogger() Logger {
	return defaultLogger
}
