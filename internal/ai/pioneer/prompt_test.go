package pioneer

import (
	"strings"
	"testing"

	"GopherPaper/pkg/constant"
)

func TestPioneerRuntimeInstructionIncludesPaperLibraryControls(t *testing.T) {
	prompt := constant.PioneerRuntimeInstruction()
	for _, want := range []string{
		"list_my_papers",
		"search_my_papers",
		"search_conference_proceedings",
		"search_openreview_papers",
		"download_paper",
		"pdf_url",
		"不要为了同一篇论文重新查 Semantic Scholar 或 arXiv",
		"官方 proceedings/OpenReview",
		"本站论文 ID",
		"Semantic Scholar paper_id",
		"delete_my_paper",
		"confirmation_required",
		"弹窗",
		"强副作用",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("PioneerRuntimeInstruction missing %q", want)
		}
	}
}
