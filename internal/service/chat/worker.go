// 会话消息的异步入库：生产侧投递任务，消费侧 worker 落库。
package chat

import (
	"context"
	"encoding/json"
	"fmt"

	chatdao "GopherPaper/internal/dao/chat"
	"GopherPaper/internal/model"
	"GopherPaper/internal/zlog"
)

// persistTask 是投递给入库消费者的任务负载，承载一轮对话的待落库消息。
type persistTask struct {
	SessionID string           `json:"session_id"`
	Messages  []*model.Message `json:"messages"`
}

// publishPersist 把一轮消息投递到入库队列，交由 worker 异步落库。
func publishPersist(ctx context.Context, sessionID string, msgs []*model.Message) error {
	body, err := json.Marshal(persistTask{SessionID: sessionID, Messages: msgs})
	if err != nil {
		return fmt.Errorf("service: 序列化入库任务失败: %w", err)
	}
	return mqClient.Publish(ctx, chatQueue, body)
}

// startPersistWorker 起一个后台 goroutine 消费入库队列，把消息落 MySQL 并刷新会话。
// 连接关闭时投递通道随之关闭，goroutine 自然退出。
func startPersistWorker(ctx context.Context) error {
	deliveries, err := mqClient.Consume(chatQueue)
	if err != nil {
		return fmt.Errorf("service: 订阅入库队列失败: %w", err)
	}
	go func() {
		for d := range deliveries {
			var task persistTask
			if err := json.Unmarshal(d.Body, &task); err != nil {
				// 毒消息无法解析，直接丢弃避免重投死循环。
				zlog.Error("入库任务解析失败，丢弃", "err", err)
				_ = d.Ack(false)
				continue
			}
			if err := chatdao.CreateMessages(ctx, task.Messages); err != nil {
				// 可能是瞬时故障，重新入队等待重试。TODO(dlx) 接死信队列防毒消息死循环。
				zlog.Error("入库任务落库失败，重入队", "session_id", task.SessionID, "err", err)
				_ = d.Nack(false, true)
				continue
			}
			_ = chatdao.TouchSession(ctx, task.SessionID) // 刷新失败不影响落库
			_ = d.Ack(false)
		}
		zlog.Info("入库消费者已退出")
	}()
	zlog.Info("入库消费者已启动", "queue", chatQueue)
	return nil
}
