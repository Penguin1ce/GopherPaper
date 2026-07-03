package pioneer

import (
	"context"
	"fmt"
	"strings"

	"GopherPaper/internal/ai/core"
	"GopherPaper/internal/ai/toolkit"
)

func RelatedResearch(ctx context.Context, paperTitle string, rawRefs []string) (*core.Reply, error) {
	refs, err := toolkit.ResolveRelatedReferences(ctx, paperTitle, rawRefs, 10)
	if err != nil && len(refs) == 0 {
		return nil, err
	}
	var b strings.Builder
	b.WriteString("# 相关研究\n\n")
	b.WriteString("以下条目由小云雀沿当前论文的参考文献链路检索整理,优先给出可访问链接。\n\n")
	if len(refs) == 0 {
		b.WriteString("暂未检索到可确认链接的参考文献。可以稍后重试,或在论文参考文献页补充更清晰的条目信息。\n")
		return &core.Reply{Content: b.String(), Meta: map[string]any{"related_references": refs}}, nil
	}
	for i, ref := range refs {
		fmt.Fprintf(&b, "## %d. %s\n\n", i+1, fallbackText(ref.Title, "未命名文献"))
		if len(ref.Authors) > 0 {
			fmt.Fprintf(&b, "- 作者: %s\n", strings.Join(ref.Authors, "、"))
		}
		if ref.Year > 0 || ref.Venue != "" {
			fmt.Fprintf(&b, "- 来源: %s%s\n", yearText(ref.Year), venueText(ref.Venue))
		}
		if ref.CitationCount > 0 {
			fmt.Fprintf(&b, "- 被引: %d\n", ref.CitationCount)
		}
		if ref.URL != "" {
			fmt.Fprintf(&b, "- 链接: [%s](%s)\n", linkLabel(ref), ref.URL)
		}
		if ref.DOI != "" {
			fmt.Fprintf(&b, "- DOI: `%s`\n", ref.DOI)
		}
		if ref.ArxivID != "" {
			fmt.Fprintf(&b, "- arXiv: `%s`\n", ref.ArxivID)
		}
		b.WriteString("\n")
	}
	return &core.Reply{
		Content: strings.TrimSpace(b.String()),
		Meta: map[string]any{
			"related_references": refs,
			"source":             "pioneer_semantic_scholar",
		},
	}, nil
}

func fallbackText(s, fallback string) string {
	if strings.TrimSpace(s) != "" {
		return strings.TrimSpace(s)
	}
	return fallback
}

func yearText(year int) string {
	if year <= 0 {
		return ""
	}
	return fmt.Sprint(year)
}

func venueText(venue string) string {
	venue = strings.TrimSpace(venue)
	if venue == "" {
		return ""
	}
	return " " + venue
}

func linkLabel(ref toolkit.RelatedReference) string {
	if ref.PDFURL != "" {
		return "PDF"
	}
	if ref.DOI != "" {
		return "DOI"
	}
	if ref.ArxivID != "" {
		return "arXiv"
	}
	return "Semantic Scholar"
}
