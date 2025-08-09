package middlewares

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorYellow = "\033[33m"
	ColorGreen  = "\033[32m"
	ColorBlue   = "\033[34m"
	ColorCyan   = "\033[36m"
	ColorWhite  = "\033[37m"
	ColorGray   = "\033[90m"
)

func shouldUseColor() bool {
	if os.Getenv("CLICOLOR") == "0" {
		return false
	}
	if os.Getenv("CLICOLOR_FORCE") == "1" || os.Getenv("FORCE_COLOR") == "1" {
		return true
	}
	if os.Getenv("CLICOLOR") == "1" || os.Getenv("COLOR") == "1" || os.Getenv("COLOR") == "true" {
		return isTerminal()
	}
	if colorterm := os.Getenv("COLORTERM"); colorterm != "" {
		return isTerminal()
	}
	return false
}

func isTerminal() bool {
	fileInfo, _ := os.Stdout.Stat()
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

type Fields []interface{};

func formatFields(useColor bool, levelColor string, fields ...any) string {
	if len(fields) == 0 {
		return ""
	}

	var result strings.Builder
	result.WriteString("\n")

	for i := 0; i < len(fields); i += 2 {
		if i+1 >= len(fields) {
			break
		}

		key := fmt.Sprintf("%v", fields[i])
		value := fields[i+1]

		if useColor {
			result.WriteString(fmt.Sprintf("  %s%s: %s", ColorGray, key, ColorReset))
		} else {
			result.WriteString(fmt.Sprintf("  %s: ", key))
		}

		switch v := value.(type) {
		case map[string]interface{}:
			result.WriteString("{\n")
			for k, val := range v {
				if useColor {
					result.WriteString(fmt.Sprintf("    %s%s: %s%v%s\n", ColorGray, k, levelColor, val, ColorReset))
				} else {
					result.WriteString(fmt.Sprintf("    %s: %v\n", k, val))
				}
			}
			result.WriteString("  }")
		default:
			if useColor {
				result.WriteString(fmt.Sprintf("%s%v%s", levelColor, v, ColorReset))
			} else {
				result.WriteString(fmt.Sprintf("%v", v))
			}
		}

		result.WriteString("\n")
	}

	return result.String()
}

func getLevelColor(useColor bool, level Level) string {
	if !useColor {
		return ""
	}
	switch level {
	case LevelError:
		return ColorRed
	case LevelWarn:
		return ColorYellow
	case LevelInfo:
		return ColorGreen
	case LevelDebug:
		return ColorBlue
	default:
		return ColorWhite
	}
}

func logWithFormat(logger mcp.Logger, ctx context.Context, level Level, useColor bool, msg string, fields ...any) {
	levelColor := getLevelColor(useColor, level)
	var formattedMsg strings.Builder
	if useColor {
		formattedMsg.WriteString(fmt.Sprintf(" %s[%s]%s", levelColor, level.String(), ColorReset))
		formattedMsg.WriteString(fmt.Sprintf(" %s%s%s", levelColor, msg, ColorReset))
	} else {
		formattedMsg.WriteString(fmt.Sprintf(" [%s]", level.String()))
		formattedMsg.WriteString(fmt.Sprintf(" %s", msg))
	}
	formattedMsg.WriteString(formatFields(useColor, levelColor, fields...))
	switch level {
	case LevelDebug:
		logger.Debug(formattedMsg.String())
	case LevelInfo:
		logger.Info(formattedMsg.String())
	case LevelWarn:
		logger.Warn(formattedMsg.String())
	case LevelError:
		logger.Error(formattedMsg.String())
	case LevelFatal:
		logger.Fatal(formattedMsg.String())
	default:
		logger.Info(formattedMsg.String()) // Default to Info for unknown levels
	}
}

// options structure preserve all configurable options
type options struct {
	shouldLog func(level Level, duration time.Duration, err error) bool

	logPayload bool

	fieldsFromCtx func(ctx context.Context) Fields

	useColor bool
}

// Option is a func to change options struct
type Option func(*options)

type Level int

const (
	LevelDebug Level = -4
	LevelInfo  Level = 0
	LevelWarn  Level = 4
	LevelError Level = 8
	LevelFatal Level = 12
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"

	}
}

// Enabled returns true if the level is enabled for logging.
// This allows for level-based filtering similar to slog.
func (l Level) Enabled(level Level) bool {
	return l >= level
}

// WithShouldLog setting a function to determine if a request should be logged based on level, duration, and error presence.
func WithShouldLog(f func(level Level, duration time.Duration, err error) bool) Option {
	return func(o *options) {
		o.shouldLog = f
	}
}

// WithPayloadLogging enables or disables logging of request and response payloads.
func WithPayloadLogging(enabled bool) Option {
	return func(o *options) {
		o.logPayload = enabled
	}
}

// WithFieldsFromContext allows adding custom fields to logs extracted from the context.
func WithFieldsFromContext(f func(ctx context.Context) Fields) Option {
	return func(o *options) {
		o.fieldsFromCtx = f
	}
}

// WithColor enables or disables colored log output based on the provided boolean and terminal capabilities.
func WithColor(enabled bool) Option {
	return func(o *options) {
		o.useColor = enabled && shouldUseColor()
	}
}

// defaultShouldLog only logs errors by default
var defaultShouldLog = func(level Level, duration time.Duration, err error) bool {
	return level >= LevelError
}

func NewLoggingMiddleware(logger mcp.Logger, opts ...Option) mcp.MiddlewareFunc {
	// 初始化默认配置
	o := &options{
		shouldLog:  defaultShouldLog,
		logPayload: false,
		useColor:   false, // disable color by default
	}

	for _, opt := range opts {
		opt(o)
	}

	return func(ctx context.Context, req *mcp.JSONRPCRequest, session mcp.Session, next mcp.HandleFunc) (mcp.JSONRPCMessage, error) {
		start := time.Now()
		//stage1 start log
		startFields := Fields{
			"event", "request_started",
			"system", "mcp",
			"span.kind", "server",
			"method", req.Method,
			"start_time", start.Format(time.RFC3339),
		}

		// Add session_id if session is not nil
		if session != nil {
			startFields = append(startFields, "session_id", session.GetID())
		}

		if o.fieldsFromCtx != nil {
			startFields = append(startFields, o.fieldsFromCtx(ctx)...)
		}

		if o.logPayload {
			startFields = append(startFields, "request", map[string]interface{}{
				"params": req.Params,
			})
		}
		if o.shouldLog(LevelInfo, 0, nil) {
			logWithFormat(logger, ctx, LevelInfo, o.useColor, "Request started", startFields...)
		}
		resp, err := next(ctx, req, session)
		duration := time.Since(start)

		// check if there is an error (either traditional error or JSON-RPC error)
		var hasError bool
		var errorMessage string
		var errorType string

		if err != nil {
			// traditional error
			hasError = true
			errorMessage = err.Error()
			errorType = fmt.Sprintf("%T", err)
		} else if resp != nil {
			// check for JSON-RPC error in the response
			if errorResp, ok := resp.(*mcp.JSONRPCError); ok {
				hasError = true
				errorMessage = errorResp.Error.Message
				errorType = "JSONRPCErrorResponse"
				if errorResp.Error.Data != nil {
					errorMessage += fmt.Sprintf(" (Data: %v)", errorResp.Error.Data)
				}
			}
		}

		// determine if we should log based on error presence and custom logic
		shouldLogError := hasError && o.shouldLog(LevelError, duration, err)
		shouldLogSuccess := !hasError && o.shouldLog(LevelInfo, duration, err)

		if !shouldLogError && !shouldLogSuccess {
			return resp, err
		}

		// stage2 : request reuslt log
		resultFields := []any{
			"event", "request_completed",
			"method", req.Method,
			"duration_ms", duration.Milliseconds(),
		}

		if o.logPayload && resp != nil {
			if jsonResp, ok := resp.(*mcp.JSONRPCResponse); ok {
				resultFields = append(resultFields, "response", map[string]interface{}{
					"result": jsonResp.Result,
				})
			} else if errorResp, ok := resp.(*mcp.JSONRPCError); ok {
				resultFields = append(resultFields, "response", map[string]interface{}{
					"error": errorResp.Error,
				})
			}
		}

		if hasError {
			// stage3: error log
			errorFields := append(resultFields,
				"event", "request_failed",
				"error", map[string]interface{}{
					"message": errorMessage,
					"type":    errorType,
				},
			)
			logWithFormat(logger, ctx, LevelError, o.useColor, "Request failed", errorFields...)
		} else {
			// stage4: finish log
			logWithFormat(logger, ctx, LevelInfo, o.useColor, "Request completed", resultFields...)
		}

		return resp, err
	}

}
