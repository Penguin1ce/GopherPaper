package paper

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"GopherPaper/internal/ai"
	"GopherPaper/internal/ai/core"
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

	saveFigures(task, doc) // 图片落盘并回填 ImgURI,best-effort 不阻断
	if err := ai.DescribeFigures(ctx, doc.Figures); err != nil {
		// 图描述失败不阻断,图块退化为只用 caption 召回。
		zlog.Error("图片描述生成失败,降级只用 caption", "paper_id", task.PaperID, "err", err)
	}
	chunks := buildChunks(task, doc)
	chunks = append(chunks, buildFigureChunks(task, doc)...)
	if _, err := knowledge.UpsertChunksTRPC(ctx, chunks); err != nil {
		fail(ctx, task, "写入向量库失败", err)
		return
	}
	setStatus(ctx, task, constant.PaperIndexed, "")

	setStatus(ctx, task, constant.PaperReady, "")
	zlog.Info("论文解析入库完成", "paper_id", task.PaperID, "chunks", len(chunks))

	// 就绪后扇出全部研读报告并发预生成,用户点击即取缓存,不再逐个等待。
	enqueueReports(ctx, task)
}

// saveStructured 落库结构化元信息、章节,并回填标题与页数。
func saveStructured(ctx context.Context, paperID string, s *core.PaperStructured, doc *core.ParsedDoc) error {
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
func toSections(paperID string, doc *core.ParsedDoc) []model.PaperSection {
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
// 切分策略:把同一标题、同一页的连续碎段合并成一个块,并把章节路径前缀进正文一起
// 向量化,让小标题语义进入向量(问"实验方法"能召回方法段);换标题、换页或累计超
// MaxChunkRunes 即切块,保证页码出处精确、块不过大。
func buildChunks(task parseTask, doc *core.ParsedDoc) []knowledge.Chunk {
	var chunks []knowledge.Chunk
	var buf []string
	var bufRunes int
	var curSection string
	var curPage int

	flush := func() {
		if len(buf) == 0 {
			return
		}
		body := strings.Join(buf, "\n\n")
		content := body
		if curSection != "" {
			content = curSection + "\n\n" + body // 标题语境进 embedding
		}
		chunks = append(chunks, knowledge.Chunk{
			Content:    content,
			Scope:      constant.KnowledgeScopePrivate,
			OwnerID:    task.OwnerID,
			DocID:      task.PaperID,
			SourceFile: task.FileName,
			PageNo:     int64(curPage),
			ChunkIndex: int64(len(chunks)),
			Metadata:   map[string]any{"section": curSection},
		})
		buf = buf[:0]
		bufRunes = 0
	}

	for _, p := range doc.Paragraphs {
		text := strings.TrimSpace(p.Text)
		if text == "" {
			continue
		}
		n := len([]rune(text))
		// 边界:换标题、换页或累计超上限,先冲刷已攒的块再开新块。
		if len(buf) > 0 && (p.SectionPath != curSection || p.PageNo != curPage || bufRunes+n > constant.MaxChunkRunes) {
			flush()
		}
		curSection = p.SectionPath
		curPage = p.PageNo
		buf = append(buf, text)
		bufRunes += n
	}
	flush()
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
