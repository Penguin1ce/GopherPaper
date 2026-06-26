// Package paper 是论文上传与解析入库的业务逻辑，包级函数直接读写 dao/paper、parser、ai、knowledge。
//
//	上传 Upload：存文件 + 建 Paper(uploaded) + 投 parse_queue,投递失败兜底后台解析
//	解析 worker：异步消费 parse_queue,逐步 解析→抽取→分块入库,每步更新状态并经 sse 推送
//	状态主推走 SSE,GetStatus 仅作断线兜底查询
package paper

import (
	"context"
	"fmt"
	"os"

	"GopherPaper/internal/ai/toolkit"
	"GopherPaper/internal/mq"
	"GopherPaper/internal/tenant"
)

// 解析链路依赖的 MQ 句柄、队列名与文件存储目录，由 Init 注入。
var (
	mqClient          *mq.Client
	parseQueue        string
	reportQueue       string
	reportConcurrency int
	storageDir        = "data/papers"
)

// Init 注入 MQ 句柄、解析与报告队列名，建好存储目录并拉起后台解析、报告消费者。须在 ai.Init 之后调用。
func Init(ctx context.Context, client *mq.Client, parseQ, reportQ string, reportConc int) error {
	mqClient = client
	parseQueue = parseQ
	reportQueue = reportQ
	reportConcurrency = reportConc
	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		return fmt.Errorf("service/paper: 创建存储目录失败: %w", err)
	}
	// 反向注入论文入库实现给 toolkit 的 download_paper 工具:下载在 toolkit,落盘建记录并投解析队列在本包。
	// 复用 Upload 走完整流水线,破 toolkit↔paperservice 依赖环;owner 从 ctx 的 tenant 取,即论文归属。
	toolkit.RegisterPaperIngest(func(ctx context.Context, fileName string, data []byte) (toolkit.IngestedPaper, error) {
		p, err := Upload(ctx, tenant.MustStudentID(ctx), fileName, data)
		if err != nil {
			return toolkit.IngestedPaper{}, err
		}
		return toolkit.IngestedPaper{PaperID: p.ID, Title: p.Title, Status: string(p.Status)}, nil
	})
	toolkit.RegisterPaperDelete(Delete)
	if err := startParseWorker(ctx); err != nil {
		return err
	}
	return startReportWorker(ctx)
}
