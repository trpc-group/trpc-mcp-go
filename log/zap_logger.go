package log

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ZapLogger is a Zap-based implementation of the logger.
type ZapLogger struct {
	logger *zap.SugaredLogger
	level  Level
}

// Ensure ZapLogger implements the Logger interface.
var _ Logger = (*ZapLogger)(nil)

// NewZapLogger creates a new Zap-based logger instance.
func NewZapLogger() *ZapLogger {
	config := zap.NewProductionEncoderConfig()
	config.EncodeTime = zapcore.ISO8601TimeEncoder
	config.EncodeLevel = zapcore.CapitalColorLevelEncoder

	// Read log level from environment variable, default is InfoLevel.
	level := InfoLevel
	if envLevel := os.Getenv("LOG_LEVEL"); envLevel != "" {
		for l := DebugLevel; l <= FatalLevel; l++ {
			if l.String() == envLevel {
				level = l
				break
			}
		}
	}

	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(config),
		zapcore.AddSync(os.Stdout),
		zapToLevel(level),
	)

	zapLogger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
	return &ZapLogger{
		logger: zapLogger.Sugar(),
		level:  level,
	}
}

// zapToLevel converts our log level to Zap log level.
func zapToLevel(level Level) zapcore.LevelEnabler {
	switch level {
	case DebugLevel:
		return zapcore.DebugLevel
	case InfoLevel:
		return zapcore.InfoLevel
	case NoticeLevel, WarningLevel:
		return zapcore.WarnLevel
	case ErrorLevel:
		return zapcore.ErrorLevel
	case FatalLevel:
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel
	}
}

// Debug implements the Logger interface.
func (z *ZapLogger) Debug(args ...interface{}) {
	z.logger.Debug(args...)
}

func (z *ZapLogger) Debugf(format string, args ...interface{}) {
	z.logger.Debugf(format, args...)
}

func (z *ZapLogger) Info(args ...interface{}) {
	z.logger.Info(args...)
}

func (z *ZapLogger) Infof(format string, args ...interface{}) {
	z.logger.Infof(format, args...)
}

func (z *ZapLogger) Notice(args ...interface{}) {
	prefix := "[NOTICE]"
	newArgs := append([]interface{}{prefix}, args...)
	z.logger.Info(newArgs...)
}

func (z *ZapLogger) Noticef(format string, args ...interface{}) {
	z.logger.Infof("[NOTICE] "+format, args...)
}

func (z *ZapLogger) Warning(args ...interface{}) {
	z.logger.Warn(args...)
}

func (z *ZapLogger) Warningf(format string, args ...interface{}) {
	z.logger.Warnf(format, args...)
}

func (z *ZapLogger) Error(args ...interface{}) {
	z.logger.Error(args...)
}

func (z *ZapLogger) Errorf(format string, args ...interface{}) {
	z.logger.Errorf(format, args...)
}

func (z *ZapLogger) Fatal(args ...interface{}) {
	z.logger.Fatal(args...)
}

func (z *ZapLogger) Fatalf(format string, args ...interface{}) {
	z.logger.Fatalf(format, args...)
}

func (z *ZapLogger) WithFields(fields map[string]interface{}) Logger {
	// Create a new field array.
	args := make([]interface{}, 0, len(fields)*2)
	for k, v := range fields {
		args = append(args, k, v)
	}

	// Create a new ZapLogger.
	newLogger := &ZapLogger{
		logger: z.logger.With(args...),
		level:  z.level,
	}

	return newLogger
}

func (z *ZapLogger) SetLevel(level Level) {
	z.level = level
	// Note: We cannot dynamically modify the level of an already created zap logger.
	// In production, it might be necessary to recreate the logger.
}

func (z *ZapLogger) GetLevel() Level {
	return z.level
}
