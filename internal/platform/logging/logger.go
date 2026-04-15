package logging

import (
	"log/slog"
	"os"
	"strings"
)

type LoggerConfig struct {
	Service string
	Env     string

	Level     string // 日志级别
	AddSource bool   // 是否显示日志调用位置
}

func NewLogger(cfg LoggerConfig) *slog.Logger {
	// 解析日志级别，默认为 Info
	level := parseLevel(cfg.Level)

	hopts := &slog.HandlerOptions{
		Level:     level,
		AddSource: cfg.AddSource,
	}

	// 默认附带字段 service 和 env
	attrs := []slog.Attr{
		slog.String("service", defaultString(cfg.Service, "cixing-api")),
		slog.String("env", defaultString(cfg.Env, "local")),
	}

	// 创建 JSON 格式的日志 Handler
	handler := slog.NewJSONHandler(os.Stdout, hopts).WithAttrs(attrs)
	return slog.New(handler)
}

// 解析日志级别，默认为 Info
func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// 取 string，默认值为 def
func defaultString(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}
