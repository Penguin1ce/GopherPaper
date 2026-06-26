package paper

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"GopherPaper/internal/ai/core"
	"GopherPaper/internal/sse"
	"GopherPaper/internal/tenant"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/constant"
	"GopherPaper/pkg/errs"
)

// reportTask 是投递给报告消费者的按需生成任务负载,一类报告一条。
type reportTask struct {
	PaperID    string              `json:"paper_id"`
	OwnerID    string              `json:"owner_id"`
	ReportType constant.ReportType `json:"report_type"`
}

// enqueueReport 把单类报告任务投递到报告队列,交并发 worker 池生成。
// 投递失败兜底后台 goroutine 生成,保证不因 MQ 抖动丢任务;重复投递由生成锁去重。
func enqueueReport(ctx context.Context, rt reportTask) {
	body, err := json.Marshal(rt)
	if err != nil {
		zlog.Error("序列化报告任务失败", "paper_id", rt.PaperID, "type", string(rt.ReportType), "err", err)
		return
	}
	if err := mqClient.Publish(ctx, reportQueue, body); err != nil {
		zlog.Error("投递报告队列失败,改后台生成", "paper_id", rt.PaperID, "type", string(rt.ReportType), "err", err)
		go runReportTask(context.WithoutCancel(ctx), rt)
	}
}

// startReportWorker 起 reportConcurrency 个后台 goroutine 并发消费报告队列。
// 连接关闭时通道随之关闭,goroutine 退出。
func startReportWorker(ctx context.Context) error {
	if reportConcurrency <= 0 {
		reportConcurrency = 1
	}
	deliveries, err := mqClient.Consume(reportQueue)
	if err != nil {
		return fmt.Errorf("service/paper: 订阅报告队列失败: %w", err)
	}
	for i := 0; i < reportConcurrency; i++ {
		go func() {
			for d := range deliveries {
				var task reportTask
				if err := json.Unmarshal(d.Body, &task); err != nil {
					// 毒消息无法解析,直接丢弃避免重投死循环。
					zlog.Error("报告任务无法解码,丢弃", "err", err)
					_ = d.Ack(false)
					continue
				}
				runReportTask(ctx, task)
				_ = d.Ack(false)
			}
		}()
	}
	zlog.Info("报告消费者已启动", "queue", reportQueue, "concurrency", reportConcurrency)
	return nil
}

// runReportTask 按需生成单类报告:注入 owner 租户后经 ensureReport 抢锁生成并落库。
// ctx 注入流式处理器,把小囊鼠流水线各阶段事件(规划→撰写→评审)桥到 sse 实时推前端执行计划;
// 已命中缓存或正被并发生成都视为正常,不算失败。
func runReportTask(ctx context.Context, task reportTask) {
	ctx = tenant.With(ctx, tenant.Tenant{StudentID: task.OwnerID})
	ctx = core.WithStream(ctx, func(ev core.StreamEvent) {
		if ev.Kind == constant.StreamEventPlan {
			sse.PushReportProgress(task.OwnerID, task.PaperID, string(task.ReportType), ev.Phase, ev.Delta)
		}
	})
	if _, err := ensureReport(ctx, task.PaperID, task.ReportType); err != nil {
		// 别人正在生成,由那条链路负责阶段与就绪推送,这里静默退出。
		if errors.Is(err, errs.ErrReportGenerating) {
			return
		}
		zlog.Error("报告生成失败", "paper_id", task.PaperID, "type", string(task.ReportType), "err", err)
		// 推一条失败阶段,前端把对应报告卡标记为失败态。
		sse.PushReportProgress(task.OwnerID, task.PaperID, string(task.ReportType), constant.ReportPhaseFailed, "生成失败")
		return
	}
	// 就绪即经 sse 通知前端,免轮询直接拉缓存。
	sse.PushReport(task.OwnerID, task.PaperID, string(task.ReportType))
	zlog.Info("报告生成完成", "paper_id", task.PaperID, "type", string(task.ReportType))
}
