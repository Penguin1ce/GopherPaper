package dao

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"

	"GopherCPP/internal/config"
)

// RDB 是全局 Redis 句柄，由 InitRedis 初始化。
var RDB *redis.Client

// InitRedis 连接 Redis 并 Ping 探活，结果存入包级 RDB，用于会话、缓存、限流。
func InitRedis(ctx context.Context, cfg config.RedisConfig) error {
	cli := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	if err := cli.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("dao: 连接 Redis 失败: %w", err)
	}
	RDB = cli
	return nil
}
