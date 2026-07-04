"use client";

import {
  ArrowLeft,
  BookOpenText,
  Check,
  Copy,
  Download,
  FileText,
  GitCompareArrows,
  Hash,
  Maximize2,
  MessageCircle,
  MousePointer2,
  Pin,
  PinOff,
  Redo2,
  RefreshCw,
  RotateCcw,
  Search,
  Tags,
  Undo2,
  X,
  ZoomIn,
  ZoomOut,
} from "lucide-react";
import Link from "next/link";
import {
  PointerEvent as ReactPointerEvent,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";

import { Button, buttonVariants } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import * as api from "@/lib/gopherpaper/api";
import {
  PAPER_CHAT_DRAFT_KEY,
  PIONEER_REFERENCE_DRAFT_KEY,
} from "@/lib/gopherpaper/navigation-drafts";
import { useApp } from "@/lib/gopherpaper/store";
import type {
  EntityGraph,
  EntityGraphEdge,
  EntityGraphNode,
  GraphRebuildJob,
  NameCount,
} from "@/lib/gopherpaper/types";
import { paperTitle } from "@/lib/gopherpaper/utils";
import { cn } from "@/lib/utils";
import { Empty } from "./app-ui";
import { AgentIntro } from "./agent-intro";

const VIEW_W = 1000;
const VIEW_H = 620;
const MIN_ZOOM = 0.35;
const MAX_ZOOM = 2.6;
const NODE_DRAG_THRESHOLD = 8;
const GRAPH_KEYWORD_LIMIT = 10000;
const DEFAULT_GRAPH_DISPLAY_LIMIT = 60;
const ALL_RELATIONSHIPS_FILTER = "*";
const HISTORY_LIMIT = 30;
const MINIMAP_W = 168;
const MINIMAP_H = 96;
const PAPER_NODE_COLOR = "#35A98D";
const GRAPH_EDGE_COLOR = "#94a3b8";
const GRAPH_SELECTED_COLOR = "#22c55e";
const GRAPH_TOOL_BUTTON_CLASS =
  "w-auto bg-emerald-600 text-white hover:bg-emerald-500 hover:text-white dark:bg-emerald-600 dark:text-white dark:hover:bg-emerald-500";
const GRAPH_TOP_BUTTON_CLASS =
  "border-slate-200 bg-white shadow-sm hover:bg-slate-50 aria-expanded:bg-white dark:border-slate-200 dark:bg-white dark:text-slate-900 dark:hover:bg-slate-50";
const PAPER_ID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const QUESTION_CONTEXT_NODE_TYPES = new Set([
  "experiment",
  "future_work",
  "innovation",
  "limitation",
  "research_question",
  "result",
  "method",
  "keyword",
]);
const QUESTION_CONTEXT_LABELS: Record<string, string> = {
  experiment: "实验",
  future_work: "未来工作",
  innovation: "创新点",
  limitation: "局限性",
  research_question: "研究问题",
  result: "结果",
  method: "研究方法",
  keyword: "关键词",
};

const TYPE_META: Record<string, { label: string; color: string; radius: number }> = {
  paper: { label: "Paper", color: PAPER_NODE_COLOR, radius: 36 },
  author: { label: "Author", color: "#6CCB72", radius: 30 },
  affiliation: { label: "Affiliation", color: "#36B9B5", radius: 30 },
  keyword: { label: "Keyword", color: "#4EA8F1", radius: 30 },
  research_question: { label: "ResearchQuestion", color: "#A88AF0", radius: 30 },
  method: { label: "Method", color: "#F0A14A", radius: 28 },
  experiment: { label: "Experiment", color: "#DDB33F", radius: 27 },
  result: { label: "Result", color: "#E66C73", radius: 28 },
  innovation: { label: "Innovation", color: "#E578B7", radius: 26 },
  limitation: { label: "Limitation", color: "#8A94A6", radius: 26 },
  future_work: { label: "FutureWork", color: "#35BFD0", radius: 25 },
  reference: { label: "Reference", color: "#F2C84B", radius: 25 },
  venue: { label: "Venue", color: "#7184A1", radius: 22 },
  entity: { label: "Entity", color: "#8A94A6", radius: 21 },
};

type SimNode = EntityGraphNode & {
  x: number;
  y: number;
  vx: number;
  vy: number;
  fx?: number;
  fy?: number;
};
type SelectedItem =
  | { kind: "node"; node: EntityGraphNode }
  | {
      kind: "edge";
      edge: EntityGraphEdge;
      source?: EntityGraphNode;
      target?: EntityGraphNode;
    };

type GraphKeywordItem = {
  name: string;
  count?: number;
};

type GraphFilter =
  | { kind: "all" }
  | { kind: "node"; type: string; expanded: boolean }
  | { kind: "edge"; type: string };

type SceneSnapshot = {
  nodes: Array<{
    id: string;
    x: number;
    y: number;
    fx?: number;
    fy?: number;
  }>;
  zoom: number;
  pan: { x: number; y: number };
  pinnedNodeIDs: string[];
  graphFilter: GraphFilter;
  expandedNodeIDs: string[];
  displayLimit: number;
};

type BoxSelection = {
  x1: number;
  y1: number;
  x2: number;
  y2: number;
};

function clamp(n: number, min: number, max: number) {
  return Math.max(min, Math.min(max, n));
}

function shortText(s: string, n = 18) {
  const t = (s || "").trim();
  if (!t) return "未命名";
  const chars = [...t];
  return chars.length > n ? `${chars.slice(0, n).join("")}...` : t;
}

function nodeLabelText(node: EntityGraphNode) {
  const radius = nodeMeta(node.type).radius;
  const maxChars = node.type === "paper" ? 6 : Math.max(4, Math.floor(radius / 4.6));
  return shortText(node.label, maxChars);
}

function nodeLabelFontSize(node: EntityGraphNode) {
  const length = [...nodeLabelText(node)].length;
  if (node.type === "paper") return length > 7 ? 9.5 : 10.5;
  if (length > 6) return 8.5;
  if (length > 4) return 9.5;
  return 10.5;
}

function nodeLabelLines(node: EntityGraphNode) {
  const perLine = node.type === "paper" ? 7 : 6;
  const text = shortText(node.label, perLine * 2);
  const chars = [...text];
  if (chars.length <= perLine) return [text];
  return [
    chars.slice(0, perLine).join(""),
    chars.slice(perLine, perLine * 2).join(""),
  ];
}

function edgeKey(edge: EntityGraphEdge) {
  return edge.id || `${edge.source}:${edge.type}:${edge.target}:${edge.label}`;
}

function edgeLinePoints(a: SimNode, b: SimNode) {
  const dx = b.x - a.x;
  const dy = b.y - a.y;
  const dist = Math.max(1, Math.hypot(dx, dy));
  const ax = (dx / dist) * (nodeMeta(a.type).radius + 5);
  const ay = (dy / dist) * (nodeMeta(a.type).radius + 5);
  const bx = (dx / dist) * (nodeMeta(b.type).radius + 8);
  const by = (dy / dist) * (nodeMeta(b.type).radius + 8);
  return {
    x1: a.x + ax,
    y1: a.y + ay,
    x2: b.x - bx,
    y2: b.y - by,
  };
}

function nodeMeta(type: string) {
  return TYPE_META[type] || TYPE_META.entity;
}

function normalizePaperID(value?: string) {
  const id = (value || "").trim().replace(/^paper:/, "");
  return PAPER_ID_RE.test(id) ? id : "";
}

function normalizePaperTitle(value?: string) {
  return (value || "").trim().replace(/\.pdf$/i, "").toLowerCase();
}

function paperIDFromNode(node: EntityGraphNode) {
  return normalizePaperID(node.details?.paper_id) || normalizePaperID(node.id);
}

function referenceSearchText(node: EntityGraphNode) {
  const raw = (node.details?.title || node.details?.name || node.label || "").trim();
  if (!raw) return "";
  const quoted = raw.match(/"([^"]{8,180})"/);
  const text = quoted?.[1] || raw.replace(/^\s*(?:\[\d+\]|\(\d+\)|\d+\.)\s*/, "");
  return text.replace(/\s+/g, " ").trim();
}

function edgeQuestionContext(
  selected: Extract<SelectedItem, { kind: "edge" }>,
) {
  const source = selected.source;
  const target = selected.target;
  if (!source || !target) return null;
  const paper = source.type === "paper" ? source : target.type === "paper" ? target : null;
  const entity = paper === source ? target : paper === target ? source : null;
  if (!paper || !entity || !QUESTION_CONTEXT_NODE_TYPES.has(entity.type)) {
    return null;
  }
  return { paper, entity };
}

async function copyTextToClipboard(text: string) {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(text);
    return;
  }
  const textarea = document.createElement("textarea");
  textarea.value = text;
  textarea.setAttribute("readonly", "");
  textarea.style.position = "fixed";
  textarea.style.left = "-9999px";
  textarea.style.opacity = "0";
  document.body.appendChild(textarea);
  textarea.select();
  document.execCommand("copy");
  textarea.remove();
}

function GraphDetailField({
  label,
  value,
  copyKey,
  copiedKey,
  onCopy,
}: {
  label: string;
  value: string;
  copyKey: string;
  copiedKey: string;
  onCopy: (key: string, value: string) => void;
}) {
  if (!value) return null;
  const copied = copiedKey === copyKey;
  return (
    <div className="border-b border-border/70 py-3 last:border-b-0">
      <div className="mb-1.5 flex items-center justify-between gap-3">
        <span className="text-xs font-medium text-muted-foreground">
          {label}
        </span>
        <Button
          type="button"
          variant="ghost"
          size="icon-xs"
          className="text-muted-foreground hover:text-foreground"
          title={`复制${label}`}
          aria-label={`复制${label}`}
          onClick={() => onCopy(copyKey, value)}
        >
          {copied ? (
            <Check className="size-3.5 text-emerald-600" />
          ) : (
            <Copy className="size-3.5" />
          )}
        </Button>
      </div>
      <div className="whitespace-pre-wrap break-words text-sm leading-6 text-foreground">
        {value}
      </div>
    </div>
  );
}

function edgeTypeLabel(edge: EntityGraphEdge) {
  return edge.type || edge.label || "RELATION";
}

function dedupeEntityNodes(nodes: EntityGraphNode[]) {
  const result = new Map<string, EntityGraphNode>();
  for (const node of nodes) {
    if (!node?.id) continue;
    const current = result.get(node.id);
    result.set(
      node.id,
      current
        ? {
            ...current,
            label: current.label || node.label,
            type: current.type || node.type,
            details: { ...(current.details || {}), ...(node.details || {}) },
          }
        : node,
    );
  }
  return [...result.values()];
}

function dedupeEntityEdges(edges: EntityGraphEdge[]) {
  const result = new Map<string, EntityGraphEdge>();
  for (const edge of edges) {
    if (!edge?.source || !edge?.target) continue;
    const key = edgeKey(edge);
    const current = result.get(key);
    result.set(
      key,
      current
        ? {
            ...current,
            label: current.label || edge.label,
            type: current.type || edge.type,
            details: { ...(current.details || {}), ...(edge.details || {}) },
          }
        : edge,
    );
  }
  return [...result.values()];
}

function collisionRadius(node: Pick<EntityGraphNode, "type" | "label">) {
  const base = nodeMeta(node.type).radius;
  const labelPad = Math.min(17, Math.max(5, [...(node.label || "")].length * 1.25));
  return base + labelPad;
}

function sceneBounds(nodes: SimNode[]) {
  if (nodes.length === 0) {
    return { minX: 0, minY: 0, maxX: VIEW_W, maxY: VIEW_H };
  }
  const xs = nodes.map((node) => node.x);
  const ys = nodes.map((node) => node.y);
  const padding = 80;
  return {
    minX: Math.min(...xs) - padding,
    minY: Math.min(...ys) - padding,
    maxX: Math.max(...xs) + padding,
    maxY: Math.max(...ys) + padding,
  };
}

function ForceEntityGraph({
  graph,
  baseGraph,
  mode = "detail",
  keywordItems = [],
  showKeywordCount = false,
  onPaperRead,
  resolvePaperID,
}: {
  graph: EntityGraph;
  baseGraph?: EntityGraph | null;
  mode?: "overview" | "detail";
  keywordItems?: GraphKeywordItem[];
  showKeywordCount?: boolean;
  onPaperRead?: (paperID: string) => void;
  resolvePaperID?: (node: EntityGraphNode) => string;
}) {
  const svgRef = useRef<SVGSVGElement | null>(null);
  const nodesRef = useRef<SimNode[]>([]);
  const edgesRef = useRef<EntityGraphEdge[]>([]);
  const alphaRef = useRef(0.75);
  const dragRef = useRef<{
    mode: "node" | "pan" | "box";
    id?: string;
    startX: number;
    startY: number;
    panX: number;
    panY: number;
    moved: boolean;
    offsetX?: number;
    offsetY?: number;
    additive?: boolean;
    before?: SceneSnapshot;
  } | null>(null);
  const movedRef = useRef(false);
  const boxSelectionRef = useRef<BoxSelection | null>(null);
  const pendingSceneSnapshotRef = useRef<SceneSnapshot | null>(null);
  const [, setVersion] = useState(0);
  const [zoom, setZoom] = useState(1);
  const [pan, setPan] = useState({ x: 0, y: 0 });
  const [selected, setSelected] = useState<SelectedItem | null>(null);
  const [searchOpen, setSearchOpen] = useState(false);
  const [searchTerm, setSearchTerm] = useState("");
  const [keywordsOpen, setKeywordsOpen] = useState(false);
  const [limitOpen, setLimitOpen] = useState(false);
  const [downloadOpen, setDownloadOpen] = useState(false);
  const [displayLimit, setDisplayLimit] = useState(DEFAULT_GRAPH_DISPLAY_LIMIT);
  const [displayLimitDraft, setDisplayLimitDraft] = useState(
    String(DEFAULT_GRAPH_DISPLAY_LIMIT),
  );
  const [graphFilter, setGraphFilter] = useState<GraphFilter>({ kind: "all" });
  const [expandedNodeIDs, setExpandedNodeIDs] = useState<string[]>([]);
  const [selectedNodeIDs, setSelectedNodeIDs] = useState<string[]>([]);
  const [pinnedNodeIDs, setPinnedNodeIDs] = useState<string[]>([]);
  const [boxSelectMode, setBoxSelectMode] = useState(false);
  const [boxSelection, setBoxSelection] = useState<BoxSelection | null>(null);
  const [undoStack, setUndoStack] = useState<SceneSnapshot[]>([]);
  const [redoStack, setRedoStack] = useState<SceneSnapshot[]>([]);
  const [copiedDetailKey, setCopiedDetailKey] = useState("");
  const filterClickTimerRef = useRef<number | null>(null);
  const copyResetTimerRef = useRef<number | null>(null);
  const paperIDForNode = useCallback(
    (node: EntityGraphNode) => resolvePaperID?.(node) || paperIDFromNode(node),
    [resolvePaperID],
  );
  const copyDetailValue = useCallback(async (key: string, value: string) => {
    try {
      await copyTextToClipboard(value);
      setCopiedDetailKey(key);
      if (copyResetTimerRef.current != null) {
        window.clearTimeout(copyResetTimerRef.current);
      }
      copyResetTimerRef.current = window.setTimeout(() => {
        setCopiedDetailKey("");
        copyResetTimerRef.current = null;
      }, 1400);
    } catch {
      setCopiedDetailKey("");
    }
  }, []);

  const allGraphNodes = useMemo(
    () => dedupeEntityNodes(Array.isArray(graph.nodes) ? graph.nodes : []),
    [graph.nodes],
  );
  const allGraphEdges = useMemo(
    () => dedupeEntityEdges(Array.isArray(graph.edges) ? graph.edges : []),
    [graph.edges],
  );
  const allNodeByID = useMemo(() => new Map(allGraphNodes.map((node) => [node.id, node])), [allGraphNodes]);
  const baseGraphEdges = useMemo(
    () => (Array.isArray(baseGraph?.edges) ? baseGraph.edges : allGraphEdges),
    [allGraphEdges, baseGraph],
  );
  const baseGraphNodeIDs = useMemo(() => {
    const ids = new Set<string>();
    for (const edge of baseGraphEdges) {
      ids.add(edge.source);
      ids.add(edge.target);
    }
    if (baseGraph?.nodes?.length) {
      for (const node of baseGraph.nodes) ids.add(node.id);
    }
    if (ids.size === 0) {
      for (const node of allGraphNodes) {
        if (node.type === "paper") ids.add(node.id);
      }
    }
    return ids;
  }, [allGraphNodes, baseGraph, baseGraphEdges]);
  const expandedNodeIDSet = useMemo(() => new Set(expandedNodeIDs), [expandedNodeIDs]);
  const baseFilteredEdges = useMemo(() => {
    if (graphFilter.kind === "edge") {
      return graphFilter.type === ALL_RELATIONSHIPS_FILTER
        ? allGraphEdges
        : allGraphEdges.filter((edge) => edge.type === graphFilter.type);
    }
    if (graphFilter.kind === "node") {
      if (!graphFilter.expanded) return [];
      if (graphFilter.type === "paper") {
        return allGraphEdges.filter((edge) => allNodeByID.get(edge.source)?.type === "paper" || allNodeByID.get(edge.target)?.type === "paper");
      }
      return allGraphEdges.filter((edge) => {
        const source = allNodeByID.get(edge.source);
        const target = allNodeByID.get(edge.target);
        return (
          (source?.type === graphFilter.type && target?.type === "paper") ||
          (target?.type === graphFilter.type && source?.type === "paper")
        );
      });
    }
    return baseGraphEdges;
  }, [allGraphEdges, allNodeByID, baseGraphEdges, graphFilter]);
  const expandedNodeEdges = useMemo(() => {
    if (expandedNodeIDSet.size === 0) return [];
    return allGraphEdges.filter((edge) => {
      const expandedIDs = [edge.source, edge.target].filter((id) => expandedNodeIDSet.has(id));
      if (expandedIDs.length === 0) return false;
      return expandedIDs.some((id) => {
        const node = allNodeByID.get(id);
        if (node?.type === "paper") return true;
        const otherID = id === edge.source ? edge.target : edge.source;
        return allNodeByID.get(otherID)?.type === "paper";
      });
    });
  }, [allGraphEdges, allNodeByID, expandedNodeIDSet]);
  const graphEdgeCandidates = useMemo(
    () => dedupeEntityEdges([...expandedNodeEdges, ...baseFilteredEdges]),
    [baseFilteredEdges, expandedNodeEdges],
  );
  const graphNodeCandidates = useMemo(() => {
    const ids = new Set<string>();
    if (graphFilter.kind === "all") {
      for (const id of baseGraphNodeIDs) ids.add(id);
    } else if (graphFilter.kind === "edge") {
      for (const edge of baseFilteredEdges) {
        ids.add(edge.source);
        ids.add(edge.target);
      }
    } else {
      if (graphFilter.expanded) {
        for (const edge of baseFilteredEdges) {
          ids.add(edge.source);
          ids.add(edge.target);
        }
      }
      for (const node of allGraphNodes) {
        if (node.type === graphFilter.type) ids.add(node.id);
      }
    }
    for (const edge of expandedNodeEdges) {
      ids.add(edge.source);
      ids.add(edge.target);
    }
    for (const id of expandedNodeIDs) {
      ids.add(id);
    }
    return allGraphNodes.filter((node) => ids.has(node.id));
  }, [allGraphNodes, baseFilteredEdges, baseGraphNodeIDs, expandedNodeEdges, expandedNodeIDs, graphFilter]);
  const visibleGraphNodeIDs = useMemo(() => {
    const ids = new Set<string>();
    const add = (id: string) => {
      if (ids.size < displayLimit) ids.add(id);
    };
    for (const id of expandedNodeIDs) add(id);
    for (const edge of graphEdgeCandidates) {
      add(edge.source);
      add(edge.target);
    }
    for (const node of graphNodeCandidates) add(node.id);
    return ids;
  }, [displayLimit, expandedNodeIDs, graphEdgeCandidates, graphNodeCandidates]);
  const graphNodes = useMemo(
    () => allGraphNodes.filter((node) => visibleGraphNodeIDs.has(node.id)),
    [allGraphNodes, visibleGraphNodeIDs],
  );
  const graphEdges = useMemo(
    () =>
      graphEdgeCandidates
        .filter((edge) => visibleGraphNodeIDs.has(edge.source) && visibleGraphNodeIDs.has(edge.target))
        .slice(0, displayLimit),
    [displayLimit, graphEdgeCandidates, visibleGraphNodeIDs],
  );
  const graphStructureKey = useMemo(
    () => [
      mode,
      graphNodes.map((node) => node.id).join("|"),
      graphEdges.map(edgeKey).join("|"),
    ].join("::"),
    [graphEdges, graphNodes, mode],
  );
  const rawNodeFilterItems = useMemo(() => {
    const counts = new Map<string, number>();
    for (const node of allGraphNodes) counts.set(node.type, (counts.get(node.type) || 0) + 1);
    return [...counts.entries()]
      .map(([type, count]) => ({ type, count, meta: nodeMeta(type) }))
      .sort((a, b) => Object.keys(TYPE_META).indexOf(a.type) - Object.keys(TYPE_META).indexOf(b.type));
  }, [allGraphNodes]);
  const nodeFilterItems = rawNodeFilterItems;
  const rawEdgeFilterItems = useMemo(() => {
    const counts = new Map<string, { count: number; label: string }>();
    for (const edge of allGraphEdges) {
      const current = counts.get(edge.type);
      counts.set(edge.type, {
        count: (current?.count || 0) + 1,
        label: edgeTypeLabel(edge),
      });
    }
    return [...counts.entries()]
      .map(([type, item]) => ({ type, count: item.count, label: item.label }))
      .sort((a, b) => a.label.localeCompare(b.label, "en"));
  }, [allGraphEdges]);
  const edgeFilterItems = rawEdgeFilterItems;
  const selectedNodeIDSet = useMemo(
    () => new Set(selectedNodeIDs),
    [selectedNodeIDs],
  );
  const pinnedNodeIDSet = useMemo(
    () => new Set(pinnedNodeIDs),
    [pinnedNodeIDs],
  );
  const captureSceneSnapshot = useCallback(
    (): SceneSnapshot => ({
      nodes: nodesRef.current.map((node) => ({
        id: node.id,
        x: node.x,
        y: node.y,
        fx: node.fx,
        fy: node.fy,
      })),
      zoom,
      pan: { ...pan },
      pinnedNodeIDs: [...pinnedNodeIDs],
      graphFilter,
      expandedNodeIDs: [...expandedNodeIDs],
      displayLimit,
    }),
    [
      displayLimit,
      expandedNodeIDs,
      graphFilter,
      pan,
      pinnedNodeIDs,
      zoom,
    ],
  );

  const applySnapshotPositions = useCallback((snapshot: SceneSnapshot) => {
    const positions = new Map(snapshot.nodes.map((node) => [node.id, node]));
    const pinned = new Set(snapshot.pinnedNodeIDs);
    for (const node of nodesRef.current) {
      const saved = positions.get(node.id);
      if (!saved) continue;
      node.x = saved.x;
      node.y = saved.y;
      node.vx = 0;
      node.vy = 0;
      node.fx = pinned.has(node.id) ? saved.x : undefined;
      node.fy = pinned.has(node.id) ? saved.y : undefined;
    }
    alphaRef.current = 0.25;
    setVersion((version) => version + 1);
  }, []);

  const restoreSceneSnapshot = useCallback(
    (snapshot: SceneSnapshot) => {
      pendingSceneSnapshotRef.current = snapshot;
      setZoom(snapshot.zoom);
      setPan(snapshot.pan);
      setPinnedNodeIDs(snapshot.pinnedNodeIDs);
      setGraphFilter(snapshot.graphFilter);
      setExpandedNodeIDs(snapshot.expandedNodeIDs);
      setDisplayLimit(snapshot.displayLimit);
      applySnapshotPositions(snapshot);
    },
    [applySnapshotPositions],
  );

  const recordSceneSnapshot = useCallback(
    (snapshot = captureSceneSnapshot()) => {
      setUndoStack((current) => [
        ...current.slice(-(HISTORY_LIMIT - 1)),
        snapshot,
      ]);
      setRedoStack([]);
    },
    [captureSceneSnapshot],
  );

  const undoScene = useCallback(() => {
    const snapshot = undoStack.at(-1);
    if (!snapshot) return;
    const current = captureSceneSnapshot();
    setUndoStack((stack) => stack.slice(0, -1));
    setRedoStack((stack) => [
      ...stack.slice(-(HISTORY_LIMIT - 1)),
      current,
    ]);
    restoreSceneSnapshot(snapshot);
  }, [captureSceneSnapshot, restoreSceneSnapshot, undoStack]);

  const redoScene = useCallback(() => {
    const snapshot = redoStack.at(-1);
    if (!snapshot) return;
    const current = captureSceneSnapshot();
    setRedoStack((stack) => stack.slice(0, -1));
    setUndoStack((stack) => [
      ...stack.slice(-(HISTORY_LIMIT - 1)),
      current,
    ]);
    restoreSceneSnapshot(snapshot);
  }, [captureSceneSnapshot, redoStack, restoreSceneSnapshot]);

  const clearFilterClickTimer = useCallback(() => {
    if (filterClickTimerRef.current == null) return;
    window.clearTimeout(filterClickTimerRef.current);
    filterClickTimerRef.current = null;
  }, []);

  const selectAllFilter = useCallback(() => {
    clearFilterClickTimer();
    recordSceneSnapshot();
    setGraphFilter({ kind: "all" });
    setExpandedNodeIDs([]);
    setDisplayLimit(DEFAULT_GRAPH_DISPLAY_LIMIT);
  }, [clearFilterClickTimer, recordSceneSnapshot]);

  const selectAllRelationships = useCallback(() => {
    clearFilterClickTimer();
    recordSceneSnapshot();
    setGraphFilter({ kind: "edge", type: ALL_RELATIONSHIPS_FILTER });
    setExpandedNodeIDs([]);
    setDisplayLimit(DEFAULT_GRAPH_DISPLAY_LIMIT);
    setDisplayLimitDraft(String(DEFAULT_GRAPH_DISPLAY_LIMIT));
  }, [clearFilterClickTimer, recordSceneSnapshot]);

  const selectNodeFilter = useCallback(
    (type: string) => {
      clearFilterClickTimer();
      const before = captureSceneSnapshot();
      filterClickTimerRef.current = window.setTimeout(() => {
        recordSceneSnapshot(before);
        setGraphFilter({ kind: "node", type, expanded: false });
        setExpandedNodeIDs([]);
        setDisplayLimit(DEFAULT_GRAPH_DISPLAY_LIMIT);
        filterClickTimerRef.current = null;
      }, 180);
    },
    [captureSceneSnapshot, clearFilterClickTimer, recordSceneSnapshot],
  );

  const toggleNodeExpansion = useCallback(
    (type: string) => {
      clearFilterClickTimer();
      recordSceneSnapshot();
      setDisplayLimit(DEFAULT_GRAPH_DISPLAY_LIMIT);
      setGraphFilter((current) =>
        current.kind === "node" && current.type === type && current.expanded
          ? { kind: "node", type, expanded: false }
          : { kind: "node", type, expanded: true },
      );
    },
    [clearFilterClickTimer, recordSceneSnapshot],
  );

  const toggleNodeRelationExpansion = useCallback((node: EntityGraphNode) => {
    recordSceneSnapshot();
    setExpandedNodeIDs((current) =>
      current.includes(node.id)
        ? current.filter((id) => id !== node.id)
        : [...current, node.id],
    );
  }, [recordSceneSnapshot]);

  const selectEdgeFilter = useCallback(
    (type: string) => {
      clearFilterClickTimer();
      recordSceneSnapshot();
      setGraphFilter({ kind: "edge", type });
      setExpandedNodeIDs([]);
      setDisplayLimit(DEFAULT_GRAPH_DISPLAY_LIMIT);
      setDisplayLimitDraft(String(DEFAULT_GRAPH_DISPLAY_LIMIT));
    },
    [clearFilterClickTimer, recordSceneSnapshot],
  );

  useEffect(
    () => () => {
      clearFilterClickTimer();
      if (copyResetTimerRef.current != null) {
        window.clearTimeout(copyResetTimerRef.current);
      }
    },
    [clearFilterClickTimer],
  );

  const resetLayout = (clearPinned = false) => {
    const center = { x: VIEW_W / 2, y: VIEW_H / 2 };
    const next: SimNode[] = [];
    if (mode === "detail") {
      const others = graphNodes.filter((n) => n.type !== "paper");
      const paper = graphNodes.find((n) => n.type === "paper") || graphNodes[0];
      if (paper) next.push({ ...paper, x: center.x, y: center.y, vx: 0, vy: 0 });
      others.forEach((n, i) => {
        const angle = -Math.PI / 2 + (i * Math.PI * 2) / Math.max(1, others.length);
        const ring = 92 + 20 * (i % 3);
        next.push({
          ...n,
          x: center.x + Math.cos(angle) * ring,
          y: center.y + Math.sin(angle) * ring,
          vx: 0,
          vy: 0,
        });
      });
    } else {
      const paperNodes = graphNodes.filter((n) => n.type === "paper");
      const nonPaperNodes = graphNodes.filter((n) => n.type !== "paper");
      const paperPositions = new Map<string, { x: number; y: number }>();
      const paperRing = paperNodes.length <= 1 ? 0 : clamp(44 + paperNodes.length * 10, 72, 135);

      paperNodes.forEach((n, i) => {
        const angle = -Math.PI / 2 + (i * Math.PI * 2) / Math.max(1, paperNodes.length);
        const x = center.x + Math.cos(angle) * paperRing;
        const y = center.y + Math.sin(angle) * paperRing;
        paperPositions.set(n.id, { x, y });
        next.push({ ...n, x, y, vx: 0, vy: 0 });
      });

      const edgePapersByNode = new Map<string, string[]>();
      for (const edge of graphEdges) {
        const sourceIsPaper = edge.source.startsWith("paper:");
        const targetIsPaper = edge.target.startsWith("paper:");
        if (sourceIsPaper && !targetIsPaper) {
          edgePapersByNode.set(edge.target, [...(edgePapersByNode.get(edge.target) || []), edge.source]);
        }
        if (targetIsPaper && !sourceIsPaper) {
          edgePapersByNode.set(edge.source, [...(edgePapersByNode.get(edge.source) || []), edge.target]);
        }
      }

      const singlePaperNodeCounts = new Map<string, number>();
      nonPaperNodes.forEach((n, i) => {
        const connectedPapers = [...new Set(edgePapersByNode.get(n.id) || [])]
          .map((paperID) => paperPositions.get(paperID))
          .filter(Boolean) as { x: number; y: number }[];
        if (connectedPapers.length === 1) {
          const paperID = (edgePapersByNode.get(n.id) || [])[0];
          const slot = singlePaperNodeCounts.get(paperID) || 0;
          singlePaperNodeCounts.set(paperID, slot + 1);
          const angle = -Math.PI / 2 + slot * 2.3999632297;
          const ring = 82 + 20 * (slot % 3);
          const base = connectedPapers[0];
          next.push({
            ...n,
            x: base.x + Math.cos(angle) * ring,
            y: base.y + Math.sin(angle) * ring,
            vx: 0,
            vy: 0,
          });
          return;
        }
        if (connectedPapers.length > 1) {
          const x = connectedPapers.reduce((sum, p) => sum + p.x, 0) / connectedPapers.length;
          const y = connectedPapers.reduce((sum, p) => sum + p.y, 0) / connectedPapers.length;
          const angle = -Math.PI / 2 + (i * Math.PI * 2) / Math.max(1, nonPaperNodes.length);
          next.push({ ...n, x: x + Math.cos(angle) * 26, y: y + Math.sin(angle) * 26, vx: 0, vy: 0 });
          return;
        }
        const angle = -Math.PI / 2 + (i * Math.PI * 2) / Math.max(1, graphNodes.length);
        const ring = 80 + 16 * (i % 3);
        next.push({ ...n, x: center.x + Math.cos(angle) * ring, y: center.y + Math.sin(angle) * ring, vx: 0, vy: 0 });
      });
    }
    const retainedPinnedIDs = clearPinned
      ? new Set<string>()
      : pinnedNodeIDSet;
    for (const node of next) {
      if (!retainedPinnedIDs.has(node.id)) continue;
      node.fx = node.x;
      node.fy = node.y;
    }
    if (clearPinned) setPinnedNodeIDs([]);
    nodesRef.current = next;
    edgesRef.current = graphEdges;
    alphaRef.current = 0.9;
    setPan({ x: 0, y: 0 });
    setVersion((v) => v + 1);
  };

  useEffect(() => {
    setGraphFilter({ kind: "all" });
    setExpandedNodeIDs([]);
    setSelectedNodeIDs([]);
    setPinnedNodeIDs([]);
    setDisplayLimit(DEFAULT_GRAPH_DISPLAY_LIMIT);
    setLimitOpen(false);
    setSelected(null);
    setZoom(1);
    setSearchTerm("");
    setKeywordsOpen(false);
    setUndoStack([]);
    setRedoStack([]);
  }, [graph, mode]);

  useEffect(() => {
    resetLayout();
    const pending = pendingSceneSnapshotRef.current;
    if (pending) {
      applySnapshotPositions(pending);
      setZoom(pending.zoom);
      setPan(pending.pan);
      pendingSceneSnapshotRef.current = null;
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [graphStructureKey]);

  useEffect(() => {
    let raf = 0;
    const tick = () => {
      const nodes = nodesRef.current;
      const nodeByID = new Map(nodes.map((n) => [n.id, n]));
      const alpha = alphaRef.current;
      if (nodes.length > 1 && alpha > 0.01) {
        for (let i = 0; i < nodes.length; i += 1) {
          for (let j = i + 1; j < nodes.length; j += 1) {
            const a = nodes[i];
            const b = nodes[j];
            let dx = b.x - a.x;
            let dy = b.y - a.y;
            let d2 = dx * dx + dy * dy;
            if (d2 < 1) {
              dx = 1;
              dy = 0;
              d2 = 1;
            }
            const dist = Math.sqrt(d2);
            const minDist = collisionRadius(a) + collisionRadius(b) + (mode === "overview" ? 6 : 0);
            const overlap = Math.max(0, minDist - dist);
            const force = (60 * alpha) / d2 + (overlap * 0.02) / dist;
            const fx = dx * force;
            const fy = dy * force;
            if ((mode === "overview" || a.type !== "paper") && a.fx == null) {
              a.vx -= fx;
              a.vy -= fy;
            }
            if ((mode === "overview" || b.type !== "paper") && b.fx == null) {
              b.vx += fx;
              b.vy += fy;
            }
          }
        }

        for (const edge of edgesRef.current) {
          const a = nodeByID.get(edge.source);
          const b = nodeByID.get(edge.target);
          if (!a || !b) continue;
          const dx = b.x - a.x;
          const dy = b.y - a.y;
          const dist = Math.max(1, Math.hypot(dx, dy));
          const ideal = Math.max(
            mode === "overview" ? 22 : b.type === "keyword" || b.type === "author" ? 8 : 10,
            collisionRadius(a) + collisionRadius(b) + (mode === "overview" ? 6 : 0),
          );
          const pull = (dist - ideal) * (mode === "overview" ? 0.028 : 0.022) * alpha;
          const fx = (dx / dist) * pull;
          const fy = (dy / dist) * pull;
          if (a.fx == null) {
            a.vx += fx;
            a.vy += fy;
          }
          if (b.fx == null) {
            b.vx -= fx;
            b.vy -= fy;
          }
        }

        for (const n of nodes) {
          if (n.fx != null && n.fy != null) {
            n.x = n.fx;
            n.y = n.fy;
            n.vx = 0;
            n.vy = 0;
            continue;
          }
          const centerStrength = n.type === "paper" ? (mode === "overview" ? 0.007 : 0.025) : mode === "overview" ? 0.005 : 0.004;
          n.vx += (VIEW_W / 2 - n.x) * centerStrength * alpha;
          n.vy += (VIEW_H / 2 - n.y) * centerStrength * alpha;
          n.vx = clamp(n.vx * 0.86, -22, 22);
          n.vy = clamp(n.vy * 0.86, -22, 22);
          n.x = clamp(n.x + n.vx, -1200, 2200);
          n.y = clamp(n.y + n.vy, -900, 1600);
        }
        alphaRef.current *= 0.992;
        setVersion((v) => v + 1);
      }
      raf = requestAnimationFrame(tick);
    };
    raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf);
  }, [graph, mode]);

  const graphToWorld = useCallback((clientX: number, clientY: number) => {
    const rect = svgRef.current?.getBoundingClientRect();
    if (!rect) return { x: VIEW_W / 2, y: VIEW_H / 2 };
    const gx = ((clientX - rect.left) / rect.width) * VIEW_W;
    const gy = ((clientY - rect.top) / rect.height) * VIEW_H;
    return { x: (gx - pan.x) / zoom, y: (gy - pan.y) / zoom };
  }, [pan.x, pan.y, zoom]);

  const graphPoint = useCallback((clientX: number, clientY: number) => {
    const rect = svgRef.current?.getBoundingClientRect();
    if (!rect) return { x: 0, y: 0 };
    return { x: ((clientX - rect.left) / rect.width) * VIEW_W, y: ((clientY - rect.top) / rect.height) * VIEW_H };
  }, []);

  useEffect(() => {
    const el = svgRef.current;
    if (!el) return;
    const handleWheel = (event: WheelEvent) => {
      event.preventDefault();
      const before = graphToWorld(event.clientX, event.clientY);
      const point = graphPoint(event.clientX, event.clientY);
      const nextZoom = clamp(zoom * (event.deltaY > 0 ? 0.9 : 1.1), MIN_ZOOM, MAX_ZOOM);
      setZoom(nextZoom);
      setPan({ x: point.x - before.x * nextZoom, y: point.y - before.y * nextZoom });
    };
    el.addEventListener("wheel", handleWheel, { passive: false });
    return () => el.removeEventListener("wheel", handleWheel);
  }, [graphPoint, graphToWorld, zoom]);

  const onNodeDown = (event: ReactPointerEvent<SVGGElement>, id: string) => {
    event.stopPropagation();
    event.currentTarget.setPointerCapture(event.pointerId);
    const p = graphToWorld(event.clientX, event.clientY);
    const node = nodesRef.current.find((n) => n.id === id);
    alphaRef.current = Math.max(alphaRef.current, 0.45);
    movedRef.current = false;
    dragRef.current = {
      mode: "node",
      id,
      startX: event.clientX,
      startY: event.clientY,
      panX: pan.x,
      panY: pan.y,
      moved: false,
      offsetX: node ? node.x - p.x : 0,
      offsetY: node ? node.y - p.y : 0,
      before: captureSceneSnapshot(),
    };
  };

  const onSvgDown = (event: ReactPointerEvent<SVGSVGElement>) => {
    event.currentTarget.setPointerCapture(event.pointerId);
    movedRef.current = false;
    if (boxSelectMode) {
      const point = graphPoint(event.clientX, event.clientY);
      dragRef.current = {
        mode: "box",
        startX: event.clientX,
        startY: event.clientY,
        panX: pan.x,
        panY: pan.y,
        moved: false,
        additive: event.shiftKey || event.ctrlKey || event.metaKey,
      };
      const selection = {
        x1: point.x,
        y1: point.y,
        x2: point.x,
        y2: point.y,
      };
      boxSelectionRef.current = selection;
      setBoxSelection(selection);
      return;
    }
    dragRef.current = {
      mode: "pan",
      startX: event.clientX,
      startY: event.clientY,
      panX: pan.x,
      panY: pan.y,
      moved: false,
      before: captureSceneSnapshot(),
    };
  };

  const onPointerMove = (event: ReactPointerEvent<SVGSVGElement>) => {
    const drag = dragRef.current;
    if (!drag) return;
    const dx = event.clientX - drag.startX;
    const dy = event.clientY - drag.startY;
    if (Math.hypot(dx, dy) > NODE_DRAG_THRESHOLD) {
      drag.moved = true;
      movedRef.current = true;
    }
    if (drag.mode === "pan") {
      const rect = svgRef.current?.getBoundingClientRect();
      if (!rect) return;
      setPan({ x: drag.panX + (dx / rect.width) * VIEW_W, y: drag.panY + (dy / rect.height) * VIEW_H });
      return;
    }
    if (drag.mode === "box") {
      const point = graphPoint(event.clientX, event.clientY);
      const current = boxSelectionRef.current;
      const selection = current
        ? { ...current, x2: point.x, y2: point.y }
        : { x1: point.x, y1: point.y, x2: point.x, y2: point.y };
      boxSelectionRef.current = selection;
      setBoxSelection(selection);
      return;
    }
    if (!drag.moved) return;
    const node = nodesRef.current.find((n) => n.id === drag.id);
    if (!node) return;
    const p = graphToWorld(event.clientX, event.clientY);
    const nextX = p.x + (drag.offsetX || 0);
    const nextY = p.y + (drag.offsetY || 0);
    node.fx = nextX;
    node.fy = nextY;
    node.x = nextX;
    node.y = nextY;
    alphaRef.current = Math.max(alphaRef.current, 0.35);
    setVersion((v) => v + 1);
  };

  const onPointerUp = () => {
    const drag = dragRef.current;
    const completedBox = boxSelectionRef.current;
    if (drag?.mode === "pan" && !drag.moved) {
      setSelected(null);
      setSelectedNodeIDs([]);
    }
    if (drag?.mode === "box" && completedBox) {
      const minX = Math.min(completedBox.x1, completedBox.x2);
      const maxX = Math.max(completedBox.x1, completedBox.x2);
      const minY = Math.min(completedBox.y1, completedBox.y2);
      const maxY = Math.max(completedBox.y1, completedBox.y2);
      const selectedIDs = nodesRef.current
        .filter((node) => {
          const x = pan.x + node.x * zoom;
          const y = pan.y + node.y * zoom;
          return x >= minX && x <= maxX && y >= minY && y <= maxY;
        })
        .map((node) => node.id);
      setSelectedNodeIDs((current) =>
        drag.additive
          ? [...new Set([...current, ...selectedIDs])]
          : selectedIDs,
      );
      const last = nodesRef.current.find(
        (node) => node.id === selectedIDs.at(-1),
      );
      if (last) setSelected({ kind: "node", node: last });
      boxSelectionRef.current = null;
      setBoxSelection(null);
    }
    if (drag?.mode === "node") {
      const node = nodesRef.current.find((n) => n.id === drag.id);
      if (node) {
        if (pinnedNodeIDSet.has(node.id)) {
          node.fx = node.x;
          node.fy = node.y;
        } else {
          node.fx = undefined;
          node.fy = undefined;
        }
      }
      alphaRef.current = Math.max(alphaRef.current, 0.35);
    }
    if (drag?.moved && drag.before && drag.mode !== "box") {
      recordSceneSnapshot(drag.before);
    }
    dragRef.current = null;
  };

  const nodes = nodesRef.current;
  const nodeByID = new Map(nodes.map((n) => [n.id, n]));
  const renderedNodes = nodes;
  const renderedEdges = edgesRef.current;
  const viewportWorld = {
    minX: -pan.x / zoom,
    minY: -pan.y / zoom,
    maxX: (VIEW_W - pan.x) / zoom,
    maxY: (VIEW_H - pan.y) / zoom,
  };
  const rawMinimapBounds = sceneBounds(nodes);
  const minimapBounds = {
    minX: Math.min(rawMinimapBounds.minX, viewportWorld.minX),
    minY: Math.min(rawMinimapBounds.minY, viewportWorld.minY),
    maxX: Math.max(rawMinimapBounds.maxX, viewportWorld.maxX),
    maxY: Math.max(rawMinimapBounds.maxY, viewportWorld.maxY),
  };
  const minimapPadding = 6;
  const minimapScale = Math.min(
    (MINIMAP_W - minimapPadding * 2) /
      Math.max(1, minimapBounds.maxX - minimapBounds.minX),
    (MINIMAP_H - minimapPadding * 2) /
      Math.max(1, minimapBounds.maxY - minimapBounds.minY),
  );
  const minimapX = (x: number) =>
    minimapPadding + (x - minimapBounds.minX) * minimapScale;
  const minimapY = (y: number) =>
    minimapPadding + (y - minimapBounds.minY) * minimapScale;
  const query = searchTerm.trim().toLowerCase();
  const isSearching = query.length > 0;
  const matchesSearch = (node: EntityGraphNode) => {
    if (!isSearching) return true;
    const meta = nodeMeta(node.type);
    const detailText = Object.entries(node.details || {}).map(([key, value]) => `${key} ${value}`).join(" ");
    return `${node.id} ${node.type} ${meta.label} ${node.label} ${detailText}`.toLowerCase().includes(query);
  };
  const selectionForActions =
    selectedNodeIDs.length > 0
      ? selectedNodeIDs
      : selected?.kind === "node"
        ? [selected.node.id]
        : [];
  const selectionIsPinned =
    selectionForActions.length > 0 &&
    selectionForActions.every((id) => pinnedNodeIDSet.has(id));
  const selectedAccent =
    selected?.kind === "node"
      ? nodeMeta(selected.node.type).color
      : GRAPH_SELECTED_COLOR;
  const selectedInspectorTitle =
    selected?.kind === "node"
      ? nodeMeta(selected.node.type).label
      : selected?.kind === "edge"
        ? edgeTypeLabel(selected.edge)
        : "";
  const selectedInspectorSubtitle =
    selected?.kind === "node"
      ? selected.node.label
      : selected?.kind === "edge"
        ? `${selected.source?.label || selected.edge.source} → ${
            selected.target?.label || selected.edge.target
          }`
        : "";
  const selectedDetailFields = useMemo(() => {
    if (!selected) return [];
    if (selected.kind === "node") {
      const base = [
        { key: "name", label: "名称", value: selected.node.label },
        {
          key: "entity_type",
          label: "实体类型",
          value: nodeMeta(selected.node.type).label,
        },
        { key: "entity_id", label: "实体 ID", value: selected.node.id },
      ];
      const details = Object.entries(selected.node.details || {})
        .filter(([key, value]) => {
          if (!value) return false;
          if (key === "name" && value === selected.node.label) return false;
          return key !== "type";
        })
        .map(([key, value]) => ({
          key: `detail:${key}`,
          label: key,
          value,
        }));
      return [...base, ...details];
    }
    return [
      {
        key: "relationship",
        label: "关系名称",
        value: edgeTypeLabel(selected.edge),
      },
      {
        key: "source",
        label: "起点实体",
        value: selected.source?.label || selected.edge.source,
      },
      {
        key: "target",
        label: "连接实体",
        value: selected.target?.label || selected.edge.target,
      },
      {
        key: "relationship_type",
        label: "关系类型",
        value: selected.edge.type,
      },
      {
        key: "relationship_id",
        label: "关系 ID",
        value: selected.edge.id,
      },
      ...Object.entries(selected.edge.details || {})
        .filter(([, value]) => Boolean(value))
        .map(([key, value]) => ({
          key: `detail:${key}`,
          label: key,
          value,
        })),
    ];
  }, [selected]);
  const selectedQuestionContext =
    selected?.kind === "edge" ? edgeQuestionContext(selected) : null;
  const selectedQuestionPaperID = selectedQuestionContext
    ? paperIDForNode(selectedQuestionContext.paper)
    : "";
  const selectedSemanticPaperIDs = useMemo(() => {
    if (
      selected?.kind !== "edge" ||
      selected.edge.type !== "SEMANTIC_SIMILAR"
    ) {
      return [];
    }
    const ids = [selected.source, selected.target]
      .filter((node): node is EntityGraphNode => node?.type === "paper")
      .map((node) => paperIDForNode(node))
      .filter(Boolean);
    return [...new Set(ids)].slice(0, 2);
  }, [paperIDForNode, selected]);
  const selectedPaperID =
    selected?.kind === "node" && selected.node.type === "paper"
      ? paperIDForNode(selected.node)
      : "";
  const selectedReferenceText =
    selected?.kind === "node" && selected.node.type === "reference"
      ? referenceSearchText(selected.node)
      : "";
  const hasSelectedTools = Boolean(
    selectedPaperID ||
      selectedReferenceText ||
      (selectedQuestionContext && selectedQuestionPaperID) ||
      selectedSemanticPaperIDs.length === 2,
  );
  const togglePinnedSelection = () => {
    if (selectionForActions.length === 0) return;
    recordSceneSnapshot();
    const nextPinned = new Set(pinnedNodeIDs);
    for (const id of selectionForActions) {
      if (selectionIsPinned) nextPinned.delete(id);
      else nextPinned.add(id);
      const node = nodesRef.current.find((item) => item.id === id);
      if (!node) continue;
      node.fx = selectionIsPinned ? undefined : node.x;
      node.fy = selectionIsPinned ? undefined : node.y;
    }
    setPinnedNodeIDs([...nextPinned]);
    alphaRef.current = 0.35;
    setVersion((version) => version + 1);
  };
  const fitSelectedNodes = () => {
    const selectedNodes = nodesRef.current.filter((node) =>
      selectedNodeIDSet.has(node.id),
    );
    if (selectedNodes.length === 0) return;
    recordSceneSnapshot();
    const minX = Math.min(
      ...selectedNodes.map((node) => node.x - nodeMeta(node.type).radius),
    );
    const maxX = Math.max(
      ...selectedNodes.map((node) => node.x + nodeMeta(node.type).radius),
    );
    const minY = Math.min(
      ...selectedNodes.map((node) => node.y - nodeMeta(node.type).radius),
    );
    const maxY = Math.max(
      ...selectedNodes.map((node) => node.y + nodeMeta(node.type).radius),
    );
    const width = Math.max(80, maxX - minX);
    const height = Math.max(80, maxY - minY);
    const nextZoom = clamp(
      Math.min((VIEW_W - 180) / width, (VIEW_H - 140) / height),
      MIN_ZOOM,
      MAX_ZOOM,
    );
    setZoom(nextZoom);
    setPan({
      x: VIEW_W / 2 - ((minX + maxX) / 2) * nextZoom,
      y: VIEW_H / 2 - ((minY + maxY) / 2) * nextZoom,
    });
  };
  const resetGraphView = () => {
    recordSceneSnapshot();
    setZoom(1);
    setSearchTerm("");
    resetLayout(true);
  };
  const changeZoom = (factor: number) => {
    const nextZoom = clamp(zoom * factor, MIN_ZOOM, MAX_ZOOM);
    if (nextZoom === zoom) return;
    recordSceneSnapshot();
    const centerWorldX = (VIEW_W / 2 - pan.x) / zoom;
    const centerWorldY = (VIEW_H / 2 - pan.y) / zoom;
    setZoom(nextZoom);
    setPan({
      x: VIEW_W / 2 - centerWorldX * nextZoom,
      y: VIEW_H / 2 - centerWorldY * nextZoom,
    });
  };
  const graphDownloadName = (ext: "svg" | "png") =>
    mode === "overview" ? `overview-knowledge-graph.${ext}` : `paper-knowledge-graph.${ext}`;
  const exportFilterLabel = () => {
    if (graphFilter.kind === "node") {
      return `节点：${nodeMeta(graphFilter.type).label}${graphFilter.expanded ? "（含关系）" : ""}`;
    }
    if (graphFilter.kind === "edge") return `关系：${graphFilter.type}`;
    return "全部节点与关系";
  };
  const serializedGraphSvg = () => {
    const svg = svgRef.current;
    if (!svg) return null;
    const namespace = "http://www.w3.org/2000/svg";
    const clone = svg.cloneNode(true) as SVGSVGElement;
    const legendTypes = [...new Set(renderedNodes.map((node) => node.type))];
    const legendColumns = 7;
    const legendRows = Math.max(1, Math.ceil(legendTypes.length / legendColumns));
    const headerHeight = 74 + legendRows * 25;
    const exportHeight = headerHeight + VIEW_H + 30;
    const output = document.createElementNS(namespace, "svg");
    output.setAttribute("xmlns", namespace);
    output.setAttribute("width", String(VIEW_W));
    output.setAttribute("height", String(exportHeight));
    output.setAttribute("viewBox", `0 0 ${VIEW_W} ${exportHeight}`);

    const background = document.createElementNS(namespace, "rect");
    background.setAttribute("width", String(VIEW_W));
    background.setAttribute("height", String(exportHeight));
    background.setAttribute("fill", "#ffffff");
    output.appendChild(background);

    const style = document.createElementNS(namespace, "style");
    style.textContent =
      "text{font-family:Arial,'Microsoft YaHei',sans-serif}.fill-muted-foreground{fill:#64748b}.fill-foreground{fill:#fff}.fill-primary-foreground{fill:#fff}";
    output.appendChild(style);

    const addText = (
      text: string,
      x: number,
      y: number,
      size: number,
      color: string,
      weight = "400",
    ) => {
      const element = document.createElementNS(namespace, "text");
      element.setAttribute("x", String(x));
      element.setAttribute("y", String(y));
      element.setAttribute("font-size", String(size));
      element.setAttribute("font-weight", weight);
      element.setAttribute("fill", color);
      element.textContent = text;
      output.appendChild(element);
    };

    const generatedAt = new Intl.DateTimeFormat("zh-CN", {
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
    }).format(new Date());
    addText(mode === "overview" ? "总览知识图谱" : "论文知识图谱", 24, 30, 18, "#0f172a", "700");
    addText(
      `筛选条件：${exportFilterLabel()}  ·  节点 ${renderedNodes.length}  ·  关系 ${renderedEdges.length}`,
      24,
      53,
      11,
      "#475569",
    );
    addText(`生成时间：${generatedAt}`, 760, 30, 10, "#64748b");

    legendTypes.forEach((type, index) => {
      const meta = nodeMeta(type);
      const column = index % legendColumns;
      const row = Math.floor(index / legendColumns);
      const x = 24 + column * 138;
      const y = 78 + row * 25;
      const circle = document.createElementNS(namespace, "circle");
      circle.setAttribute("cx", String(x + 6));
      circle.setAttribute("cy", String(y - 4));
      circle.setAttribute("r", "5");
      circle.setAttribute("fill", meta.color);
      output.appendChild(circle);
      addText(meta.label, x + 17, y, 10, "#334155", "600");
    });

    clone.setAttribute("x", "0");
    clone.setAttribute("y", String(headerHeight));
    clone.setAttribute("width", String(VIEW_W));
    clone.setAttribute("height", String(VIEW_H));
    clone.setAttribute("viewBox", `0 0 ${VIEW_W} ${VIEW_H}`);
    clone.removeAttribute("class");
    output.appendChild(clone);
    addText(
      `当前缩放 ${Math.round(zoom * 100)}% · 导出包含当前筛选结果`,
      24,
      exportHeight - 10,
      10,
      "#64748b",
    );
    return {
      source: new XMLSerializer().serializeToString(output),
      width: VIEW_W,
      height: exportHeight,
    };
  };
  const triggerDownload = (url: string, filename: string) => {
    const a = document.createElement("a");
    a.href = url;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    a.remove();
  };
  const downloadSvg = () => {
    const exported = serializedGraphSvg();
    if (!exported) return;
    const blob = new Blob([`<?xml version="1.0" encoding="UTF-8"?>\n${exported.source}`], { type: "image/svg+xml;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    triggerDownload(url, graphDownloadName("svg"));
    URL.revokeObjectURL(url);
    setDownloadOpen(false);
  };
  const downloadPng = () => {
    const exported = serializedGraphSvg();
    if (!exported) return;
    const blob = new Blob([exported.source], { type: "image/svg+xml;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const image = new Image();
    image.onload = () => {
      const scale = 2;
      const canvas = document.createElement("canvas");
      canvas.width = exported.width * scale;
      canvas.height = exported.height * scale;
      const ctx = canvas.getContext("2d");
      if (!ctx) {
        URL.revokeObjectURL(url);
        return;
      }
      ctx.fillStyle = "#ffffff";
      ctx.fillRect(0, 0, canvas.width, canvas.height);
      ctx.setTransform(scale, 0, 0, scale, 0, 0);
      ctx.drawImage(image, 0, 0, exported.width, exported.height);
      canvas.toBlob((pngBlob) => {
        URL.revokeObjectURL(url);
        if (!pngBlob) return;
        const pngUrl = URL.createObjectURL(pngBlob);
        triggerDownload(pngUrl, graphDownloadName("png"));
        URL.revokeObjectURL(pngUrl);
      }, "image/png");
    };
    image.onerror = () => URL.revokeObjectURL(url);
    image.src = url;
    setDownloadOpen(false);
  };
  const searchReferenceInPioneer = async (node: EntityGraphNode) => {
    const text = referenceSearchText(node);
    if (!text) return;
    try {
      await copyTextToClipboard(text);
    } catch {
      // Clipboard permission can fail; the pioneer draft below still keeps the flow usable.
    }
    try {
      sessionStorage.setItem(PIONEER_REFERENCE_DRAFT_KEY, `检索这篇论文：${text}`);
    } catch {
      // sessionStorage may be unavailable in strict browser modes.
    }
    window.location.assign("/pioneer");
  };

  const openPaperChat = (
    paperID: string,
    draft = "请结合论文原文，概括这篇论文的研究问题、方法和主要结论。",
  ) => {
    onPaperRead?.(paperID);
    try {
      sessionStorage.setItem(PAPER_CHAT_DRAFT_KEY, draft);
    } catch {
      // 严格隐私模式下 sessionStorage 可能不可用。
    }
    window.location.assign(
      `/?paper_id=${encodeURIComponent(paperID)}`,
    );
  };

  const searchRelatedPapersInPioneer = (node: EntityGraphNode) => {
    const title = (node.details?.title || node.label || "").trim();
    if (!title) return;
    try {
      sessionStorage.setItem(
        PIONEER_REFERENCE_DRAFT_KEY,
        `请帮我找找与论文《${title}》相关的论文，并说明它们与该论文的研究问题或方法有什么联系。`,
      );
    } catch {
      // 严格隐私模式下 sessionStorage 可能不可用。
    }
    window.location.assign("/pioneer");
  };

  const openSemanticCompare = (paperIDs: string[]) => {
    if (paperIDs.length !== 2) return;
    const params = new URLSearchParams();
    params.set("compare_ids", paperIDs.join(","));
    window.location.assign(`/reports?${params.toString()}`);
  };

  const filterPanel = (
    <div className="flex h-full min-h-0 flex-col bg-white">
      <div className="border-b px-4 py-3">
        <div className="text-base font-semibold">Database information</div>
        <div className="mt-1 text-xs text-muted-foreground">单击筛选，双击节点标签展开或收回论文关系。</div>
      </div>
      <ScrollArea className="min-h-0 flex-1 overflow-hidden">
        <div className="space-y-5 p-4">
          {expandedNodeIDs.length > 0 && (
            <div className="flex items-center justify-between gap-2 rounded-md border border-primary/20 bg-primary/5 px-3 py-2 text-xs">
              <span className="text-muted-foreground">已展开 {expandedNodeIDs.length} 个节点</span>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="h-7 px-2 text-xs"
                onClick={() => {
                  recordSceneSnapshot();
                  setExpandedNodeIDs([]);
                }}
              >
                清除展开
              </Button>
            </div>
          )}

          <div>
            <div className="mb-3 text-sm font-semibold">Nodes ({allGraphNodes.length})</div>
            <div className="flex flex-wrap gap-2">
              <button
                type="button"
                className={cn(
                  "inline-flex h-8 items-center rounded-full px-3 text-sm font-semibold text-slate-950 transition-transform hover:scale-[1.03]",
                  graphFilter.kind === "all" ? "ring-2 ring-slate-500/40" : "",
                )}
                style={{ background: "#C4A5F4" }}
                title="Show overview graph"
                onClick={selectAllFilter}
              >
                Overview
              </button>
              {nodeFilterItems.map((item) => {
                const active = graphFilter.kind === "node" && graphFilter.type === item.type;
                return (
                  <button
                    key={item.type}
                    type="button"
                    className={cn(
                      "inline-flex h-8 items-center gap-1.5 rounded-full px-3 text-sm font-semibold text-slate-950 transition-transform hover:scale-[1.03]",
                      active ? "ring-2 ring-slate-500/45" : "",
                    )}
                    style={{ background: item.meta.color }}
                    aria-pressed={active}
                    title="单击仅显示该类节点；双击展开/收回与 Paper 的关系"
                    onClick={() => selectNodeFilter(item.type)}
                    onDoubleClick={(event) => {
                      event.preventDefault();
                      toggleNodeExpansion(item.type);
                    }}
                  >
                    {item.meta.label}
                    <span className="text-xs font-medium text-slate-800/70">{item.count}</span>
                  </button>
                );
              })}
            </div>
          </div>

          <div>
            <div className="mb-3 text-sm font-semibold">Relationships ({allGraphEdges.length})</div>
            {edgeFilterItems.length === 0 ? (
              <p className="rounded-md border bg-muted/25 px-2 py-2 text-xs text-muted-foreground">暂无关系</p>
            ) : (
              <div className="flex flex-wrap gap-2">
                <button
                  type="button"
                  className={cn(
                    "inline-flex h-8 items-center px-4 pl-5 text-sm font-semibold text-slate-950 transition-transform hover:scale-[1.03]",
                    graphFilter.kind === "edge" &&
                      graphFilter.type === ALL_RELATIONSHIPS_FILTER
                      ? "ring-2 ring-slate-500/40"
                      : "",
                  )}
                  style={{
                    background: "#e5e7eb",
                    clipPath: "polygon(0 0, calc(100% - 10px) 0, 100% 50%, calc(100% - 10px) 100%, 0 100%, 10px 50%)",
                  }}
                  title="展示全部关系与相关节点"
                  onClick={selectAllRelationships}
                >
                  *
                </button>
                {edgeFilterItems.map((item) => {
                  const active = graphFilter.kind === "edge" && graphFilter.type === item.type;
                  return (
                    <button
                      key={item.type}
                      type="button"
                      className={cn(
                        "inline-flex h-8 items-center gap-1.5 px-4 pl-5 text-sm font-semibold text-slate-950 transition-transform hover:scale-[1.03]",
                        active ? "ring-2 ring-slate-500/45" : "",
                      )}
                      style={{
                        background: "#e5e7eb",
                        clipPath: "polygon(0 0, calc(100% - 10px) 0, 100% 50%, calc(100% - 10px) 100%, 0 100%, 10px 50%)",
                      }}
                      aria-pressed={active}
                      title="Show node pairs connected by this relationship"
                      onClick={() => selectEdgeFilter(item.type)}
                    >
                      {item.label}
                      <span className="text-xs font-medium text-slate-800/70">{item.count}</span>
                    </button>
                  );
                })}
              </div>
            )}
          </div>
        </div>
      </ScrollArea>
    </div>
  );

  return (
    <div className="grid h-full min-h-0 flex-1 grid-cols-[14rem_minmax(0,1fr)] overflow-hidden rounded-lg border bg-background lg:grid-cols-[16rem_minmax(0,1fr)]">
      <aside className="min-h-0 overflow-hidden border-r bg-white">{filterPanel}</aside>
      <div className="relative min-h-0 flex-1 overflow-hidden bg-background">
      <div className="absolute left-3 right-3 top-3 z-10 flex flex-wrap items-start justify-between gap-2">
        <div className="flex flex-wrap items-center gap-2">
          <div className="relative">
            <Button
              variant={limitOpen ? "secondary" : "outline"}
              size="sm"
              className={GRAPH_TOP_BUTTON_CLASS}
              title="设置图谱展示数量上限"
              onClick={() =>
                setLimitOpen((open) => {
                  const next = !open;
                  if (next) setDisplayLimitDraft(String(displayLimit));
                  return next;
                })
              }
            >
              <Hash className="size-4" />
              数量 {displayLimit}
            </Button>
            {limitOpen && (
              <div className="absolute left-0 top-[calc(100%+0.5rem)] z-20 flex items-center gap-2 rounded-md border bg-white p-2 shadow-md">
                <input
                  type="number"
                  min={0}
                  value={displayLimitDraft}
                  onChange={(event) => {
                    const raw = event.target.value;
                    setDisplayLimitDraft(raw);
                    if (raw.trim() === "") {
                      setDisplayLimit(0);
                      return;
                    }
                    const parsed = Number.parseInt(raw, 10);
                    setDisplayLimit(
                      Number.isFinite(parsed) ? clamp(parsed, 0, 9999) : 0,
                    );
                  }}
                  className="h-8 w-24 rounded-md border bg-white px-2 text-sm outline-none focus:ring-2 focus:ring-primary/20"
                  autoFocus
                />
                <Button
                  variant="ghost"
                  size="icon-xs"
                  title="恢复默认数量"
                  onClick={() => {
                    setDisplayLimit(DEFAULT_GRAPH_DISPLAY_LIMIT);
                    setDisplayLimitDraft(String(DEFAULT_GRAPH_DISPLAY_LIMIT));
                  }}
                >
                  <RotateCcw className="size-3.5" />
                </Button>
              </div>
            )}
          </div>
          <Button
            variant={searchOpen ? "secondary" : "outline"}
            size="sm"
            className={GRAPH_TOP_BUTTON_CLASS}
            title="搜索当前图谱节点"
            onClick={() => {
              setSearchOpen((open) => {
                const next = !open;
                if (!next) setSearchTerm("");
                return next;
              });
            }}
          >
            <Search className="size-4" />
            搜索
          </Button>
          {searchOpen && (
            <div className="flex items-center gap-1 rounded-md border bg-white px-2 py-1 shadow-md">
              <Search className="size-3.5 text-muted-foreground" />
              <input
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                placeholder="输入节点名称或详情"
                className="h-7 w-52 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
                autoFocus
              />
              {searchTerm && (
                <Button variant="ghost" size="icon-xs" title="清空搜索" onClick={() => setSearchTerm("")}>
                  <X className="size-3.5" />
                </Button>
              )}
            </div>
          )}
          <div className="relative">
            <Button variant={keywordsOpen ? "secondary" : "outline"} size="sm" className={GRAPH_TOP_BUTTON_CLASS} title="查看关键词" onClick={() => setKeywordsOpen((open) => !open)}>
              <Tags className="size-4" />
              关键词
            </Button>
            {keywordsOpen && (
              <div className="absolute left-0 top-[calc(100%+0.5rem)] z-20 w-[min(34rem,calc(100vw-19rem))] min-w-80 overflow-hidden rounded-lg border bg-white shadow-lg">
                <div className="flex items-center justify-between border-b px-3 py-2">
                  <div className="flex items-center gap-2 text-sm font-medium">
                    关键词汇总
                    <span className="text-xs font-normal tabular-nums text-muted-foreground">
                      {keywordItems.length}
                    </span>
                  </div>
                  <Button variant="ghost" size="icon-xs" title="关闭" onClick={() => setKeywordsOpen(false)}>
                    <X className="size-3.5" />
                  </Button>
                </div>
                <div className="max-h-[min(62vh,34rem)] overflow-y-auto overscroll-contain">
                  <div className="p-3">
                    {keywordItems.length === 0 ? (
                      <p className="text-sm text-muted-foreground">暂无关键词数据</p>
                    ) : (
                      <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
                        {keywordItems.map((item, index) => (
                          <span
                            key={`${item.name}:${index}`}
                            className="flex min-w-0 items-start justify-between gap-2 rounded-md border bg-white px-3 py-2 text-xs shadow-xs"
                            title={item.name}
                          >
                            <span className="min-w-0 whitespace-normal break-words leading-5">
                              {item.name}
                            </span>
                            {showKeywordCount && item.count != null && (
                              <span className="shrink-0 rounded-full bg-background px-1.5 py-0.5 tabular-nums text-muted-foreground">
                                {item.count}
                              </span>
                            )}
                          </span>
                        ))}
                      </div>
                    )}
                  </div>
                </div>
              </div>
            )}
          </div>
          <Button
            variant={boxSelectMode ? "secondary" : "outline"}
            size="sm"
            className={GRAPH_TOP_BUTTON_CLASS}
            title="框选节点；按住 Shift 可追加选择"
            aria-pressed={boxSelectMode}
            onClick={() => {
              setBoxSelectMode((active) => !active);
              boxSelectionRef.current = null;
              setBoxSelection(null);
            }}
          >
            <MousePointer2 className="size-4" />
            框选
          </Button>
          <Button
            variant="outline"
            size="icon-sm"
            className={GRAPH_TOP_BUTTON_CLASS}
            title={selectionIsPinned ? "取消固定选中节点" : "固定选中节点"}
            disabled={selectionForActions.length === 0}
            onClick={togglePinnedSelection}
          >
            {selectionIsPinned ? (
              <PinOff className="size-4" />
            ) : (
              <Pin className="size-4" />
            )}
          </Button>
          <Button
            variant="outline"
            size="icon-sm"
            className={GRAPH_TOP_BUTTON_CLASS}
            title="适应选中区域"
            disabled={selectedNodeIDs.length === 0}
            onClick={fitSelectedNodes}
          >
            <Maximize2 className="size-4" />
          </Button>
          <Button
            variant="outline"
            size="icon-sm"
            className={GRAPH_TOP_BUTTON_CLASS}
            title="缩小图谱"
            disabled={zoom <= MIN_ZOOM}
            onClick={() => changeZoom(0.9)}
          >
            <ZoomOut className="size-4" />
          </Button>
          <Button
            variant="outline"
            size="icon-sm"
            className={GRAPH_TOP_BUTTON_CLASS}
            title="放大图谱"
            disabled={zoom >= MAX_ZOOM}
            onClick={() => changeZoom(1.1)}
          >
            <ZoomIn className="size-4" />
          </Button>
          <Button
            variant="outline"
            size="icon-sm"
            className={GRAPH_TOP_BUTTON_CLASS}
            title="撤销"
            disabled={undoStack.length === 0}
            onClick={undoScene}
          >
            <Undo2 className="size-4" />
          </Button>
          <Button
            variant="outline"
            size="icon-sm"
            className={GRAPH_TOP_BUTTON_CLASS}
            title="重做"
            disabled={redoStack.length === 0}
            onClick={redoScene}
          >
            <Redo2 className="size-4" />
          </Button>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" className={GRAPH_TOP_BUTTON_CLASS} title="重置位置并解除全部固定" onClick={resetGraphView}>
            <RotateCcw className="size-4" />
            重置
          </Button>
          <div className="relative">
            <Button variant={downloadOpen ? "secondary" : "outline"} size="sm" className={GRAPH_TOP_BUTTON_CLASS} title="下载知识图谱" onClick={() => setDownloadOpen((open) => !open)}>
              <Download className="size-4" />
              下载
            </Button>
            {downloadOpen && (
              <div className="absolute right-0 top-[calc(100%+0.5rem)] z-20 w-36 overflow-hidden rounded-md border bg-white p-1 shadow-lg">
                <button type="button" className="flex w-full items-center rounded px-2.5 py-2 text-left text-sm hover:bg-accent" onClick={downloadSvg}>
                  SVG
                </button>
                <button type="button" className="flex w-full items-center rounded px-2.5 py-2 text-left text-sm hover:bg-accent" onClick={downloadPng}>
                  PNG
                </button>
              </div>
            )}
          </div>
        </div>
      </div>
      <svg
        ref={svgRef}
        xmlns="http://www.w3.org/2000/svg"
        viewBox={`0 0 ${VIEW_W} ${VIEW_H}`}
        className="h-full min-h-0 w-full touch-none select-none bg-[radial-gradient(circle_at_center,color-mix(in_srgb,var(--muted)_72%,transparent)_1px,transparent_1px)] [background-size:22px_22px]"
        role="img"
        aria-label="论文知识图谱"
        onPointerDown={onSvgDown}
        onPointerMove={onPointerMove}
        onPointerUp={onPointerUp}
        onPointerCancel={onPointerUp}
      >
        <defs>
          <style>
            {`
              @keyframes graphSelectedNodeRingIn {
                from {
                  opacity: 0;
                  transform: scale(0.82);
                }
                to {
                  opacity: 1;
                  transform: scale(1);
                }
              }
              .graph-selected-node-ring {
                animation: graphSelectedNodeRingIn 180ms ease-out;
                transform-box: fill-box;
                transform-origin: center;
              }
            `}
          </style>
          <filter id="graph-selected-shadow" x="-60%" y="-60%" width="220%" height="220%">
            <feDropShadow dx="0" dy="2" stdDeviation="3" floodColor="#64748b" floodOpacity="0.28" />
          </filter>
          <marker id="graph-edge-arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="5.5" markerHeight="5.5" orient="auto" markerUnits="strokeWidth">
            <path d="M 0 0 L 10 5 L 0 10 z" fill="context-stroke" />
          </marker>
        </defs>
        <g transform={`translate(${pan.x} ${pan.y}) scale(${zoom})`}>
          {renderedEdges.map((edge) => {
            const a = nodeByID.get(edge.source);
            const b = nodeByID.get(edge.target);
            if (!a || !b) return null;
            const selectedEdge = selected?.kind === "edge" && edgeKey(selected.edge) === edgeKey(edge);
            const color = selectedEdge ? GRAPH_SELECTED_COLOR : isSearching ? "#cbd5e1" : GRAPH_EDGE_COLOR;
            const line = edgeLinePoints(a, b);
            const midX = (line.x1 + line.x2) / 2;
            const midY = (line.y1 + line.y2) / 2;
            const dx = line.x2 - line.x1;
            const dy = line.y2 - line.y1;
            const edgeLen = Math.max(1, Math.hypot(dx, dy));
            const rawAngle = (Math.atan2(dy, dx) * 180) / Math.PI;
            const labelAngle = rawAngle > 90 || rawAngle < -90 ? rawAngle + 180 : rawAngle;
            const labelOffset = selectedEdge ? 12:8;
            const labelX = midX + (-dy / edgeLen) * labelOffset;
            const labelY = midY + (dx / edgeLen) * labelOffset;
            return (
              <g
                key={edgeKey(edge)}
                className="cursor-pointer"
                onPointerDown={(e) => e.stopPropagation()}
                onClick={(e) => {
                  e.stopPropagation();
                  setSelectedNodeIDs([]);
                  setSelected({ kind: "edge", edge, source: a, target: b });
                }}
              >
                <line x1={line.x1} y1={line.y1} x2={line.x2} y2={line.y2} stroke="transparent" strokeWidth={18} />
                <line
                  x1={line.x1}
                  y1={line.y1}
                  x2={line.x2}
                  y2={line.y2}
                  stroke={GRAPH_SELECTED_COLOR}
                  strokeLinecap="round"
                  strokeOpacity={selectedEdge ? 0.2 : 0}
                  strokeWidth={selectedEdge ? 7 : 0}
                  style={{
                    transition: "stroke-width 180ms ease-out, stroke-opacity 180ms ease-out",
                  }}
                />
                <line
                  x1={line.x1}
                  y1={line.y1}
                  x2={line.x2}
                  y2={line.y2}
                  stroke={color}
                  strokeLinecap="round"
                  strokeOpacity={selectedEdge ? 0.95 : isSearching ? 0.35 : 0.42}
                  strokeWidth={selectedEdge ? 2.4 : 1.6}
                  markerEnd="url(#graph-edge-arrow)"
                  filter={selectedEdge ? "url(#graph-selected-shadow)" : undefined}
                  style={{
                    transition: "stroke 180ms ease-out, stroke-width 180ms ease-out, stroke-opacity 180ms ease-out",
                  }}
                />
                <g transform={`translate(${labelX} ${labelY}) rotate(${labelAngle})`}>
                  <text
                    x={0}
                    y={0}
                    textAnchor="middle"
                    dominantBaseline="middle"
                    className="fill-muted-foreground"
                    style={{
                      fill: selectedEdge ? GRAPH_SELECTED_COLOR : isSearching ? "#94a3b8" : "#64748b",
                      fontSize: selectedEdge ? 7.2 : 5,
                      fontWeight: selectedEdge ? 700 : 500,
                      paintOrder: "stroke",
                      stroke: "#ffffff",
                      strokeWidth: selectedEdge ? 3 : 2.5,
                      transform: `scale(${selectedEdge ? 1.12 : 1})`,
                      transformBox: "fill-box",
                      transformOrigin: "center",
                      transition: "fill 180ms ease-out, font-size 180ms ease-out, font-weight 180ms ease-out, stroke-width 180ms ease-out, transform 180ms ease-out",
                    }}
                  >
                    {edgeTypeLabel(edge)}
                  </text>
                </g>
              </g>
            );
          })}
          {renderedNodes.map((node) => {
            const meta = nodeMeta(node.type);
            const matched = matchesSearch(node);
            const selectedNode =
              selectedNodeIDSet.has(node.id) ||
              (selected?.kind === "node" && selected.node.id === node.id);
            const pinnedNode = pinnedNodeIDSet.has(node.id);
            const labelLines = nodeLabelLines(node);
            const nodeColor = matched ? (node.type === "paper" ? PAPER_NODE_COLOR : meta.color) : "#cbd5e1";
            const labelColor = matched ? "#ffffff" : "#94a3b8";
            return (
              <g
                key={node.id}
                className="cursor-grab active:cursor-grabbing"
                transform={`translate(${node.x} ${node.y})`}
                onPointerDown={(e) => onNodeDown(e, node.id)}
                onClick={(e) => {
                  e.stopPropagation();
                  if (movedRef.current) return;
                  const additive = e.shiftKey || e.ctrlKey || e.metaKey;
                  setSelectedNodeIDs((current) => {
                    if (!additive) return [node.id];
                    return current.includes(node.id)
                      ? current.filter((id) => id !== node.id)
                      : [...current, node.id];
                  });
                  setSelected({ kind: "node", node });
                }}
                onDoubleClick={(e) => {
                  e.stopPropagation();
                  if (!movedRef.current) {
                    toggleNodeRelationExpansion(node);
                    setSelected({ kind: "node", node });
                  }
                }}
              >
                <g
                  transform={`scale(${selectedNode ? 1.055 : 1})`}
                  style={{ transition: "transform 180ms ease-out" }}
                  filter={selectedNode ? "url(#graph-selected-shadow)" : undefined}
                >
                  {selectedNode && (
                    <circle
                      className="graph-selected-node-ring"
                      r={meta.radius + 3}
                      fill="none"
                      stroke={GRAPH_SELECTED_COLOR}
                      strokeOpacity={0.92}
                      strokeWidth={2.5}
                      style={{ transition: "r 180ms ease-out, stroke-opacity 180ms ease-out, stroke-width 180ms ease-out" }}
                    />
                  )}
                  <circle
                    r={meta.radius}
                    fill={nodeColor}
                    fillOpacity={matched ? 1 : 0.32}
                    stroke="transparent"
                    strokeWidth={0}
                    style={{ transition: "stroke 180ms ease-out, stroke-width 180ms ease-out, fill-opacity 180ms ease-out" }}
                  />
                  {pinnedNode && (
                    <circle
                      cx={meta.radius * 0.62}
                      cy={-meta.radius * 0.62}
                      r={3.2}
                      fill="#ffffff"
                      fillOpacity={0.96}
                    />
                  )}
                  <text
                    y={labelLines.length > 1 ? -3 : 4}
                    textAnchor="middle"
                    className={cn("pointer-events-none font-medium", node.type === "paper" ? "fill-primary-foreground text-[11px]" : "fill-foreground text-[12px]")}
                    style={{ fill: labelColor, fontSize: nodeLabelFontSize(node) }}
                  >
                    {labelLines.map((line, index) => (
                      <tspan
                        key={`${node.id}:label:${index}`}
                        x={0}
                        dy={index === 0 ? 0 : 11}
                      >
                        {line}
                      </tspan>
                    ))}
                  </text>
                </g>
              </g>
            );
          })}
        </g>
        {boxSelection && (
          <rect
            x={Math.min(boxSelection.x1, boxSelection.x2)}
            y={Math.min(boxSelection.y1, boxSelection.y2)}
            width={Math.abs(boxSelection.x2 - boxSelection.x1)}
            height={Math.abs(boxSelection.y2 - boxSelection.y1)}
            rx={4}
            fill={GRAPH_SELECTED_COLOR}
            fillOpacity={0.08}
            stroke={GRAPH_SELECTED_COLOR}
            strokeDasharray="6 4"
            strokeWidth={1.5}
            className="pointer-events-none"
          />
        )}
      </svg>
      <div className="absolute bottom-3 right-3 z-10 w-48 overflow-hidden rounded-md border bg-background/92 shadow-sm backdrop-blur">
        <div className="flex items-center justify-between border-b px-2.5 py-1.5 text-[11px]">
          <span className="font-medium">小地图</span>
          <span className="tabular-nums text-muted-foreground">
            {Math.round(zoom * 100)}%
          </span>
        </div>
        <svg
          viewBox={`0 0 ${MINIMAP_W} ${MINIMAP_H}`}
          className="h-24 w-full cursor-crosshair bg-muted/20"
          aria-label="知识图谱小地图"
          onClick={(event) => {
            const rect = event.currentTarget.getBoundingClientRect();
            const x =
              ((event.clientX - rect.left) / rect.width) * MINIMAP_W;
            const y =
              ((event.clientY - rect.top) / rect.height) * MINIMAP_H;
            const worldX =
              (x - minimapPadding) / minimapScale + minimapBounds.minX;
            const worldY =
              (y - minimapPadding) / minimapScale + minimapBounds.minY;
            recordSceneSnapshot();
            setPan({
              x: VIEW_W / 2 - worldX * zoom,
              y: VIEW_H / 2 - worldY * zoom,
            });
          }}
        >
          {renderedEdges.map((edge) => {
            const source = nodeByID.get(edge.source);
            const target = nodeByID.get(edge.target);
            if (!source || !target) return null;
            return (
              <line
                key={`minimap:${edgeKey(edge)}`}
                x1={minimapX(source.x)}
                y1={minimapY(source.y)}
                x2={minimapX(target.x)}
                y2={minimapY(target.y)}
                stroke="#cbd5e1"
                strokeWidth={0.7}
              />
            );
          })}
          {renderedNodes.map((node) => (
            <circle
              key={`minimap:${node.id}`}
              cx={minimapX(node.x)}
              cy={minimapY(node.y)}
              r={node.type === "paper" ? 2.7 : 1.8}
              fill={nodeMeta(node.type).color}
              stroke={
                selectedNodeIDSet.has(node.id)
                  ? GRAPH_SELECTED_COLOR
                  : "transparent"
              }
              strokeWidth={1}
            />
          ))}
          <rect
            x={minimapX(viewportWorld.minX)}
            y={minimapY(viewportWorld.minY)}
            width={Math.max(
              2,
              (viewportWorld.maxX - viewportWorld.minX) * minimapScale,
            )}
            height={Math.max(
              2,
              (viewportWorld.maxY - viewportWorld.minY) * minimapScale,
            )}
            fill="none"
            stroke="#475569"
            strokeWidth={1}
          />
        </svg>
      </div>
      <div className="pointer-events-none absolute bottom-3 left-3 z-10 max-w-[calc(100%-14rem)] rounded-md bg-background/82 px-2.5 py-1.5 text-xs text-muted-foreground shadow-sm backdrop-blur">
        鼠标滚轮缩放图谱大小，拖拽空白区域平移，拖拽节点调整位置。
        {mode === "overview" && " 总览页可双击任意节点展开或收回它与论文节点的关系。"}
      </div>
      {selected && (
        <div className="absolute right-3 top-14 z-20 flex max-h-[min(36rem,calc(100%-8rem))] w-[min(25rem,calc(100%-1.5rem))] flex-col overflow-hidden rounded-md border bg-background shadow-xl shadow-slate-950/10">
          <div
            className="h-1 shrink-0"
            style={{ backgroundColor: selectedAccent }}
          />
          <div className="flex shrink-0 items-start justify-between gap-3 border-b px-4 py-3">
            <div className="flex min-w-0 items-start gap-3">
              <span
                className="mt-1 size-3 shrink-0 rounded-full"
                style={{ backgroundColor: selectedAccent }}
              />
              <div className="min-w-0">
                <div className="text-xs font-semibold text-muted-foreground">
                  {selected.kind === "node" ? "实体详情" : "关系详情"}
                </div>
                <div className="mt-0.5 break-words text-sm font-semibold leading-5">
                  {selectedInspectorTitle}
                </div>
                <div className="mt-1 line-clamp-2 break-words text-xs leading-5 text-muted-foreground">
                  {selectedInspectorSubtitle}
                </div>
              </div>
            </div>
            <Button
              variant="ghost"
              size="icon-sm"
              className="shrink-0"
              title="关闭详情"
              onClick={() => setSelected(null)}
            >
              <X className="size-4" />
            </Button>
          </div>

          {(selected.kind === "node" || hasSelectedTools) && (
            <div className="shrink-0 space-y-3 border-b bg-muted/10 px-4 py-3">
              {selected.kind === "node" && (
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className="w-auto"
                  onClick={togglePinnedSelection}
                >
                  {selectionIsPinned ? (
                    <PinOff className="size-3.5" />
                  ) : (
                    <Pin className="size-3.5" />
                  )}
                  {selectionIsPinned ? "取消固定" : "固定节点"}
                </Button>
              )}

              {hasSelectedTools && (
                <div>
                  <div className="mb-2 text-xs font-semibold text-muted-foreground">
                    工具
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    {selected.kind === "node" && selectedPaperID && (
                      <>
                        <Link
                          href={`/reader?id=${encodeURIComponent(selectedPaperID)}`}
                          className={cn(
                            buttonVariants({
                              variant: "default",
                              size: "sm",
                            }),
                            GRAPH_TOOL_BUTTON_CLASS,
                          )}
                          onClick={() => onPaperRead?.(selectedPaperID)}
                        >
                          <FileText className="size-3.5" />
                          进入精读
                        </Link>
                        <Link
                          href={`/reports?paper_id=${encodeURIComponent(selectedPaperID)}`}
                          className={cn(
                            buttonVariants({
                              variant: "default",
                              size: "sm",
                            }),
                            GRAPH_TOOL_BUTTON_CLASS,
                          )}
                          onClick={() => onPaperRead?.(selectedPaperID)}
                        >
                          <BookOpenText className="size-3.5" />
                          查看研读报告
                        </Link>
                        <Button
                          type="button"
                          variant="default"
                          size="sm"
                          className={GRAPH_TOOL_BUTTON_CLASS}
                          onClick={() => openPaperChat(selectedPaperID)}
                        >
                          <MessageCircle className="size-3.5" />
                          对话问答
                        </Button>
                        <Button
                          type="button"
                          variant="default"
                          size="sm"
                          className={GRAPH_TOOL_BUTTON_CLASS}
                          onClick={() =>
                            searchRelatedPapersInPioneer(selected.node)
                          }
                        >
                          <Search className="size-3.5" />
                          检索相关论文
                        </Button>
                      </>
                    )}
                    {selected.kind === "node" && selectedReferenceText && (
                      <Button
                        type="button"
                        variant="default"
                        size="sm"
                        className={GRAPH_TOOL_BUTTON_CLASS}
                        onClick={() =>
                          void searchReferenceInPioneer(selected.node)
                        }
                      >
                        <Search className="size-3.5" />
                        小云雀检索
                      </Button>
                    )}
                    {selected.kind === "edge" &&
                      selectedQuestionContext &&
                      selectedQuestionPaperID && (
                        <Button
                          type="button"
                          variant="default"
                          size="sm"
                          className={GRAPH_TOOL_BUTTON_CLASS}
                          onClick={() =>
                            openPaperChat(
                              selectedQuestionPaperID,
                              `请结合论文《${selectedQuestionContext.paper.label}》的原文，具体解释以下${
                                QUESTION_CONTEXT_LABELS[
                                  selectedQuestionContext.entity.type
                                ] || "实体"
                              }，并说明它在论文中的作用、依据和结论：\n${selectedQuestionContext.entity.label}`,
                            )
                          }
                        >
                          <MessageCircle className="size-3.5" />
                          对话问答
                        </Button>
                      )}
                    {selected.kind === "edge" &&
                      selectedSemanticPaperIDs.length === 2 && (
                        <Button
                          type="button"
                          variant="default"
                          size="sm"
                          className={GRAPH_TOOL_BUTTON_CLASS}
                          onClick={() =>
                            openSemanticCompare(selectedSemanticPaperIDs)
                          }
                        >
                          <GitCompareArrows className="size-3.5" />
                          对比分析
                        </Button>
                      )}
                  </div>
                </div>
              )}
            </div>
          )}

          <div className="min-h-0 overflow-y-auto overscroll-contain px-4 pb-1">
            {selectedDetailFields.map((field) => (
              <GraphDetailField
                key={field.key}
                label={field.label}
                value={field.value}
                copyKey={`${selected.kind}:${selected.kind === "node" ? selected.node.id : edgeKey(selected.edge)}:${field.key}`}
                copiedKey={copiedDetailKey}
                onCopy={(key, value) => void copyDetailValue(key, value)}
              />
            ))}
          </div>
        </div>
      )}
      </div>
    </div>
  );
}
function graphRequestErrorMessage(err: unknown, fallback: string) {
  return err instanceof api.ApiError && err.message ? err.message : fallback;
}

function normalizeEntityGraph(graph: EntityGraph | null | undefined): EntityGraph | null {
  if (!graph || !Array.isArray(graph.nodes)) return null;
  return {
    nodes: dedupeEntityNodes(graph.nodes),
    edges: dedupeEntityEdges(Array.isArray(graph.edges) ? graph.edges : []),
  };
}

function normalizeGraphRebuildJob(job: GraphRebuildJob): GraphRebuildJob {
  const total = Number.isFinite(job.total) ? job.total : 0;
  const completed = Number.isFinite(job.completed) ? job.completed : 0;
  return {
    ...job,
    total,
    completed,
    succeeded: Number.isFinite(job.succeeded) ? job.succeeded : 0,
    failed: Number.isFinite(job.failed) ? job.failed : 0,
    work_total: Number.isFinite(job.work_total) ? job.work_total : total * 2,
    work_done: Number.isFinite(job.work_done)
      ? job.work_done
      : completed * 2,
    errors: Array.isArray(job.errors) ? job.errors : [],
  };
}

export function GraphView() {
  const { authReady, authed, papers, selectPaper } = useApp();

  const [keywords, setKeywords] = useState<NameCount[]>([]);
  const [entityGraph, setEntityGraph] = useState<EntityGraph | null>(null);
  const [overviewGraph, setOverviewGraph] = useState<EntityGraph | null>(null);
  const [loadingGraph, setLoadingGraph] = useState(false);
  const [rebuildingGraph, setRebuildingGraph] = useState(false);
  const [rebuildJob, setRebuildJob] = useState<GraphRebuildJob | null>(null);
  const [graphError, setGraphError] = useState("");
  const [graphNotice, setGraphNotice] = useState("");

  useEffect(() => {
    if (!authed) return;
    let cancelled = false;
    api
      .graphKeywords(GRAPH_KEYWORD_LIMIT)
      .then((value) => {
        if (!cancelled) setKeywords(value);
      })
      .catch(() => {
        if (!cancelled) setKeywords([]);
      });
    return () => {
      cancelled = true;
    };
  }, [authed]);

  const loadGraphData = useCallback(async () => {
    const [overview, full] = await Promise.all([
      api.graphNetwork(),
      api.graphNetworkEntities(),
    ]);
    const normalizedOverview = normalizeEntityGraph(overview);
    const normalizedFull = normalizeEntityGraph(full);
    setOverviewGraph(normalizedOverview);
    setEntityGraph(normalizedFull || normalizedOverview);
  }, []);

  useEffect(() => {
    if (!authed) {
      setEntityGraph(null);
      setOverviewGraph(null);
      return;
    }
    let cancelled = false;
    setGraphError("");
    setGraphNotice("");
    setLoadingGraph(true);
    loadGraphData()
      .then(() => {})
      .catch((err) => {
        if (!cancelled) {
          setEntityGraph(null);
          setGraphError(graphRequestErrorMessage(err, "鎬昏鐭ヨ瘑鍥捐氨鍔犺浇澶辫触"));
        }
      })
      .finally(() => {
        if (!cancelled) setLoadingGraph(false);
      });
    return () => {
      cancelled = true;
    };
  }, [authed, loadGraphData]);

  const rebuildCurrentGraph = async () => {
    if (rebuildingGraph) return;
    setGraphError("");
    setGraphNotice("");
    setRebuildingGraph(true);
    try {
      let job = normalizeGraphRebuildJob(await api.rebuildGraphNetwork());
      setRebuildJob(job);
      while (job.status === "queued" || job.status === "running") {
        await new Promise((resolve) => window.setTimeout(resolve, 900));
        job = normalizeGraphRebuildJob(await api.graphRebuildJob(job.id));
        setRebuildJob(job);
      }
      if (job.status === "failed") {
        throw new api.ApiError("知识图谱更新任务执行失败", 500);
      }
      await Promise.all([
        loadGraphData(),
        api
          .graphKeywords(GRAPH_KEYWORD_LIMIT)
          .then(setKeywords)
          .catch(() => setKeywords([])),
      ]);
      setGraphNotice(
        job.failed > 0
          ? `图谱更新完成：成功 ${job.succeeded} 篇，失败 ${job.failed} 篇`
          : `图谱更新完成：成功同步 ${job.succeeded} 篇论文`,
      );
    } catch (err) {
      setGraphError(
        graphRequestErrorMessage(err, "总览知识图谱更新失败"),
      );
    } finally {
      setRebuildingGraph(false);
    }
  };

  const graphKeywordItems = useMemo<GraphKeywordItem[]>(() => {
    return keywords.map((k) => ({ name: k.name, count: k.count }));
  }, [keywords]);
  const resolveGraphPaperID = useCallback(
    (node: EntityGraphNode) => {
      const direct = paperIDFromNode(node);
      if (direct) return direct;
      const title = normalizePaperTitle(node.label);
      if (!title) return "";
      const matched = papers.find((p) =>
        [p.id, p.title, p.file_name, paperTitle(p)]
          .map(normalizePaperTitle)
          .some((candidate) => candidate === title),
      );
      return matched?.id || "";
    },
    [papers],
  );

  if (!authReady) {
    return (
      <main className="flex h-dvh items-center justify-center bg-muted/50 p-6">
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <RefreshCw className="size-4 animate-spin" />
          正在恢复登录状态...
        </div>
      </main>
    );
  }

  if (!authed) {
    return (
      <main className="flex h-dvh items-center justify-center bg-muted/50 p-6">
        <div className="text-center">
          <Empty title="请先登录" text="登录后即可查看论文知识图谱。" />
          <Link href="/" className={cn(buttonVariants({ variant: "outline" }), "mt-4")}>
            返回首页
          </Link>
        </div>
      </main>
    );
  }

  return (
    <main className="flex h-dvh min-h-0 flex-col overflow-hidden bg-muted/50">
      <header className="flex h-14 shrink-0 items-center gap-3 border-b bg-background px-4">
        <Link
          href="/"
          className={buttonVariants({ variant: "ghost", size: "icon", className: "shrink-0" })}
          title="返回工作台"
        >
          <ArrowLeft className="size-4" />
        </Link>
        <AgentIntro kind="graph" />
        <span className="rounded-full border bg-muted/40 px-2 py-0.5 text-xs tabular-nums text-muted-foreground">
          论文 {papers.length}
        </span>
        <div className="ml-auto flex shrink-0 items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={rebuildCurrentGraph}
            disabled={rebuildingGraph}
            title="从 MySQL 更新总览图谱"
          >
            <RefreshCw className={cn("size-4", rebuildingGraph && "animate-spin")} />
            更新总览
          </Button>
        </div>
      </header>

      <div className="min-h-0 flex-1 overflow-hidden bg-background p-3">
        <section className="flex h-full min-h-0 min-w-0 flex-col">
            {graphError && (
              <div className="mb-3 shrink-0 rounded-md border border-destructive/25 bg-destructive/10 px-3 py-2 text-sm text-destructive">
                {graphError}
              </div>
            )}
            {graphNotice && !graphError && (
              <div className="mb-3 shrink-0 rounded-md border border-emerald-500/25 bg-emerald-500/10 px-3 py-2 text-sm text-emerald-700">
                {graphNotice}
              </div>
            )}
            {rebuildJob &&
              (rebuildingGraph || rebuildJob.failed > 0) && (
                <div className="mb-3 shrink-0 rounded-md border bg-background px-3 py-2.5 text-xs">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <span className="font-medium">
                      {rebuildingGraph ? "正在增量同步知识图谱" : "图谱同步已完成"}
                      {rebuildingGraph && rebuildJob.phase
                        ? ` · ${
                            rebuildJob.phase === "metadata"
                              ? "同步论文实体"
                              : "更新语义关系"
                          }`
                        : ""}
                    </span>
                    <span className="tabular-nums text-muted-foreground">
                      已处理 {rebuildJob.completed}/{rebuildJob.total} · 成功{" "}
                      {rebuildJob.succeeded} · 失败 {rebuildJob.failed}
                    </span>
                  </div>
                  <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-muted">
                    <div
                      className="h-full rounded-full bg-primary transition-[width] duration-300"
                      style={{
                        width: `${
                          (rebuildJob.work_total ||
                            rebuildJob.total * 2) > 0
                            ? Math.round(
                                ((rebuildJob.work_done ??
                                  rebuildJob.completed * 2) /
                                  (rebuildJob.work_total ||
                                    rebuildJob.total * 2)) *
                                  100,
                              )
                            : rebuildJob.status === "queued"
                              ? 4
                              : 10
                        }%`,
                      }}
                    />
                  </div>
                  {(rebuildJob.errors ?? []).length > 0 && (
                    <div className="mt-2 max-h-20 space-y-1 overflow-y-auto text-destructive">
                      {(rebuildJob.errors ?? []).map((item, index) => (
                        <div key={`${item.paper_id || "global"}:${item.stage}:${index}`}>
                          {item.paper_id ? `${item.paper_id}：` : ""}
                          {item.message}
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              )}

            {loadingGraph ? (
              <div className="flex min-h-0 flex-1 items-center justify-center text-sm text-muted-foreground">
                加载中...
              </div>
            ) : !entityGraph || entityGraph.nodes.length <= 1 ? (
              <Empty
                title="暂无知识图谱"
                text="解析完成后会自动生成论文知识图谱；共享关系会在总览中形成连接。"
                compact
              />
            ) : (
              <ForceEntityGraph
                graph={entityGraph}
                baseGraph={overviewGraph}
                mode="overview"
                keywordItems={graphKeywordItems}
                showKeywordCount
                onPaperRead={selectPaper}
                resolvePaperID={resolveGraphPaperID}
              />
            )}
        </section>
      </div>
    </main>
  );
}
