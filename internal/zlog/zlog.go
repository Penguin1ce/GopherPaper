// Package zlog 是对标准库 slog 的极薄封装，提供全局 logger。
package zlog

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

var logger = slog.New(slog.NewTextHandler(os.Stdout, nil))

// out 是当前日志输出目标，供 Writer 交给 gin 等共用同一去向。
var out io.Writer = os.Stdout

// Init 初始化全局 logger，file 为空则输出到 stdout，否则写入该文件（自动建目录、追加写）。
func Init(level, file string) error {
	if file != "" {
		if dir := filepath.Dir(file); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
		}
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

// Writer 返回当前日志输出目标，供 gin 访问日志等导向同一文件。
func Writer() io.Writer { return out }

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
