import type {
  MindMapEdge,
  MindMapGraph,
  MindMapNode,
  MindMapNodeData,
  MindMapNodeType,
  Paper,
  PaperAnnotation,
  PaperSection,
} from "./types";

const ROOT_X = 40;
const ROOT_Y = 80;
const LEVEL_GAP = 300;
const ROW_GAP = 150;

interface BuildMindMapInput {
  paper: Paper | null;
  outline: PaperSection[];
  highlights: PaperAnnotation[];
  annotations: PaperAnnotation[];
}

interface SectionBuildNode {
  key: string;
  section: PaperSection;
  level: number;
  parentKey: string;
  children: string[];
  highlights: PaperAnnotation[];
  include: boolean;
}

export function buildMindMapFromReadingData({
  paper,
  outline,
  highlights,
  annotations,
}: BuildMindMapInput): MindMapGraph {
  const paperID = paper?.id || "";
  const rootID = paperNodeID(paperID);
  const graph: MindMapGraph = {
    version: 1,
    paper_id: paperID,
    nodes: [
      node(rootID, "paper", "", {
        label: paper?.title?.trim() || paper?.file_name?.trim() || "论文",
      }),
    ],
    edges: [],
    source_counts: {
      sections: outline.length,
      highlights: highlights.length,
      annotations: annotations.filter((item) => item.note?.trim()).length,
    },
  };

  const { sections, order } = buildSectionTree(outline);
  const highlightIDs = new Set<number>();
  const unclassified: PaperAnnotation[] = [];

  if (order.length === 0) {
    const groupID = "group:paper-excerpts";
    graph.nodes.push(node(groupID, "group", rootID, { label: "论文摘录", parentId: rootID }));
    graph.edges.push(edge(rootID, groupID));
    for (const h of [...highlights].sort(compareHighlights)) {
      highlightIDs.add(h.id);
      addReadingMark(graph, groupID, undefined, h, annotations);
    }
  } else {
    for (const h of highlights) {
      highlightIDs.add(h.id);
      const sectionKey = matchSectionForHighlight(sections, order, h);
      if (!sectionKey) {
        unclassified.push(h);
        continue;
      }
      sections.get(sectionKey)?.highlights.push(h);
      markIncluded(sections, sectionKey);
    }
    for (const section of sections.values()) {
      section.highlights.sort(compareHighlights);
    }
    appendIncludedSections(graph, rootID, sections, order, annotations);
  }

  unclassified.sort(compareHighlights);
  if (unclassified.length > 0) {
    const groupID = "group:unclassified";
    graph.nodes.push(node(groupID, "group", rootID, { label: "未归类摘录", parentId: rootID }));
    graph.edges.push(edge(rootID, groupID));
    for (const h of unclassified) addReadingMark(graph, groupID, undefined, h, annotations);
  }

  const orphanAnnotations = annotations
    .filter((item) => item.note?.trim() && !highlightIDs.has(item.id))
    .sort(compareHighlights);
  if (orphanAnnotations.length > 0) {
    const groupID = "group:orphan-annotations";
    graph.nodes.push(node(groupID, "group", rootID, { label: "未关联批注", parentId: rootID }));
    graph.edges.push(edge(rootID, groupID));
    for (const item of orphanAnnotations) addAnnotation(graph, groupID, undefined, item);
  }

  if (graph.source_counts) {
    graph.source_counts.unclassified = unclassified.length;
    graph.source_counts.orphan_annotations = orphanAnnotations.length;
  }
  layoutGraph(graph, rootID);
  return graph;
}

export function mergeMindMapGraphPositions(current: MindMapGraph, next: MindMapGraph): MindMapGraph {
  const oldNodes = new Map(current.nodes.map((item) => [item.id, item]));
  const nodeIDs = new Set<string>();
  const nodes = next.nodes.map((item) => {
    nodeIDs.add(item.id);
    const old = previousNodeForMerge(item, oldNodes);
    return old
      ? {
          ...item,
          position: old.position,
          data: { ...item.data, collapsed: old.data.collapsed },
        }
      : item;
  });
  for (const item of current.nodes) {
    if (nodeIDs.has(item.id) || !isManualNode(item)) continue;
    nodes.push(item);
    nodeIDs.add(item.id);
  }

  const edgeIDs = new Set(next.edges.map((item) => item.id));
  const edges = [...next.edges];
  for (const item of current.edges) {
    if (edgeIDs.has(item.id) || !isManualEdge(item, oldNodes)) continue;
    if (!nodeIDs.has(item.source) || !nodeIDs.has(item.target)) continue;
    edges.push(item);
    edgeIDs.add(item.id);
  }
  return { ...next, nodes, edges };
}

function previousNodeForMerge(node: MindMapNode, oldNodes: Map<string, MindMapNode>) {
  if (node.type === "annotation") {
    const id = node.data.highlightId || node.data.annotationId;
    if (typeof id === "number") {
      const oldHighlight = oldNodes.get(highlightNodeID(id));
      if (oldHighlight) return oldHighlight;
    }
  }
  return oldNodes.get(node.id);
}

function buildSectionTree(outline: PaperSection[]) {
  const ordered = [...outline].sort((a, b) => {
    if (a.order_idx !== b.order_idx) return a.order_idx - b.order_idx;
    if (a.page_no !== b.page_no) return a.page_no - b.page_no;
    return a.id - b.id;
  });
  const sections = new Map<string, SectionBuildNode>();
  const order: string[] = [];
  const stack: SectionBuildNode[] = [];
  ordered.forEach((section, index) => {
    const level = Math.max(1, Math.round(section.level || 1));
    const key = sectionKey(section, index);
    const item: SectionBuildNode = {
      key,
      section,
      level,
      parentKey: "",
      children: [],
      highlights: [],
      include: false,
    };
    while (stack.length > 0 && stack[stack.length - 1].level >= level) stack.pop();
    const parent = stack[stack.length - 1];
    if (parent) {
      item.parentKey = parent.key;
      parent.children.push(key);
    }
    sections.set(key, item);
    order.push(key);
    stack.push(item);
  });
  return { sections, order };
}

function appendIncludedSections(
  graph: MindMapGraph,
  rootID: string,
  sections: Map<string, SectionBuildNode>,
  order: string[],
  annotations: PaperAnnotation[],
) {
  for (const key of order) {
    const item = sections.get(key);
    if (!item?.include) continue;
    const parent = item.parentKey && sections.get(item.parentKey)?.include ? item.parentKey : rootID;
    graph.nodes.push(
      node(item.key, "section", parent, {
        label: item.section.title,
        parentId: parent,
        sectionId: item.section.id,
        pageNumber: item.section.page_no,
      }),
    );
    graph.edges.push(edge(parent, item.key));
    for (const h of item.highlights) addReadingMark(graph, item.key, item.section.id, h, annotations);
  }
}

function addReadingMark(
  graph: MindMapGraph,
  parentID: string,
  sectionID: number | undefined,
  h: PaperAnnotation,
  annotations: PaperAnnotation[],
) {
  const annotation = noteAnnotationForHighlight(h, annotations);
  if (annotation) {
    addAnnotation(graph, parentID, sectionID, annotation);
    return;
  }
  addHighlight(graph, parentID, sectionID, h);
}

function addHighlight(
  graph: MindMapGraph,
  parentID: string,
  sectionID: number | undefined,
  h: PaperAnnotation,
) {
  const id = highlightNodeID(h.id);
  graph.nodes.push(
    node(id, "highlight", parentID, {
      label: h.text,
      parentId: parentID,
      sectionId: sectionID,
      highlightId: h.id,
      pageNumber: h.page_no,
      text: h.text,
      color: h.color,
      boundingRect: h.bounding_rect,
      rects: h.rects,
    }),
  );
  graph.edges.push(edge(parentID, id));
}

function addAnnotation(
  graph: MindMapGraph,
  parentID: string,
  sectionID: number | undefined,
  ann: PaperAnnotation,
) {
  const note = ann.note?.trim();
  if (!note) return;
  const id = annotationNodeID(ann.id);
  graph.nodes.push(
    node(id, "annotation", parentID, {
      label: note,
      parentId: parentID,
      sectionId: sectionID,
      highlightId: ann.id,
      annotationId: ann.id,
      pageNumber: ann.page_no,
      text: ann.text,
      note,
      color: ann.color,
      boundingRect: ann.bounding_rect,
      rects: ann.rects,
    }),
  );
  graph.edges.push(edge(parentID, id));
}

function noteAnnotationForHighlight(h: PaperAnnotation, annotations: PaperAnnotation[]) {
  if (h.note?.trim()) return h;
  return annotations.find((item) => item.id === h.id && item.note?.trim()) || null;
}

function matchSectionForHighlight(sections: Map<string, SectionBuildNode>, order: string[], h: PaperAnnotation) {
  if (h.page_no <= 0) return "";
  const candidates = sectionCandidatesBeforePage(sections, order, h.page_no);
  if (candidates.length === 0) return "";

  const byY = matchSectionForHighlightByY(sections, candidates, h);
  if (byY) return byY;

  const highlightText = normalizeMindMapMatchText(h.text);
  for (let i = candidates.length - 1; i >= 0; i -= 1) {
    const section = sections.get(candidates[i]);
    if (!section || section.section.page_no !== h.page_no) continue;
    if (sectionHasY(section.section) && section.section.y1! > h.bounding_rect.y1) continue;
    if (sectionTitleMatchesHighlight(section.section.title, highlightText)) {
      return candidates[i];
    }
  }
  return matchSectionForHighlightFallback(sections, candidates, h);
}

function sectionCandidatesBeforePage(sections: Map<string, SectionBuildNode>, order: string[], pageNo: number) {
  const candidates: string[] = [];
  for (const key of order) {
    const section = sections.get(key)?.section;
    if (!section || section.page_no <= 0 || section.page_no > pageNo) continue;
    candidates.push(key);
  }
  return candidates;
}

function matchSectionForHighlightByY(
  sections: Map<string, SectionBuildNode>,
  candidates: string[],
  h: PaperAnnotation,
) {
  const highlightY = h.bounding_rect.y1;
  const hasCurrentPageBBox = candidates.some((key) => {
    const section = sections.get(key)?.section;
    return Boolean(section && section.page_no === h.page_no && sectionHasY(section));
  });
  if (!hasCurrentPageBBox) return "";

  let best = "";
  for (const key of candidates) {
    const section = sections.get(key)?.section;
    if (!section) continue;
    if (section.page_no < h.page_no) {
      best = key;
      continue;
    }
    if (section.page_no === h.page_no && sectionHasY(section) && section.y1! <= highlightY) {
      best = key;
    }
  }
  return best;
}

function matchSectionForHighlightFallback(
  sections: Map<string, SectionBuildNode>,
  candidates: string[],
  h: PaperAnnotation,
) {
  let bestPage = 0;
  const sameBestPage: string[] = [];
  for (const key of candidates) {
    const section = sections.get(key)?.section;
    if (!section) continue;
    if (section.page_no > bestPage) {
      bestPage = section.page_no;
      sameBestPage.length = 0;
    }
    if (section.page_no === bestPage) sameBestPage.push(key);
  }
  if (sameBestPage.length === 0) return "";
  if (bestPage !== h.page_no || sameBestPage.length === 1) return sameBestPage[sameBestPage.length - 1];

  return sameBestPage.reduce((best, key) => {
    const current = sections.get(key);
    const previous = sections.get(best);
    return current && previous && current.level < previous.level ? key : best;
  }, sameBestPage[0]);
}

function sectionTitleMatchesHighlight(title: string, highlightText: string) {
  const titleText = normalizeMindMapMatchText(title.replace(/^\s*((\d+(\.\d+)*)|[IVXLC]+|[A-Z])[.)]?\s+/i, ""));
  return titleText.length >= 4 && highlightText.includes(titleText);
}

function sectionHasY(section: PaperSection) {
  return typeof section.y1 === "number" && Number.isFinite(section.y1);
}

function normalizeMindMapMatchText(text: string) {
  return text.toLowerCase().replace(/[^a-z0-9]+/g, "");
}

function markIncluded(sections: Map<string, SectionBuildNode>, key: string) {
  let cur = key;
  while (cur) {
    const item = sections.get(cur);
    if (!item) return;
    item.include = true;
    cur = item.parentKey;
  }
}

function compareHighlights(a: PaperAnnotation, b: PaperAnnotation) {
  if (a.page_no !== b.page_no) return a.page_no - b.page_no;
  if (a.bounding_rect.y1 !== b.bounding_rect.y1) return a.bounding_rect.y1 - b.bounding_rect.y1;
  if (a.bounding_rect.x1 !== b.bounding_rect.x1) return a.bounding_rect.x1 - b.bounding_rect.x1;
  return a.id - b.id;
}

function layoutGraph(graph: MindMapGraph, rootID: string) {
  const nodeByID = new Map(graph.nodes.map((item, index) => [item.id, index]));
  const children = new Map<string, string[]>();
  for (const item of graph.edges) {
    if (!nodeByID.has(item.source) || !nodeByID.has(item.target)) continue;
    children.set(item.source, [...(children.get(item.source) || []), item.target]);
  }
  let cursor = ROOT_Y;
  const place = (id: string, depth: number): number => {
    const index = nodeByID.get(id);
    if (index == null) return cursor;
    const kids = children.get(id) || [];
    if (kids.length === 0) {
      const y = cursor;
      cursor += ROW_GAP;
      graph.nodes[index].position = { x: ROOT_X + depth * LEVEL_GAP, y };
      return y;
    }
    const ys = kids.map((child) => place(child, depth + 1));
    const y = (ys[0] + ys[ys.length - 1]) / 2;
    graph.nodes[index].position = { x: ROOT_X + depth * LEVEL_GAP, y };
    return y;
  };
  place(rootID, 0);
}

function node(id: string, type: MindMapNodeType, parentId: string, data: MindMapNodeData): MindMapNode {
  return {
    id,
    type,
    position: { x: ROOT_X, y: ROOT_Y },
    data: {
      ...data,
      label: data.label || type,
      parentId: data.parentId || parentId || undefined,
    },
  };
}

function edge(source: string, target: string, manual = false): MindMapEdge {
  return { id: `${source}->${target}`, source, target, data: manual ? { manual } : undefined };
}

function paperNodeID(id: string) {
  return id ? `paper:${id}` : "paper:unknown";
}

function highlightNodeID(id: number) {
  return `highlight:${id}`;
}

function annotationNodeID(id: number) {
  return `annotation:${id}`;
}

function sectionKey(section: PaperSection, index: number) {
  return section.id ? `section:${section.id}` : `section:order:${index}`;
}

function isManualNode(node?: MindMapNode) {
  return Boolean(node && (node.type === "manual" || node.data.manual));
}

function isManualEdge(edge: MindMapEdge, nodes: Map<string, MindMapNode>) {
  return Boolean(edge.data?.manual || isManualNode(nodes.get(edge.source)) || isManualNode(nodes.get(edge.target)));
}
