// Package extract 实现论文结构化抽取，把解析后的正文抽成 PaperStructured。
// 由 parse worker 在 PDF 解析完成后调用，输出题目/作者/方法/结果/创新点等字段。
//
// 抽取走 map-reduce:长论文一次性塞给模型会触发 lost-in-the-middle 漏字段,故
// 先把正文按章节边界切成若干窗口(map),每个窗口各自抽一份只含本窗口有依据字段
// 的 JSON,再把多份结果合并成最终一份(reduce)。窗口小、模型注意力集中,且不依赖
// MinerU 章节标题质量,避免旧版"关键词匹配不中就整片字段空"的脆弱路由。
package extract

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"GopherPaper/internal/ai/agentrt"
	"GopherPaper/internal/ai/core"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/constant"
)

const (
	// maxBodyChars 纳入抽取的正文字符总上限,远小于模型上下文窗,够覆盖绝大多数论文核心章节。
	maxBodyChars = 60000
	// mapWindowRunes 单个 map 窗口字符数,小窗口规避 lost-in-the-middle。
	mapWindowRunes = 9000
	// maxMapWindows map 调用数上限,防超长论文调用爆炸。
	maxMapWindows = 7
)

var quotedTitleRe = regexp.MustCompile(`["“]([^"”]{8,200})["”]`)

// Extract 经带工具 chat agent 把解析后的论文抽成结构化信息。ctx 须注入论文 owner。
// 单窗口直接抽,多窗口走 map-reduce 合并。
func Extract(ctx context.Context, doc *core.ParsedDoc) (*core.PaperStructured, error) {
	logParsedAssets(doc)
	windows := buildWindows(doc)
	if len(windows) == 0 {
		return &core.PaperStructured{}, nil
	}

	partials := make([]*core.PaperStructured, 0, len(windows))
	var firstErr error
	for i, w := range windows {
		p, err := extractWindow(ctx, w, i+1, len(windows))
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			zlog.Error("抽取片段失败,跳过", "window", i+1, "total", len(windows), "err", err)
			continue
		}
		partials = append(partials, p)
	}
	if len(partials) == 0 {
		return nil, fmt.Errorf("extract: 所有片段抽取失败: %w", firstErr)
	}
	if len(partials) == 1 {
		return partials[0], nil
	}
	return reducePartials(ctx, partials), nil
}

func logParsedAssets(doc *core.ParsedDoc) {
	if doc == nil {
		return
	}
	zlog.Info("论文抽取输入资产",
		"figures", figureLogItems(doc.Figures),
		"reference_titles", referenceTitleLogItems(doc.References),
		"figure_count", len(doc.Figures),
		"reference_count", len(doc.References),
	)
}

func figureLogItems(figs []core.Figure) []string {
	out := make([]string, 0, len(figs))
	for i, fig := range figs {
		caption := summarizeLogText(firstNonEmpty(fig.Caption, fig.Desc), 120)
		if caption == "" {
			caption = filepathBase(fig.ImgPath)
		}
		if caption == "" {
			caption = fmt.Sprintf("figure_%d", i+1)
		}
		if fig.PageNo > 0 {
			caption = fmt.Sprintf("p%d %s", fig.PageNo, caption)
		}
		out = append(out, caption)
	}
	return out
}

func referenceTitleLogItems(refs []string) []string {
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		title := referenceTitle(ref)
		if title == "" {
			continue
		}
		out = append(out, title)
	}
	return out
}

func referenceTitle(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	if m := quotedTitleRe.FindStringSubmatch(ref); len(m) == 2 {
		return summarizeLogText(m[1], 160)
	}
	return summarizeLogText(stripReferencePrefix(ref), 160)
}

func stripReferencePrefix(ref string) string {
	ref = strings.TrimSpace(ref)
	if i := strings.Index(ref, "]"); strings.HasPrefix(ref, "[") && i >= 0 && i+1 < len(ref) {
		ref = strings.TrimSpace(ref[i+1:])
	}
	return ref
}

func summarizeLogText(s string, maxRunes int) string {
	s = strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
	if maxRunes <= 0 {
		return s
	}
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes]) + "..."
}

func filepathBase(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	path = strings.TrimRight(path, "/\\")
	if i := strings.LastIndexAny(path, "/\\"); i >= 0 {
		return path[i+1:]
	}
	return path
}

// extractWindow 对单个窗口抽取一份只含本窗口有依据字段的 JSON。
func extractWindow(ctx context.Context, text string, idx, total int) (*core.PaperStructured, error) {
	sysPrompt := strings.ReplaceAll(constant.ExtractPrompt, "{context}", text)
	// 须带 user 轮次,否则推理模型只见 system 指令会返回空 content。
	query := fmt.Sprintf("这是论文的第 %d/%d 段材料,请输出本段能抽取到的字段 JSON。", idx, total)
	content, err := agentrt.Generate(ctx, sysPrompt, nil, query)
	if err != nil {
		return nil, err
	}
	return parseStructured(content)
}

// reducePartials 把多窗口的抽取结果合并成最终一份:先确定性并集兜底,再经模型凝练文本字段。
func reducePartials(ctx context.Context, partials []*core.PaperStructured) *core.PaperStructured {
	merged := mergePartials(partials)
	refined, err := llmReduce(ctx, partials)
	if err != nil {
		zlog.Error("抽取合并降级为确定性合并", "err", err)
		return merged
	}
	// 模型偶尔漏填某些列表/题目,用确定性合并结果补缺。
	return backfillStructured(refined, merged)
}

// llmReduce 把各片段的抽取 JSON 交给模型综合成一份连贯结果。
func llmReduce(ctx context.Context, partials []*core.PaperStructured) (*core.PaperStructured, error) {
	payload, err := json.Marshal(partials)
	if err != nil {
		return nil, err
	}
	sysPrompt := strings.ReplaceAll(constant.ExtractReducePrompt, "{context}", string(payload))
	content, err := agentrt.Generate(ctx, sysPrompt, nil, "请合并以上各片段的抽取结果,输出最终唯一 JSON。")
	if err != nil {
		return nil, err
	}
	return parseStructured(content)
}

type sectionGroup struct {
	Title string
	Text  string
}

// buildWindows 把正文按章节边界顺序切成若干 map 窗口。
// 小章节合并、大章节硬切,窗口数与总字符数都有上限,不依赖标题关键词,故对章节切分质量鲁棒。
func buildWindows(doc *core.ParsedDoc) []string {
	groups := groupParagraphsBySection(doc)
	if len(groups) == 0 {
		return nil
	}

	var windows []string
	var cur strings.Builder
	curRunes, totalRunes := 0, 0

	flush := func() {
		if cur.Len() == 0 {
			return
		}
		windows = append(windows, strings.TrimSpace(cur.String()))
		cur.Reset()
		curRunes = 0
	}
	// add 追加一段文本,放不下当前窗口就在此处切窗;返回 false 表示已达全局上限须停止。
	add := func(s string) bool {
		if totalRunes >= maxBodyChars || len(windows) >= maxMapWindows {
			return false
		}
		n := len([]rune(s))
		if curRunes > 0 && curRunes+n > mapWindowRunes {
			flush()
			if len(windows) >= maxMapWindows {
				return false
			}
		}
		cur.WriteString(s)
		curRunes += n
		totalRunes += n
		return true
	}

	for _, g := range groups {
		body := strings.TrimSpace(g.Text)
		if body == "" {
			continue
		}
		seg := "## " + g.Title + "\n" + body + "\n"
		stop := false
		for _, piece := range chunkRunes(seg, mapWindowRunes) {
			if !add(piece) {
				stop = true
				break
			}
		}
		if stop {
			break
		}
	}
	flush()
	return windows
}

// groupParagraphsBySection 把段落按所属章节合并,保持文档顺序,空标题归到"正文"。
func groupParagraphsBySection(doc *core.ParsedDoc) []sectionGroup {
	if doc == nil {
		return nil
	}
	index := map[string]int{}
	groups := []sectionGroup{}
	for _, p := range doc.Paragraphs {
		text := strings.TrimSpace(p.Text)
		if text == "" {
			continue
		}
		title := strings.TrimSpace(p.SectionPath)
		if title == "" {
			title = "正文"
		}
		if i, ok := index[title]; ok {
			groups[i].Text += "\n" + text
			continue
		}
		index[title] = len(groups)
		groups = append(groups, sectionGroup{Title: title, Text: text})
	}
	return groups
}

// chunkRunes 把字符串按 size 个字符硬切成多段,size<=0 或不超长时原样返回。
func chunkRunes(s string, size int) []string {
	if size <= 0 {
		return []string{s}
	}
	r := []rune(s)
	if len(r) <= size {
		return []string{s}
	}
	out := make([]string, 0, len(r)/size+1)
	for i := 0; i < len(r); i += size {
		out = append(out, string(r[i:min(i+size, len(r))]))
	}
	return out
}

// mergePartials 确定性合并各片段:列表并集去重,题目/摘要取首个非空,方法/实验/结果取最长。
func mergePartials(ps []*core.PaperStructured) *core.PaperStructured {
	out := &core.PaperStructured{}
	for _, p := range ps {
		if p == nil {
			continue
		}
		out.Title = firstNonEmpty(out.Title, p.Title)
		out.Venue = firstNonEmpty(out.Venue, p.Venue)
		if out.PublishYear == 0 {
			out.PublishYear = p.PublishYear
		}
		out.Abstract = firstNonEmpty(out.Abstract, p.Abstract)
		out.Methods = longer(out.Methods, p.Methods)
		out.Experiments = longer(out.Experiments, p.Experiments)
		out.Results = longer(out.Results, p.Results)
		out.Authors = unionStrings(out.Authors, p.Authors)
		out.Affiliations = unionStrings(out.Affiliations, p.Affiliations)
		out.Keywords = unionStrings(out.Keywords, p.Keywords)
		out.ResearchQuestions = unionStrings(out.ResearchQuestions, p.ResearchQuestions)
		out.Innovations = unionStrings(out.Innovations, p.Innovations)
		out.Limitations = unionStrings(out.Limitations, p.Limitations)
		out.FutureWork = unionStrings(out.FutureWork, p.FutureWork)
	}
	return out
}

// backfillStructured 以 primary 为准,空字段用 fallback 补,保证模型漏填也不丢信息。
func backfillStructured(primary, fallback *core.PaperStructured) *core.PaperStructured {
	if primary == nil {
		return fallback
	}
	primary.Title = firstNonEmpty(primary.Title, fallback.Title)
	primary.Venue = firstNonEmpty(primary.Venue, fallback.Venue)
	if primary.PublishYear == 0 {
		primary.PublishYear = fallback.PublishYear
	}
	primary.Abstract = firstNonEmpty(primary.Abstract, fallback.Abstract)
	primary.Methods = firstNonEmpty(primary.Methods, fallback.Methods)
	primary.Experiments = firstNonEmpty(primary.Experiments, fallback.Experiments)
	primary.Results = firstNonEmpty(primary.Results, fallback.Results)
	primary.Authors = fallbackList(primary.Authors, fallback.Authors)
	primary.Affiliations = fallbackList(primary.Affiliations, fallback.Affiliations)
	primary.Keywords = fallbackList(primary.Keywords, fallback.Keywords)
	primary.ResearchQuestions = fallbackList(primary.ResearchQuestions, fallback.ResearchQuestions)
	primary.Innovations = fallbackList(primary.Innovations, fallback.Innovations)
	primary.Limitations = fallbackList(primary.Limitations, fallback.Limitations)
	primary.FutureWork = fallbackList(primary.FutureWork, fallback.FutureWork)
	return primary
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func longer(a, b string) string {
	if len([]rune(strings.TrimSpace(b))) > len([]rune(strings.TrimSpace(a))) {
		return b
	}
	return a
}

func fallbackList(a, b []string) []string {
	if len(a) > 0 {
		return a
	}
	return b
}

// unionStrings 顺序并集去重,去掉空项。
func unionStrings(a, b []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, v := range append(append([]string{}, a...), b...) {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

// parseStructured 容错解析模型返回的 JSON，剥离可能的代码围栏。
// 模型常把 LaTeX 公式(如 $\mathcal{L}$)原样写进字段,单反斜杠在 JSON 里是非法转义,
// 故首解析失败时修复字符串内的非法转义再重试。
func parseStructured(content string) (*core.PaperStructured, error) {
	raw := stripFence(content)
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("extract: 模型返回空内容")
	}
	var s core.PaperStructured
	if err := json.Unmarshal([]byte(raw), &s); err == nil {
		return &s, nil
	}
	repaired := repairJSONEscapes(raw)
	if err := json.Unmarshal([]byte(repaired), &s); err != nil {
		return nil, fmt.Errorf("extract: 解析抽取结果失败: %w (content_len=%d, raw=%q)", err, len(content), snippet(raw, 300))
	}
	return &s, nil
}

// repairJSONEscapes 把 JSON 字符串内的非法反斜杠转义改成字面反斜杠(\\),修复模型直出的 LaTeX。
// 只在字符串字面量内动手,合法转义(含 \uXXXX)原样保留,结构外的反斜杠不碰。
func repairJSONEscapes(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 16)
	inString := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !inString {
			b.WriteByte(c)
			if c == '"' {
				inString = true
			}
			continue
		}
		switch c {
		case '"':
			b.WriteByte(c)
			inString = false
		case '\\':
			if i+1 < len(s) && isValidEscapeAt(s, i+1) {
				if s[i+1] == 'u' {
					b.WriteString(s[i : i+6])
					i += 5
				} else {
					b.WriteByte(c)
					b.WriteByte(s[i+1])
					i++
				}
			} else {
				b.WriteString(`\\`)
			}
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// isValidEscapeAt 判断 s[i] 是否是合法 JSON 转义字符(\u 须后接 4 位十六进制)。
func isValidEscapeAt(s string, i int) bool {
	switch s[i] {
	case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
		return true
	case 'u':
		if i+5 >= len(s) {
			return false
		}
		for j := i + 1; j <= i+4; j++ {
			if !isHex(s[j]) {
				return false
			}
		}
		return true
	}
	return false
}

func isHex(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

// snippet 截取前 n 个字符,用于报错回显模型返回。
func snippet(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

// stripFence 去掉 ```json ... ``` 围栏，并截取首个 { 到末个 }。
func stripFence(s string) string {
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
