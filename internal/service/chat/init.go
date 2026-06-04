// Package chat 是会话与多轮对话的业务逻辑，包级函数直接读写 dao/chat 与 ai。
//
//	会话归属校验：任何按 id 的操作先核对 session.StudentID 与当前学生是否一致
//	多轮对话 SendMessage：读上下文缓存 → 调 AI → 写缓存并投递入库队列 → 直接返回
//	入库消费者 worker：异步消费队列把消息落 MySQL，做持久化兜底
package chat

import (
	"context"

	"GopherPaper/internal/mq"
)

// 入库链路依赖的 MQ 句柄与队列名，由 Init 注入。
var (
	mqClient  *mq.Client
	chatQueue string
)

// Init 注入 MQ 句柄与入库队列名，并拉起后台入库消费者。须在 ai.Init 之后调用。
func Init(ctx context.Context, client *mq.Client, queue string) error {
	mqClient = client
	chatQueue = queue
	return startPersistWorker(ctx)
}
