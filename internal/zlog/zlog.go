// Package zlog 是对标准库 slog 的极薄封装，提供全局 logger。
// 写文件时按日期切分(每天一个文件)，单文件超额滚动到序号后缀，并按数量清理历史文件。
package zlog

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

var logger = slog.New(slog.NewTextHandler(os.Stdout, nil))

// out 是当前日志输出目标，供 Writer 交给 gin 等共用同一去向。
var out io.Writer = os.Stdout

// Init 初始化全局 logger。file 为空则输出到 stdout，否则按日期切分写入该路径所在目录:
// 每天一个文件(prefix-2006-01-02.log)，单文件超过 maxSizeMB 同日滚动到 .N 后缀，
// 历史文件数超过 maxBackups 时按修改时间清理最旧的。总磁盘占用约束在约 maxBackups×maxSizeMB。
func Init(level, file string, maxSizeMB, maxBackups int) error {
	if file != "" {
		w, err := newRotateWriter(file, maxSizeMB, maxBackups)
		if err != nil {
			return err
		}
		out = w
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
