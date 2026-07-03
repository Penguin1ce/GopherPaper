"use client";

import {
  ArrowLeft,
  Download,
  FileText,
  Hash,
  Network,
  RefreshCw,
  RotateCcw,
  Search,
  Tags,
  X,
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
import { useApp } from "@/lib/gopherpaper/store";
import type {
  EntityGraph,
  EntityGraphEdge,
  EntityGraphNode,
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
const PAPER_NODE_COLOR = "#35A98D";
const GRAPH_EDGE_COLOR = "#94a3b8";
const GRAPH_SELECTED_COLOR = "#22c55e";
const PAPER_ID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const PIONEER_REFERENCE_DRAFT_KEY = "gopherpaper.pioneer.referenceDraft";

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
    mode: "node" | "pan";
    id?: string;
    startX: number;
    startY: number;
    panX: number;
    panY: number;
    moved: boolean;
    offsetX?: number;
    offsetY?: number;
  } | null>(null);
  const movedRef = useRef(false);
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
  const [graphFilter, setGraphFilter] = useState<GraphFilter>({ kind: "all" });
  const [expandedNodeIDs, setExpandedNodeIDs] = useState<string[]>([]);
  const filterClickTimerRef = useRef<number | null>(null);
  const paperIDForNode = useCallback(
    (node: EntityGraphNode) => resolvePaperID?.(node) || paperIDFromNode(node),
    [resolvePaperID],
  );

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
      return allGraphEdges.filter((edge) => edge.type === graphFilter.type);
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
  const clearFilterClickTimer = useCallback(() => {
    if (filterClickTimerRef.current == null) return;
    window.clearTimeout(filterClickTimerRef.current);
    filterClickTimerRef.current = null;
  }, []);

  const selectAllFilter = useCallback(() => {
    clearFilterClickTimer();
    setGraphFilter({ kind: "all" });
    setExpandedNodeIDs([]);
    setDisplayLimit(DEFAULT_GRAPH_DISPLAY_LIMIT);
  }, [clearFilterClickTimer]);

  const selectNodeFilter = useCallback(
    (type: string) => {
      clearFilterClickTimer();
      filterClickTimerRef.current = window.setTimeout(() => {
        setGraphFilter({ kind: "node", type, expanded: false });
        setExpandedNodeIDs([]);
        setDisplayLimit(DEFAULT_GRAPH_DISPLAY_LIMIT);
        filterClickTimerRef.current = null;
      }, 180);
    },
    [clearFilterClickTimer],
  );

  const toggleNodeExpansion = useCallback(
    (type: string) => {
      clearFilterClickTimer();
      setDisplayLimit(DEFAULT_GRAPH_DISPLAY_LIMIT);
      setGraphFilter((current) =>
        current.kind === "node" && current.type === type && current.expanded
          ? { kind: "node", type, expanded: false }
          : { kind: "node", type, expanded: true },
      );
    },
    [clearFilterClickTimer],
  );

  const toggleNodeRelationExpansion = useCallback((node: EntityGraphNode) => {
    setExpandedNodeIDs((current) =>
      current.includes(node.id)
        ? current.filter((id) => id !== node.id)
        : [...current, node.id],
    );
  }, []);

  const selectEdgeFilter = useCallback(
    (type: string) => {
      clearFilterClickTimer();
      setGraphFilter({ kind: "edge", type });
      setExpandedNodeIDs([]);
      setDisplayLimit(DEFAULT_GRAPH_DISPLAY_LIMIT);
    },
    [clearFilterClickTimer],
  );

  useEffect(() => () => clearFilterClickTimer(), [clearFilterClickTimer]);

  const resetLayout = () => {
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
    nodesRef.current = next;
    edgesRef.current = graphEdges;
    alphaRef.current = 0.9;
    setPan({ x: 0, y: 0 });
    setVersion((v) => v + 1);
  };

  useEffect(() => {
    setGraphFilter({ kind: "all" });
    setExpandedNodeIDs([]);
    setDisplayLimit(DEFAULT_GRAPH_DISPLAY_LIMIT);
    setLimitOpen(false);
  }, [graph, mode]);

  useEffect(() => {
    resetLayout();
    setSelected(null);
    setZoom(1);
    setSearchTerm("");
    setKeywordsOpen(false);
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
    };
  };

  const onSvgDown = (event: ReactPointerEvent<SVGSVGElement>) => {
    event.currentTarget.setPointerCapture(event.pointerId);
    movedRef.current = false;
    dragRef.current = { mode: "pan", startX: event.clientX, startY: event.clientY, panX: pan.x, panY: pan.y, moved: false };
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
    if (drag?.mode === "node") {
      const node = nodesRef.current.find((n) => n.id === drag.id);
      if (node) {
        node.fx = undefined;
        node.fy = undefined;
      }
      alphaRef.current = Math.max(alphaRef.current, 0.35);
    }
    dragRef.current = null;
  };

  const nodes = nodesRef.current;
  const nodeByID = new Map(nodes.map((n) => [n.id, n]));
  const query = searchTerm.trim().toLowerCase();
  const isSearching = query.length > 0;
  const matchesSearch = (node: EntityGraphNode) => {
    if (!isSearching) return true;
    const meta = nodeMeta(node.type);
    const detailText = Object.entries(node.details || {}).map(([key, value]) => `${key} ${value}`).join(" ");
    return `${node.id} ${node.type} ${meta.label} ${node.label} ${detailText}`.toLowerCase().includes(query);
  };
  const resetGraphView = () => {
    setZoom(1);
    setSearchTerm("");
    resetLayout();
  };
  const graphDownloadName = (ext: "svg" | "png") =>
    mode === "overview" ? `overview-knowledge-graph.${ext}` : `paper-knowledge-graph.${ext}`;
  const serializedGraphSvg = () => {
    const svg = svgRef.current;
    if (!svg) return "";
    const clone = svg.cloneNode(true) as SVGSVGElement;
    clone.setAttribute("xmlns", "http://www.w3.org/2000/svg");
    clone.setAttribute("width", String(VIEW_W));
    clone.setAttribute("height", String(VIEW_H));
    clone.setAttribute("viewBox", `0 0 ${VIEW_W} ${VIEW_H}`);
    return new XMLSerializer().serializeToString(clone);
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
    const source = serializedGraphSvg();
    if (!source) return;
    const blob = new Blob([`<?xml version="1.0" encoding="UTF-8"?>\n${source}`], { type: "image/svg+xml;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    triggerDownload(url, graphDownloadName("svg"));
    URL.revokeObjectURL(url);
    setDownloadOpen(false);
  };
  const downloadPng = () => {
    const source = serializedGraphSvg();
    if (!source) return;
    const blob = new Blob([source], { type: "image/svg+xml;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const image = new Image();
    image.onload = () => {
      const scale = 2;
      const canvas = document.createElement("canvas");
      canvas.width = VIEW_W * scale;
      canvas.height = VIEW_H * scale;
      const ctx = canvas.getContext("2d");
      if (!ctx) {
        URL.revokeObjectURL(url);
        return;
      }
      ctx.fillStyle = "#ffffff";
      ctx.fillRect(0, 0, canvas.width, canvas.height);
      ctx.setTransform(scale, 0, 0, scale, 0, 0);
      ctx.drawImage(image, 0, 0, VIEW_W, VIEW_H);
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

  const filterPanel = (
    <div className="flex h-full min-h-0 flex-col bg-background">
      <div className="border-b px-4 py-3">
        <div className="text-base font-semibold">Database information</div>
        <div className="mt-1 text-xs text-muted-foreground">单击筛选，双击节点标签展开或收回论文关系。</div>
      </div>
      <ScrollArea className="min-h-0 flex-1 overflow-hidden">
        <div className="space-y-5 p-4">
          {expandedNodeIDs.length > 0 && (
            <div className="flex items-center justify-between gap-2 rounded-md border border-primary/20 bg-primary/5 px-3 py-2 text-xs">
              <span className="text-muted-foreground">已展开 {expandedNodeIDs.length} 个节点</span>
              <Button type="button" variant="ghost" size="sm" className="h-7 px-2 text-xs" onClick={() => setExpandedNodeIDs([])}>
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
                *
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
                    graphFilter.kind === "all" ? "ring-2 ring-slate-500/40" : "",
                  )}
                  style={{
                    background: "#e5e7eb",
                    clipPath: "polygon(0 0, calc(100% - 10px) 0, 100% 50%, calc(100% - 10px) 100%, 0 100%, 10px 50%)",
                  }}
                  title="Show overview graph"
                  onClick={selectAllFilter}
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
      <aside className="min-h-0 overflow-hidden border-r bg-background">{filterPanel}</aside>
      <div className="relative min-h-0 flex-1 overflow-hidden bg-background">
      <div className="absolute left-3 right-3 top-3 z-10 flex flex-wrap items-start justify-between gap-2">
        <div className="flex flex-wrap items-center gap-2">
          <div className="relative">
            <Button
              variant={limitOpen ? "secondary" : "outline"}
              size="sm"
              title="设置图谱展示数量上限"
              onClick={() => setLimitOpen((open) => !open)}
            >
              <Hash className="size-4" />
              数量 {displayLimit}
            </Button>
            {limitOpen && (
              <div className="absolute left-0 top-[calc(100%+0.5rem)] z-20 flex items-center gap-2 rounded-md border bg-background/95 p-2 shadow-sm backdrop-blur">
                <input
                  type="number"
                  min={1}
                  value={displayLimit}
                  onChange={(event) => setDisplayLimit(clamp(Number.parseInt(event.target.value, 10) || 1, 1, 9999))}
                  className="h-8 w-24 rounded-md border bg-background px-2 text-sm outline-none focus:ring-2 focus:ring-primary/20"
                  autoFocus
                />
                <Button
                  variant="ghost"
                  size="icon-xs"
                  title="恢复默认数量"
                  onClick={() => setDisplayLimit(DEFAULT_GRAPH_DISPLAY_LIMIT)}
                >
                  <RotateCcw className="size-3.5" />
                </Button>
              </div>
            )}
          </div>
          <Button
            variant={searchOpen ? "secondary" : "outline"}
            size="sm"
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
            <div className="flex items-center gap-1 rounded-md border bg-background/95 px-2 py-1 shadow-sm backdrop-blur">
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
            <Button variant={keywordsOpen ? "secondary" : "outline"} size="sm" title="查看关键词" onClick={() => setKeywordsOpen((open) => !open)}>
              <Tags className="size-4" />
              关键词
            </Button>
            {keywordsOpen && (
              <div className="absolute left-0 top-[calc(100%+0.5rem)] z-20 w-72 max-w-[calc(100vw-2rem)] overflow-hidden rounded-lg border bg-popover shadow-lg">
                <div className="flex items-center justify-between border-b px-3 py-2">
                  <div className="text-sm font-medium">关键词</div>
                  <Button variant="ghost" size="icon-xs" title="关闭" onClick={() => setKeywordsOpen(false)}>
                    <X className="size-3.5" />
                  </Button>
                </div>
                <ScrollArea className="max-h-72">
                  <div className="p-3">
                    {keywordItems.length === 0 ? (
                      <p className="text-sm text-muted-foreground">暂无关键词数据</p>
                    ) : (
                      <div className="flex flex-wrap gap-2">
                        {keywordItems.map((item) => (
                          <span key={item.name} className="inline-flex items-center gap-1.5 rounded-full border bg-muted/45 px-3 py-1 text-xs" title={showKeywordCount && item.count != null ? `${item.count}` : item.name}>
                            {item.name}
                            {showKeywordCount && item.count != null && <span className="tabular-nums text-muted-foreground">{item.count}</span>}
                          </span>
                        ))}
                      </div>
                    )}
                  </div>
                </ScrollArea>
              </div>
            )}
          </div>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" title="重置图谱位置" onClick={resetGraphView}>
            <RotateCcw className="size-4" />
            重置
          </Button>
          <div className="relative">
            <Button variant={downloadOpen ? "secondary" : "outline"} size="sm" title="下载知识图谱" onClick={() => setDownloadOpen((open) => !open)}>
              <Download className="size-4" />
              下载
            </Button>
            {downloadOpen && (
              <div className="absolute right-0 top-[calc(100%+0.5rem)] z-20 w-36 overflow-hidden rounded-md border bg-popover p-1 shadow-lg">
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
        className="h-full min-h-[520px] w-full touch-none select-none bg-[radial-gradient(circle_at_center,color-mix(in_srgb,var(--muted)_72%,transparent)_1px,transparent_1px)] [background-size:22px_22px]"
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
          {edgesRef.current.map((edge) => {
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
          {nodes.map((node) => {
            const meta = nodeMeta(node.type);
            const matched = matchesSearch(node);
            const selectedNode = selected?.kind === "node" && selected.node.id === node.id;
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
                  if (!movedRef.current) setSelected({ kind: "node", node });
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
                      r={meta.radius +3}
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
                  <text
                    y={5}
                    textAnchor="middle"
                    className={cn("pointer-events-none font-medium", node.type === "paper" ? "fill-primary-foreground text-[11px]" : "fill-foreground text-[12px]")}
                    style={{ fill: labelColor, fontSize: nodeLabelFontSize(node) }}
                  >
                    {nodeLabelText(node)}
                  </text>
                </g>
              </g>
            );
          })}
        </g>
      </svg>
      <div className="pointer-events-none absolute bottom-3 left-3 z-10 max-w-[calc(100%-2rem)] rounded-md bg-background/82 px-2.5 py-1.5 text-xs text-muted-foreground shadow-sm backdrop-blur">
        鼠标滚轮缩放图谱大小，拖拽空白区域平移，拖拽节点调整位置。
        {mode === "overview" && " 总览页可双击任意节点展开或收回它与论文节点的关系。"}
      </div>
      {selected && (
        <div className="absolute bottom-3 right-3 z-20 flex max-h-[min(32rem,calc(100%-5.5rem))] w-fit min-w-56 max-w-[min(26rem,calc(100%-1.5rem))] flex-col overflow-hidden rounded-lg border bg-popover shadow-lg">
          <div className="flex shrink-0 items-center justify-between border-b px-3 py-2">
            <div className="min-w-0">
              <div className="truncate text-sm font-medium">{selected.kind === "node" ? nodeMeta(selected.node.type).label : "关系详情"}</div>
              <div className="truncate text-xs text-muted-foreground">{selected.kind === "node" ? selected.node.id : selected.edge.id}</div>
            </div>
            <Button variant="ghost" size="icon-xs" title="关闭" onClick={() => setSelected(null)}>
              <X className="size-3.5" />
            </Button>
          </div>
          <div className="min-h-0 overflow-y-auto overscroll-contain">
            <div className="space-y-3 p-3 pr-4">
              {selected.kind === "node" ? (
                <>
                  <div>
                    <div className="mb-1 text-xs text-muted-foreground">名称</div>
                    <div className="whitespace-pre-wrap break-words text-sm">{selected.node.label}</div>
                  </div>
                  {selected.node.type === "paper" &&
                    (() => {
                      const paperID = paperIDForNode(selected.node);
                      if (!paperID) {
                        return (
                          <div className="rounded-md border bg-muted/35 px-2.5 py-2 text-xs text-muted-foreground">
                            缺少论文 ID，无法跳转
                          </div>
                        );
                      }
                      return (
                        <Link
                          href={`/reader?id=${encodeURIComponent(paperID)}`}
                          className={cn(buttonVariants({ variant: "default", size: "sm" }), "w-full")}
                          onClick={() => onPaperRead?.(paperID)}
                        >
                          <FileText className="size-3.5" />
                          进入论文精读
                        </Link>
                      );
                    })()}
                  {selected.node.type === "reference" && referenceSearchText(selected.node) && (
                    <Button
                      type="button"
                      size="sm"
                      className="w-full"
                      onClick={() => void searchReferenceInPioneer(selected.node)}
                    >
                      <Search className="size-3.5" />
                      小云雀检索
                    </Button>
                  )}
                  {Object.entries(selected.node.details || {})
                    .filter(([k, v]) => {
                      if (!v) return false;
                      if (k === "name" && v === selected.node.label) return false;
                      if (k === "type") return false;
                      return true;
                    })
                    .map(([k, v]) => (
                      <div key={k}>
                        <div className="mb-1 text-xs text-muted-foreground">{k}</div>
                        <div className="whitespace-pre-wrap break-words text-sm">{v}</div>
                      </div>
                    ))}
                </>
              ) : (
                <>
                  <div>
                    <div className="mb-1 text-xs text-muted-foreground">关系</div>
                    <div className="whitespace-pre-wrap break-words text-sm">{edgeTypeLabel(selected.edge)}</div>
                  </div>
                  <div>
                    <div className="mb-1 text-xs text-muted-foreground">起点实体</div>
                    <div className="whitespace-pre-wrap break-words text-sm">{selected.source?.label || selected.edge.source}</div>
                  </div>
                  <div>
                    <div className="mb-1 text-xs text-muted-foreground">连接实体</div>
                    <div className="whitespace-pre-wrap break-words text-sm">{selected.target?.label || selected.edge.target}</div>
                  </div>
                  <div>
                    <div className="mb-1 text-xs text-muted-foreground">关系类型</div>
                    <div className="whitespace-pre-wrap break-words text-sm">{selected.edge.type}</div>
                  </div>
                  {Object.entries(selected.edge.details || {})
                    .filter(([, value]) => Boolean(value))
                    .map(([key, value]) => (
                      <div key={key}>
                        <div className="mb-1 text-xs text-muted-foreground">{key}</div>
                        <div className="whitespace-pre-wrap break-words text-sm">{value}</div>
                      </div>
                    ))}
                </>
              )}
            </div>
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

function mergeEntityGraphs(base: EntityGraph, addition: EntityGraph): EntityGraph {
  const baseNodes = dedupeEntityNodes(Array.isArray(base.nodes) ? base.nodes : []);
  const additionNodes = dedupeEntityNodes(Array.isArray(addition.nodes) ? addition.nodes : []);
  const baseEdges = dedupeEntityEdges(Array.isArray(base.edges) ? base.edges : []);
  const additionEdges = dedupeEntityEdges(Array.isArray(addition.edges) ? addition.edges : []);
  const nodes = new Map(baseNodes.map((node) => [node.id, node]));
  for (const node of additionNodes) {
    const current = nodes.get(node.id);
    nodes.set(
      node.id,
      current ? { ...current, details: { ...(current.details || {}), ...(node.details || {}) } } : node,
    );
  }

  const edges = new Map<string, EntityGraphEdge>();
  for (const edge of [...baseEdges, ...additionEdges]) {
    const key = edgeKey(edge);
    if (!edges.has(key)) edges.set(key, edge);
  }

  return {
    nodes: [...nodes.values()],
    edges: [...edges.values()],
  };
}

export function GraphView() {
  const { authReady, authed, papers, selectPaper } = useApp();

  const [keywords, setKeywords] = useState<NameCount[]>([]);
  const [entityGraph, setEntityGraph] = useState<EntityGraph | null>(null);
  const [overviewGraph, setOverviewGraph] = useState<EntityGraph | null>(null);
  const [loadingGraph, setLoadingGraph] = useState(false);
  const [rebuildingGraph, setRebuildingGraph] = useState(false);
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

  const loadFullOverviewGraph = useCallback(
    async (base: EntityGraph | null) => {
      let combined = normalizeEntityGraph(base) || { nodes: [], edges: [] };
      const detailResults = await Promise.allSettled(
        papers.map(async (paper) => normalizeEntityGraph(await api.paperEntityGraph(paper.id))),
      );
      for (const result of detailResults) {
        if (result.status !== "fulfilled" || !result.value) continue;
        combined = mergeEntityGraphs(combined, result.value);
      }
      return combined.nodes.length > 0 ? combined : null;
    },
    [papers],
  );

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
    api
      .graphNetwork()
      .then(async (g) => {
        if (!cancelled) {
          const normalized = normalizeEntityGraph(g);
          const fullGraph = await loadFullOverviewGraph(normalized);
          if (!cancelled) {
            setOverviewGraph(normalized);
            setEntityGraph(fullGraph || normalized);
          }
        }
      })
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
  }, [authed, loadFullOverviewGraph]);

  const rebuildCurrentGraph = () => {
    if (rebuildingGraph) return;
    setGraphError("");
    setGraphNotice("");
    setRebuildingGraph(true);
    const refreshStats = () => {
      api.graphKeywords(GRAPH_KEYWORD_LIMIT).then(setKeywords).catch(() => setKeywords([]));
    };
    setLoadingGraph(true);
    api
      .rebuildGraphNetwork()
      .then(async (g) => {
        const normalized = normalizeEntityGraph(g);
        const fullGraph = await loadFullOverviewGraph(normalized);
        setOverviewGraph(normalized);
        setEntityGraph(fullGraph || normalized);
        setGraphNotice("总览图谱已更新");
        refreshStats();
      })
      .catch((err) => {
        setGraphError(graphRequestErrorMessage(err, "总览知识图谱更新失败"));
      })
      .finally(() => {
        setLoadingGraph(false);
        setRebuildingGraph(false);
      });
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

      <div className="min-h-0 flex-1 overflow-hidden bg-background p-5">
        <section className="flex h-full min-h-0 min-w-0 flex-col rounded-lg border bg-card p-4 shadow-sm">
            <div className="mb-3 flex shrink-0 items-center gap-2 text-sm font-medium">
              <Network className="size-4 text-primary" />
              总览知识图谱
              <span className="rounded-full border bg-muted/45 px-2 py-0.5 text-xs font-normal text-muted-foreground">
                论文 {papers.length}
              </span>
            </div>

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
