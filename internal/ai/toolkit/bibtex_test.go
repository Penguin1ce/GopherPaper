package toolkit

import (
	"strings"
	"testing"
)

// TestGenerateBibTeX 覆盖三类条目:纯预印本 misc、会议 inproceedings、期刊 article,
// 以及引用键拼装、特殊字符转义、arXiv 去版本。
func TestGenerateBibTeX(t *testing.T) {
	// 纯预印本:无 venue → @misc,带 eprint 且去版本号。
	misc := generateBibTeX(bibEntry{
		Title:   "Attention Is All You Need",
		Authors: []string{"Ashish Vaswani", "Noam Shazeer"},
		Year:    2017,
		ArxivID: "1706.03762v5",
	})
	if !strings.HasPrefix(misc, "@misc{vaswani2017attention,") {
		t.Errorf("misc 类型/引用键不符:\n%s", misc)
	}
	if !strings.Contains(misc, "eprint={1706.03762}") {
		t.Errorf("eprint 应去版本号:\n%s", misc)
	}
	if !strings.Contains(misc, "author={Ashish Vaswani and Noam Shazeer}") {
		t.Errorf("作者应以 and 连接:\n%s", misc)
	}
	if !strings.Contains(misc, "archivePrefix={arXiv}") {
		t.Errorf("应带 archivePrefix:\n%s", misc)
	}

	// 会议 venue → @inproceedings + booktitle。
	conf := generateBibTeX(bibEntry{
		Title:   "SimCSE",
		Authors: []string{"Tianyu Gao"},
		Year:    2021,
		Venue:   "Conference on Empirical Methods in Natural Language Processing",
		DOI:     "10.18653/v1/2021.emnlp",
	})
	if !strings.HasPrefix(conf, "@inproceedings{gao2021simcse,") {
		t.Errorf("会议应为 inproceedings:\n%s", conf)
	}
	if !strings.Contains(conf, "booktitle={Conference on Empirical") {
		t.Errorf("会议应填 booktitle:\n%s", conf)
	}
	if !strings.Contains(conf, "doi={10.18653/v1/2021.emnlp}") {
		t.Errorf("应带 doi:\n%s", conf)
	}

	// 期刊 venue(无会议关键词)→ @article + journal。
	art := generateBibTeX(bibEntry{
		Title: "Deep Learning", Authors: []string{"Yann LeCun"}, Year: 2015, Venue: "Nature",
	})
	if !strings.HasPrefix(art, "@article{lecun2015deep,") {
		t.Errorf("期刊应为 article:\n%s", art)
	}
	if !strings.Contains(art, "journal={Nature}") {
		t.Errorf("期刊应填 journal:\n%s", art)
	}
}

// TestBibTeXEscapeAndFallback 验证特殊字符转义与缺作者/标题虚词跳过。
func TestBibTeXEscapeAndFallback(t *testing.T) {
	// & % _ 等需转义;标题首词为虚词 The 应跳到 cost。
	out := generateBibTeX(bibEntry{
		Title:   "The Cost & Scale_up of 100% Models",
		Authors: nil,
		Year:    2024,
	})
	if !strings.HasPrefix(out, "@misc{anon2024cost,") {
		t.Errorf("缺作者应补 anon、标题虚词应跳过:\n%s", out)
	}
	if !strings.Contains(out, `\&`) || !strings.Contains(out, `\%`) || !strings.Contains(out, `\_`) {
		t.Errorf("特殊字符应转义:\n%s", out)
	}
}
