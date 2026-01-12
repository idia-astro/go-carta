package helpers

import (
	"log/slog"
	"os"
)

// NewLogger New creates a new Logger with structured logging using slog
// logLevel can be "debug", "info", "warn", or "error"
func NewLogger(serviceName, logLevel string) *slog.Logger {
	var level slog.Level
	err := level.UnmarshalText([]byte(logLevel))
	if err != nil {
		level = slog.LevelInfo
		return nil
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})

	logger := slog.New(handler).With("service", serviceName)
	return logger
}
