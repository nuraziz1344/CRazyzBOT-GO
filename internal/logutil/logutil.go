package logutil

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// contextKey is a private type to avoid context key collisions.
type contextKey string

var projectRoot string

func init() {
	_, file, _, ok := runtime.Caller(0)
	if ok {
		// We are at internal/logutil/logutil.go — go up 3 dirs to reach project root
		projectRoot = filepath.Dir(filepath.Dir(filepath.Dir(file)))
	}
}

const (
	// LogIDKey is the context key for the log correlation ID.
	LogIDKey contextKey = "logID"
	// LoggerKey is the context key for the request-scoped slog.Logger.
	LoggerKey contextKey = "logger"
)

// InitJSONLogger configures the global slog logger to output structured JSON to stdout.
func InitJSONLogger(level string) {
	var l slog.Level
	switch level {
	case "debug":
		l = slog.LevelDebug
	case "info":
		l = slog.LevelInfo
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelWarn
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: l,
	})
	slog.SetDefault(slog.New(handler))
}

// NewID generates a new UUID v4 string for use as a log correlation ID.
func NewID() string {
	return uuid.New().String()
}

// WithLogID injects a logID and a request-scoped logger into the context.
// Returns the enriched context.
func WithLogID(ctx context.Context) context.Context {
	logID := NewID()
	ctx = context.WithValue(ctx, LogIDKey, logID)
	logger := slog.Default().With("logID", logID)
	ctx = context.WithValue(ctx, LoggerKey, logger)
	return ctx
}

// GetLogID extracts the logID from context. Returns empty string if not set.
func GetLogID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id, ok := ctx.Value(LogIDKey).(string)
	if !ok {
		return ""
	}
	return id
}

// sourceAttr returns the "source" log attribute with the caller's file:line.
// skip is the number of frames up the stack (0 = caller of sourceAttr).
func sourceAttr(skip int) (string, string) {
	_, file, line, ok := runtime.Caller(skip + 1)
	if !ok {
		return "source", "unknown"
	}
	rel := strings.TrimPrefix(file, projectRoot+string(filepath.Separator))
	return "source", rel + ":" + strconv.Itoa(line)
}

// LoggerFromContext extracts the request-scoped logger from context.
// Falls back to the default slog logger if none is set.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return slog.Default()
	}
	logger, ok := ctx.Value(LoggerKey).(*slog.Logger)
	if !ok || logger == nil {
		return slog.Default()
	}
	return logger
}

// Debug logs at debug level with context-scoped logger.
func Debug(ctx context.Context, msg string, args ...any) {
	k, v := sourceAttr(1)
	args = append([]any{k, v}, args...)
	LoggerFromContext(ctx).Debug(msg, args...)
}

// Info logs at info level with context-scoped logger.
func Info(ctx context.Context, msg string, args ...any) {
	k, v := sourceAttr(1)
	args = append([]any{k, v}, args...)
	LoggerFromContext(ctx).Info(msg, args...)
}

// Warn logs at warn level with context-scoped logger.
func Warn(ctx context.Context, msg string, args ...any) {
	k, v := sourceAttr(1)
	args = append([]any{k, v}, args...)
	LoggerFromContext(ctx).Warn(msg, args...)
}

// Error logs at error level with context-scoped logger.
func Error(ctx context.Context, msg string, args ...any) {
	k, v := sourceAttr(1)
	args = append([]any{k, v}, args...)
	LoggerFromContext(ctx).Error(msg, args...)
}

// UserError returns a user-friendly error message that references the logID.
// Use this for user-facing responses instead of leaking err.Error() details.
func UserError(ctx context.Context) string {
	return "An error occurred. Reference: " + GetLogID(ctx)
}
