// toolkit bibtex.go 把检索到的论文元数据拼成 BibTeX 引用条目,供 search_arxiv / search_semantic_scholar
// 在结果里附带,用户可直接复制进 .bib 文件用于论文写作。纯字符串拼接,不发网络请求。
package toolkit

import (
	"fmt"
	"strings"
	"unicode"
)

// bibEntry 是生成 BibTeX 所需的论文元数据,各检索工具按自己的字段填充。
type bibEntry struct {
	Title   string
	Authors []string
	Year    int
	Venue   string // 会议或期刊名,空表示仅预印本
	ArxivID string // 含或不含版本号均可,内部会去版本
	DOI     string
}

// bibConferenceMarkers 用于把 venue 判成会议(@inproceedings)而非期刊(@article)。
var bibConferenceMarkers = []string{
	"conference", "proceedings", "workshop", "symposium", "annual meeting",
	"neurips", "nips", "icml", "iclr", "cvpr", "iccv", "eccv",
	"acl", "emnlp", "naacl", "coling", "aaai", "ijcai", "kdd", "sigir",
	"www", "wsdm", "interspeech", "icassp",
}

// generateBibTeX 按元数据拼一条 BibTeX:有会议/期刊用 inproceedings/article,纯预印本用 misc。
func generateBibTeX(e bibEntry) string {
	key := bibCiteKey(e.Authors, e.Year, e.Title)
	typ := "misc"
	switch {
	case e.Venue != "" && isConferenceVenue(e.Venue):
		typ = "inproceedings"
	case e.Venue != "":
		typ = "article"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "@%s{%s,\n", typ, key)
	fmt.Fprintf(&b, "  title={%s},\n", escapeBib(e.Title))
	if len(e.Authors) > 0 {
		fmt.Fprintf(&b, "  author={%s},\n", strings.Join(e.Authors, " and "))
	}
	if e.Year > 0 {
		fmt.Fprintf(&b, "  year={%d},\n", e.Year)
	}
	switch typ {
	case "inproceedings":
		fmt.Fprintf(&b, "  booktitle={%s},\n", escapeBib(e.Venue))
	case "article":
		fmt.Fprintf(&b, "  journal={%s},\n", escapeBib(e.Venue))
	}
	if id := stripArxivVersion(e.ArxivID); id != "" {
		fmt.Fprintf(&b, "  eprint={%s},\n  archivePrefix={arXiv},\n", id)
	}
	if e.DOI != "" {
		fmt.Fprintf(&b, "  doi={%s},\n", e.DOI)
	}
	b.WriteString("}")
	return b.String()
}

// isConferenceVenue 粗判 venue 是否为会议:命中常见会议关键词或缩写即认为是。
func isConferenceVenue(venue string) bool {
	v := strings.ToLower(venue)
	for _, m := range bibConferenceMarkers {
		if strings.Contains(v, m) {
			return true
		}
	}
	return false
}

// bibCiteKey 造引用键:首作者姓 + 年份 + 标题首个实词,如 vaswani2017attention。
// 缺作者用 anon,缺年份省略,全空兜底 ref。
func bibCiteKey(authors []string, year int, title string) string {
	var parts []string
	if len(authors) > 0 {
		if s := lastNameAlnum(authors[0]); s != "" {
			parts = append(parts, s)
		}
	}
	// 缺作者时补 anon,避免键以年份开头不易读。
	if len(parts) == 0 {
		parts = append(parts, "anon")
	}
	if year > 0 {
		parts = append(parts, fmt.Sprint(year))
	}
	if w := firstTitleWord(title); w != "" {
		parts = append(parts, w)
	}
	return strings.Join(parts, "")
}

// lastNameAlnum 取姓名末段(姓)并只留小写字母数字。
func lastNameAlnum(name string) string {
	fields := strings.Fields(name)
	if len(fields) == 0 {
		return ""
	}
	return keepAlnumLower(fields[len(fields)-1])
}

// bibStopWords 造引用键时跳过的标题虚词。
var bibStopWords = map[string]bool{
	"a": true, "an": true, "the": true, "on": true, "of": true, "for": true,
	"and": true, "to": true, "in": true, "with": true, "is": true, "are": true,
}

// firstTitleWord 取标题首个非虚词,只留小写字母数字。
func firstTitleWord(title string) string {
	for w := range strings.FieldsSeq(title) {
		clean := keepAlnumLower(w)
		if clean != "" && !bibStopWords[clean] {
			return clean
		}
	}
	return ""
}

// keepAlnumLower 只保留字母数字并转小写。
func keepAlnumLower(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

// stripArxivVersion 去掉 arXiv 编号尾部的版本号,如 2503.03480v1 -> 2503.03480。
func stripArxivVersion(id string) string {
	id = strings.TrimSpace(id)
	if i := strings.LastIndexByte(id, 'v'); i > 0 {
		if rest := id[i+1:]; rest != "" && isAllDigits(rest) {
			return id[:i]
		}
	}
	return id
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return s != ""
}

// bibReplacer 转义 BibTeX/LaTeX 里的特殊字符,避免下游编译出错。
var bibReplacer = strings.NewReplacer(
	"&", `\&`, "%", `\%`, "$", `\$`, "#", `\#`, "_", `\_`,
)

func escapeBib(s string) string { return bibReplacer.Replace(s) }
