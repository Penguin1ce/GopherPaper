package dao

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"

	"GopherCPP/internal/config"
)

// NewRedis 连接 Redis 并 Ping 探活，用于会话、缓存、限流。
func NewRedis(ctx context.Context, cfg config.RedisConfig) (*redis.Client, error) {
	cli := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	if err := cli.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("dao: 连接 Redis 失败: %w", err)
	}
	return cli, nil
}
