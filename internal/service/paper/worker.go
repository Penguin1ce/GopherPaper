package paper

import (
	"context"
	"encoding/json"
	"fmt"

	"GopherPaper/internal/agent"
	"GopherPaper/internal/ai"
	paperdao "GopherPaper/internal/dao/paper"
	"GopherPaper/internal/knowledge"
	"GopherPaper/internal/model"
	"GopherPaper/internal/parser"
	"GopherPaper/internal/tenant"
	"GopherPaper/internal/ws"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/constant"
)

// startParseWorker 起后台 goroutine 消费解析队列。连接关闭时通道随之关闭,goroutine 退出。
func startParseWorker(ctx context.Context) error {
	deliveries, err := mqClient.Consume(parseQueue)
	if err != nil {
		return fmt.Errorf("service/paper: 订阅解析队列失败: %w", err)
	}
	go func() {
		for d := range deliveries {
			var task parseTask
			if err := json.Unmarshal(d.Body, &task); err != nil {
				// 毒消息无法解析,直接丢弃避免重投死循环。
				zlog.Error("解析任务无法解码,丢弃", "err", err)
				_ = d.Ack(false)
				continue
			}
			runPipeline(ctx, task)
			_ = d.Ack(false)
		}
		zlog.Info("解析消费者已退出")
	}()
	zlog.Info("解析消费者已启动", "queue", parseQueue)
	return nil
}

// runPipeline 跑完整解析流水线,逐步更新状态并经 ws 推送。任一步失败标记 failed。
func runPipeline(ctx context.Context, task parseTask) {
	// 注入论文 owner,供 ai.Extract 选到该用户模型,knowledge 写入按 owner 隔离。
	ctx = tenant.With(ctx, tenant.Tenant{StudentID: task.OwnerID})

	setStatus(ctx, task, constant.PaperParsing, "")

	doc, err := parser.Parse(ctx, task.FileURI)
	if err != nil {
		fail(ctx, task, "解析 PDF 失败", err)
		return
	}

	structured, err := ai.Extract(ctx, doc)
	if err != nil {
		fail(ctx, task, "结构化抽取失败", err)
		return
	}
	if err := saveStructured(ctx, task.PaperID, structured, doc); err != nil {
		fail(ctx, task, "落库元信息失败", err)
		return
	}
	setStatus(ctx, task, constant.PaperExtracted, "")

	chunks := buildChunks(task, doc)
	if _, err := knowledge.UpsertChunks(ctx, chunks); err != nil {
		fail(ctx, task, "写入向量库失败", err)
		return
	}
	setStatus(ctx, task, constant.PaperIndexed, "")

	setStatus(ctx, task, constant.PaperReady, "")
	zlog.Info("论文解析入库完成", "paper_id", task.PaperID, "chunks", len(chunks))
}

// saveStructured 落库结构化元信息、章节,并回填标题与页数。
func saveStructured(ctx context.Context, paperID string, s *agent.PaperStructured, doc *agent.ParsedDoc) error {
	meta := &model.PaperMeta{
		PaperID:           paperID,
		Authors:           s.Authors,
		Affiliations:      s.Affiliations,
		Abstract:          s.Abstract,
		Keywords:          s.Keywords,
		ResearchQuestions: s.ResearchQuestions,
		Methods:           s.Methods,
		Experiments:       s.Experiments,
		Results:           s.Results,
		Innovations:       s.Innovations,
		Limitations:       s.Limitations,
		FutureWork:        s.FutureWork,
	}
	if err := paperdao.SaveMeta(ctx, meta); err != nil {
		return err
	}
	if err := paperdao.SaveSections(ctx, paperID, toSections(paperID, doc)); err != nil {
		return err
	}
	return paperdao.UpdateInfo(ctx, paperID, s.Title, doc.PageCount)
}

// toSections 把解析出的章节转成落库模型。
func toSections(paperID string, doc *agent.ParsedDoc) []model.PaperSection {
	out := make([]model.PaperSection, 0, len(doc.Sections))
	for _, sec := range doc.Sections {
		out = append(out, model.PaperSection{
			PaperID:  paperID,
			Level:    sec.Level,
			Title:    sec.Title,
			PageNo:   sec.PageNo,
			OrderIdx: sec.OrderIdx,
		})
	}
	return out
}

// buildChunks 把正文段落切成带页码出处的知识块,归属上传者私有库。
func buildChunks(task parseTask, doc *agent.ParsedDoc) []knowledge.Chunk {
	chunks := make([]knowledge.Chunk, 0, len(doc.Paragraphs))
	for i, p := range doc.Paragraphs {
		chunks = append(chunks, knowledge.Chunk{
			Content:    p.Text,
			Scope:      constant.KnowledgeScopePrivate,
			OwnerID:    task.OwnerID,
			DocID:      task.PaperID,
			SourceFile: task.FileName,
			PageNo:     int64(p.PageNo),
			ChunkIndex: int64(i),
			Metadata:   map[string]any{"section": p.SectionPath},
		})
	}
	return chunks
}

func setStatus(ctx context.Context, task parseTask, status constant.PaperStatus, detail string) {
	if err := paperdao.UpdateStatus(ctx, task.PaperID, status, detail); err != nil {
		zlog.Error("更新解析状态失败", "paper_id", task.PaperID, "status", status, "err", err)
	}
	ws.PushStatus(task.OwnerID, task.PaperID, string(status), detail)
}

func fail(ctx context.Context, task parseTask, msg string, err error) {
	zlog.Error("论文解析失败", "paper_id", task.PaperID, "stage", msg, "err", err)
	setStatus(ctx, task, constant.PaperFailed, msg)
}
