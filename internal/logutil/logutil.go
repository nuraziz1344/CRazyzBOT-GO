package logutil

import (
	"context"
	"log/slog"
	"os"

	"github.com/google/uuid"
)

// contextKey is a private type to avoid context key collisions.
type contextKey string

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
