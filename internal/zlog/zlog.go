// Package zlog 是对标准库 slog 的极薄封装，提供全局 logger。
package zlog

import (
	"log/slog"
	"os"
	"strings"
)

var logger = slog.New(slog.NewTextHandler(os.Stdout, nil))

// Init 初始化全局 logger，file 为空则输出到 stdout。
func Init(level, file string) error {
	out := os.Stdout
	if file != "" {
		f, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return err
		}
		out = f
	}
	logger = slog.New(slog.NewTextHandler(out, &slog.HandlerOptions{Level: parseLevel(level)}))
	slog.SetDefault(logger)
	return nil
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func Info(msg string, args ...any)  { logger.Info(msg, args...) }
func Warn(msg string, args ...any)  { logger.Warn(msg, args...) }
func Error(msg string, args ...any) { logger.Error(msg, args...) }
func Debug(msg string, args ...any) { logger.Debug(msg, args...) }
