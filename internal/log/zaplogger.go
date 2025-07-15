// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package log

import (
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

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

// defaultTimeFormat returns the default time format "2006-01-02 15:04:05.000".
func defaultTimeFormat(t time.Time) []byte {
	t = t.Local()
	year, month, day := t.Date()
	hour, minute, second := t.Clock()
	micros := t.Nanosecond() / 1000

	buf := make([]byte, 23)
	buf[0] = byte((year/1000)%10) + '0'
	buf[1] = byte((year/100)%10) + '0'
	buf[2] = byte((year/10)%10) + '0'
	buf[3] = byte(year%10) + '0'
	buf[4] = '-'
	buf[5] = byte((month)/10) + '0'
	buf[6] = byte((month)%10) + '0'
	buf[7] = '-'
	buf[8] = byte((day)/10) + '0'
	buf[9] = byte((day)%10) + '0'
	buf[10] = ' '
	buf[11] = byte((hour)/10) + '0'
	buf[12] = byte((hour)%10) + '0'
	buf[13] = ':'
	buf[14] = byte((minute)/10) + '0'
	buf[15] = byte((minute)%10) + '0'
	buf[16] = ':'
	buf[17] = byte((second)/10) + '0'
	buf[18] = byte((second)%10) + '0'
	buf[19] = '.'
	buf[20] = byte((micros/100000)%10) + '0'
	buf[21] = byte((micros/10000)%10) + '0'
	buf[22] = byte((micros/1000)%10) + '0'
	return buf
}

// NewTimeEncoder creates a time format encoder.
func NewTimeEncoder() zapcore.TimeEncoder {
	return func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendByteString(defaultTimeFormat(t))
	}
}

// NewZapLogger creates a ZapLogger with trpc-go style zap config.
func NewZapLogger() *ZapLogger {
	// Create encoder config compatible with trpc-go.
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "T",
		LevelKey:       "L",
		NameKey:        "N",
		CallerKey:      "C",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "M",
		StacktraceKey:  "S",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     NewTimeEncoder(),
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// Create console encoder.
	consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)

	// Create core.
	core := zapcore.NewCore(
		consoleEncoder,
		zapcore.Lock(os.Stderr),
		zap.NewAtomicLevelAt(zapcore.InfoLevel),
	)

	// Create logger, add caller information, and set caller skip level to 2.
	logger := zap.New(
		core,
		zap.AddCaller(),
		zap.AddCallerSkip(2),
	)

	return &ZapLogger{logger: logger.Sugar()}
}
