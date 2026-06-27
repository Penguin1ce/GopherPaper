package toolkit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConferenceProceedingsNeurIPS(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/paper_files/paper/2025" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`
<html><body>
<ul class="paper-list">
  <li class="conference" data-track="conference">
    <div class="paper-content">
      <a title="paper title" href="/paper_files/paper/2025/hash/abc123-Abstract-Conference.html">TheAgentCompany: Benchmarking LLM Agents</a>
      <span class="paper-authors">Alice Zhang, Bob Li</span>
    </div>
    <span class="paper-track-badge">Main Conference Track</span>
  </li>
  <li class="datasets" data-track="datasets">
    <div class="paper-content">
      <a title="paper title" href="/paper_files/paper/2025/hash/def456-Abstract-Datasets_and_Benchmarks_Track.html">Vision Dataset</a>
      <span class="paper-authors">Carol Wu</span>
    </div>
    <span class="paper-track-badge">Datasets and Benchmarks Track</span>
  </li>
</ul>
</body></html>`))
	}))
	defer srv.Close()

	oldBase := neuripsProceedingsBaseURL
	neuripsProceedingsBaseURL = srv.URL
	t.Cleanup(func() { neuripsProceedingsBaseURL = oldBase })

	out, err := searchConferenceProceedings(context.Background(), conferenceProceedingsInput{
		Venue: "NeurIPS", Year: 2025, Query: "Agent 相关", MaxResults: 5,
	})
	if err != nil {
		t.Fatalf("searchConferenceProceedings: %v", err)
	}
	if len(out.Papers) != 1 {
		t.Fatalf("应只召回 agent 相关论文,得到 %d", len(out.Papers))
	}
	p := out.Papers[0]
	if p.Title != "TheAgentCompany: Benchmarking LLM Agents" {
		t.Fatalf("title = %q", p.Title)
	}
	if len(p.Authors) != 2 || p.Authors[0] != "Alice Zhang" || p.Authors[1] != "Bob Li" {
		t.Fatalf("authors = %+v", p.Authors)
	}
	if p.Track != "Main Conference Track" {
		t.Fatalf("track = %q", p.Track)
	}
	wantPDF := srv.URL + "/paper_files/paper/2025/file/abc123-Paper-Conference.pdf"
	if p.PDFURL != wantPDF {
		t.Fatalf("pdf_url = %q, want %q", p.PDFURL, wantPDF)
	}
}

func TestOpenReviewSearchByVenueYear(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if got := q.Get("content.venueid"); got != "ICLR.cc/2025/Conference" {
			t.Fatalf("venueid = %q", got)
		}
		if got := q.Get("offset"); got != "0" {
			t.Fatalf("offset = %q", got)
		}
		_, _ = w.Write([]byte(`{"notes":[
  {"id":"n1","forum":"forum1","content":{
    "title":{"value":"Agentic Memory for LLM Agents"},
    "authors":{"value":["Alice Zhang","Bob Li"]},
    "keywords":{"value":["agent","memory"]},
    "abstract":{"value":"A memory system for LLM agents."},
    "venue":{"value":"ICLR 2025 Poster"},
    "venueid":{"value":"ICLR.cc/2025/Conference"},
    "pdf":{"value":"/pdf/forum1.pdf"},
    "_bibtex":{"value":"@inproceedings{agentic2025}"}
  }},
  {"id":"n2","forum":"forum2","content":{
    "title":{"value":"A Graph Method"},
    "authors":{"value":["Carol Wu"]},
    "abstract":{"value":"Graph learning."},
    "venue":{"value":"ICLR 2025 Poster"},
    "pdf":{"value":"/pdf/forum2.pdf"}
  }}
]}`))
	}))
	defer srv.Close()

	oldAPI, oldWeb := openReviewAPIBaseURL, openReviewWebBaseURL
	openReviewAPIBaseURL = srv.URL
	openReviewWebBaseURL = "https://openreview.test"
	t.Cleanup(func() {
		openReviewAPIBaseURL = oldAPI
		openReviewWebBaseURL = oldWeb
	})

	out, err := searchOpenReviewPapers(context.Background(), openReviewInput{
		Venue: "ICLR", Year: 2025, Query: "LLM agent", MaxResults: 10,
	})
	if err != nil {
		t.Fatalf("searchOpenReviewPapers: %v", err)
	}
	if len(out.Papers) != 1 {
		t.Fatalf("应只保留 agent 论文,得到 %d", len(out.Papers))
	}
	p := out.Papers[0]
	if p.URL != "https://openreview.test/forum?id=forum1" {
		t.Fatalf("url = %q", p.URL)
	}
	if p.PDFURL != "https://openreview.test/pdf/forum1.pdf" {
		t.Fatalf("pdf_url = %q", p.PDFURL)
	}
	if p.BibTeX == "" || !strings.Contains(p.BibTeX, "agentic2025") {
		t.Fatalf("bibtex = %q", p.BibTeX)
	}
}

func TestOpenReviewVenueIDResolution(t *testing.T) {
	id, err := resolveOpenReviewVenueID(openReviewInput{Venue: "NeurIPS", Year: 2025})
	if err != nil {
		t.Fatalf("resolveOpenReviewVenueID: %v", err)
	}
	if id != "NeurIPS.cc/2025/Conference" {
		t.Fatalf("id = %q", id)
	}
	if _, err := resolveOpenReviewVenueID(openReviewInput{Venue: "ACL", Year: 2025}); err == nil {
		t.Fatal("未知 venue 应提示传 venue_id")
	}
}
