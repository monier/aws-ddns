package framework

import (
	"log/slog"
	"os"
)

// NewLogger builds the application logger: structured JSON on stdout, the sink a
// container runtime collects.
func NewLogger(level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}
