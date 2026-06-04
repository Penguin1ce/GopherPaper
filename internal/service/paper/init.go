// Package paper 是论文上传与解析入库的业务逻辑，包级函数直接读写 dao/paper、parser、ai、knowledge。
//
//	上传 Upload：存文件 + 建 Paper(uploaded) + 投 parse_queue,投递失败兜底后台解析
//	解析 worker：异步消费 parse_queue,逐步 解析→抽取→分块入库,每步更新状态并经 ws 推送
//	状态主推走 WebSocket,GetStatus 仅作断线兜底查询
package paper

import (
	"context"
	"fmt"
	"os"

	"GopherPaper/internal/mq"
)

// 解析链路依赖的 MQ 句柄、队列名与文件存储目录，由 Init 注入。
var (
	mqClient   *mq.Client
	parseQueue string
	storageDir = "data/papers"
)

// Init 注入 MQ 句柄与解析队列名，建好存储目录并拉起后台解析消费者。须在 ai.Init 之后调用。
func Init(ctx context.Context, client *mq.Client, queue string) error {
	mqClient = client
	parseQueue = queue
	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		return fmt.Errorf("service/paper: 创建存储目录失败: %w", err)
	}
	return startParseWorker(ctx)
}
