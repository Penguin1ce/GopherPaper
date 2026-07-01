package paper

import (
	"errors"
	"testing"

	"GopherPaper/internal/model"
	"GopherPaper/pkg/constant"
	"GopherPaper/pkg/errs"
)

func TestBuildMindMapFromReadingDataClassifiesHighlightsAndNotes(t *testing.T) {
	graph := BuildMindMapFromReadingData(MindMapReadingData{
		Paper: &model.Paper{ID: "paper-1", Title: "Graph Paper"},
		Outline: []model.PaperSection{
			{ID: 10, PaperID: "paper-1", Level: 1, Title: "Introduction", PageNo: 2, OrderIdx: 1},
			{ID: 20, PaperID: "paper-1", Level: 1, Title: "Method", PageNo: 4, OrderIdx: 2},
			{ID: 30, PaperID: "paper-1", Level: 1, Title: "Empty", PageNo: 8, OrderIdx: 3},
		},
		Highlights: []model.PaperAnnotation{
			annotationAt(1, 2, 40, "intro highlight", "yellow", "note for intro"),
			annotationAt(2, 4, 10, "method highlight", "blue", ""),
			annotationAt(3, 1, 5, "before outline", "green", ""),
		},
		Annotations: []model.PaperAnnotation{
			annotationAt(1, 2, 40, "intro highlight", "yellow", "note for intro"),
		},
	})

	intro := requireNode(t, graph, "annotation:1")
	if intro.Type != constant.MindMapNodeAnnotation || intro.Data.SectionID != 10 {
		t.Fatalf("annotation:1 should be under section 10, got type=%s section=%d", intro.Type, intro.Data.SectionID)
	}
	if intro.Data.Text != "intro highlight" || intro.Data.BoundingRect == nil {
		t.Fatalf("annotation node should retain source highlight metadata: %#v", intro.Data)
	}
	if hasNode(graph, "highlight:1") {
		t.Fatal("noted highlight should not create an extra highlight node")
	}
	method := requireNode(t, graph, "highlight:2")
	if method.Data.SectionID != 20 {
		t.Fatalf("highlight:2 section = %d, want 20", method.Data.SectionID)
	}
	if intro.Data.Note != "note for intro" {
		t.Fatalf("annotation node not preserved: %#v", intro)
	}
	requireEdge(t, graph, "section:10", "annotation:1")
	requireEdge(t, graph, "group:unclassified", "highlight:3")
	if hasNode(graph, "section:30") {
		t.Fatal("empty section should be hidden")
	}
	if countNodes(graph, constant.MindMapNodeHighlight) != 2 {
		t.Fatalf("all highlights must appear, got %d", countNodes(graph, constant.MindMapNodeHighlight))
	}
	if countNodes(graph, constant.MindMapNodeAnnotation) != 1 {
		t.Fatalf("all note annotations must appear, got %d", countNodes(graph, constant.MindMapNodeAnnotation))
	}
}

func TestBuildMindMapFromReadingDataFallsBackWithoutOutline(t *testing.T) {
	graph := BuildMindMapFromReadingData(MindMapReadingData{
		Paper:      &model.Paper{ID: "paper-2", FileName: "fallback.pdf"},
		Highlights: []model.PaperAnnotation{annotationAt(7, 3, 20, "only highlight", "pink", "")},
	})

	group := requireNode(t, graph, "group:paper-excerpts")
	if group.Data.Label != "论文摘录" {
		t.Fatalf("fallback group label = %q", group.Data.Label)
	}
	requireEdge(t, graph, "group:paper-excerpts", "highlight:7")
}

func TestBuildMindMapFromReadingDataPrefersParentSectionOnSamePage(t *testing.T) {
	graph := BuildMindMapFromReadingData(MindMapReadingData{
		Paper: &model.Paper{ID: "paper-related", Title: "Related"},
		Outline: []model.PaperSection{
			{ID: 100, PaperID: "paper-related", Level: 1, Title: "3 Related Work", PageNo: 3, OrderIdx: 1},
			{ID: 110, PaperID: "paper-related", Level: 2, Title: "3.1 Flow-based Steering", PageNo: 3, OrderIdx: 2},
		},
		Highlights: []model.PaperAnnotation{
			annotationAt(11, 3, 20, "Linear activation steering.", "purple", ""),
			annotationAt(12, 3, 120, "Flow-based Steering", "orange", ""),
		},
	})

	related := requireNode(t, graph, "highlight:11")
	if related.Data.SectionID != 100 {
		t.Fatalf("same-page non-title highlight section = %d, want Related Work", related.Data.SectionID)
	}
	flowBased := requireNode(t, graph, "highlight:12")
	if flowBased.Data.SectionID != 110 {
		t.Fatalf("title-matching highlight section = %d, want Flow-based Steering", flowBased.Data.SectionID)
	}
	requireEdge(t, graph, "section:100", "section:110")
	requireEdge(t, graph, "section:100", "highlight:11")
	requireEdge(t, graph, "section:110", "highlight:12")
}

func TestBuildMindMapFromReadingDataUsesSectionYOnSamePage(t *testing.T) {
	graph := BuildMindMapFromReadingData(MindMapReadingData{
		Paper: &model.Paper{ID: "paper-flow", Title: "Flow"},
		Outline: []model.PaperSection{
			sectionAtY(200, "paper-flow", 1, "2 Related Work", 4, 1, 89),
			sectionAtY(300, "paper-flow", 1, "3 Method", 4, 2, 713),
			sectionAtY(310, "paper-flow", 2, "3.1 Flow-based Steering", 4, 3, 746),
		},
		Highlights: []model.PaperAnnotation{
			annotationAt(21, 4, 240, "Linear activation steering.", "purple", ""),
			annotationAt(22, 4, 690, "Flow matching has been explored.", "yellow", ""),
			annotationAt(26, 4, 700, "flow-based steering is discussed before Method.", "pink", ""),
			annotationAt(23, 4, 720, "method setup bridge.", "blue", ""),
			annotationAt(24, 4, 748, "Flow-based Steering", "orange", ""),
			annotationAt(25, 4, 780, "layers and hidden states.", "green", ""),
		},
	})

	if got := requireNode(t, graph, "highlight:21").Data.SectionID; got != 200 {
		t.Fatalf("Linear activation steering section = %d, want Related Work", got)
	}
	if got := requireNode(t, graph, "highlight:22").Data.SectionID; got != 200 {
		t.Fatalf("Flow matching section = %d, want Related Work", got)
	}
	if got := requireNode(t, graph, "highlight:26").Data.SectionID; got != 200 {
		t.Fatalf("flow-based steering mention before Method section = %d, want Related Work", got)
	}
	if got := requireNode(t, graph, "highlight:23").Data.SectionID; got != 300 {
		t.Fatalf("method bridge section = %d, want Method", got)
	}
	if got := requireNode(t, graph, "highlight:24").Data.SectionID; got != 310 {
		t.Fatalf("Flow-based Steering title section = %d, want Flow-based Steering", got)
	}
	if got := requireNode(t, graph, "highlight:25").Data.SectionID; got != 310 {
		t.Fatalf("layers and hidden section = %d, want Flow-based Steering", got)
	}
}

func TestBuildMindMapFromReadingDataKeepsOrphanAnnotations(t *testing.T) {
	graph := BuildMindMapFromReadingData(MindMapReadingData{
		Paper:      &model.Paper{ID: "paper-3", Title: "Orphans"},
		Highlights: []model.PaperAnnotation{annotationAt(1, 1, 20, "highlight", "yellow", "")},
		Annotations: []model.PaperAnnotation{
			annotationAt(2, 1, 30, "missing highlight text", "orange", "orphan note"),
		},
	})

	requireNode(t, graph, "group:paper-excerpts")
	orphan := requireNode(t, graph, "annotation:2")
	if orphan.Data.Note != "orphan note" {
		t.Fatalf("orphan annotation note = %q", orphan.Data.Note)
	}
	requireEdge(t, graph, "group:orphan-annotations", "annotation:2")
}

func TestValidateMindMapGraphRejectsBrokenReferences(t *testing.T) {
	graph := model.MindMapGraph{
		Version: 1,
		PaperID: "paper-1",
		Nodes: []model.MindMapNode{
			{
				ID:       "paper:paper-1",
				Type:     constant.MindMapNodePaper,
				Position: model.MindMapPosition{X: 0, Y: 0},
				Data:     model.MindMapNodeData{Label: "Paper"},
			},
		},
		Edges: []model.MindMapEdge{
			{ID: "paper:paper-1->missing", Source: "paper:paper-1", Target: "missing"},
		},
	}

	err := validateMindMapGraph(graph)
	if !errors.Is(err, errs.ErrMindMapInvalid) {
		t.Fatalf("validateMindMapGraph error = %v, want ErrMindMapInvalid", err)
	}
}

func sectionAtY(id uint64, paperID string, level int, title string, page, order int, y float64) model.PaperSection {
	x1, y1, x2, y2 := 10.0, y, 500.0, y+20
	return model.PaperSection{
		ID:       id,
		PaperID:  paperID,
		Level:    level,
		Title:    title,
		PageNo:   page,
		OrderIdx: order,
		X1:       &x1,
		Y1:       &y1,
		X2:       &x2,
		Y2:       &y2,
	}
}

func annotationAt(id uint64, page int, y float64, text, color, note string) model.PaperAnnotation {
	return model.PaperAnnotation{
		ID:      id,
		PaperID: "paper-1",
		OwnerID: "student-1",
		PageNo:  page,
		Text:    text,
		Note:    note,
		Color:   color,
		BoundingRect: model.AnnotationRect{
			X1: 10, Y1: y, X2: 80, Y2: y + 12, Width: 70, Height: 12, PageNumber: page,
		},
		Rects: model.AnnotationRects{
			{X1: 10, Y1: y, X2: 80, Y2: y + 12, Width: 70, Height: 12, PageNumber: page},
		},
	}
}

func requireNode(t *testing.T, graph model.MindMapGraph, id string) model.MindMapNode {
	t.Helper()
	for _, node := range graph.Nodes {
		if node.ID == id {
			return node
		}
	}
	t.Fatalf("node %s not found", id)
	return model.MindMapNode{}
}

func hasNode(graph model.MindMapGraph, id string) bool {
	for _, node := range graph.Nodes {
		if node.ID == id {
			return true
		}
	}
	return false
}

func requireEdge(t *testing.T, graph model.MindMapGraph, source, target string) {
	t.Helper()
	for _, edge := range graph.Edges {
		if edge.Source == source && edge.Target == target {
			return
		}
	}
	t.Fatalf("edge %s -> %s not found", source, target)
}

func countNodes(graph model.MindMapGraph, nodeType constant.MindMapNodeType) int {
	n := 0
	for _, node := range graph.Nodes {
		if node.Type == nodeType {
			n++
		}
	}
	return n
}
