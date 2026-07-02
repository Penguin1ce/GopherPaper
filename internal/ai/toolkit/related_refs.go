package toolkit

import (
	"context"
	"regexp"
	"strings"
)

type RelatedReference struct {
	Title         string   `json:"title"`
	Authors       []string `json:"authors,omitempty"`
	Year          int      `json:"year,omitempty"`
	Venue         string   `json:"venue,omitempty"`
	CitationCount int      `json:"citation_count,omitempty"`
	PaperID       string   `json:"paper_id,omitempty"`
	ArxivID       string   `json:"arxiv_id,omitempty"`
	DOI           string   `json:"doi,omitempty"`
	PDFURL        string   `json:"pdf_url,omitempty"`
	URL           string   `json:"url,omitempty"`
	Source        string   `json:"source,omitempty"`
}

var referenceLeadRe = regexp.MustCompile(`^\s*(?:\[\d+\]|\d+[\.)])\s*`)

// ResolveRelatedReferences 复用小云雀的 Semantic Scholar 工具链:
// 先按当前论文标题定位 S2 paper_id,再顺着 references 端点取参考文献链接;
// 若当前论文未被 S2 收录,退回按解析出的参考文献条目逐条检索。
func ResolveRelatedReferences(ctx context.Context, paperTitle string, rawRefs []string, maxResults int) ([]RelatedReference, error) {
	if maxResults <= 0 {
		maxResults = 10
	}
	if maxResults > 20 {
		maxResults = 20
	}
	if title := strings.TrimSpace(paperTitle); title != "" {
		found, err := s2Search(ctx, semanticScholarAPIKey, s2Input{Query: title, MaxResults: 1})
		if err == nil && len(found.Papers) > 0 && strings.TrimSpace(found.Papers[0].PaperID) != "" {
			refs, rErr := s2References(ctx, semanticScholarAPIKey, s2RelatedInput{
				PaperID:    found.Papers[0].PaperID,
				MaxResults: maxResults,
			})
			if rErr == nil && len(refs.Papers) > 0 {
				return relatedReferencesFromS2(refs.Papers, "semantic_scholar_references"), nil
			}
		}
	}
	return resolveRawReferenceLinks(ctx, rawRefs, maxResults)
}

func resolveRawReferenceLinks(ctx context.Context, rawRefs []string, maxResults int) ([]RelatedReference, error) {
	out := make([]RelatedReference, 0, maxResults)
	seen := map[string]bool{}
	var firstErr error
	for _, raw := range rawRefs {
		if len(out) >= maxResults {
			break
		}
		query := referenceSearchQuery(raw)
		if query == "" || seen[strings.ToLower(query)] {
			continue
		}
		seen[strings.ToLower(query)] = true
		found, err := s2Search(ctx, semanticScholarAPIKey, s2Input{Query: query, MaxResults: 1})
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if len(found.Papers) == 0 {
			continue
		}
		out = append(out, relatedReferenceFromS2(found.Papers[0], "parsed_reference_search"))
	}
	if len(out) > 0 {
		return out, nil
	}
	return nil, firstErr
}

func relatedReferencesFromS2(papers []s2Paper, source string) []RelatedReference {
	out := make([]RelatedReference, 0, len(papers))
	for _, p := range papers {
		out = append(out, relatedReferenceFromS2(p, source))
	}
	return out
}

func relatedReferenceFromS2(p s2Paper, source string) RelatedReference {
	return RelatedReference{
		Title:         p.Title,
		Authors:       p.Authors,
		Year:          p.Year,
		Venue:         p.Venue,
		CitationCount: p.CitationCount,
		PaperID:       p.PaperID,
		ArxivID:       p.ArxivID,
		DOI:           p.DOI,
		PDFURL:        p.PDFURL,
		URL:           referenceURL(p),
		Source:        source,
	}
}

func referenceURL(p s2Paper) string {
	if strings.TrimSpace(p.PDFURL) != "" {
		return strings.TrimSpace(p.PDFURL)
	}
	if doi := strings.TrimSpace(p.DOI); doi != "" {
		return "https://doi.org/" + strings.TrimPrefix(doi, "https://doi.org/")
	}
	if arxivID := strings.TrimSpace(p.ArxivID); arxivID != "" {
		return "https://arxiv.org/abs/" + arxivID
	}
	if id := strings.TrimSpace(p.PaperID); id != "" {
		return "https://www.semanticscholar.org/paper/" + id
	}
	return ""
}

func referenceSearchQuery(raw string) string {
	raw = referenceLeadRe.ReplaceAllString(strings.TrimSpace(raw), "")
	raw = strings.Join(strings.Fields(raw), " ")
	if raw == "" {
		return ""
	}
	r := []rune(raw)
	if len(r) > 180 {
		raw = string(r[:180])
	}
	return raw
}
