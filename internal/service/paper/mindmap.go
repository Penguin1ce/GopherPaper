package paper

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	paperdao "GopherPaper/internal/dao/paper"
	"GopherPaper/internal/model"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/constant"
	"GopherPaper/pkg/errs"
)

const (
	mindMapVersion = 1
	rootX          = 40.0
	rootY          = 80.0
	levelGap       = 300.0
	rowGap         = 150.0
)

var (
	mindMapHeadingNumberPattern = regexp.MustCompile(`(?i)^\s*((\d+(\.\d+)*)|[IVXLC]+|[A-Z])[.)]?\s+`)
	mindMapNonWordPattern       = regexp.MustCompile(`[^a-z0-9]+`)
)

type MindMapReadingData struct {
	Paper       *model.Paper
	Outline     []model.PaperSection
	Highlights  []model.PaperAnnotation
	Annotations []model.PaperAnnotation
}

type sectionBuildNode struct {
	key         string
	section     model.PaperSection
	level       int
	parentKey   string
	children    []string
	highlights  []model.PaperAnnotation
	annotations []model.PaperAnnotation
	include     bool
}

// BuildMindMapFromReadingData builds a deterministic graph from reading data only.
func BuildMindMapFromReadingData(in MindMapReadingData) model.MindMapGraph {
	paperID, title := mindMapPaperIdentity(in.Paper)
	graph := model.MindMapGraph{
		Version: mindMapVersion,
		PaperID: paperID,
		Nodes: []model.MindMapNode{
			newMindMapNode(paperNodeID(paperID), constant.MindMapNodePaper, "", model.MindMapNodeData{
				Label: title,
			}),
		},
		Edges: []model.MindMapEdge{},
		SourceCounts: map[string]int{
			"sections":    len(in.Outline),
			"highlights":  len(in.Highlights),
			"annotations": countNoteAnnotations(in.Annotations),
		},
	}

	rootID := paperNodeID(paperID)
	sections, sectionOrder := buildSectionTree(in.Outline)
	highlightIDs := make(map[uint64]bool, len(in.Highlights))
	unclassified := make([]model.PaperAnnotation, 0)

	if len(sectionOrder) == 0 {
		excerptGroupID := "group:paper-excerpts"
		graph.Nodes = append(graph.Nodes, newMindMapNode(excerptGroupID, constant.MindMapNodeGroup, rootID, model.MindMapNodeData{
			Label:    "论文摘录",
			ParentID: rootID,
		}))
		graph.Edges = append(graph.Edges, newMindMapEdge(rootID, excerptGroupID, false))
		sortHighlights(in.Highlights)
		for _, h := range in.Highlights {
			highlightIDs[h.ID] = true
			addReadingMarkNode(&graph, excerptGroupID, 0, h, in.Annotations)
		}
		for _, ann := range standaloneAnnotations(in.Annotations, highlightIDs) {
			highlightIDs[ann.ID] = true
			addAnnotationNode(&graph, excerptGroupID, 0, ann)
		}
	} else {
		for _, h := range in.Highlights {
			highlightIDs[h.ID] = true
			key := matchSectionForHighlight(sections, sectionOrder, h)
			if key == "" {
				unclassified = append(unclassified, h)
				continue
			}
			node := sections[key]
			node.highlights = append(node.highlights, h)
			markSectionIncluded(sections, key)
		}
		for _, ann := range standaloneAnnotations(in.Annotations, highlightIDs) {
			highlightIDs[ann.ID] = true
			key := matchSectionForHighlight(sections, sectionOrder, ann)
			if key == "" {
				unclassified = append(unclassified, ann)
				continue
			}
			node := sections[key]
			node.annotations = append(node.annotations, ann)
			markSectionIncluded(sections, key)
		}
		for _, key := range sectionOrder {
			sortHighlights(sections[key].highlights)
			sortHighlights(sections[key].annotations)
		}
		appendIncludedSections(&graph, rootID, sections, sectionOrder, in.Annotations)
	}

	sortHighlights(unclassified)
	if len(unclassified) > 0 {
		groupID := "group:unclassified"
		graph.Nodes = append(graph.Nodes, newMindMapNode(groupID, constant.MindMapNodeGroup, rootID, model.MindMapNodeData{
			Label:    "未归类摘录",
			ParentID: rootID,
		}))
		graph.Edges = append(graph.Edges, newMindMapEdge(rootID, groupID, false))
		for _, h := range unclassified {
			if mindMapAnnotationKind(h) == constant.AnnotationKindSelection {
				addReadingMarkNode(&graph, groupID, 0, h, in.Annotations)
			} else {
				addAnnotationNode(&graph, groupID, 0, h)
			}
		}
	}

	orphanAnnotations := orphanNoteAnnotations(in.Annotations, highlightIDs)
	if len(orphanAnnotations) > 0 {
		groupID := "group:orphan-annotations"
		graph.Nodes = append(graph.Nodes, newMindMapNode(groupID, constant.MindMapNodeGroup, rootID, model.MindMapNodeData{
			Label:    "未关联批注",
			ParentID: rootID,
		}))
		graph.Edges = append(graph.Edges, newMindMapEdge(rootID, groupID, false))
		for _, ann := range orphanAnnotations {
			addAnnotationNode(&graph, groupID, 0, ann)
		}
	}

	graph.SourceCounts["unclassified"] = len(unclassified)
	graph.SourceCounts["orphan_annotations"] = len(orphanAnnotations)
	layoutMindMap(&graph, rootID)
	return graph
}

func BuildMindMap(ctx context.Context, ownerID, paperID string) (*model.MindMap, error) {
	mindMap, err := buildCurrentMindMap(ctx, ownerID, paperID)
	if err != nil {
		return nil, err
	}
	if existing, err := paperdao.GetMindMapByPaper(ctx, ownerID, paperID); err == nil {
		mindMap.GraphJSON = mergeMindMapGraphs(existing.GraphJSON, mindMap.GraphJSON)
	} else if !errors.Is(err, errs.ErrMindMapNotFound) {
		return nil, err
	}
	mindMap.GraphJSON.UpdatedAt = time.Now()
	if err := paperdao.SaveMindMap(ctx, mindMap); err != nil {
		return nil, err
	}
	return mindMap, nil
}

func GetMindMap(ctx context.Context, ownerID, paperID string) (*model.MindMap, error) {
	if _, err := owned(ctx, ownerID, paperID); err != nil {
		return nil, err
	}
	return paperdao.GetMindMapByPaper(ctx, ownerID, paperID)
}

func UpdateMindMap(ctx context.Context, ownerID string, mindMapID uint64, graph model.MindMapGraph) (*model.MindMap, error) {
	mindMap, err := ownedMindMap(ctx, ownerID, mindMapID)
	if err != nil {
		return nil, err
	}
	graph.PaperID = mindMap.PaperID
	if graph.Version == 0 {
		graph.Version = mindMapVersion
	}
	if graph.Nodes == nil {
		graph.Nodes = []model.MindMapNode{}
	}
	if graph.Edges == nil {
		graph.Edges = []model.MindMapEdge{}
	}
	if err := validateMindMapGraph(graph); err != nil {
		return nil, err
	}
	graph.UpdatedAt = time.Now()
	if err := paperdao.UpdateMindMapGraph(ctx, mindMap.ID, graph); err != nil {
		return nil, err
	}
	mindMap.GraphJSON = graph
	return mindMap, nil
}

func SyncMindMap(ctx context.Context, ownerID string, mindMapID uint64) (*model.MindMap, error) {
	current, err := ownedMindMap(ctx, ownerID, mindMapID)
	if err != nil {
		return nil, err
	}
	next, err := buildCurrentMindMap(ctx, ownerID, current.PaperID)
	if err != nil {
		return nil, err
	}
	next.ID = current.ID
	next.GraphJSON = mergeMindMapGraphs(current.GraphJSON, next.GraphJSON)
	next.GraphJSON.UpdatedAt = time.Now()
	if err := paperdao.UpdateMindMapGraph(ctx, current.ID, next.GraphJSON); err != nil {
		return nil, err
	}
	return next, nil
}

func buildCurrentMindMap(ctx context.Context, ownerID, paperID string) (*model.MindMap, error) {
	p, err := owned(ctx, ownerID, paperID)
	if err != nil {
		return nil, err
	}
	sections, err := paperdao.ListSections(ctx, paperID)
	if err != nil {
		return nil, err
	}
	sections = ensureMindMapSectionCoordinates(ctx, p, sections)
	annotations, err := paperdao.ListAnnotations(ctx, paperID)
	if err != nil {
		return nil, err
	}
	highlights := make([]model.PaperAnnotation, 0, len(annotations))
	notes := make([]model.PaperAnnotation, 0, len(annotations))
	for _, ann := range annotations {
		if mindMapAnnotationKind(ann) == constant.AnnotationKindSelection {
			highlights = append(highlights, ann)
			if strings.TrimSpace(ann.Note) != "" {
				notes = append(notes, ann)
			}
			continue
		}
		notes = append(notes, ann)
	}
	graph := BuildMindMapFromReadingData(MindMapReadingData{
		Paper:       p,
		Outline:     sections,
		Highlights:  highlights,
		Annotations: notes,
	})
	return &model.MindMap{
		PaperID:   paperID,
		OwnerID:   ownerID,
		GraphJSON: graph,
	}, nil
}

func ensureMindMapSectionCoordinates(ctx context.Context, p *model.Paper, sections []model.PaperSection) []model.PaperSection {
	if p == nil || !sectionsNeedCoordinateBackfill(sections) {
		return sections
	}
	rebuilt, err := sectionsFromArtifactDir(p.ID, mineruDir(p.ID), p.Title)
	if err != nil {
		zlog.Warn("脑图章节坐标回填失败,沿用现有目录", "paper_id", p.ID, "err", err)
		return sections
	}
	if len(rebuilt) == 0 {
		return sections
	}
	if len(sections) > 0 && !sectionsHaveAnyCoordinates(rebuilt) {
		return sections
	}
	if err := paperdao.SaveSections(ctx, p.ID, rebuilt); err != nil {
		zlog.Warn("脑图章节坐标回填落库失败,沿用现有目录", "paper_id", p.ID, "err", err)
		return sections
	}
	saved, err := paperdao.ListSections(ctx, p.ID)
	if err != nil {
		zlog.Warn("脑图章节坐标回填后重新读取失败,使用重建目录", "paper_id", p.ID, "err", err)
		return rebuilt
	}
	return saved
}

func sectionsNeedCoordinateBackfill(sections []model.PaperSection) bool {
	if len(sections) == 0 {
		return true
	}
	for _, section := range sections {
		if !sectionHasBBox(section) {
			return true
		}
	}
	return false
}

func sectionsHaveAnyCoordinates(sections []model.PaperSection) bool {
	for _, section := range sections {
		if sectionHasBBox(section) {
			return true
		}
	}
	return false
}

func validateMindMapGraph(graph model.MindMapGraph) error {
	if graph.PaperID == "" {
		return fmt.Errorf("%w: 缺少论文 ID", errs.ErrMindMapInvalid)
	}
	if len(graph.Nodes) == 0 {
		return fmt.Errorf("%w: 至少需要一个节点", errs.ErrMindMapInvalid)
	}

	nodeIDs := make(map[string]bool, len(graph.Nodes))
	for _, node := range graph.Nodes {
		id := strings.TrimSpace(node.ID)
		if id == "" {
			return fmt.Errorf("%w: 节点 ID 为空", errs.ErrMindMapInvalid)
		}
		if nodeIDs[id] {
			return fmt.Errorf("%w: 节点 ID 重复 %s", errs.ErrMindMapInvalid, id)
		}
		if !node.Type.Valid() {
			return fmt.Errorf("%w: 节点类型无效 %s", errs.ErrMindMapInvalid, node.Type)
		}
		nodeIDs[id] = true
	}

	for _, node := range graph.Nodes {
		parentID := strings.TrimSpace(node.Data.ParentID)
		if parentID == "" {
			continue
		}
		if parentID == node.ID {
			return fmt.Errorf("%w: 节点不能连接自己 %s", errs.ErrMindMapInvalid, node.ID)
		}
		if !nodeIDs[parentID] {
			return fmt.Errorf("%w: 父节点不存在 %s", errs.ErrMindMapInvalid, parentID)
		}
	}

	edgeIDs := make(map[string]bool, len(graph.Edges))
	for _, edge := range graph.Edges {
		if strings.TrimSpace(edge.ID) == "" {
			return fmt.Errorf("%w: 边 ID 为空", errs.ErrMindMapInvalid)
		}
		if edgeIDs[edge.ID] {
			return fmt.Errorf("%w: 边 ID 重复 %s", errs.ErrMindMapInvalid, edge.ID)
		}
		edgeIDs[edge.ID] = true
		source := strings.TrimSpace(edge.Source)
		target := strings.TrimSpace(edge.Target)
		if source == "" || target == "" {
			return fmt.Errorf("%w: 边端点为空", errs.ErrMindMapInvalid)
		}
		if source == target {
			return fmt.Errorf("%w: 边不能连接自己 %s", errs.ErrMindMapInvalid, source)
		}
		if !nodeIDs[source] {
			return fmt.Errorf("%w: 边起点不存在 %s", errs.ErrMindMapInvalid, source)
		}
		if !nodeIDs[target] {
			return fmt.Errorf("%w: 边终点不存在 %s", errs.ErrMindMapInvalid, target)
		}
	}
	return nil
}

func ownedMindMap(ctx context.Context, ownerID string, mindMapID uint64) (*model.MindMap, error) {
	mindMap, err := paperdao.GetMindMap(ctx, mindMapID)
	if err != nil {
		return nil, err
	}
	if mindMap.OwnerID != ownerID {
		return nil, errs.ErrPaperForbidden
	}
	if _, err := owned(ctx, ownerID, mindMap.PaperID); err != nil {
		return nil, err
	}
	return mindMap, nil
}

func mindMapPaperIdentity(p *model.Paper) (string, string) {
	if p == nil {
		return "", "论文"
	}
	title := strings.TrimSpace(p.Title)
	if title == "" {
		title = strings.TrimSpace(p.FileName)
	}
	if title == "" {
		title = "论文"
	}
	return p.ID, title
}

func buildSectionTree(outline []model.PaperSection) (map[string]*sectionBuildNode, []string) {
	sections := append([]model.PaperSection(nil), outline...)
	sort.SliceStable(sections, func(i, j int) bool {
		if sections[i].OrderIdx != sections[j].OrderIdx {
			return sections[i].OrderIdx < sections[j].OrderIdx
		}
		if sections[i].PageNo != sections[j].PageNo {
			return sections[i].PageNo < sections[j].PageNo
		}
		return sections[i].ID < sections[j].ID
	})

	nodes := make(map[string]*sectionBuildNode, len(sections))
	order := make([]string, 0, len(sections))
	stack := make([]*sectionBuildNode, 0)
	for idx, section := range sections {
		level := section.Level
		if level <= 0 {
			level = 1
		}
		key := sectionKey(section, idx)
		node := &sectionBuildNode{key: key, section: section, level: level}
		for len(stack) > 0 && stack[len(stack)-1].level >= level {
			stack = stack[:len(stack)-1]
		}
		if len(stack) > 0 {
			node.parentKey = stack[len(stack)-1].key
			stack[len(stack)-1].children = append(stack[len(stack)-1].children, key)
		}
		nodes[key] = node
		order = append(order, key)
		stack = append(stack, node)
	}
	return nodes, order
}

func sectionKey(section model.PaperSection, idx int) string {
	if section.ID > 0 {
		return fmt.Sprintf("section:%d", section.ID)
	}
	return fmt.Sprintf("section:order:%d", idx)
}

func matchSectionForHighlight(sections map[string]*sectionBuildNode, order []string, h model.PaperAnnotation) string {
	if h.PageNo <= 0 {
		return ""
	}
	candidates := sectionCandidatesBeforePage(sections, order, h.PageNo)
	if len(candidates) == 0 {
		return ""
	}

	if key := matchSectionForHighlightByY(sections, candidates, h); key != "" {
		return key
	}

	highlightText := normalizeMindMapMatchText(h.Text)
	for i := len(candidates) - 1; i >= 0; i-- {
		section := sections[candidates[i]]
		if section == nil || section.section.PageNo != h.PageNo {
			continue
		}
		if sectionHasY(section.section) && section.section.Y1 != nil && *section.section.Y1 > h.BoundingRect.Y1 {
			continue
		}
		if sectionTitleMatchesHighlight(section.section.Title, highlightText) {
			return candidates[i]
		}
	}
	return matchSectionForHighlightFallback(sections, candidates, h)
}

func sectionCandidatesBeforePage(sections map[string]*sectionBuildNode, order []string, pageNo int) []string {
	candidates := make([]string, 0, len(order))
	for _, key := range order {
		section := sections[key].section
		if section.PageNo <= 0 || section.PageNo > pageNo {
			continue
		}
		candidates = append(candidates, key)
	}
	return candidates
}

func matchSectionForHighlightByY(sections map[string]*sectionBuildNode, candidates []string, h model.PaperAnnotation) string {
	highlightY := h.BoundingRect.Y1
	hasCurrentPageBBox := false
	for _, key := range candidates {
		section := sections[key].section
		if section.PageNo == h.PageNo && sectionHasY(section) {
			hasCurrentPageBBox = true
			break
		}
	}
	if !hasCurrentPageBBox {
		return ""
	}

	best := ""
	for _, key := range candidates {
		section := sections[key].section
		switch {
		case section.PageNo < h.PageNo:
			best = key
		case section.PageNo == h.PageNo:
			if !sectionHasY(section) || section.Y1 == nil || *section.Y1 > highlightY {
				continue
			}
			best = key
		}
	}
	return best
}

func matchSectionForHighlightFallback(sections map[string]*sectionBuildNode, candidates []string, h model.PaperAnnotation) string {
	bestPage := 0
	sameBestPage := make([]string, 0, len(candidates))
	for _, key := range candidates {
		section := sections[key].section
		if section.PageNo > bestPage {
			bestPage = section.PageNo
			sameBestPage = sameBestPage[:0]
		}
		if section.PageNo == bestPage {
			sameBestPage = append(sameBestPage, key)
		}
	}
	if len(sameBestPage) == 0 {
		return ""
	}
	if bestPage != h.PageNo || len(sameBestPage) == 1 {
		return sameBestPage[len(sameBestPage)-1]
	}

	best := sameBestPage[0]
	for _, key := range sameBestPage[1:] {
		if sections[key].level < sections[best].level {
			best = key
		}
	}
	return best
}

func sectionHasY(section model.PaperSection) bool {
	return section.Y1 != nil
}

func sectionHasBBox(section model.PaperSection) bool {
	return section.X1 != nil && section.Y1 != nil && section.X2 != nil && section.Y2 != nil
}

func sectionTitleMatchesHighlight(title, highlightText string) bool {
	titleText := normalizeMindMapMatchText(mindMapHeadingNumberPattern.ReplaceAllString(title, ""))
	return len(titleText) >= 4 && strings.Contains(highlightText, titleText)
}

func normalizeMindMapMatchText(text string) string {
	return mindMapNonWordPattern.ReplaceAllString(strings.ToLower(text), "")
}

func markSectionIncluded(sections map[string]*sectionBuildNode, key string) {
	for key != "" {
		node := sections[key]
		if node == nil {
			return
		}
		node.include = true
		key = node.parentKey
	}
}

func appendIncludedSections(graph *model.MindMapGraph, rootID string, sections map[string]*sectionBuildNode, order []string, annotations []model.PaperAnnotation) {
	for _, key := range order {
		section := sections[key]
		if section == nil || !section.include {
			continue
		}
		parentID := rootID
		if section.parentKey != "" && sections[section.parentKey] != nil && sections[section.parentKey].include {
			parentID = section.parentKey
		}
		graph.Nodes = append(graph.Nodes, newMindMapNode(section.key, constant.MindMapNodeSection, parentID, model.MindMapNodeData{
			Label:      section.section.Title,
			ParentID:   parentID,
			SectionID:  section.section.ID,
			PageNumber: section.section.PageNo,
		}))
		graph.Edges = append(graph.Edges, newMindMapEdge(parentID, section.key, false))
		for _, h := range section.highlights {
			addReadingMarkNode(graph, section.key, section.section.ID, h, annotations)
		}
		for _, ann := range section.annotations {
			addAnnotationNode(graph, section.key, section.section.ID, ann)
		}
	}
}

func addReadingMarkNode(graph *model.MindMapGraph, parentID string, sectionID uint64, h model.PaperAnnotation, annotations []model.PaperAnnotation) {
	if ann, ok := noteAnnotationForHighlight(h, annotations); ok {
		addAnnotationNode(graph, parentID, sectionID, ann)
		return
	}
	addHighlightNode(graph, parentID, sectionID, h)
}

func addHighlightNode(graph *model.MindMapGraph, parentID string, sectionID uint64, h model.PaperAnnotation) {
	rect := h.BoundingRect
	highlightID := highlightNodeID(h.ID)
	graph.Nodes = append(graph.Nodes, newMindMapNode(highlightID, constant.MindMapNodeHighlight, parentID, model.MindMapNodeData{
		Label:        h.Text,
		ParentID:     parentID,
		SectionID:    sectionID,
		HighlightID:  h.ID,
		PageNumber:   h.PageNo,
		Text:         h.Text,
		Color:        h.Color,
		BoundingRect: &rect,
		Rects:        append(model.AnnotationRects(nil), h.Rects...),
	}))
	graph.Edges = append(graph.Edges, newMindMapEdge(parentID, highlightID, false))
}

func addAnnotationNode(graph *model.MindMapGraph, parentID string, sectionID uint64, ann model.PaperAnnotation) {
	label := annotationMindMapLabel(ann)
	if label == "" {
		return
	}
	rect := ann.BoundingRect
	nodeID := annotationNodeID(ann.ID)
	graph.Nodes = append(graph.Nodes, newMindMapNode(nodeID, constant.MindMapNodeAnnotation, parentID, model.MindMapNodeData{
		Label:        label,
		ParentID:     parentID,
		SectionID:    sectionID,
		HighlightID:  ann.ID,
		AnnotationID: ann.ID,
		PageNumber:   ann.PageNo,
		Text:         ann.Text,
		Note:         strings.TrimSpace(ann.Note),
		Color:        ann.Color,
		BoundingRect: &rect,
		Rects:        append(model.AnnotationRects(nil), ann.Rects...),
		Meta: map[string]any{
			"kind": mindMapAnnotationKind(ann),
		},
	}))
	graph.Edges = append(graph.Edges, newMindMapEdge(parentID, nodeID, false))
}

func noteAnnotationForHighlight(h model.PaperAnnotation, annotations []model.PaperAnnotation) (model.PaperAnnotation, bool) {
	if strings.TrimSpace(h.Note) != "" {
		return h, true
	}
	for _, ann := range annotations {
		if ann.ID == h.ID && strings.TrimSpace(ann.Note) != "" {
			return ann, true
		}
	}
	return model.PaperAnnotation{}, false
}

func standaloneAnnotations(annotations []model.PaperAnnotation, linkedIDs map[uint64]bool) []model.PaperAnnotation {
	out := make([]model.PaperAnnotation, 0)
	for _, ann := range annotations {
		if linkedIDs[ann.ID] {
			continue
		}
		if mindMapAnnotationKind(ann) != constant.AnnotationKindSelection {
			out = append(out, ann)
		}
	}
	sortHighlights(out)
	return out
}

func orphanNoteAnnotations(annotations []model.PaperAnnotation, highlightIDs map[uint64]bool) []model.PaperAnnotation {
	out := make([]model.PaperAnnotation, 0)
	for _, ann := range annotations {
		if (strings.TrimSpace(ann.Note) != "" || mindMapAnnotationKind(ann) != constant.AnnotationKindSelection) && !highlightIDs[ann.ID] {
			out = append(out, ann)
		}
	}
	sortHighlights(out)
	return out
}

func countNoteAnnotations(annotations []model.PaperAnnotation) int {
	n := 0
	for _, ann := range annotations {
		if strings.TrimSpace(ann.Note) != "" || mindMapAnnotationKind(ann) != constant.AnnotationKindSelection {
			n++
		}
	}
	return n
}

func mindMapAnnotationKind(ann model.PaperAnnotation) constant.AnnotationKind {
	if ann.Kind == "" {
		return constant.AnnotationKindSelection
	}
	if ann.Kind.Valid() {
		return ann.Kind
	}
	return constant.AnnotationKindSelection
}

func annotationMindMapLabel(ann model.PaperAnnotation) string {
	if note := strings.TrimSpace(ann.Note); note != "" {
		return note
	}
	if text := strings.TrimSpace(ann.Text); text != "" {
		return text
	}
	if mindMapAnnotationKind(ann) == constant.AnnotationKindDrawing {
		return "手绘标注"
	}
	return ""
}

func sortHighlights(items []model.PaperAnnotation) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].PageNo != items[j].PageNo {
			return items[i].PageNo < items[j].PageNo
		}
		if items[i].BoundingRect.Y1 != items[j].BoundingRect.Y1 {
			return items[i].BoundingRect.Y1 < items[j].BoundingRect.Y1
		}
		if items[i].BoundingRect.X1 != items[j].BoundingRect.X1 {
			return items[i].BoundingRect.X1 < items[j].BoundingRect.X1
		}
		return items[i].ID < items[j].ID
	})
}

func newMindMapNode(id string, nodeType constant.MindMapNodeType, parentID string, data model.MindMapNodeData) model.MindMapNode {
	if data.Label == "" {
		data.Label = string(nodeType)
	}
	if data.ParentID == "" {
		data.ParentID = parentID
	}
	return model.MindMapNode{
		ID:       id,
		Type:     nodeType,
		Position: model.MindMapPosition{X: rootX, Y: rootY},
		Data:     data,
	}
}

func newMindMapEdge(source, target string, manual bool) model.MindMapEdge {
	return model.MindMapEdge{
		ID:     fmt.Sprintf("%s->%s", source, target),
		Source: source,
		Target: target,
		Data:   model.MindMapEdgeData{Manual: manual},
	}
}

func paperNodeID(paperID string) string {
	if paperID == "" {
		return "paper:unknown"
	}
	return "paper:" + paperID
}

func highlightNodeID(id uint64) string {
	return fmt.Sprintf("highlight:%d", id)
}

func annotationNodeID(id uint64) string {
	return fmt.Sprintf("annotation:%d", id)
}

func layoutMindMap(graph *model.MindMapGraph, rootID string) {
	nodeIndex := make(map[string]int, len(graph.Nodes))
	for i := range graph.Nodes {
		nodeIndex[graph.Nodes[i].ID] = i
	}
	children := make(map[string][]string, len(graph.Nodes))
	for _, edge := range graph.Edges {
		if _, ok := nodeIndex[edge.Source]; ok {
			if _, ok := nodeIndex[edge.Target]; ok {
				children[edge.Source] = append(children[edge.Source], edge.Target)
			}
		}
	}
	cursor := rootY
	var place func(id string, depth int) float64
	place = func(id string, depth int) float64 {
		idx, ok := nodeIndex[id]
		if !ok {
			return cursor
		}
		kids := children[id]
		if len(kids) == 0 {
			y := cursor
			cursor += rowGap
			graph.Nodes[idx].Position = model.MindMapPosition{X: rootX + float64(depth)*levelGap, Y: y}
			return y
		}
		first := place(kids[0], depth+1)
		last := first
		for _, childID := range kids[1:] {
			last = place(childID, depth+1)
		}
		y := (first + last) / 2
		graph.Nodes[idx].Position = model.MindMapPosition{X: rootX + float64(depth)*levelGap, Y: y}
		return y
	}
	place(rootID, 0)
}

func mergeMindMapGraphs(current, next model.MindMapGraph) model.MindMapGraph {
	currentNodes := make(map[string]model.MindMapNode, len(current.Nodes))
	for _, node := range current.Nodes {
		currentNodes[node.ID] = node
	}
	nextNodeIDs := make(map[string]bool, len(next.Nodes))
	for i := range next.Nodes {
		nextNodeIDs[next.Nodes[i].ID] = true
		if old, ok := preferredPreviousMindMapNode(next.Nodes[i], currentNodes); ok {
			next.Nodes[i].Position = old.Position
			next.Nodes[i].Data.Collapsed = old.Data.Collapsed
		}
	}
	for _, node := range current.Nodes {
		if nextNodeIDs[node.ID] || !isManualMindMapNode(node) {
			continue
		}
		next.Nodes = append(next.Nodes, node)
		nextNodeIDs[node.ID] = true
	}

	edgeIDs := make(map[string]bool, len(next.Edges))
	for _, edge := range next.Edges {
		edgeIDs[edge.ID] = true
	}
	for _, edge := range current.Edges {
		if edgeIDs[edge.ID] || !isManualMindMapEdge(edge, currentNodes) {
			continue
		}
		if !nextNodeIDs[edge.Source] || !nextNodeIDs[edge.Target] {
			continue
		}
		next.Edges = append(next.Edges, edge)
		edgeIDs[edge.ID] = true
	}
	return next
}

func preferredPreviousMindMapNode(next model.MindMapNode, current map[string]model.MindMapNode) (model.MindMapNode, bool) {
	if next.Type == constant.MindMapNodeAnnotation {
		if id := next.Data.HighlightID; id > 0 {
			if old, ok := current[highlightNodeID(id)]; ok {
				return old, true
			}
		}
	}
	old, ok := current[next.ID]
	return old, ok
}

func isManualMindMapNode(node model.MindMapNode) bool {
	return node.Type == constant.MindMapNodeManual || node.Data.Manual
}

func isManualMindMapEdge(edge model.MindMapEdge, nodes map[string]model.MindMapNode) bool {
	if edge.Data.Manual {
		return true
	}
	return isManualMindMapNode(nodes[edge.Source]) || isManualMindMapNode(nodes[edge.Target])
}
