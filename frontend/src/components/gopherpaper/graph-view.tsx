"use client";

import {
  ArrowLeft,
  Download,
  FileText,
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

const VIEW_W = 1000;
const VIEW_H = 620;
const MIN_ZOOM = 0.35;
const MAX_ZOOM = 2.6;
const NODE_DRAG_THRESHOLD = 8;
const GRAPH_KEYWORD_LIMIT = 10000;
const PAPER_NODE_COLOR = "#35A98D";
const GRAPH_EDGE_COLOR = "#94a3b8";

const TYPE_META: Record<string, { label: string; color: string; radius: number }> = {
  paper: { label: "论文名称", color: PAPER_NODE_COLOR, radius: 50 },
  author: { label: "作者", color: "#6CCB72", radius: 30 },
  affiliation: { label: "机构", color: "#36B9B5", radius: 30 },
  keyword: { label: "关键词", color: "#4EA8F1", radius: 30 },
  research_question: { label: "研究问题", color: "#A88AF0", radius: 30 },
  method: { label: "方法", color: "#F0A14A", radius: 28 },
  experiment: { label: "实验", color: "#DDB33F", radius: 27 },
  result: { label: "结果", color: "#E66C73", radius: 28 },
  innovation: { label: "创新点", color: "#E578B7", radius: 26 },
  limitation: { label: "局限性", color: "#8A94A6", radius: 26 },
  future_work: { label: "未来工作", color: "#35BFD0", radius: 25 },
  venue: { label: "发表来源", color: "#7184A1", radius: 22 },
  entity: { label: "实体", color: "#8A94A6", radius: 21 },
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
  const maxChars = node.type === "paper" ? 9 : Math.max(4, Math.floor(radius / 4.6));
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

function collisionRadius(node: Pick<EntityGraphNode, "type" | "label">) {
  const base = nodeMeta(node.type).radius;
  const labelPad = Math.min(17, Math.max(5, [...(node.label || "")].length * 1.25));
  return base + labelPad;
}

function ForceEntityGraph({
  graph,
  mode = "detail",
  keywordItems = [],
  showKeywordCount = false,
  onPaperDoubleClick,
}: {
  graph: EntityGraph;
  mode?: "overview" | "detail";
  keywordItems?: GraphKeywordItem[];
  showKeywordCount?: boolean;
  onPaperDoubleClick?: (paperID: string) => void;
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
  const graphNodes = Array.isArray(graph.nodes) ? graph.nodes : [];
  const graphEdges = Array.isArray(graph.edges) ? graph.edges : [];
  const [searchOpen, setSearchOpen] = useState(false);
  const [searchTerm, setSearchTerm] = useState("");
  const [keywordsOpen, setKeywordsOpen] = useState(false);

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
    resetLayout();
    setSelected(null);
    setZoom(1);
    setSearchTerm("");
    setKeywordsOpen(false);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [graph, mode]);

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
  const downloadSvg = () => {
    const svg = svgRef.current;
    if (!svg) return;
    const clone = svg.cloneNode(true) as SVGSVGElement;
    clone.setAttribute("xmlns", "http://www.w3.org/2000/svg");
    clone.setAttribute("width", String(VIEW_W));
    clone.setAttribute("height", String(VIEW_H));
    const source = new XMLSerializer().serializeToString(clone);
    const blob = new Blob([`<?xml version="1.0" encoding="UTF-8"?>\n${source}`], { type: "image/svg+xml;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = mode === "overview" ? "overview-knowledge-graph.svg" : "paper-knowledge-graph.svg";
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  };

  return (
    <div className="relative min-h-[520px] flex-1 overflow-hidden rounded-lg border bg-background">
      <div className="absolute left-3 right-3 top-3 z-10 flex flex-wrap items-start justify-between gap-2">
        <div className="flex flex-wrap items-center gap-2">
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
          <Button variant="outline" size="sm" title="下载 SVG" onClick={downloadSvg}>
            <Download className="size-4" />
            下载
          </Button>
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
            const color = isSearching ? "#cbd5e1" : GRAPH_EDGE_COLOR;
            const line = edgeLinePoints(a, b);
            const midX = (line.x1 + line.x2) / 2;
            const midY = (line.y1 + line.y2) / 2;
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
                  stroke={color}
                  strokeOpacity={isSearching ? 0.35 : 0.42}
                  strokeWidth={1.6}
                  markerEnd="url(#graph-edge-arrow)"
                  filter={selectedEdge ? "url(#graph-selected-shadow)" : undefined}
                />
                <text x={midX} y={midY - 5} textAnchor="middle" className="fill-muted-foreground text-[10px]" style={{ fill: isSearching ? "#94a3b8" : "#64748b", fontWeight: 500 }}>
                  {edge.label}
                </text>
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
                  if (node.type === "paper") onPaperDoubleClick?.(node.id.replace(/^paper:/, ""));
                }}
              >
                <g transform={`scale(${selectedNode ? 1.045 : 1})`} style={{ transition: "transform 160ms ease-out" }} filter={selectedNode ? "url(#graph-selected-shadow)" : undefined}>
                  <circle r={meta.radius} fill={nodeColor} fillOpacity={matched ? 1 : 0.32} stroke="transparent" strokeWidth={0} />
                  <text
                    y={node.type === "paper" ? -4 : 5}
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
        {mode === "overview" && " 总览页可双击论文节点展开或收回详细节点信息。"}
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
                    <div className="whitespace-pre-wrap break-words text-sm">{selected.edge.label}</div>
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
  );
}
function graphRequestErrorMessage(err: unknown, fallback: string) {
  return err instanceof api.ApiError && err.message ? err.message : fallback;
}

function normalizeEntityGraph(graph: EntityGraph | null | undefined): EntityGraph | null {
  if (!graph || !Array.isArray(graph.nodes)) return null;
  return {
    nodes: graph.nodes,
    edges: Array.isArray(graph.edges) ? graph.edges : [],
  };
}

function mergeEntityGraphs(base: EntityGraph, addition: EntityGraph): EntityGraph {
  const baseNodes = Array.isArray(base.nodes) ? base.nodes : [];
  const additionNodes = Array.isArray(addition.nodes) ? addition.nodes : [];
  const baseEdges = Array.isArray(base.edges) ? base.edges : [];
  const additionEdges = Array.isArray(addition.edges) ? addition.edges : [];
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

function composeOverviewGraph(base: EntityGraph | null, expandedGraphs: Record<string, EntityGraph>) {
  let next = normalizeEntityGraph(base);
  for (const addition of Object.values(expandedGraphs)) {
    const normalized = normalizeEntityGraph(addition);
    if (!normalized) continue;
    next = next ? mergeEntityGraphs(next, normalized) : normalized;
  }
  return next;
}

export function GraphView() {
  const { authed, papers, activePaperID, selectPaper } = useApp();

  const [keywords, setKeywords] = useState<NameCount[]>([]);
  const [entityGraph, setEntityGraph] = useState<EntityGraph | null>(null);
  const [overviewBaseGraph, setOverviewBaseGraph] = useState<EntityGraph | null>(null);
  const [expandedPaperGraphs, setExpandedPaperGraphs] = useState<Record<string, EntityGraph>>({});
  const [loadingGraph, setLoadingGraph] = useState(false);
  const [rebuildingGraph, setRebuildingGraph] = useState(false);
  const [graphError, setGraphError] = useState("");
  const [graphNotice, setGraphNotice] = useState("");
  const [graphMode, setGraphMode] = useState<"overview" | "detail">("overview");
  const [detailPaperID, setDetailPaperID] = useState("");

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

  useEffect(() => {
    if (!authed) {
      setEntityGraph(null);
      setOverviewBaseGraph(null);
      setExpandedPaperGraphs({});
      return;
    }
    let cancelled = false;
    setGraphError("");
    setGraphNotice("");
    setLoadingGraph(true);
    api
      .graphNetwork()
      .then((g) => {
        if (!cancelled) {
          const normalized = normalizeEntityGraph(g);
          setOverviewBaseGraph(normalized);
          setExpandedPaperGraphs({});
          setEntityGraph(normalized);
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
  }, [authed]);

  useEffect(() => {
    if (!authed || graphMode !== "detail" || !detailPaperID) return;
    let cancelled = false;
    setGraphError("");
    setGraphNotice("");
    setLoadingGraph(true);
    api
      .paperEntityGraph(detailPaperID)
      .then((g) => {
        if (!cancelled) setEntityGraph(normalizeEntityGraph(g));
      })
      .catch((err) => {
        if (!cancelled) {
          setEntityGraph(null);
          setGraphError(graphRequestErrorMessage(err, "璁烘枃鐭ヨ瘑鍥捐氨鍔犺浇澶辫触"));
        }
      })
      .finally(() => {
        if (!cancelled) setLoadingGraph(false);
      });
    return () => {
      cancelled = true;
    };
  }, [authed, graphMode, detailPaperID]);

  const openOverviewGraph = () => {
    setGraphMode("overview");
    setDetailPaperID("");
    setGraphError("");
    setGraphNotice("");
    setLoadingGraph(true);
    api
      .graphNetwork()
      .then((g) => {
        const normalized = normalizeEntityGraph(g);
        setOverviewBaseGraph(normalized);
        setExpandedPaperGraphs({});
        setEntityGraph(normalized);
      })
      .catch((err) => {
        setEntityGraph(null);
        setGraphError(graphRequestErrorMessage(err, "鎬昏鐭ヨ瘑鍥捐氨鍔犺浇澶辫触"));
      })
      .finally(() => setLoadingGraph(false));
  };

  const openDetailGraph = (paperID: string) => {
    setGraphNotice("");
    selectPaper(paperID);
    setDetailPaperID(paperID);
    setGraphMode("detail");
  };

  const expandPaperInOverview = (paperID: string) => {
    if (graphMode !== "overview") return;
    setGraphError("");
    setGraphNotice("");
    selectPaper(paperID);
    setDetailPaperID(paperID);
    if (expandedPaperGraphs[paperID]) {
      const nextExpanded = { ...expandedPaperGraphs };
      delete nextExpanded[paperID];
      setExpandedPaperGraphs(nextExpanded);
      setEntityGraph(composeOverviewGraph(overviewBaseGraph, nextExpanded));
      setGraphNotice("已收回论文实体");
      return;
    }
    api
      .paperEntityGraph(paperID)
      .then((g) => {
        const nextGraph = normalizeEntityGraph(g);
        if (!nextGraph) return;
        const nextExpanded = { ...expandedPaperGraphs, [paperID]: nextGraph };
        setExpandedPaperGraphs(nextExpanded);
        setEntityGraph(composeOverviewGraph(overviewBaseGraph || entityGraph, nextExpanded));
        if (nextGraph) setGraphNotice("已在总览图谱中展开论文实体");
      })
      .catch((err) => {
        setGraphError(graphRequestErrorMessage(err, "论文知识图谱展开失败"));
      });
  };

  const rebuildCurrentGraph = () => {
    if (rebuildingGraph) return;
    setGraphError("");
    setGraphNotice("");
    setRebuildingGraph(true);
    const refreshStats = () => {
      api.graphKeywords(GRAPH_KEYWORD_LIMIT).then(setKeywords).catch(() => setKeywords([]));
    };
    if (graphMode === "overview") {
      setGraphMode("overview");
      setDetailPaperID("");
      setLoadingGraph(true);
      api
        .rebuildGraphNetwork()
        .then((g) => {
          const normalized = normalizeEntityGraph(g);
          setOverviewBaseGraph(normalized);
          setExpandedPaperGraphs({});
          setEntityGraph(normalized);
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
      return;
    }
    const paperID = detailPaperID || activePaperID;
    if (!paperID) {
      setGraphError("请先选择一篇论文");
      setRebuildingGraph(false);
      return;
    }
    api
      .rebuildPaperEntityGraph(paperID)
      .then((g) => {
        setDetailPaperID(paperID);
        setEntityGraph(normalizeEntityGraph(g));
        setGraphNotice("论文图谱已更新");
        refreshStats();
      })
      .catch((err) => {
        setGraphError(graphRequestErrorMessage(err, "论文知识图谱更新失败"));
      })
      .finally(() => setRebuildingGraph(false));
  };

  const centerPaper = useMemo(
    () => papers.find((p) => p.id === activePaperID) || null,
    [papers, activePaperID],
  );
  const detailKeywords = useMemo(() => {
    const graphKeywords =
      entityGraph?.nodes
        .filter((node) => node.type === "keyword")
        .map((node) => node.label.trim())
        .filter(Boolean) || [];
    const fallbackKeywords = centerPaper?.keywords?.map((kw) => kw.trim()).filter(Boolean) || [];
    return [...new Set(graphKeywords.length > 0 ? graphKeywords : fallbackKeywords)];
  }, [centerPaper, entityGraph]);
  const graphKeywordItems = useMemo<GraphKeywordItem[]>(() => {
    if (graphMode === "overview") return keywords.map((k) => ({ name: k.name, count: k.count }));
    return detailKeywords.map((name) => ({ name }));
  }, [detailKeywords, graphMode, keywords]);

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
    <main className="flex h-dvh min-h-[640px] flex-col overflow-hidden bg-muted/50">
      <header className="flex h-14 shrink-0 items-center gap-3 border-b bg-background px-4">
        <Link
          href="/"
          className={buttonVariants({ variant: "ghost", size: "icon", className: "shrink-0" })}
          title="返回工作台"
        >
          <ArrowLeft className="size-4" />
        </Link>
        <div className="min-w-0">
          <h1 className="font-serif text-base font-semibold tracking-tight">
            {graphMode === "overview" ? "知识图谱 · 总览" : "知识图谱 · 论文实体"}
          </h1>
          <p className="truncate text-xs text-muted-foreground">
            {graphMode === "overview"
              ? "双击论文节点展开详细知识图谱"
              : centerPaper ? paperTitle(centerPaper) : "选择一篇论文"}
          </p>
        </div>
        <div className="ml-auto flex shrink-0 items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={rebuildCurrentGraph}
            disabled={rebuildingGraph || (graphMode === "detail" && !detailPaperID && !activePaperID)}
            title={graphMode === "overview" ? "从 MySQL 更新总览图谱" : "从 MySQL 更新当前论文图谱"}
          >
            <RefreshCw className={cn("size-4", rebuildingGraph && "animate-spin")} />
            {graphMode === "overview" ? "更新总览" : "更新论文"}
          </Button>
          <Button variant="outline" size="sm" className="lg:hidden" onClick={openOverviewGraph}>
            总览图谱
          </Button>
        </div>
      </header>

      <div className="grid min-h-0 flex-1 grid-cols-1 lg:grid-cols-[18rem_minmax(0,1fr)]">
        <aside className="hidden min-h-0 flex-col border-r bg-background lg:flex">
          <div className="sticky top-0 z-10 border-b bg-background p-3">
            <button
              type="button"
              onClick={openOverviewGraph}
              className={cn(
                "flex w-full items-center gap-2 rounded-md border px-3 py-2.5 text-left transition-colors",
                graphMode === "overview"
                  ? "border-primary/35 bg-primary/10 shadow-sm"
                  : "border-border bg-muted/40 hover:bg-muted",
              )}
            >
              <Network className="size-4 shrink-0 text-primary" />
              <span className="min-w-0 flex-1">
                <span className="block text-sm font-semibold">总览图谱</span>
                <span className="block truncate text-xs text-muted-foreground">
                  论文 - 共享点 - 论文
                </span>
              </span>
            </button>
          </div>
          <div className="flex items-center justify-between px-4 py-3 text-xs font-medium uppercase tracking-wide text-muted-foreground">
            <span>论文</span>
            <span className="rounded-full border bg-muted/45 px-2 py-0.5 tabular-nums">{papers.length}</span>
          </div>
          <ScrollArea className="min-h-0 flex-1">
            <div className="space-y-1 px-3 pb-4">
              {papers.length === 0 ? (
                <Empty title="还没有论文" text="先在工作台上传并解析论文。" compact />
              ) : (
                papers.map((p) => (
                  <button
                    key={p.id}
                    type="button"
                    onClick={() => openDetailGraph(p.id)}
                    className={cn(
                      "flex w-full items-start gap-2 rounded-md px-2.5 py-2 text-left transition-colors",
                      p.id === activePaperID
                        ? "bg-card shadow-sm ring-1 ring-primary/20"
                        : "hover:bg-accent/60",
                    )}
                  >
                    <FileText
                      className={cn(
                        "mt-0.5 size-3.5 shrink-0",
                        p.id === activePaperID ? "text-primary" : "text-muted-foreground",
                      )}
                    />
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-sm font-medium">{paperTitle(p)}</span>
                      {p.status !== "ready" && (
                        <span className="text-xs text-muted-foreground">{p.status}</span>
                      )}
                    </span>
                  </button>
                ))
              )}
            </div>
          </ScrollArea>
        </aside>

        <ScrollArea className="h-full min-h-0 bg-background">
          <div className="h-full min-h-0 p-5">
            <div className="grid h-full min-h-0">
              <section className="flex min-h-[calc(100dvh-7rem)] min-w-0 flex-col rounded-lg border bg-card p-4 shadow-sm xl:min-h-0">
                <div className="mb-3 flex shrink-0 flex-wrap items-center justify-between gap-3">
                  <div className="flex items-center gap-2 text-sm font-medium">
                    <Network className="size-4 text-primary" />
                    {graphMode === "overview" ? "总览知识图谱" : "论文中心图谱"}
                  </div>
                  <div className="flex flex-wrap gap-2">
                    {Object.entries(TYPE_META)
                      .filter(([k]) => !["entity", "venue"].includes(k))
                      .map(([k, meta]) => (
                        <span key={k} className="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
                          <span className="size-2.5 rounded-full" style={{ background: meta.color }} />
                          {meta.label}
                        </span>
                      ))}
                  </div>
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

                {graphMode === "detail" && !centerPaper ? (
                  <Empty title="选择一篇论文" text="图谱会显示论文与结构化实体的关系。" compact />
                ) : loadingGraph ? (
                  <div className="flex min-h-[520px] flex-1 items-center justify-center text-sm text-muted-foreground">
                    加载中...
                  </div>
                ) : !entityGraph || entityGraph.nodes.length <= 1 ? (
                  <Empty
                    title={graphMode === "overview" ? "暂无共享关系" : "暂无实体关系"}
                    text={
                      graphMode === "overview"
                        ? "当论文共享作者、关键词或机构时，会在这里形成总览关系。"
                        : "解析完成后会自动生成论文知识图谱。"
                    }
                    compact
                  />
                ) : (
                  <ForceEntityGraph
                    graph={entityGraph}
                    mode={graphMode}
                    keywordItems={graphKeywordItems}
                    showKeywordCount={graphMode === "overview"}
                    onPaperDoubleClick={graphMode === "overview" ? expandPaperInOverview : undefined}
                  />
                )}
              </section>
            </div>
          </div>
        </ScrollArea>
      </div>
    </main>
  );
}
