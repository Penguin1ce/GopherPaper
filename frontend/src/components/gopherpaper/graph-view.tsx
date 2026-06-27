"use client";

import {
  ArrowLeft,
  FileText,
  LocateFixed,
  Maximize2,
  Network,
  RefreshCw,
  RotateCcw,
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
  GraphStats,
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

const TYPE_META: Record<string, { label: string; color: string; radius: number }> = {
  paper: { label: "论文名称", color: "#2563eb", radius: 50 },
  author: { label: "作者", color: "#16a34a", radius: 30 },
  affiliation: { label: "机构", color: "#0f766e", radius: 30 },
  keyword: { label: "关键词", color: "#0284c7", radius: 30 },
  research_question: { label: "研究问题", color: "#7c3aed", radius: 30 },
  method: { label: "方法", color: "#ea580c", radius: 28 },
  experiment: { label: "实验", color: "#ca8a04", radius: 27 },
  result: { label: "结果", color: "#dc2626", radius: 28 },
  innovation: { label: "创新点", color: "#db2777", radius: 26 },
  limitation: { label: "局限性", color: "#64748b", radius: 26 },
  future_work: { label: "未来工作", color: "#0891b2", radius: 25 },
  venue: { label: "发表来源", color: "#475569", radius: 22 },
  entity: { label: "实体", color: "#64748b", radius: 21 },
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

function clamp(n: number, min: number, max: number) {
  return Math.max(min, Math.min(max, n));
}

function shortText(s: string, n = 18) {
  const t = (s || "").trim();
  if (!t) return "未命名";
  const chars = [...t];
  return chars.length > n ? `${chars.slice(0, n).join("")}...` : t;
}

function nodeMeta(type: string) {
  return TYPE_META[type] || TYPE_META.entity;
}

function collisionRadius(node: Pick<EntityGraphNode, "type" | "label">) {
  const base = nodeMeta(node.type).radius;
  const labelPad = Math.min(17, Math.max(5, [...(node.label || "")].length * 1.25));
  return base + labelPad;
}

function StatChip({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="rounded-lg border bg-card px-4 py-2.5 shadow-sm">
      <div className="text-xl font-semibold tabular-nums">{value}</div>
      <div className="text-xs text-muted-foreground">{label}</div>
    </div>
  );
}

function ForceEntityGraph({
  graph,
  mode = "detail",
  onPaperDoubleClick,
}: {
  graph: EntityGraph;
  mode?: "overview" | "detail";
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

  const resetLayout = () => {
    const center = { x: VIEW_W / 2, y: VIEW_H / 2 };
    const next: SimNode[] = [];
    if (mode === "detail") {
      const others = graphNodes.filter((n) => n.type !== "paper");
      const paper = graphNodes.find((n) => n.type === "paper") || graphNodes[0];
      if (paper) {
        next.push({ ...paper, x: center.x, y: center.y, vx: 0, vy: 0 });
      }
      others.forEach((n, i) => {
        const angle = (-Math.PI / 2) + (i * Math.PI * 2) / Math.max(1, others.length);
        const ring = 80 + 20 * (i % 3);
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
      const paperRing = paperNodes.length <= 1 ? 0 : clamp(40 + paperNodes.length * 10, 60, 120);

      paperNodes.forEach((n, i) => {
        const angle = (-Math.PI / 2) + (i * Math.PI * 2) / Math.max(1, paperNodes.length);
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
          const angle = (-Math.PI / 2) + slot * 2.3999632297;
          const ring = 74 + 18 * (slot % 3);
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
          const angle = (-Math.PI / 2) + (i * Math.PI * 2) / Math.max(1, nonPaperNodes.length);
          next.push({
            ...n,
            x: x + Math.cos(angle) * 24,
            y: y + Math.sin(angle) * 24,
            vx: 0,
            vy: 0,
          });
          return;
        }
        const angle = (-Math.PI / 2) + (i * Math.PI * 2) / Math.max(1, graphNodes.length);
        const ring = 70 + 15 * (i % 3);
        next.push({
          ...n,
          x: center.x + Math.cos(angle) * ring,
          y: center.y + Math.sin(angle) * ring,
          vx: 0,
          vy: 0,
        });
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
            const force =
              mode === "overview"
                ? (120 * alpha) / d2 + (overlap * 0.05) / dist
                : (220 * alpha) / d2 + (overlap * 0.08) / dist;
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
            mode === "overview" ? 18 : b.type === "keyword" || b.type === "author" ? 5 : 8,
            collisionRadius(a) + collisionRadius(b) + (mode === "overview" ? 4 : 0),
          );
          const pull = (dist - ideal) * (mode === "overview" ? 0.026 : 0.020) * alpha;
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
          if (n.type === "paper") {
            const centerStrength = mode === "overview" ? 0.007 : 0.025;
            n.vx += (VIEW_W / 2 - n.x) * centerStrength * alpha;
            n.vy += (VIEW_H / 2 - n.y) * centerStrength * alpha;
          } else {
            const centerStrength = mode === "overview" ? 0.005 : 0.004;
            n.vx += (VIEW_W / 2 - n.x) * centerStrength * alpha;
            n.vy += (VIEW_H / 2 - n.y) * centerStrength * alpha;
          }
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
    return {
      x: ((clientX - rect.left) / rect.width) * VIEW_W,
      y: ((clientY - rect.top) / rect.height) * VIEW_H,
    };
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
      setPan({
        x: point.x - before.x * nextZoom,
        y: point.y - before.y * nextZoom,
      });
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
    dragRef.current = {
      mode: "pan",
      startX: event.clientX,
      startY: event.clientY,
      panX: pan.x,
      panY: pan.y,
      moved: false,
    };
  };

  const onPointerMove = (event: ReactPointerEvent<SVGSVGElement>) => {
    const drag = dragRef.current;
    if (!drag) return;
    const dx = event.clientX - drag.startX;
    const dy = event.clientY - drag.startY;
    const dragDistance = Math.hypot(dx, dy);
    if (dragDistance > NODE_DRAG_THRESHOLD) {
      drag.moved = true;
      movedRef.current = true;
    }
    if (drag.mode === "pan") {
      const rect = svgRef.current?.getBoundingClientRect();
      if (!rect) return;
      setPan({
        x: drag.panX + (dx / rect.width) * VIEW_W,
        y: drag.panY + (dy / rect.height) * VIEW_H,
      });
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

  return (
    <div className="relative overflow-hidden rounded-lg border bg-background">
      <div className="absolute right-3 top-3 z-10 flex gap-2">
        <Button variant="outline" size="icon-sm" title="重置缩放" onClick={() => setZoom(1)}>
          <Maximize2 className="size-4" />
        </Button>
        <Button
          variant="outline"
          size="icon-sm"
          title="重置位置"
          onClick={() => {
            setZoom(1);
            resetLayout();
          }}
        >
          <LocateFixed className="size-4" />
        </Button>
        <Button
          variant="outline"
          size="icon-sm"
          title="力导向布局"
          onClick={() => {
            alphaRef.current = 0.85;
            nodesRef.current.forEach((n) => {
              n.vx += (Math.random() - 0.5) * 8;
              n.vy += (Math.random() - 0.5) * 8;
            });
          }}
        >
          <RotateCcw className="size-4" />
        </Button>
      </div>
      <svg
        ref={svgRef}
        viewBox={`0 0 ${VIEW_W} ${VIEW_H}`}
        className="h-[62vh] min-h-[420px] w-full touch-none select-none bg-[radial-gradient(circle_at_center,color-mix(in_srgb,var(--muted)_72%,transparent)_1px,transparent_1px)] [background-size:22px_22px]"
        role="img"
        aria-label="论文知识图谱"
        onPointerDown={onSvgDown}
        onPointerMove={onPointerMove}
        onPointerUp={onPointerUp}
        onPointerCancel={onPointerUp}
      >
        <g transform={`translate(${pan.x} ${pan.y}) scale(${zoom})`}>
          {edgesRef.current.map((edge) => {
            const a = nodeByID.get(edge.source);
            const b = nodeByID.get(edge.target);
            if (!a || !b) return null;
            const color = nodeMeta(b.type).color;
            const midX = (a.x + b.x) / 2;
            const midY = (a.y + b.y) / 2;
            return (
              <g
                key={edge.id}
                className="cursor-pointer"
                onPointerDown={(e) => e.stopPropagation()}
                onClick={(e) => {
                  e.stopPropagation();
                  setSelected({ kind: "edge", edge, source: a, target: b });
                }}
              >
                <line
                  x1={a.x}
                  y1={a.y}
                  x2={b.x}
                  y2={b.y}
                  stroke="transparent"
                  strokeWidth={18}
                />
                <line
                  x1={a.x}
                  y1={a.y}
                  x2={b.x}
                  y2={b.y}
                  stroke={color}
                  strokeOpacity={0.42}
                  strokeWidth={1.6}
                />
                <text
                  x={midX}
                  y={midY - 5}
                  textAnchor="middle"
                  className="fill-muted-foreground text-[10px]"
                >
                  {edge.label}
                </text>
              </g>
            );
          })}
          {nodes.map((node) => {
            const meta = nodeMeta(node.type);
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
                  if (node.type === "paper") {
                    onPaperDoubleClick?.(node.id.replace(/^paper:/, ""));
                  }
                }}
              >
                <circle
                  r={meta.radius}
                  fill={meta.color}
                  fillOpacity={node.type === "paper" ? 0.95 : 0.16}
                  stroke={meta.color}
                  strokeWidth={node.type === "paper" ? 0 : 2}
                />
                <text
                  y={node.type === "paper" ? -4 : 5}
                  textAnchor="middle"
                  className={cn(
                    "pointer-events-none font-medium",
                    node.type === "paper"
                      ? "fill-primary-foreground text-[11px]"
                      : "fill-foreground text-[12px]",
                  )}
                >
                  {shortText(node.label, node.type === "paper" ? 10 : 12)}
                </text>
                <text
                  y={meta.radius + 15}
                  textAnchor="middle"
                  className="pointer-events-none fill-muted-foreground text-[12px]"
                >
                  {node.type === "paper" ? "" : meta.label}
                </text>
              </g>
            );
          })}
        </g>
      </svg>
      {selected && (
        <div className="absolute bottom-3 right-3 z-20 max-h-[70%] w-[min(24rem,calc(100%-1.5rem))] overflow-hidden rounded-lg border bg-popover shadow-lg">
          <div className="flex items-center justify-between border-b px-3 py-2">
            <div className="min-w-0">
              <div className="truncate text-sm font-medium">
                {selected.kind === "node" ? nodeMeta(selected.node.type).label : "关系详情"}
              </div>
              <div className="truncate text-xs text-muted-foreground">
                {selected.kind === "node" ? selected.node.id : selected.edge.id}
              </div>
            </div>
            <Button variant="ghost" size="icon-xs" title="关闭" onClick={() => setSelected(null)}>
              <X className="size-3.5" />
            </Button>
          </div>
          <ScrollArea className="max-h-[20rem]">
            <div className="space-y-3 p-3">
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
                    <div className="whitespace-pre-wrap break-words text-sm">
                      {selected.source?.label || selected.edge.source}
                    </div>
                  </div>
                  <div>
                    <div className="mb-1 text-xs text-muted-foreground">连接实体</div>
                    <div className="whitespace-pre-wrap break-words text-sm">
                      {selected.target?.label || selected.edge.target}
                    </div>
                  </div>
                  <div>
                    <div className="mb-1 text-xs text-muted-foreground">关系类型</div>
                    <div className="whitespace-pre-wrap break-words text-sm">{selected.edge.type}</div>
                  </div>
                </>
              )}
            </div>
          </ScrollArea>
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
    const key = edge.id || `${edge.source}:${edge.type}:${edge.target}:${edge.label}`;
    if (!edges.has(key)) edges.set(key, edge);
  }

  return {
    nodes: [...nodes.values()],
    edges: [...edges.values()],
  };
}

export function GraphView() {
  const { authed, papers, activePaperID, selectPaper } = useApp();

  const [overview, setOverview] = useState<GraphStats | null>(null);
  const [keywords, setKeywords] = useState<NameCount[]>([]);
  const [entityGraph, setEntityGraph] = useState<EntityGraph | null>(null);
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
      .graphOverview()
      .then((value) => {
        if (!cancelled) setOverview(value);
      })
      .catch(() => {
        if (!cancelled) setOverview(null);
      });
    api
      .graphKeywords(24)
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
      return;
    }
    let cancelled = false;
    setGraphError("");
    setGraphNotice("");
    setLoadingGraph(true);
    api
      .graphNetwork()
      .then((g) => {
        if (!cancelled) setEntityGraph(normalizeEntityGraph(g));
      })
      .catch((err) => {
        if (!cancelled) {
          setEntityGraph(null);
          setGraphError(graphRequestErrorMessage(err, "总览知识图谱加载失败"));
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
          setGraphError(graphRequestErrorMessage(err, "论文知识图谱加载失败"));
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
      .then((g) => setEntityGraph(normalizeEntityGraph(g)))
      .catch((err) => {
        setEntityGraph(null);
        setGraphError(graphRequestErrorMessage(err, "总览知识图谱加载失败"));
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
    api
      .paperEntityGraph(paperID)
      .then((g) => {
        const nextGraph = normalizeEntityGraph(g);
        setEntityGraph((current) => {
          const currentGraph = normalizeEntityGraph(current);
          if (!currentGraph) return nextGraph;
          if (!nextGraph) return currentGraph;
          return mergeEntityGraphs(currentGraph, nextGraph);
        });
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
      api.graphOverview().then(setOverview).catch(() => setOverview(null));
      api.graphKeywords(24).then(setKeywords).catch(() => setKeywords([]));
    };
    if (graphMode === "overview") {
      setGraphMode("overview");
      setDetailPaperID("");
      setLoadingGraph(true);
      api
        .rebuildGraphNetwork()
        .then((g) => {
          setEntityGraph(normalizeEntityGraph(g));
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
  const maxKw = Math.max(1, ...keywords.map((k) => k.count));

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
          <div className="px-4 py-3 text-xs font-medium uppercase tracking-wide text-muted-foreground">
            论文
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

        <ScrollArea className="min-h-0 bg-background">
          <div className="space-y-6 p-5">
            {graphMode === "overview" && (
            <section className="grid grid-cols-2 gap-3 sm:grid-cols-4">
              <StatChip label="论文" value={overview?.papers ?? "-"} />
              <StatChip label="作者" value={overview?.authors ?? "-"} />
              <StatChip label="关键词" value={overview?.keywords ?? "-"} />
              <StatChip label="实体节点" value={entityGraph?.nodes.length ?? "-"} />
            </section>
            )}

            <section className="rounded-lg border bg-card p-4 shadow-sm">
              <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
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
                <div className="mb-3 rounded-md border border-destructive/25 bg-destructive/10 px-3 py-2 text-sm text-destructive">
                  {graphError}
                </div>
              )}
              {graphNotice && !graphError && (
                <div className="mb-3 rounded-md border border-emerald-500/25 bg-emerald-500/10 px-3 py-2 text-sm text-emerald-700">
                  {graphNotice}
                </div>
              )}

              {graphMode === "detail" && !centerPaper ? (
                <Empty title="选择一篇论文" text="图谱会显示论文与结构化实体的关系。" compact />
              ) : loadingGraph ? (
                <div className="flex h-[420px] items-center justify-center text-sm text-muted-foreground">
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
                  onPaperDoubleClick={graphMode === "overview" ? expandPaperInOverview : undefined}
                />
              )}
            </section>

            <section className="grid grid-cols-1 gap-4">
              <div className="rounded-lg border bg-card p-4 shadow-sm">
                <div className="mb-3 text-sm font-medium">关键词汇总</div>
                {graphMode === "overview" &&
                  (keywords.length === 0 ? (
                    <p className="text-sm text-muted-foreground">暂无关键词数据</p>
                  ) : (
                    <div className="flex flex-wrap gap-2">
                      {keywords.map((k) => (
                        <span
                          key={k.name}
                          className="inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-xs"
                          style={{
                            background: `color-mix(in srgb, var(--primary) ${8 + 22 * (k.count / maxKw)}%, transparent)`,
                          }}
                          title={`${k.count}`}
                        >
                          {k.name}
                          <span className="tabular-nums text-muted-foreground">{k.count}</span>
                        </span>
                      ))}
                    </div>
                  ))}
                {graphMode === "detail" &&
                  (detailKeywords.length === 0 ? (
                    <p className="text-sm text-muted-foreground">暂无关键词数据</p>
                  ) : (
                    <div className="flex flex-wrap gap-2">
                      {detailKeywords.map((keyword) => (
                        <span
                          key={keyword}
                          className="inline-flex items-center rounded-full border bg-muted/45 px-3 py-1 text-xs"
                        >
                          {keyword}
                        </span>
                      ))}
                    </div>
                  ))}
              </div>
            </section>
          </div>
        </ScrollArea>
      </div>
    </main>
  );
}
