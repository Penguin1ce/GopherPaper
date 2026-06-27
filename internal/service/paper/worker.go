package paper

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"GopherPaper/internal/ai"
	"GopherPaper/internal/ai/core"
	paperdao "GopherPaper/internal/dao/paper"
	"GopherPaper/internal/graph"
	"GopherPaper/internal/knowledge"
	"GopherPaper/internal/model"
	"GopherPaper/internal/parser"
	"GopherPaper/internal/service/metrics"
	"GopherPaper/internal/sse"
	"GopherPaper/internal/tenant"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/constant"
	"GopherPaper/pkg/errs"
)

var getPaperForPipeline = paperdao.Get

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
	start := time.Now()
	// 注入论文 owner,供 ai.Extract 选到该用户模型,knowledge 写入按 owner 隔离。
	ctx = tenant.With(ctx, tenant.Tenant{StudentID: task.OwnerID})

	setStatus(ctx, task, constant.PaperParsing, "")

	doc, err := parser.Parse(ctx, task.FileURI)
	if err != nil {
		fail(ctx, task, "解析 PDF 失败", err, start)
		return
	}
	saveArtifact(task, doc) // 解析一拿到就归档原始产物,后续步骤失败也能据此离线重建

	structured, err := ai.Extract(ctx, doc)
	if err != nil {
		fail(ctx, task, "结构化抽取失败", err, start)
		return
	}
	if !paperPresentForPipeline(ctx, task, "save_structured") {
		return
	}
	if err := saveStructured(ctx, task.PaperID, structured, doc); err != nil {
		fail(ctx, task, "落库元信息失败", err, start)
		return
	}
	setStatus(ctx, task, constant.PaperExtracted, "")

	if !paperPresentForPipeline(ctx, task, "save_figures") {
		return
	}

	saveFigures(task, doc) // 图片落盘并回填 ImgURI,best-effort 不阻断
	if err := ai.DescribeFigures(ctx, doc.Figures); err != nil {
		// 图描述失败不阻断,图块退化为只用 caption 召回。
		zlog.Error("图片描述生成失败,降级只用 caption", "paper_id", task.PaperID, "err", err)
	}
	chunks := buildMetaChunks(task, structured)
	chunks = append(chunks, buildChunks(task, doc)...)
	chunks = append(chunks, buildFigureChunks(task, doc)...)
	chunks = append(chunks, buildTableChunks(task, doc)...)
	if !paperPresentForPipeline(ctx, task, "upsert_chunks") {
		return
	}
	// 重解析先清掉该论文旧 chunk,避免内容变化后旧切分残留成孤儿(UpsertChunks 仅按本批主键删)。
	// 首次解析时无旧数据,删除是 no-op。
	if err := knowledge.DeletePaperChunks(ctx, task.OwnerID, task.PaperID); err != nil {
		fail(ctx, task, "清理旧 chunks 失败", err, start)
		return
	}
	if _, err := knowledge.UpsertChunks(ctx, chunks); err != nil {
		fail(ctx, task, "写入向量库失败", err, start)
		return
	}
	setStatus(ctx, task, constant.PaperIndexed, "")

	// 写入知识图谱:论文与作者/关键词/机构/参考文献的关系,供关系发现与趋势分析。
	// best-effort,失败只记日志不阻断论文就绪(图谱是增强能力)。
	if !paperPresentForPipeline(ctx, task, "upsert_graph") {
		return
	}
	upsertGraph(ctx, task, structured, doc)

	if !paperPresentForPipeline(ctx, task, "mark_ready") {
		return
	}
	setStatus(ctx, task, constant.PaperReady, "")
	metrics.Record(ctx, metrics.ServiceParse, task.OwnerID, task.PaperID, "", true, time.Since(start), nil)
	zlog.Info("论文解析入库完成", "paper_id", task.PaperID, "chunks", len(chunks))

	// 研读报告改按需生成:用户在画廊里点某类报告才起小囊鼠长任务,解析完成不再全量预生成,
	// 省下没人看的报告的多 agent 流水线开销。
}

// saveStructured 落库结构化元信息、章节,并回填标题与页数。
func paperPresentForPipeline(ctx context.Context, task parseTask, stage string) bool {
	p, err := getPaperForPipeline(ctx, task.PaperID)
	if err == nil && p != nil && p.OwnerID == task.OwnerID {
		return true
	}
	if errors.Is(err, errs.ErrPaperNotFound) {
		zlog.Info("论文已删除,停止解析写回", "paper_id", task.PaperID, "stage", stage)
		return false
	}
	if err == nil && p != nil {
		zlog.Warn("论文归属已变化,停止解析写回", "paper_id", task.PaperID, "owner", task.OwnerID, "actual_owner", p.OwnerID, "stage", stage)
		return false
	}
	if err == nil {
		zlog.Warn("论文查询返回空结果,停止解析写回", "paper_id", task.PaperID, "stage", stage)
		return false
	}
	zlog.Error("检查论文是否仍存在失败,停止解析写回", "paper_id", task.PaperID, "stage", stage, "err", err)
	return false
}

func saveStructured(ctx context.Context, paperID string, s *core.PaperStructured, doc *core.ParsedDoc) error {
	meta := &model.PaperMeta{
		PaperID:           paperID,
		Authors:           s.Authors,
		Affiliations:      s.Affiliations,
		PublishYear:       s.PublishYear,
		Venue:             s.Venue,
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

// upsertGraph 把论文及其作者/关键词/机构/会议与参考文献写入知识图谱,best-effort。
func upsertGraph(ctx context.Context, task parseTask, s *core.PaperStructured, doc *core.ParsedDoc) {
	if err := graph.UpsertPaper(ctx, graph.PaperGraph{
		Owner:        task.OwnerID,
		ID:           task.PaperID,
		Title:        graphTitle(ctx, task, s),
		Year:         s.PublishYear,
		Venue:        s.Venue,
		Authors:      s.Authors,
		Keywords:     s.Keywords,
		Affiliations: s.Affiliations,
		Embedding:    paperEmbedding(ctx, s),
	}); err != nil {
		zlog.Error("图谱写入论文失败,降级", "paper_id", task.PaperID, "err", err)
	}
	if err := graph.UpsertCitations(ctx, task.OwnerID, task.PaperID, doc.References); err != nil {
		zlog.Error("图谱写入引用失败,降级", "paper_id", task.PaperID, "err", err)
	}
}

// graphTitle 取写入图谱的论文标题:优先抽取标题,空则回退到已落库标题(上传时按文件名兜底),
// 再空回退到论文 ID,保证图谱节点不出现空 title。与 BackfillGraph 的回退口径一致。
func graphTitle(ctx context.Context, task parseTask, s *core.PaperStructured) string {
	if t := strings.TrimSpace(s.Title); t != "" {
		return t
	}
	if p, err := paperdao.Get(ctx, task.PaperID); err == nil {
		if t := strings.TrimSpace(p.Title); t != "" {
			return t
		}
	}
	return task.PaperID
}

// paperEmbedding 用论文结构化语义摘要算向量供图谱相似边,失败或为空返回 nil(降级不建相似边)。
func paperEmbedding(ctx context.Context, s *core.PaperStructured) []float64 {
	text := graphSemanticText(s)
	if text == "" {
		return nil
	}
	vec, err := knowledge.Embed(ctx, text)
	if err != nil {
		zlog.Error("论文向量化失败,跳过相似边", "err", err)
		return nil
	}
	return vec
}

// graphSemanticText 汇集能稳定表达论文主题的信息,用于同领域论文的语义连边。
// 只放主题、问题、方法和贡献,不放作者/机构/年份,避免非内容字段拉近距离。
func graphSemanticText(s *core.PaperStructured) string {
	if s == nil {
		return ""
	}
	var b strings.Builder
	writeText := func(label, value string, maxRunes int) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if maxRunes > 0 {
			r := []rune(value)
			if len(r) > maxRunes {
				value = string(r[:maxRunes])
			}
		}
		fmt.Fprintf(&b, "%s: %s\n", label, value)
	}
	writeList := func(label string, values []string, limit int) {
		values = compactStrings(values)
		if limit > 0 && len(values) > limit {
			values = values[:limit]
		}
		if len(values) == 0 {
			return
		}
		fmt.Fprintf(&b, "%s: %s\n", label, strings.Join(values, "；"))
	}

	writeText("标题", s.Title, 300)
	writeText("摘要", s.Abstract, 1800)
	writeList("关键词", s.Keywords, 16)
	writeList("研究问题", s.ResearchQuestions, 8)
	writeText("方法", s.Methods, 1200)
	writeList("创新点", s.Innovations, 8)
	return strings.TrimSpace(b.String())
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

// buildMetaChunks 把详细页结构化字段写入知识库,补足正文中不一定逐字出现的关键词与摘要。
func buildMetaChunks(task parseTask, s *core.PaperStructured) []knowledge.Chunk {
	content := structuredContent(s)
	if content == "" {
		return nil
	}
	return []knowledge.Chunk{{
		Content:    content,
		Scope:      constant.KnowledgeScopePrivate,
		OwnerID:    task.OwnerID,
		DocID:      task.PaperID,
		SourceFile: task.FileName,
		ChunkIndex: -1,
		Metadata: map[string]any{
			constant.MilvusFieldBlockType: constant.BlockTypeText,
			"section":                     "论文结构化详情",
			"source":                      "paper_meta",
		},
	}}
}

func structuredContent(s *core.PaperStructured) string {
	if s == nil {
		return ""
	}
	var b strings.Builder
	writeText := func(label, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		fmt.Fprintf(&b, "%s: %s\n", label, value)
	}
	writeList := func(label string, values []string) {
		values = compactStrings(values)
		if len(values) == 0 {
			return
		}
		fmt.Fprintf(&b, "%s: %s\n", label, strings.Join(values, "；"))
	}

	writeText("标题", s.Title)
	writeList("作者", s.Authors)
	writeList("单位", s.Affiliations)
	writeText("摘要", s.Abstract)
	writeList("关键词", s.Keywords)
	writeList("研究问题", s.ResearchQuestions)
	writeText("方法", s.Methods)
	writeText("实验", s.Experiments)
	writeText("结果", s.Results)
	writeList("创新点", s.Innovations)
	writeList("局限性", s.Limitations)
	writeList("未来工作", s.FutureWork)
	return strings.TrimSpace(b.String())
}

func compactStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			out = append(out, v)
		}
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
	sse.PushStatus(task.OwnerID, task.PaperID, string(status), detail)
}

func fail(ctx context.Context, task parseTask, msg string, err error, start time.Time) {
	zlog.Error("论文解析失败", "paper_id", task.PaperID, "stage", msg, "err", err)
	setStatus(ctx, task, constant.PaperFailed, msg)
	metrics.Record(ctx, metrics.ServiceParse, task.OwnerID, task.PaperID, "", false, time.Since(start), err)
}
