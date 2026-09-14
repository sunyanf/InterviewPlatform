package logger

import (
	"log/slog"
	"os"
	"strings"
)

// New 根据配置创建结构化 Logger
func New(level, format string) *slog.Logger {
	var logLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level:       logLevel,
		ReplaceAttr: replaceAttr,
	}

	var handler slog.Handler
	if strings.ToLower(format) == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

// replaceAttr 对敏感字段进行脱敏
func replaceAttr(_ []string, a slog.Attr) slog.Attr {
	switch a.Key {
	case "password", "token", "secret", "authorization":
		return slog.String(a.Key, "***REDACTED***")
	}
	return a
}
