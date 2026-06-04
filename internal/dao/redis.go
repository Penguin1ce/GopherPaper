package dao

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"GopherPaper/internal/config"
)

// RDB 是全局 Redis 句柄，由 InitRedis 初始化。
var RDB *redis.Client

// ErrCacheMiss 是键不存在时 Get 返回的哨兵错误，其他包用 errors.Is 判断即可，无需引入 go-redis。
var ErrCacheMiss = redis.Nil

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

// Set 写入键值，不设过期。
func Set(ctx context.Context, key string, val any) error {
	return RDB.Set(ctx, key, val, 0).Err()
}

// SetTTL 写入键值并设置过期时间，用于验证码、会话等带时效的数据。
func SetTTL(ctx context.Context, key string, val any, ttl time.Duration) error {
	return RDB.Set(ctx, key, val, ttl).Err()
}

// SetNX 仅当键不存在时写入并设过期，返回是否写入成功，可用作分布式锁或防重。
func SetNX(ctx context.Context, key string, val any, ttl time.Duration) (bool, error) {
	return RDB.SetNX(ctx, key, val, ttl).Result()
}

// Get 读取字符串值，键不存在时返回 ErrCacheMiss。
func Get(ctx context.Context, key string) (string, error) {
	return RDB.Get(ctx, key).Result()
}

// Del 删除一个或多个键，返回实际删除的数量。
func Del(ctx context.Context, keys ...string) (int64, error) {
	return RDB.Del(ctx, keys...).Result()
}

// Exists 判断键是否存在。
func Exists(ctx context.Context, key string) (bool, error) {
	n, err := RDB.Exists(ctx, key).Result()
	return n > 0, err
}

// Expire 为已存在的键设置过期时间，返回键是否存在。
func Expire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return RDB.Expire(ctx, key, ttl).Result()
}

// TTL 返回键的剩余存活时间，键不存在返回 -2，无过期返回 -1。
func TTL(ctx context.Context, key string) (time.Duration, error) {
	return RDB.TTL(ctx, key).Result()
}

// Incr 对键做原子自增并返回结果，键不存在按 0 起算，常用于限流计数。
func Incr(ctx context.Context, key string) (int64, error) {
	return RDB.Incr(ctx, key).Result()
}
