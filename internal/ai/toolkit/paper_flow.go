// toolkit paper_flow.go 是「论文思路图谱」工具,挂给小云雀。分两阶段流式构建:
//
//	1 骨架:读 PaperMeta 让模型抽出节点小标题与有向边(无 detail),经 StreamEventPaperFlow 推前端先画结构。
//	2 补细节:按节点顺序逐个用 retrieval 检索该环节相关原文,模型据原文写翔实 detail,
//	  经 StreamEventPaperFlowNode 逐条推前端,节点一个一个「点亮」——既详实又有 Claude 式渐进构建感。
//
// 只把一句话摘要回给模型,避免整图回灌上下文。
package toolkit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/function"

	"GopherPaper/internal/ai/core"
	"GopherPaper/internal/ai/retrieval"
	"GopherPaper/internal/aimodel"
	paperdao "GopherPaper/internal/dao/paper"
	"GopherPaper/internal/model"
	"GopherPaper/internal/tenant"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/constant"
	"GopherPaper/pkg/errs"
)

type paperFlowInput struct {
	PaperID string `json:"paper_id" jsonschema:"description=要生成思路图的论文 id,必须来自 list_my_papers 的结果,required"`
}

type paperFlowOutput struct {
	Title     string `json:"title" jsonschema:"description=思路图主旨,等同已推送给前端的图标题"`
	NodeCount int    `json:"node_count" jsonschema:"description=图中节点数"`
	Status    string `json:"status" jsonschema:"description=生成状态,rendered 表示已推送前端,not_ready 表示论文未抽取就绪"`
	Message   string `json:"message" jsonschema:"description=面向用户的提示,可据此自然语言介绍这张思路图"`
}

// flowNode 思路图节点,type 取固定枚举供前端配色;detail 第二阶段逐个补齐。
type flowNode struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Label  string `json:"label"`
	Detail string `json:"detail,omitempty"`
}

// flowEdge 思路图有向边,label 表达节点间推进关系。
type flowEdge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Label string `json:"label,omitempty"`
}

// flowFigure 是挂在主节点旁的论文真实配图旁注(缩略图),不参与主线,前端摆在父节点旁。
// 走图片检索召回该环节最相关的论文插图,前端用 doc_id+img_name 取图。
type flowFigure struct {
	ID      string `json:"id"`
	Parent  string `json:"parent"`
	DocID   string `json:"doc_id"`
	ImgName string `json:"img_name"`
	PageNo  int    `json:"page_no,omitempty"`
	Caption string `json:"caption,omitempty"`
}

// paperFlow 是推给前端的图谱骨架载荷(detail 与 figures 待补)。
type paperFlow struct {
	PaperID string       `json:"paper_id"`
	Title   string       `json:"title"`
	Nodes   []flowNode   `json:"nodes"`
	Edges   []flowEdge   `json:"edges"`
	Figures []flowFigure `json:"figures,omitempty"`
}

type PaperFlowNode = flowNode
type PaperFlowEdge = flowEdge
type PaperFlowFigure = flowFigure
type PaperFlow = paperFlow

// paperFlowNodeDetail 是某节点补齐的 detail(及可选配图),经 StreamEventPaperFlowNode 单条推送。
// 流式增量只带 detail;每节点收尾那条额外带 figure(若检索到相关论文插图)。
type paperFlowNodeDetail struct {
	PaperID string      `json:"paper_id"`
	NodeID  string      `json:"node_id"`
	Detail  string      `json:"detail"`
	Figure  *flowFigure `json:"figure,omitempty"`
}

// newPaperFlowTool 构建论文思路图工具。身份从 ctx 的 tenant 取,只对本人论文生效;
// 论文须已完成抽取(有 PaperMeta)才能成图,否则提示用户等待解析完成。
func newPaperFlowTool() tool.Tool {
	return function.NewFunctionTool(generatePaperFlow,
		function.WithName("generate_paper_flow"),
		function.WithDescription("为用户的某一篇论文生成「研究思路流程图」:把论文从问题、不足、核心思路、方法、实验到结论的脉络抽象成一张有向图,先画骨架再逐节点检索原文补充细节,实时推送到前端逐个点亮构建。当用户想看某篇论文的整体思路、逻辑脉络、技术路线或研究框架时调用;先用 list_my_papers 拿到 paper_id。论文需已解析就绪(ready/indexed)。"),
	)
}

func generatePaperFlow(ctx context.Context, in paperFlowInput) (paperFlowOutput, error) {
	owner := tenant.MustStudentID(ctx)
	if owner == "" {
		return paperFlowOutput{}, fmt.Errorf("generate_paper_flow: 缺少当前用户身份")
	}
	paperID := strings.TrimSpace(in.PaperID)
	if paperID == "" {
		return paperFlowOutput{}, fmt.Errorf("generate_paper_flow: paper_id 不能为空")
	}

	// 本轮幂等:同一论文已生成过思路图(骨架与逐节点都已推前端、FlowSink 已记)则直接返回缓存摘要。
	// React planner replan 后常会再调一次,若放行会重推一份新骨架——前端整体覆盖把已点亮的图打回
	// 占位再逐节点重画(用户看到的「清空重画」),还白跑一遍检索与生成。命中即短路,既止闪烁又省开销。
	if sink := core.FlowSinkFrom(ctx); sink != nil {
		if prev, ok := sink.Get().(paperFlow); ok && prev.PaperID == paperID {
			zlog.Info("generate_paper_flow 本轮已生成,跳过重复生成", "paper_id", paperID)
			return paperFlowOutput{
				Title:     prev.Title,
				NodeCount: len(prev.Nodes),
				Status:    "rendered",
				Message:   fmt.Sprintf("《%s》的研究思路图本轮已生成并展示在前端,无需重复生成,可直接据图向用户讲解。", prev.Title),
			}, nil
		}
	}

	p, err := getPaperForTool(ctx, paperID)
	if err != nil {
		return paperFlowOutput{}, fmt.Errorf("generate_paper_flow: 查询论文失败: %w", err)
	}
	if p.OwnerID != owner {
		return paperFlowOutput{}, fmt.Errorf("generate_paper_flow: %w", errs.ErrPaperForbidden)
	}

	meta, err := paperdao.GetMeta(ctx, paperID)
	if err != nil {
		return paperFlowOutput{
			Status:  "not_ready",
			Message: "该论文尚未完成解析抽取,暂时无法生成思路图,请等待解析就绪后再试。",
		}, nil
	}

	// 阶段一:抽骨架并先推前端画结构。
	flow, err := buildSkeleton(ctx, meta)
	if err != nil {
		return paperFlowOutput{}, err
	}
	flow.PaperID = paperID
	if flow.Title == "" {
		flow.Title = displayPaperTitle(p)
	}
	emitPaperFlow(ctx, flow)
	zlog.Info("generate_paper_flow 骨架已推送,开始逐节点补细节", "paper_id", paperID, "nodes", len(flow.Nodes))

	// 阶段二:按节点顺序逐个检索原文补 detail,边补边推前端逐个点亮。
	// 每节点限时,卡住即超时跳过,不冻住整张图。usedFigs 全局去重:既拦精确同文件,
	// 也按「页码+图说签名」拦同一张图的不同子面板(MinerU 常把一图抽成多文件),同图最多挂一个节点。
	usedFigs := map[string]bool{}
	for i := range flow.Nodes {
		n := &flow.Nodes[i]
		zlog.Info("generate_paper_flow 节点补细节开始", "node", n.ID, "label", n.Label, "idx", i)
		nodeCtx, cancel := context.WithTimeout(ctx, paperFlowNodeTimeout)
		detail := buildNodeDetail(nodeCtx, paperID, owner, meta, n)
		cancel()
		if detail == "" {
			detail = metaFallback(meta, n.Type)
		}
		n.Detail = detail
		// 给该节点配一张论文真实插图(图片检索召回最相关的一张),作为旁注缩略图。
		fig := nodeFigure(ctx, paperID, owner, n, usedFigs)
		if fig != nil {
			flow.Figures = append(flow.Figures, *fig)
		}
		emitPaperFlowNode(ctx, paperFlowNodeDetail{PaperID: paperID, NodeID: n.ID, Detail: detail, Figure: fig})
		zlog.Info("generate_paper_flow 节点补细节完成", "node", n.ID, "detail_len", len([]rune(detail)), "figure", fig != nil)
	}

	// 把补完细节的完整图交给收集器,供入口塞进 reply.Meta 持久化(刷新/重开会话不丢)。
	if sink := core.FlowSinkFrom(ctx); sink != nil {
		sink.Set(flow)
	}

	return paperFlowOutput{
		Title:     flow.Title,
		NodeCount: len(flow.Nodes),
		Status:    "rendered",
		Message:   fmt.Sprintf("已为《%s》生成研究思路图并推送到前端,共 %d 个关键节点,各节点已结合原文逐个补充细节。可据图向用户讲解这篇论文从问题到结论的脉络。", displayPaperTitle(p), len(flow.Nodes)),
	}, nil
}

// BuildPaperFlowGraph 构建与小云雀 generate_paper_flow 同款的完整思路图,
// 供非聊天入口复用,例如报告页的持久化思路图。
func BuildPaperFlowGraph(ctx context.Context, paperID string) (PaperFlow, error) {
	owner := tenant.MustStudentID(ctx)
	if owner == "" {
		return PaperFlow{}, fmt.Errorf("generate_paper_flow: 缺少当前用户身份")
	}
	paperID = strings.TrimSpace(paperID)
	if paperID == "" {
		return PaperFlow{}, fmt.Errorf("generate_paper_flow: paper_id 不能为空")
	}
	p, err := getPaperForTool(ctx, paperID)
	if err != nil {
		return PaperFlow{}, fmt.Errorf("generate_paper_flow: 查询论文失败: %w", err)
	}
	if p.OwnerID != owner {
		return PaperFlow{}, fmt.Errorf("generate_paper_flow: %w", errs.ErrPaperForbidden)
	}
	meta, err := paperdao.GetMeta(ctx, paperID)
	if errors.Is(err, errs.ErrPaperNotFound) {
		return PaperFlow{}, errs.ErrPaperNotReady
	}
	if err != nil {
		return PaperFlow{}, err
	}

	flow, err := buildSkeleton(ctx, meta)
	if err != nil {
		return PaperFlow{}, err
	}
	flow.PaperID = paperID
	if flow.Title == "" {
		flow.Title = displayPaperTitle(p)
	}
	emitPaperFlow(ctx, flow)

	usedFigs := map[string]bool{}
	for i := range flow.Nodes {
		n := &flow.Nodes[i]
		nodeCtx, cancel := context.WithTimeout(ctx, paperFlowNodeTimeout)
		detail := buildNodeDetail(nodeCtx, paperID, owner, meta, n)
		cancel()
		if detail == "" {
			detail = metaFallback(meta, n.Type)
		}
		n.Detail = detail
		fig := nodeFigure(ctx, paperID, owner, n, usedFigs)
		if fig != nil {
			flow.Figures = append(flow.Figures, *fig)
		}
		emitPaperFlowNode(ctx, paperFlowNodeDetail{PaperID: paperID, NodeID: n.ID, Detail: detail, Figure: fig})
	}
	return flow, nil
}

// buildSkeleton 让 chat 模型从论文结构化信息抽出图骨架(节点小标题 + 有向边,无 detail)。
func buildSkeleton(ctx context.Context, meta *model.PaperMeta) (paperFlow, error) {
	models, err := aimodel.ModelsForUser(tenant.MustStudentID(ctx))
	if err != nil {
		return paperFlow{}, err
	}
	req := &trpcmodel.Request{
		Messages: []trpcmodel.Message{
			trpcmodel.NewSystemMessage(constant.PaperFlowSkeletonPrompt),
			trpcmodel.NewUserMessage(metaDigest(meta)),
		},
	}
	if models.ChatMC.MaxTokens > 0 {
		req.MaxTokens = &models.ChatMC.MaxTokens
	}
	content, err := core.GenerateText(ctx, models.Chat, req)
	if err != nil {
		return paperFlow{}, fmt.Errorf("generate_paper_flow: 模型生成失败: %w", err)
	}
	var flow paperFlow
	if err := json.Unmarshal([]byte(stripJSONFence(content)), &flow); err != nil {
		return paperFlow{}, fmt.Errorf("generate_paper_flow: 解析思路图骨架失败: %w", err)
	}
	if len(flow.Nodes) == 0 {
		return paperFlow{}, fmt.Errorf("generate_paper_flow: 模型未产出有效节点")
	}
	return flow, nil
}

// paperFlowNodeTimeout 单节点补细节(检索+模型)的上限,卡住即超时跳过。
const paperFlowNodeTimeout = 45 * time.Second

// buildNodeDetail 对单个节点检索该环节相关原文,模型据原文写翔实 detail;
// 检索或模型出错时不阻断整图,返回空串由上层回退到结构化简述。
func buildNodeDetail(ctx context.Context, paperID, owner string, meta *model.PaperMeta, n *flowNode) string {
	query := strings.TrimSpace(n.Label + " " + typeKeyword(n.Type))
	docs, err := retrieval.RetrieveForPaper(ctx, query, owner, paperID)
	if err != nil {
		zlog.Warn("generate_paper_flow 节点检索失败,回退简述", "node", n.ID, "err", err)
	}
	zlog.Info("generate_paper_flow 节点检索返回", "node", n.ID, "docs", len(docs))
	passages := formatPassages(docs)

	models, err := aimodel.ModelsForUser(owner)
	if err != nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "环节小标题:%s\n类型:%s\n", n.Label, typeKeyword(n.Type))
	if abs := strings.TrimSpace(meta.Abstract); abs != "" {
		fmt.Fprintf(&b, "论文摘要(参考):%s\n", summarize(abs, 400))
	}
	if passages != "" {
		fmt.Fprintf(&b, "检索到的相关原文片段:\n%s", passages)
	} else {
		b.WriteString("(未检索到额外原文片段,请基于小标题与摘要简要说明)")
	}
	req := &trpcmodel.Request{
		Messages: []trpcmodel.Message{
			trpcmodel.NewSystemMessage(constant.PaperFlowNodeDetailPrompt),
			trpcmodel.NewUserMessage(b.String()),
		},
	}
	if models.ChatMC.MaxTokens > 0 {
		req.MaxTokens = &models.ChatMC.MaxTokens
	}
	// 流式生成:逐增量经 paper_flow_node 事件推前端,节点内 detail 逐字写出。
	// 节流到每累积约 8 字推一次,降事件量与前端重排频率;末尾的完整值由上层再补发一次。
	var acc strings.Builder
	lastEmit := 0
	onDelta := func(d string) {
		acc.WriteString(d)
		if cur := len([]rune(acc.String())); cur-lastEmit >= 8 {
			lastEmit = cur
			emitPaperFlowNode(ctx, paperFlowNodeDetail{PaperID: paperID, NodeID: n.ID, Detail: acc.String()})
		}
	}
	detail, err := core.StreamText(ctx, models.Chat, req, onDelta)
	if err != nil {
		zlog.Warn("generate_paper_flow 节点补细节失败", "node", n.ID, "err", err)
		return ""
	}
	return strings.TrimSpace(detail)
}

// metaFallback 在检索/模型补细节失败(返回空)时,按节点类型从结构化字段取一段兜底文本,
// 保证节点至少有内容、不一直停在「正在补充」。
func metaFallback(m *model.PaperMeta, t string) string {
	var s string
	switch t {
	case "problem", "gap":
		s = strings.Join(m.ResearchQuestions, "；")
		if s == "" {
			s = m.Abstract
		}
	case "idea":
		s = strings.Join(m.Innovations, "；")
	case "method":
		s = m.Methods
	case "experiment":
		s = m.Experiments
	case "result":
		s = m.Results
	case "conclusion":
		s = strings.Join(m.Innovations, "；")
		if s == "" {
			s = m.Results
		}
	}
	if s = strings.TrimSpace(s); s == "" {
		return ""
	}
	return summarize(s, 160)
}

// nodeFigure 给某节点配一张论文真实插图:按该环节 query 走图片检索,取候选里第一张「尚未被别的
// 节点用过」的图(used 全局去重,同一张图整张思路图最多出现一次),都用过或召回为空则返回 nil。
// 限时由调用方的 ctx 控制。
func nodeFigure(ctx context.Context, paperID, owner string, n *flowNode, used map[string]bool) *flowFigure {
	query := strings.TrimSpace(n.Label + " " + typeKeyword(n.Type))
	imgs, err := retrieval.RetrieveImagesForPaper(ctx, query, owner, paperID)
	if err != nil {
		zlog.Warn("generate_paper_flow 节点配图检索失败", "node", n.ID, "err", err)
		return nil
	}
	for _, img := range imgs {
		ref := retrieval.ReferenceFromDocument(img)
		if ref.ImgName == "" {
			continue
		}
		key := ref.DocID + "/" + ref.ImgName
		if used[key] {
			continue
		}
		// 同图去重:MinerU 常把一张图的多个子面板抽成不同文件(文件名不同,exact-name 拦不住),
		// 但它们共享 caption 与页码。按「页码+图说签名」判同图,命中即标记此文件已弃并跳过,
		// 避免同一张图的不同面板挂到相邻节点上「看着重复」。签名存进同一 used 表(带 sig: 前缀)。
		if sig := figureSig(ref, img); sig != "" {
			if used["sig:"+sig] {
				used[key] = true
				continue
			}
			used["sig:"+sig] = true
		}
		used[key] = true
		return &flowFigure{
			ID:      "fig-" + n.ID,
			Parent:  n.ID,
			DocID:   ref.DocID,
			ImgName: ref.ImgName,
			PageNo:  int(ref.PageNo),
			Caption: summarize(strings.TrimSpace(img.Content), 24),
		}
	}
	return nil
}

// figureSig 给图块算「图身份签名」:页码 + 归一化的图说前缀(剥掉章节路径,只取 caption 起始片段)。
// 同一张图被 MinerU 抽成多个子面板/重复文件时共享 caption 与页码,签名相同即判同图、跨节点去重;
// 不同图 caption 不同则签名不同,不误伤。caption 取不到时返回空串,退回仅按文件名去重。
// 取前 48 字符作指纹:figureContent 把 caption 排在 VLM 描述之前,故前缀落在真实图说内、不受描述差异影响。
func figureSig(ref retrieval.Reference, img *retrieval.Doc) string {
	content := strings.TrimSpace(img.Content)
	if section := strings.TrimSpace(retrieval.MetaString(img, "section")); section != "" {
		content = strings.TrimSpace(strings.TrimPrefix(content, section))
	}
	content = strings.ToLower(strings.Join(strings.Fields(content), " "))
	if content == "" {
		return ""
	}
	r := []rune(content)
	if len(r) > 48 {
		r = r[:48]
	}
	return fmt.Sprintf("%d|%s", ref.PageNo, string(r))
}

// typeKeyword 把节点类型映射成检索/提示用的中文关键词。
func typeKeyword(t string) string {
	switch t {
	case "problem":
		return "研究问题"
	case "gap":
		return "现有方法不足"
	case "idea":
		return "核心思路 创新点"
	case "method":
		return "方法 模型设计"
	case "experiment":
		return "实验设置 验证"
	case "result":
		return "实验结果 发现"
	case "conclusion":
		return "结论 贡献"
	default:
		return ""
	}
}

// formatPassages 把检索片段拼成带序号的上下文,每条截断,最多取前若干条。
func formatPassages(docs []*retrieval.Doc) string {
	const maxDocs, maxRunes = 4, 480
	var b strings.Builder
	for i, d := range docs {
		if i >= maxDocs {
			break
		}
		content := summarize(strings.TrimSpace(d.Content), maxRunes)
		if content == "" {
			continue
		}
		fmt.Fprintf(&b, "[%d] %s\n", i+1, content)
	}
	return b.String()
}

// summarize 把多余空白压平并截断到 n 个字符。
func summarize(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

// metaDigest 把论文结构化信息拼成喂模型的素材文本,只取与研究脉络相关的字段。
func metaDigest(m *model.PaperMeta) string {
	var b strings.Builder
	add := func(label, val string) {
		if val = strings.TrimSpace(val); val != "" {
			fmt.Fprintf(&b, "【%s】%s\n", label, val)
		}
	}
	add("摘要", m.Abstract)
	add("研究问题", strings.Join(m.ResearchQuestions, "；"))
	add("方法", m.Methods)
	add("实验", m.Experiments)
	add("结果", m.Results)
	add("创新点", strings.Join(m.Innovations, "；"))
	add("局限", strings.Join(m.Limitations, "；"))
	add("未来工作", strings.Join(m.FutureWork, "；"))
	return b.String()
}

// emitPaperFlow 把图骨架经 SSE 推给前端先画结构;无流式回调(同步聚合)时静默跳过。
func emitPaperFlow(ctx context.Context, flow paperFlow) {
	if stream := core.StreamFrom(ctx); stream != nil {
		stream(core.StreamEvent{Kind: constant.StreamEventPaperFlow, Payload: flow})
	}
}

// emitPaperFlowNode 把单个节点补齐的 detail 推给前端逐个点亮;无流式回调时静默跳过。
func emitPaperFlowNode(ctx context.Context, d paperFlowNodeDetail) {
	if stream := core.StreamFrom(ctx); stream != nil {
		stream(core.StreamEvent{Kind: constant.StreamEventPaperFlowNode, Payload: d})
	}
}

// stripJSONFence 去掉模型输出可能带的 ```json 围栏,截取首个 { 到末个 }。
func stripJSONFence(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return strings.TrimSpace(s)
}
