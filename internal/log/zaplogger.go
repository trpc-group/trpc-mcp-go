package log

import "go.uber.org/zap"

// ZapLogger is the default implementation of logger based on zap.logger.
type ZapLogger struct {
	logger *zap.SugaredLogger
}

// Debug logs a debug message.
func (z *ZapLogger) Debug(args ...interface{}) {
	z.logger.Debug(args...)
}

// Debugf logs a formatted debug message.
func (z *ZapLogger) Debugf(format string, args ...interface{}) {
	z.logger.Debugf(format, args...)
}

// Info logs an info message.
func (z *ZapLogger) Info(args ...interface{}) {
	z.logger.Info(args...)
}

// Infof logs a formatted info message.
func (z *ZapLogger) Infof(format string, args ...interface{}) {
	z.logger.Infof(format, args...)
}

// Warn logs a warning message.
func (z *ZapLogger) Warn(args ...interface{}) {
	z.logger.Warn(args...)
}

// Warnf logs a formatted warning message.
func (z *ZapLogger) Warnf(format string, args ...interface{}) {
	z.logger.Warnf(format, args...)
}

// Error logs an error message.
func (z *ZapLogger) Error(args ...interface{}) {
	z.logger.Error(args...)
}

// Errorf logs a formatted error message.
func (z *ZapLogger) Errorf(format string, args ...interface{}) {
	z.logger.Errorf(format, args...)
}

// Fatal logs a fatal message and exits.
func (z *ZapLogger) Fatal(args ...interface{}) {
	z.logger.Fatal(args...)
}

// Fatalf logs a formatted fatal message and exits.
func (z *ZapLogger) Fatalf(format string, args ...interface{}) {
	z.logger.Fatalf(format, args...)
}

// NewZapLogger creates a ZapLogger with default zap config.
func NewZapLogger() *ZapLogger {
	logger, _ := zap.NewProduction()
	return &ZapLogger{logger: logger.Sugar()}
}
