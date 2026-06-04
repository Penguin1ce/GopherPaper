// 会话上下文的 Redis 缓存。消息入库改为异步后，多轮对话不再实时可见于 MySQL，
// 这里用一个 List 维护每个会话最近的消息，发消息时同步写、读历史时优先读，
// MySQL 由消费者异步落库做持久化兜底。
package chat

import (
	"context"
	"encoding/json"
	"fmt"

	"GopherPaper/internal/dao"
	"GopherPaper/internal/model"
	"GopherPaper/pkg/constant"
)

// contextKey 拼接会话上下文缓存的 Redis 键。
func contextKey(sessionID string) string {
	return constant.RedisKeyChatContext + sessionID
}

// CacheRecentMessages 读会话上下文缓存最近 limit 条，命中时顺带续期。
// 第二个返回值标识是否命中：键不存在为未命中，调用方据此回落 MySQL。
func CacheRecentMessages(ctx context.Context, sessionID string, limit int) ([]model.Message, bool, error) {
	key := contextKey(sessionID)
	n, err := dao.RDB.Exists(ctx, key).Result()
	if err != nil {
		return nil, false, fmt.Errorf("dao/chat: 探测上下文缓存失败: %w", err)
	}
	if n == 0 {
		return nil, false, nil
	}

	start := int64(0)
	if limit > 0 {
		start = int64(-limit)
	}
	vals, err := dao.RDB.LRange(ctx, key, start, -1).Result()
	if err != nil {
		return nil, false, fmt.Errorf("dao/chat: 读取上下文缓存失败: %w", err)
	}
	msgs := make([]model.Message, 0, len(vals))
	for _, v := range vals {
		var m model.Message
		if err := json.Unmarshal([]byte(v), &m); err != nil {
			return nil, false, fmt.Errorf("dao/chat: 解析上下文缓存失败: %w", err)
		}
		msgs = append(msgs, m)
	}
	_ = dao.RDB.Expire(ctx, key, constant.ChatContextTTL).Err() // 续期失败不影响本次读
	return msgs, true, nil
}

// CacheAppendMessages 把本轮消息追加进会话上下文缓存，截断到最近 MaxContextMessages 条并续期。
func CacheAppendMessages(ctx context.Context, sessionID string, msgs []*model.Message) error {
	if len(msgs) == 0 {
		return nil
	}
	key := contextKey(sessionID)
	pipe := dao.RDB.TxPipeline()
	for _, m := range msgs {
		b, err := json.Marshal(m)
		if err != nil {
			return fmt.Errorf("dao/chat: 序列化上下文消息失败: %w", err)
		}
		pipe.RPush(ctx, key, b)
	}
	pipe.LTrim(ctx, key, -int64(constant.MaxContextMessages), -1)
	pipe.Expire(ctx, key, constant.ChatContextTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("dao/chat: 追加上下文缓存失败: %w", err)
	}
	return nil
}

// WarmContext 用 MySQL 历史重建会话上下文缓存，缓存未命中后回填。msgs 为空则不建缓存。
func WarmContext(ctx context.Context, sessionID string, msgs []model.Message) error {
	if len(msgs) == 0 {
		return nil
	}
	key := contextKey(sessionID)
	pipe := dao.RDB.TxPipeline()
	pipe.Del(ctx, key)
	for i := range msgs {
		b, err := json.Marshal(msgs[i])
		if err != nil {
			return fmt.Errorf("dao/chat: 序列化上下文消息失败: %w", err)
		}
		pipe.RPush(ctx, key, b)
	}
	pipe.Expire(ctx, key, constant.ChatContextTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("dao/chat: 回填上下文缓存失败: %w", err)
	}
	return nil
}
