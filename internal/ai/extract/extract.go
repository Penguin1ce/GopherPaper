// Package extract 实现论文结构化抽取，把解析后的正文抽成 PaperStructured。
// 由 parse worker 在 PDF 解析完成后调用，输出题目/作者/方法/结果/创新点等字段。
package extract

import (
	"encoding/json"
	"fmt"
	"strings"

	"GopherPaper/internal/ai/core"
)

// maxBodyChars 喂给模型的正文字符上限，超长截断避免超出上下文窗。
const maxBodyChars = 12000

// bodyText 把段落按章节拼成正文，带截断。
func bodyText(doc *core.ParsedDoc) string {
	var b strings.Builder
	lastSection := ""
	for _, p := range doc.Paragraphs {
		if p.SectionPath != "" && p.SectionPath != lastSection {
			b.WriteString("\n## ")
			b.WriteString(p.SectionPath)
			b.WriteString("\n")
			lastSection = p.SectionPath
		}
		b.WriteString(p.Text)
		b.WriteString("\n")
		if b.Len() >= maxBodyChars {
			break
		}
	}
	out := b.String()
	if len(out) > maxBodyChars {
		out = out[:maxBodyChars]
	}
	return out
}

// parseStructured 容错解析模型返回的 JSON，剥离可能的代码围栏。
func parseStructured(content string) (*core.PaperStructured, error) {
	raw := stripFence(content)
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("extract: 模型返回空内容")
	}
	var s core.PaperStructured
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return nil, fmt.Errorf("extract: 解析抽取结果失败: %w (content_len=%d, raw=%q)", err, len(content), snippet(raw, 300))
	}
	return &s, nil
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
