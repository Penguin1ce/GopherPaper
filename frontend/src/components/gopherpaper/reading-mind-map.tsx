"use client";

import {
  Background,
  Controls,
  Handle,
  MarkerType,
  PanOnScrollMode,
  Position,
  ReactFlow,
  ReactFlowProvider,
  useNodesState,
  useReactFlow,
  type Edge,
  type Node,
  type NodeMouseHandler,
  type NodeProps,
  type OnNodeDrag,
} from "@xyflow/react";
import {
  AlignHorizontalSpaceAround,
  BookOpenText,
  ChevronDown,
  ChevronRight,
  Download,
  Edit3,
  FileText,
  FolderTree,
  Highlighter,
  Loader2,
  MessageSquareText,
  Network,
  Plus,
  RefreshCw,
  StickyNote,
  Trash2,
  type LucideIcon,
} from "lucide-react";
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type CSSProperties,
  type FormEvent,
  type PointerEvent as ReactPointerEvent,
} from "react";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import * as api from "@/lib/gopherpaper/api";
import type {
  MindMap,
  MindMapEdge,
  MindMapGraph,
  MindMapNode,
  MindMapNodeData,
  MindMapNodeType,
  Paper,
} from "@/lib/gopherpaper/types";
import { cn } from "@/lib/utils";

interface ReadingMindMapProps {
  open: boolean;
  paper: Paper | null;
  onResizeStart?: (event: ReactPointerEvent<HTMLDivElement>) => void;
  onLocateHighlight: (highlightID: number, pageNumber?: number, openAnnotations?: boolean) => void;
}

interface MindMapFlowData extends MindMapNodeData {
  nodeType: MindMapNodeType;
  hasChildren: boolean;
  onDeleteManual: (id: string) => void;
  onEditManual: (id: string) => void;
  onToggleCollapse: (id: string) => void;
  [key: string]: unknown;
}

type MindMapFlowNode = Node<MindMapFlowData, "mindMapCard">;
type MindMapFlowEdge = Edge<{ manual?: boolean }>;

const NODE_TYPES = {
  mindMapCard: MindMapNodeCard,
};

const NODE_SIZE: Record<MindMapNodeType, { width: number; minHeight: number; maxWidth: number }> = {
  paper: { width: 260, minHeight: 96, maxWidth: 380 },
  section: { width: 248, minHeight: 104, maxWidth: 340 },
  highlight: { width: 300, minHeight: 118, maxWidth: 360 },
  annotation: { width: 280, minHeight: 118, maxWidth: 340 },
  group: { width: 232, minHeight: 92, maxWidth: 320 },
  manual: { width: 236, minHeight: 104, maxWidth: 320 },
};

const TYPE_META: Record<
  MindMapNodeType,
  { label: string; color: string; tint: string; icon: LucideIcon }
> = {
  paper: {
    label: "论文",
    color: "oklch(0.48 0.1 200)",
    tint: "oklch(0.95 0.022 200)",
    icon: BookOpenText,
  },
  section: {
    label: "章节",
    color: "oklch(0.54 0.12 230)",
    tint: "oklch(0.955 0.018 230)",
    icon: FileText,
  },
  highlight: {
    label: "高亮",
    color: "oklch(0.62 0.15 82)",
    tint: "oklch(0.965 0.05 88)",
    icon: Highlighter,
  },
  annotation: {
    label: "批注",
    color: "oklch(0.55 0.11 155)",
    tint: "oklch(0.955 0.04 155)",
    icon: MessageSquareText,
  },
  group: {
    label: "分组",
    color: "oklch(0.52 0.035 245)",
    tint: "oklch(0.955 0.006 245)",
    icon: FolderTree,
  },
  manual: {
    label: "普通节点",
    color: "oklch(0.56 0.12 260)",
    tint: "oklch(0.958 0.026 260)",
    icon: StickyNote,
  },
};

const COLOR_VALUE: Record<string, string> = {
  yellow: "oklch(0.965 0.052 88)",
  blue: "oklch(0.94 0.045 245)",
  green: "oklch(0.94 0.06 150)",
  pink: "oklch(0.94 0.055 5)",
  purple: "oklch(0.94 0.05 295)",
  orange: "oklch(0.94 0.065 60)",
};

const LAYOUT_MARGIN_X = 48;
const LAYOUT_MARGIN_Y = 56;
const LAYOUT_RANK_GAP = 148;
const LAYOUT_NODE_GAP = 72;
const LAYOUT_ROOT_GAP = 112;
const SVG_EXPORT_PADDING = 64;
const SVG_EDGE_PADDING = 8;
const SVG_BODY_LINE_HEIGHT = 18;
const SVG_ICON_BOX_SIZE = 30;
const SVG_ICON_SIZE = 18;

const SVG_NODE_THEME: Record<
  MindMapNodeType,
  { label: string; accent: string; tint: string; wash: string; iconBg: string }
> = {
  paper: {
    label: "论文",
    accent: "#0f766e",
    tint: "#e8f7f4",
    wash: "#f5fbfa",
    iconBg: "#ccfbf1",
  },
  section: {
    label: "章节",
    accent: "#2563eb",
    tint: "#eaf2ff",
    wash: "#f8fbff",
    iconBg: "#dbeafe",
  },
  highlight: {
    label: "高亮",
    accent: "#b98900",
    tint: "#fff7d6",
    wash: "#fffdf4",
    iconBg: "#fef3c7",
  },
  annotation: {
    label: "批注",
    accent: "#059669",
    tint: "#e9f9f1",
    wash: "#f6fdf9",
    iconBg: "#d1fae5",
  },
  group: {
    label: "分组",
    accent: "#475569",
    tint: "#eef2f7",
    wash: "#f8fafc",
    iconBg: "#e2e8f0",
  },
  manual: {
    label: "节点",
    accent: "#7c3aed",
    tint: "#f2eefe",
    wash: "#fbf8ff",
    iconBg: "#ede9fe",
  },
};

const SVG_HIGHLIGHT_THEME: Record<string, { accent: string; tint: string; iconBg: string }> = {
  yellow: { accent: "#b98900", tint: "#fff7d6", iconBg: "#fef3c7" },
  blue: { accent: "#2563eb", tint: "#eaf2ff", iconBg: "#dbeafe" },
  green: { accent: "#059669", tint: "#e9f9f1", iconBg: "#d1fae5" },
  pink: { accent: "#db2777", tint: "#fdf2f8", iconBg: "#fce7f3" },
  purple: { accent: "#7c3aed", tint: "#f2eefe", iconBg: "#ede9fe" },
  orange: { accent: "#ea580c", tint: "#fff1e8", iconBg: "#ffedd5" },
};

type MindMapNodeStyle = CSSProperties & {
  "--mind-node-accent": string;
  "--mind-node-tint": string;
};

function MindMapNodeCard({ id, data, selected }: NodeProps<MindMapFlowNode>) {
  const isSection = data.nodeType === "section";
  const isHighlight = data.nodeType === "highlight";
  const isAnnotation = data.nodeType === "annotation";
  const isManual = data.nodeType === "manual";
  const meta = TYPE_META[data.nodeType] || TYPE_META.manual;
  const Icon = meta.icon;
  const accentTint = isHighlight
    ? COLOR_VALUE[data.color || "yellow"] || COLOR_VALUE.yellow
    : meta.tint;
  const locate = isHighlight || isAnnotation;
  const displayText = isAnnotation ? data.note || data.text || data.label : data.text || data.label;
  const size = nodeSize(data.nodeType, data);
  const style: MindMapNodeStyle = {
    width: size.width,
    minHeight: size.height,
    "--mind-node-accent": meta.color,
    "--mind-node-tint": accentTint,
  };

  return (
    <div
      className={cn(
        "mind-map-node-card group/mind-node relative min-w-0 overflow-hidden rounded-lg border text-left text-foreground",
        "border-[color-mix(in_srgb,var(--mind-node-accent)_26%,var(--border))]",
        "bg-[linear-gradient(180deg,var(--mind-node-tint),var(--card)_58%)]",
        "shadow-[0_1px_0_rgba(15,23,42,0.04),0_14px_32px_-26px_rgba(15,23,42,0.42)]",
        "transition-[border-color,box-shadow,transform,background-color] duration-200",
        "will-change-transform [contain:layout_paint_style]",
        "hover:-translate-y-0.5 hover:border-[color-mix(in_srgb,var(--mind-node-accent)_48%,var(--border))] hover:shadow-[0_18px_38px_-26px_rgba(15,23,42,0.48)]",
        selected && "border-[color-mix(in_srgb,var(--mind-node-accent)_70%,var(--border))] ring-2 ring-[color-mix(in_srgb,var(--mind-node-accent)_18%,transparent)]",
        data.nodeType === "group" && "border-dashed",
        locate && "cursor-pointer",
      )}
      style={style}
    >
      <Handle
        type="target"
        position={Position.Left}
        className="!size-2 !border-0 !bg-[var(--mind-node-accent)] !opacity-0"
      />
      <Handle
        type="source"
        position={Position.Right}
        className="!size-2 !border-0 !bg-[var(--mind-node-accent)] !opacity-0"
      />
      <div className="h-1 bg-[var(--mind-node-accent)] opacity-80" />
      <div className="px-3 py-2.5">
        <div className="flex items-start justify-between gap-2">
          <div className="flex min-w-0 items-center gap-2">
            <span className="grid size-7 shrink-0 place-items-center rounded-md border border-[color-mix(in_srgb,var(--mind-node-accent)_22%,transparent)] bg-card/72 text-[var(--mind-node-accent)] shadow-sm">
              <Icon className="size-3.5" />
            </span>
            <div className="min-w-0">
              <div className="flex min-w-0 items-center gap-1.5">
                <span className="truncate text-[11px] font-semibold text-[var(--mind-node-accent)]">
                  {meta.label}
                </span>
                {isSection && data.hasChildren ? (
                  <span className="rounded-full bg-background/70 px-1.5 py-0.5 text-[10px] text-muted-foreground">
                    {data.collapsed ? "已收起" : "可折叠"}
                  </span>
                ) : null}
              </div>
              {data.pageNumber ? (
                <div className="mt-0.5 font-mono text-[10px] text-muted-foreground">
                  PAGE {data.pageNumber}
                </div>
              ) : null}
            </div>
          </div>

          <div className="nodrag flex shrink-0 items-center gap-1">
            {isSection && data.hasChildren ? (
              <button
                type="button"
                aria-label={data.collapsed ? "展开章节" : "收起章节"}
                className="grid size-6 place-items-center rounded-md border border-transparent text-muted-foreground transition-colors hover:border-border hover:bg-background/80 hover:text-foreground"
                onClick={(event) => {
                  event.stopPropagation();
                  data.onToggleCollapse(id);
                }}
              >
                {data.collapsed ? <ChevronRight className="size-3.5" /> : <ChevronDown className="size-3.5" />}
              </button>
            ) : null}
            {isManual && (
              <>
                <button
                  type="button"
                  aria-label="编辑普通节点"
                  className="grid size-6 place-items-center rounded-md border border-transparent text-muted-foreground transition-colors hover:border-border hover:bg-background/80 hover:text-foreground"
                  onClick={(event) => {
                    event.stopPropagation();
                    data.onEditManual(id);
                  }}
                >
                  <Edit3 className="size-3.5" />
                </button>
                <button
                  type="button"
                  aria-label="删除普通节点"
                  className="grid size-6 place-items-center rounded-md border border-transparent text-muted-foreground transition-colors hover:border-destructive/25 hover:bg-destructive/10 hover:text-destructive"
                  onClick={(event) => {
                    event.stopPropagation();
                    data.onDeleteManual(id);
                  }}
                >
                  <Trash2 className="size-3.5" />
                </button>
              </>
            )}
          </div>
        </div>

        <div
          className={cn(
            "mt-2.5 whitespace-pre-wrap break-words text-sm leading-5",
            data.nodeType === "paper" && "line-clamp-3 text-[15px] font-semibold tracking-tight",
            data.nodeType === "section" && "line-clamp-3 font-semibold",
            data.nodeType === "highlight" && "line-clamp-6 border-l-2 border-[var(--mind-node-accent)] pl-2.5 text-[13px] text-foreground/85",
            data.nodeType === "annotation" && "line-clamp-6 border-l-2 border-[var(--mind-node-accent)] pl-2.5 text-[13px] text-foreground/85",
            data.nodeType === "group" && "line-clamp-2 font-medium",
            data.nodeType === "manual" && "line-clamp-3 font-medium",
          )}
        >
          {displayText || "未命名节点"}
        </div>

        {(isHighlight || isAnnotation) && (
          <div className="mt-2 flex items-center justify-between gap-2 border-t border-[color-mix(in_srgb,var(--mind-node-accent)_16%,transparent)] pt-2 text-[11px] text-muted-foreground">
            <span>{isHighlight ? "点击定位原文高亮" : "点击定位批注"}</span>
            {data.pageNumber ? <span className="font-mono">p.{data.pageNumber}</span> : null}
          </div>
        )}
      </div>
    </div>
  );
}

export function ReadingMindMap(props: ReadingMindMapProps) {
  return (
    <ReactFlowProvider>
      <ReadingMindMapCanvas {...props} />
    </ReactFlowProvider>
  );
}

function ReadingMindMapCanvas({
  open,
  paper,
  onResizeStart,
  onLocateHighlight,
}: ReadingMindMapProps) {
  const { fitView } = useReactFlow();
  const requestSeq = useRef(0);
  const saveSeq = useRef(0);
  const [flowNodes, setFlowNodes, onFlowNodesChange] = useNodesState<MindMapFlowNode>([]);
  const [mindMap, setMindMap] = useState<MindMap | null>(null);
  const [loading, setLoading] = useState(false);
  const [busy, setBusy] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [selectedNodeID, setSelectedNodeID] = useState<string | null>(null);
  const [manualDialogOpen, setManualDialogOpen] = useState(false);
  const [manualEditingID, setManualEditingID] = useState<string | null>(null);
  const [manualLabel, setManualLabel] = useState("");
  const [manualParentID, setManualParentID] = useState("");

  useEffect(() => {
    if (!open || !paper?.id) return;
    const seq = requestSeq.current + 1;
    requestSeq.current = seq;
    saveSeq.current += 1;
    setLoading(true);
    setBusy("");
    setSaving(false);
    setError("");
    setMindMap(null);
    setSelectedNodeID(null);
    setManualDialogOpen(false);
    setManualEditingID(null);
    setManualLabel("");
    setManualParentID("");
    api
      .getMindMap(paper.id)
      .then((value) => {
        if (requestSeq.current !== seq) return;
        setMindMap(normalizeMindMap(value));
      })
      .catch((err) => {
        if (requestSeq.current !== seq) return;
        if (err instanceof api.ApiError && err.status === 404) {
          setMindMap(null);
          return;
        }
        setError((err as Error)?.message || "脑图加载失败");
      })
      .finally(() => {
        if (requestSeq.current === seq) setLoading(false);
      });
  }, [open, paper?.id]);

  useEffect(() => {
    if (!open) return;
    window.setTimeout(() => fitView({ padding: 0.18, duration: 220 }), 80);
  }, [fitView, mindMap?.id, open]);

  const graph = mindMap?.graph_json || null;
  const parentOptions = useMemo(() => {
    if (!graph) return [];
    if (!manualEditingID) return graph.nodes;
    const blocked = descendantNodeIDs(graph, manualEditingID);
    blocked.add(manualEditingID);
    return graph.nodes.filter((node) => !blocked.has(node.id));
  }, [graph, manualEditingID]);

  const persistGraph = useCallback(
    async (nextGraph: MindMapGraph) => {
      if (!mindMap) return;
      const seq = requestSeq.current;
      const save = saveSeq.current + 1;
      const paperID = paper?.id || "";
      saveSeq.current = save;
      setSaving(true);
      setError("");
      try {
        const saved = await api.updateMindMap(mindMap.id, nextGraph);
        if (requestSeq.current !== seq || saveSeq.current !== save || saved.paper_id !== paperID) return;
        setMindMap(normalizeMindMap(saved));
      } catch (err) {
        if (requestSeq.current !== seq || saveSeq.current !== save) return;
        setError((err as Error)?.message || "脑图保存失败");
      } finally {
        if (requestSeq.current === seq && saveSeq.current === save) setSaving(false);
      }
    },
    [mindMap, paper?.id],
  );

  const updateGraph = useCallback(
    (updater: (graph: MindMapGraph) => MindMapGraph, persist = false) => {
      let nextGraph: MindMapGraph | null = null;
      setMindMap((current) => {
        if (!current) return current;
        nextGraph = updater(current.graph_json);
        return { ...current, graph_json: nextGraph };
      });
      if (persist && nextGraph) void persistGraph(nextGraph);
    },
    [persistGraph],
  );

  const build = async () => {
    const paperID = paper?.id;
    if (!paperID || busy) return;
    const seq = requestSeq.current + 1;
    requestSeq.current = seq;
    setBusy("build");
    setError("");
    try {
      const value = await api.buildMindMap(paperID);
      if (requestSeq.current !== seq) return;
      applyFetchedMindMap(value, seq, true);
    } catch (err) {
      if (requestSeq.current !== seq) return;
      setError((err as Error)?.message || "脑图生成失败");
    } finally {
      if (requestSeq.current === seq) setBusy("");
    }
  };

  const sync = async () => {
    if (!mindMap || busy) return;
    const seq = requestSeq.current + 1;
    requestSeq.current = seq;
    setBusy("sync");
    setError("");
    try {
      const value = await api.syncMindMap(mindMap.id);
      if (requestSeq.current !== seq) return;
      applyFetchedMindMap(value, seq, true);
    } catch (err) {
      if (requestSeq.current !== seq) return;
      setError((err as Error)?.message || "脑图同步失败");
    } finally {
      if (requestSeq.current === seq) setBusy("");
    }
  };

  const refreshMindMap = () => {
    if (mindMap) void sync();
    else void build();
  };

  const applyFetchedMindMap = (value: MindMap, seq: number, autoArrange: boolean) => {
    const normalized = normalizeMindMap(value);
    const graphJSON = autoArrange ? layoutLeftToRight(normalized.graph_json) : normalized.graph_json;
    const arranged = { ...normalized, graph_json: graphJSON };
    setMindMap(arranged);
    if (!autoArrange) return;
    const save = saveSeq.current + 1;
    saveSeq.current = save;
    setSaving(true);
    api
      .updateMindMap(arranged.id, graphJSON)
      .then((saved) => {
        if (requestSeq.current !== seq || saveSeq.current !== save) return;
        setMindMap(normalizeMindMap(saved));
      })
      .catch((err) => {
        if (requestSeq.current !== seq || saveSeq.current !== save) return;
        setError((err as Error)?.message || "自动布局保存失败");
      })
      .finally(() => {
        if (requestSeq.current === seq && saveSeq.current === save) setSaving(false);
      });
  };

  const toggleCollapse = useCallback((id: string) => {
    updateGraph(
      (cur) => ({
        ...cur,
        nodes: cur.nodes.map((item) =>
          item.id === id
            ? { ...item, data: { ...item.data, collapsed: !item.data.collapsed } }
            : item,
        ),
      }),
      true,
    );
  }, [updateGraph]);

  const deleteManualNode = useCallback((id: string) => {
    updateGraph(
      (cur) => {
        const removable = removableManualNodeIDs(cur, id);
        if (removable.size === 0) return cur;
        return {
          ...cur,
          nodes: cur.nodes.filter((item) => !removable.has(item.id)),
          edges: cur.edges.filter((item) => !removable.has(item.source) && !removable.has(item.target)),
        };
      },
      true,
    );
    setSelectedNodeID((cur) => (cur === id ? null : cur));
  }, [updateGraph]);

  const editManualNode = useCallback((id: string) => {
    const node = graph?.nodes.find((item) => item.id === id && isManualNode(item));
    if (!node) return;
    setManualEditingID(id);
    setManualParentID(node.data.parentId || graph?.nodes[0]?.id || "");
    setManualLabel(node.data.label || "");
    setManualDialogOpen(true);
  }, [graph]);

  const visible = useMemo(() => (graph ? toVisibleGraph(graph) : null), [graph]);
  const nextFlowNodes = useMemo<MindMapFlowNode[]>(() => {
    if (!visible || !graph) return [];
    const childCounts = childCountMap(graph);
    return visible.nodes.map((item) => ({
      id: item.id,
      type: "mindMapCard",
      position: item.position || { x: 0, y: 0 },
      sourcePosition: Position.Right,
      targetPosition: Position.Left,
      data: {
        ...item.data,
        nodeType: item.type,
        hasChildren: (childCounts.get(item.id) || 0) > 0,
        onDeleteManual: deleteManualNode,
        onEditManual: editManualNode,
        onToggleCollapse: toggleCollapse,
      },
    }));
  }, [deleteManualNode, editManualNode, graph, toggleCollapse, visible]);

  useEffect(() => {
    setFlowNodes(nextFlowNodes);
  }, [nextFlowNodes, setFlowNodes]);

  const flowEdges = useMemo<MindMapFlowEdge[]>(() => {
    if (!visible) return [];
    return visible.edges.map((item) => ({
      id: item.id,
      source: item.source,
      target: item.target,
      type: "smoothstep",
      data: item.data,
      interactionWidth: 14,
      markerEnd: {
        type: MarkerType.ArrowClosed,
        color: item.data?.manual ? "var(--primary)" : "var(--muted-foreground)",
        width: 14,
        height: 14,
      },
      style: {
        stroke: item.data?.manual ? "var(--primary)" : "color-mix(in srgb, var(--muted-foreground) 42%, transparent)",
        strokeWidth: item.data?.manual ? 1.7 : 1.35,
        strokeDasharray: item.data?.manual ? "5 4" : undefined,
      },
    }));
  }, [visible]);

  const openManualNodeDialog = () => {
    if (!graph || !mindMap) return;
    const parentID = selectedNodeID && graph.nodes.some((item) => item.id === selectedNodeID)
      ? selectedNodeID
      : graph.nodes[0]?.id || "";
    setManualEditingID(null);
    setManualParentID(parentID);
    setManualLabel("");
    setManualDialogOpen(true);
  };

  const submitManualNode = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!graph || !mindMap) return;
    const label = manualLabel.trim();
    if (!label) return;
    const parentID = graph.nodes.some((item) => item.id === manualParentID)
      ? manualParentID
      : graph.nodes[0]?.id;
    if (!parentID) return;
    if (manualEditingID) {
      if (parentID === manualEditingID) return;
      updateGraph(
        (cur) => {
          const exists = cur.nodes.some((item) => item.id === manualEditingID && isManualNode(item));
          if (!exists) return cur;
          const edgeID = `${parentID}->${manualEditingID}`;
          const edges = [
            ...cur.edges.filter((item) => item.target !== manualEditingID),
            { id: edgeID, source: parentID, target: manualEditingID, data: { manual: true } },
          ];
          return {
            ...cur,
            nodes: cur.nodes.map((item) =>
              item.id === manualEditingID
                ? { ...item, data: { ...item.data, label, parentId: parentID, manual: true } }
                : item,
            ),
            edges,
          };
        },
        true,
      );
      setSelectedNodeID(manualEditingID);
      setManualDialogOpen(false);
      setManualEditingID(null);
      setManualLabel("");
      return;
    }
    const id = `manual:${Date.now().toString(36)}`;
    const manualNode: MindMapNode = {
      id,
      type: "manual",
      position: manualNodePosition(graph, parentID),
      data: { label, parentId: parentID, manual: true },
    };
    const manualEdge: MindMapEdge = {
      id: `${parentID}->${id}`,
      source: parentID,
      target: id,
      data: { manual: true },
    };
    updateGraph((cur) => ({ ...cur, nodes: [...cur.nodes, manualNode], edges: [...cur.edges, manualEdge] }), true);
    setSelectedNodeID(id);
    setManualDialogOpen(false);
    setManualEditingID(null);
    setManualLabel("");
  };

  const autoLayout = () => {
    if (!graph) return;
    const next = layoutLeftToRight(graph);
    updateGraph(() => next, true);
    window.setTimeout(() => fitView({ padding: 0.18, duration: 220 }), 60);
  };

  const exportSVG = () => {
    if (!mindMap || flowNodes.length === 0) return;
    try {
      downloadMindMapSVG(
        flowNodes,
        flowEdges,
        paper?.title?.trim() || paper?.file_name?.trim() || "GopherPaper 脑图",
      );
    } catch (err) {
      setError((err as Error)?.message || "导出 SVG 失败");
    }
  };

  const onNodeDragStop: OnNodeDrag<MindMapFlowNode> = (_event, node) => {
    updateGraph(
      (cur) => ({
        ...cur,
        nodes: cur.nodes.map((item) => (item.id === node.id ? { ...item, position: node.position } : item)),
      }),
      true,
    );
  };

  const onNodeClick: NodeMouseHandler<MindMapFlowNode> = (_event, node) => {
    setSelectedNodeID(node.id);
    if (node.data.nodeType === "highlight" || node.data.nodeType === "annotation") {
      const id = node.data.highlightId || node.data.annotationId;
      if (typeof id === "number") onLocateHighlight(id, node.data.pageNumber, false);
    }
  };

  if (!open) return null;

  return (
    <aside className="relative flex h-full min-h-0 flex-col border-l bg-muted/35">
      {onResizeStart && (
        <div
          role="separator"
          aria-label="拖动调整脑图宽度"
          aria-orientation="vertical"
          className="absolute inset-y-0 left-0 z-30 hidden w-2 -translate-x-1/2 cursor-col-resize lg:block"
          onPointerDown={onResizeStart}
        >
          <span className="absolute left-1/2 top-0 h-full w-px -translate-x-1/2 bg-border transition-colors" />
        </div>
      )}

      <div className="flex min-h-12 shrink-0 items-center justify-between gap-2 border-b bg-background px-3 py-2">
        <div className="min-w-0">
          <div className="flex items-center gap-2 text-sm font-semibold">
            <Network className="size-4 text-sienna" />
            精读脑图
          </div>
          <div className="truncate text-xs text-muted-foreground">
            {mindMap ? `${mindMap.graph_json.nodes.length} 节点` : "目录、高亮与批注"}
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-1">
          <Button type="button" variant="ghost" size="icon-sm" title="更新脑图" disabled={!paper?.id || Boolean(busy)} onClick={refreshMindMap}>
            {busy === "build" || busy === "sync" ? <Loader2 className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
          </Button>
          <Button type="button" variant="ghost" size="icon-sm" title="自动布局" disabled={!mindMap} onClick={autoLayout}>
            <AlignHorizontalSpaceAround className="size-4" />
          </Button>
          <Button type="button" variant="ghost" size="icon-sm" title="新增普通节点" disabled={!mindMap} onClick={openManualNodeDialog}>
            <Plus className="size-4" />
          </Button>
          <Button type="button" variant="ghost" size="icon-sm" title="导出 SVG" disabled={!mindMap} onClick={exportSVG}>
            <Download className="size-4" />
          </Button>
        </div>
      </div>

      {error && (
        <div className="mx-3 mt-3 rounded-md border border-destructive/25 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {error}
        </div>
      )}

      <div className="min-h-0 flex-1">
        {loading ? (
          <div className="flex h-full items-center justify-center gap-2 text-sm text-muted-foreground">
            <Loader2 className="size-4 animate-spin" />
            加载脑图...
          </div>
        ) : !mindMap ? (
          <div className="flex h-full items-center justify-center p-6">
            <div className="max-w-sm rounded-lg border border-dashed bg-card px-5 py-6 text-center shadow-sm">
              <div className="text-sm font-semibold">还没有精读脑图</div>
              <div className="mt-2 text-sm leading-6 text-muted-foreground">
                当前论文尚未生成脑图。
              </div>
              <Button type="button" className="mt-4" disabled={Boolean(busy)} onClick={refreshMindMap}>
                {busy === "build" && <Loader2 className="size-4 animate-spin" />}
                生成脑图
              </Button>
            </div>
          </div>
        ) : (
          <div className="relative h-full">
            {saving && (
              <div className="absolute right-3 top-3 z-20 rounded-full border bg-card/95 px-2.5 py-1 text-xs text-muted-foreground shadow-sm backdrop-blur">
                保存中...
              </div>
            )}
            <ReactFlow
              nodes={flowNodes}
              edges={flowEdges}
              nodeTypes={NODE_TYPES}
              onNodesChange={onFlowNodesChange}
              onNodeDragStop={onNodeDragStop}
              onNodeClick={onNodeClick}
              fitView
              minZoom={0.2}
              maxZoom={2.2}
              onlyRenderVisibleElements
              nodesConnectable={false}
              edgesReconnectable={false}
              edgesFocusable={false}
              nodeDragThreshold={2}
              panOnScroll
              panOnScrollMode={PanOnScrollMode.Free}
              panOnScrollSpeed={1.05}
              zoomOnScroll={false}
              zoomActivationKeyCode={["Control", "Meta"]}
              zoomOnDoubleClick={false}
              proOptions={{ hideAttribution: true }}
              defaultEdgeOptions={{ type: "smoothstep" }}
              className="mind-map-flow bg-muted/35"
            >
              <Background color="color-mix(in srgb, var(--muted-foreground) 22%, transparent)" gap={24} size={1} />
              <Controls
                showInteractive={false}
                className="!border !border-border !bg-card/95 !shadow-sm [&_button]:!border-border [&_button]:!bg-card [&_button]:!text-foreground hover:[&_button]:!bg-muted"
              />
            </ReactFlow>
          </div>
        )}
      </div>

      <Dialog
        open={manualDialogOpen}
        onOpenChange={(nextOpen) => {
          setManualDialogOpen(nextOpen);
          if (!nextOpen) setManualEditingID(null);
        }}
      >
        <DialogContent className="sm:max-w-md">
          <form className="grid gap-4" onSubmit={submitManualNode}>
            <DialogHeader>
              <DialogTitle>{manualEditingID ? "编辑普通节点" : "新增普通节点"}</DialogTitle>
            </DialogHeader>
            <div className="grid gap-2">
              <Label htmlFor="mind-map-manual-label">节点内容</Label>
              <Input
                id="mind-map-manual-label"
                value={manualLabel}
                onChange={(event) => setManualLabel(event.target.value)}
                placeholder="输入节点内容"
                autoFocus
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="mind-map-manual-parent">连接到</Label>
              <select
                id="mind-map-manual-parent"
                value={manualParentID}
                onChange={(event) => setManualParentID(event.target.value)}
                className="h-9 w-full min-w-0 rounded-lg border border-input bg-background px-2.5 text-sm outline-none focus:border-ring focus:ring-3 focus:ring-ring/40"
              >
                {parentOptions.map((node) => (
                  <option key={node.id} value={node.id}>
                    {nodeOptionLabel(node)}
                  </option>
                ))}
              </select>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setManualDialogOpen(false)}>
                取消
              </Button>
              <Button type="submit" disabled={!manualLabel.trim()}>
                {manualEditingID ? "保存" : "添加"}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </aside>
  );
}

function normalizeMindMap(value: MindMap): MindMap {
  return {
    ...value,
    graph_json: {
      ...value.graph_json,
      nodes: Array.isArray(value.graph_json?.nodes) ? value.graph_json.nodes : [],
      edges: Array.isArray(value.graph_json?.edges) ? value.graph_json.edges : [],
    },
  };
}

function downloadMindMapSVG(nodes: MindMapFlowNode[], edges: MindMapFlowEdge[], title: string) {
  const svg = mindMapToSVG(nodes, edges, title);
  const blob = new Blob([svg], { type: "image/svg+xml;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = `${safeFileName(title || "gopherpaper-mind-map")}.svg`;
  document.body.appendChild(link);
  link.click();
  link.remove();
  window.setTimeout(() => URL.revokeObjectURL(url), 1000);
}

function mindMapToSVG(nodes: MindMapFlowNode[], edges: MindMapFlowEdge[], title: string) {
  const visibleNodes = nodes.filter((node) => !node.hidden);
  if (visibleNodes.length === 0) {
    throw new Error("没有可导出的脑图节点");
  }

  const bounds = graphBounds(visibleNodes);
  const width = Math.ceil(bounds.maxX - bounds.minX + SVG_EXPORT_PADDING * 2);
  const height = Math.ceil(bounds.maxY - bounds.minY + SVG_EXPORT_PADDING * 2);
  const offsetX = SVG_EXPORT_PADDING - bounds.minX;
  const offsetY = SVG_EXPORT_PADDING - bounds.minY;
  const nodeByID = new Map(visibleNodes.map((node) => [node.id, node]));
  const edgeSVG = edges
    .filter((edge) => nodeByID.has(edge.source) && nodeByID.has(edge.target))
    .map((edge) => renderSVGEdge(edge, nodeByID, offsetX, offsetY))
    .join("\n");
  const nodeSVG = visibleNodes
    .map((node) => renderSVGNode(node, offsetX, offsetY))
    .join("\n");

  return [
    `<?xml version="1.0" encoding="UTF-8"?>`,
    `<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}" viewBox="0 0 ${width} ${height}" role="img" aria-labelledby="title desc">`,
    `<title id="title">${escapeXML(title || "GopherPaper 脑图")}</title>`,
    `<desc id="desc">GopherPaper reading mind map export</desc>`,
    `<defs>`,
    `<linearGradient id="mind-map-bg" x1="0" y1="0" x2="1" y2="1"><stop offset="0%" stop-color="#fbfdff"/><stop offset="100%" stop-color="#eef5fb"/></linearGradient>`,
    `<pattern id="mind-map-grid" width="28" height="28" patternUnits="userSpaceOnUse"><path d="M 28 0 L 0 0 0 28" fill="none" stroke="#dbe4ef" stroke-width="0.6" opacity=".55"/><circle cx="1" cy="1" r="1" fill="#cbd8e6" opacity=".55"/></pattern>`,
    `<filter id="node-shadow" x="-18%" y="-18%" width="136%" height="146%"><feDropShadow dx="0" dy="10" stdDeviation="11" flood-color="#0f172a" flood-opacity=".13"/><feDropShadow dx="0" dy="1" stdDeviation="1" flood-color="#0f172a" flood-opacity=".08"/></filter>`,
    `<filter id="soft-shadow" x="-20%" y="-20%" width="140%" height="140%"><feDropShadow dx="0" dy="4" stdDeviation="5" flood-color="#0f172a" flood-opacity=".12"/></filter>`,
    `<marker id="mind-map-arrow" viewBox="0 0 10 10" refX="8.8" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M 0 0 L 10 5 L 0 10 z" fill="#8aa0b8"/></marker>`,
    `<marker id="mind-map-arrow-manual" viewBox="0 0 10 10" refX="8.8" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M 0 0 L 10 5 L 0 10 z" fill="#2563eb"/></marker>`,
    `<style>`,
    `text{font-family:Manrope,"Microsoft YaHei","Noto Sans SC",Arial,sans-serif;text-rendering:geometricPrecision}`,
    `.node-title{font-size:11px;font-weight:800;letter-spacing:.02em}`,
    `.node-page{font-size:10px;letter-spacing:.11em;fill:#64748b;font-weight:600}`,
    `.node-body{font-size:13px;fill:#273142}`,
    `.node-footer{font-size:11px;fill:#64748b;font-weight:600}`,
    `.node-pill{font-size:10px;font-weight:700;letter-spacing:.03em}`,
    `</style>`,
    `</defs>`,
    `<rect width="100%" height="100%" fill="url(#mind-map-bg)"/>`,
    `<rect width="100%" height="100%" fill="url(#mind-map-grid)" opacity=".7"/>`,
    `<g>${edgeSVG}</g>`,
    `<g>${nodeSVG}</g>`,
    `</svg>`,
  ].join("\n");
}

function graphBounds(nodes: MindMapFlowNode[]) {
  let minX = Number.POSITIVE_INFINITY;
  let minY = Number.POSITIVE_INFINITY;
  let maxX = Number.NEGATIVE_INFINITY;
  let maxY = Number.NEGATIVE_INFINITY;
  for (const node of nodes) {
    const size = nodeSize(node.data.nodeType, node.data);
    minX = Math.min(minX, node.position.x);
    minY = Math.min(minY, node.position.y);
    maxX = Math.max(maxX, node.position.x + size.width);
    maxY = Math.max(maxY, node.position.y + size.height);
  }
  return { minX, minY, maxX, maxY };
}

function renderSVGEdge(
  edge: MindMapFlowEdge,
  nodeByID: Map<string, MindMapFlowNode>,
  offsetX: number,
  offsetY: number,
) {
  const source = nodeByID.get(edge.source);
  const target = nodeByID.get(edge.target);
  if (!source || !target) return "";
  const sourceSize = nodeSize(source.data.nodeType, source.data);
  const targetSize = nodeSize(target.data.nodeType, target.data);
  const sx = source.position.x + offsetX + sourceSize.width + SVG_EDGE_PADDING;
  const sy = source.position.y + offsetY + sourceSize.height / 2;
  const tx = target.position.x + offsetX - SVG_EDGE_PADDING;
  const ty = target.position.y + offsetY + targetSize.height / 2;
  const spread = Math.max(80, Math.abs(tx - sx) * 0.5);
  const c1x = sx + spread;
  const c2x = tx - spread;
  const manual = Boolean(edge.data?.manual);
  const stroke = manual ? "#2563eb" : "#8aa0b8";
  const dash = manual ? ` stroke-dasharray="6 5"` : "";
  const marker = manual ? "mind-map-arrow-manual" : "mind-map-arrow";
  const d = `M ${round(sx)} ${round(sy)} C ${round(c1x)} ${round(sy)}, ${round(c2x)} ${round(ty)}, ${round(tx)} ${round(ty)}`;
  return [
    `<path d="${d}" fill="none" stroke="#ffffff" stroke-width="${manual ? 5 : 4}" stroke-linecap="round" opacity=".9"/>`,
    `<path d="${d}" fill="none" stroke="${stroke}" stroke-width="${manual ? 1.9 : 1.55}" stroke-linecap="round"${dash} marker-end="url(#${marker})"/>`,
  ].join("");
}

function renderSVGNode(node: MindMapFlowNode, offsetX: number, offsetY: number) {
  const type = node.data.nodeType;
  const theme = svgThemeForNode(node);
  const x = round(node.position.x + offsetX);
  const y = round(node.position.y + offsetY);
  const displayText = type === "annotation"
    ? node.data.note || node.data.text || node.data.label
    : node.data.text || node.data.label;
  const size = nodeSize(type, node.data);
  const bodyX = type === "highlight" || type === "annotation" ? 32 : 16;
  const bodyWidth = size.width - bodyX - 18;
  const bodyLines = wrapSVGText(displayText || "未命名节点", bodyWidth, 13, bodyLineLimit(type));
  const bodyTop = node.data.pageNumber ? 69 : 61;
  const bodyHeight = Math.max(SVG_BODY_LINE_HEIGHT, bodyLines.length * SVG_BODY_LINE_HEIGHT);
  const icon = renderSVGIcon(type, theme.accent);
  const pageText = node.data.pageNumber ? `PAGE ${node.data.pageNumber}` : "";
  const pagePillWidth = pageText ? Math.ceil(estimateTextWidth(pageText, 10) + 20) : 0;
  const pagePillX = 52;
  const pagePillY = 36;
  const pagePillHeight = 17;
  const pagePillCenterX = pagePillX + pagePillWidth / 2;
  const pagePillCenterY = pagePillY + pagePillHeight / 2;
  const iconBoxX = 12;
  const iconBoxY = 18;
  const iconX = iconBoxX + (SVG_ICON_BOX_SIZE - SVG_ICON_SIZE) / 2;
  const iconY = iconBoxY + (SVG_ICON_BOX_SIZE - SVG_ICON_SIZE) / 2;
  const header = [
    `<rect x="0" y="0" width="${size.width}" height="${size.height}" rx="9" fill="#ffffff" stroke="#c8d5e4" stroke-width="1" filter="url(#node-shadow)"/>`,
    `<rect x="1" y="1" width="${size.width - 2}" height="${size.height - 2}" rx="8" fill="${theme.wash}"/>`,
    `<path d="M 9 1 H ${size.width - 9} Q ${size.width - 1} 1 ${size.width - 1} 9 V 56 H 1 V 9 Q 1 1 9 1 Z" fill="${theme.tint}"/>`,
    `<rect x="0" y="0" width="${size.width}" height="5" rx="2.5" fill="${theme.accent}"/>`,
    `<rect x="${iconBoxX}" y="${iconBoxY}" width="${SVG_ICON_BOX_SIZE}" height="${SVG_ICON_BOX_SIZE}" rx="8" fill="${theme.iconBg}" stroke="#ffffff" stroke-width="1.5" filter="url(#soft-shadow)"/>`,
    `<svg x="${iconX}" y="${iconY}" width="${SVG_ICON_SIZE}" height="${SVG_ICON_SIZE}" viewBox="0 0 18 18" overflow="visible">${icon}</svg>`,
    `<text class="node-title" x="52" y="30" fill="${theme.accent}">${escapeXML(theme.label)}</text>`,
    node.data.pageNumber
      ? `<rect x="${pagePillX}" y="${pagePillY}" width="${pagePillWidth}" height="${pagePillHeight}" rx="8.5" fill="#ffffff" opacity=".78"/><text class="node-page" x="${round(pagePillCenterX)}" y="${round(pagePillCenterY)}" text-anchor="middle" dominant-baseline="middle">${escapeXML(pageText)}</text>`
      : "",
  ].filter(Boolean);
  const body = bodyLines.map((line, index) =>
    `<text class="node-body" x="${bodyX}" y="${bodyTop + SVG_BODY_LINE_HEIGHT / 2 + index * SVG_BODY_LINE_HEIGHT}" dominant-baseline="middle">${escapeXML(line)}</text>`,
  );
  const sideRule = type === "highlight" || type === "annotation"
    ? `<rect x="16" y="${bodyTop}" width="3" height="${bodyHeight}" rx="1.5" fill="${theme.accent}"/>`
    : "";

  return [
    `<g transform="translate(${x} ${y})">`,
    ...header,
    sideRule,
    ...body,
    `</g>`,
  ].join("");
}

function bodyLineLimit(type: MindMapNodeType) {
  if (type === "highlight" || type === "annotation") return 6;
  if (type === "paper") return 4;
  if (type === "section" || type === "manual") return 3;
  return 2;
}

function svgThemeForNode(node: MindMapFlowNode) {
  const fallback = SVG_NODE_THEME[node.data.nodeType] || SVG_NODE_THEME.manual;
  if (node.data.nodeType !== "highlight") return fallback;
  const highlight = SVG_HIGHLIGHT_THEME[node.data.color || "yellow"];
  if (!highlight) return fallback;
  return { ...fallback, ...highlight };
}

function renderSVGIcon(type: MindMapNodeType, color: string) {
  const common = `fill="none" stroke="${color}" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"`;
  if (type === "paper") {
    return `<path ${common} d="M3 2.5h8.5L15 6v10.5H3z"/><path ${common} d="M11.5 2.5V6H15"/><path ${common} d="M6 9h6M6 12h5"/>`;
  }
  if (type === "section") {
    return `<path ${common} d="M3 4h10M3 8h10M3 12h7"/><path ${common} d="M2.5 2.5v13"/>`;
  }
  if (type === "highlight") {
    return `<path ${common} d="M10.5 2.5 14 6 7 13H3.5V9.5z"/><path ${common} d="M3 15h10"/>`;
  }
  if (type === "annotation") {
    return `<path ${common} d="M3 4.5A2.5 2.5 0 0 1 5.5 2h7A2.5 2.5 0 0 1 15 4.5v5A2.5 2.5 0 0 1 12.5 12H8l-4 3v-3.2A2.5 2.5 0 0 1 3 9.5z"/><path ${common} d="M6 6h6M6 9h4"/>`;
  }
  if (type === "group") {
    return `<path ${common} d="M2.5 5.5h4l1.5 2h5.5v6.5h-11z"/><path ${common} d="M2.5 5.5V4h4l1.4 1.5"/>`;
  }
  return `<path ${common} d="M4 3h8l2 2v10H4z"/><path ${common} d="M12 3v3h2"/><path ${common} d="M6.5 9h5M6.5 12h3"/>`;
}

function wrapSVGText(text: string, maxWidth: number, fontSize: number, maxLines: number) {
  const normalized = text.replace(/\s+/g, " ").trim();
  const lines: string[] = [];
  let current = "";
  for (const char of normalized) {
    const next = current + char;
    if (current && estimateTextWidth(next, fontSize) > maxWidth) {
      lines.push(current.trim());
      current = char.trimStart();
      if (lines.length >= maxLines) break;
    } else {
      current = next;
    }
  }
  if (current && lines.length < maxLines) lines.push(current.trim());
  if (lines.length === 0) lines.push("");
  if (estimateTextWidth(normalized, fontSize) > maxWidth * maxLines) {
    lines[lines.length - 1] = ellipsizeSVGLine(lines[lines.length - 1], maxWidth, fontSize);
  }
  return lines;
}

function estimateTextWidth(text: string, fontSize: number) {
  let width = 0;
  for (const char of text) {
    if (char === " ") width += fontSize * 0.34;
    else if (/[\u2E80-\u9FFF\uF900-\uFAFF]/.test(char)) width += fontSize;
    else width += fontSize * 0.56;
  }
  return width;
}

function ellipsizeSVGLine(text: string, maxWidth: number, fontSize: number) {
  let value = text;
  while (value.length > 1 && estimateTextWidth(`${value}...`, fontSize) > maxWidth) {
    value = value.slice(0, -1);
  }
  return `${value.trimEnd()}...`;
}

function escapeXML(value: string) {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&apos;");
}

function safeFileName(value: string) {
  return value
    .trim()
    .replace(/[\\/:*?"<>|]+/g, "-")
    .replace(/\s+/g, " ")
    .slice(0, 80) || "gopherpaper-mind-map";
}

function round(value: number) {
  return Number(value.toFixed(2));
}

function toVisibleGraph(graph: MindMapGraph): MindMapGraph {
  const nodeByID = new Map(graph.nodes.map((item) => [item.id, item]));
  const incoming = new Map<string, number>();
  const children = new Map<string, string[]>();
  for (const item of graph.edges) {
    if (!nodeByID.has(item.source) || !nodeByID.has(item.target)) continue;
    children.set(item.source, [...(children.get(item.source) || []), item.target]);
    incoming.set(item.target, (incoming.get(item.target) || 0) + 1);
  }

  const hidden = new Set<string>();
  const visit = (id: string, ancestorCollapsed: boolean) => {
    const node = nodeByID.get(id);
    if (!node) return;
    const collapsed = ancestorCollapsed || Boolean(node.data.collapsed);
    for (const child of children.get(id) || []) {
      if (collapsed) hidden.add(child);
      visit(child, collapsed);
    }
  };

  const roots = graph.nodes.filter((item) => item.type === "paper");
  if (roots.length === 0) {
    roots.push(...graph.nodes.filter((item) => !incoming.has(item.id)));
  }
  for (const node of roots) visit(node.id, false);

  const nodes = graph.nodes.filter((item) => !hidden.has(item.id));
  const ids = new Set(nodes.map((item) => item.id));
  return {
    ...graph,
    nodes,
    edges: graph.edges.filter((item) => ids.has(item.source) && ids.has(item.target)),
  };
}

function childCountMap(graph: MindMapGraph) {
  const counts = new Map<string, number>();
  for (const item of graph.edges) counts.set(item.source, (counts.get(item.source) || 0) + 1);
  return counts;
}

function isStructuralNode(node?: MindMapNode) {
  return node?.type === "paper" || node?.type === "section" || node?.type === "group";
}

function stackHeight(heights: number[]) {
  return heights.reduce((sum, height, index) => sum + height + (index > 0 ? LAYOUT_NODE_GAP : 0), 0);
}

function layoutLeftToRight(graph: MindMapGraph): MindMapGraph {
  const graphWithEdges = ensureParentEdges(graph);
  const nodeByID = new Map(graphWithEdges.nodes.map((item) => [item.id, item]));
  const children = new Map<string, string[]>();
  const incoming = new Map<string, number>();
  const order = new Map(graphWithEdges.nodes.map((item, index) => [item.id, index]));

  for (const edge of graphWithEdges.edges) {
    if (!nodeByID.has(edge.source) || !nodeByID.has(edge.target) || edge.source === edge.target) continue;
    const current = children.get(edge.source) || [];
    if (!current.includes(edge.target)) children.set(edge.source, [...current, edge.target]);
    incoming.set(edge.target, (incoming.get(edge.target) || 0) + 1);
  }

  const roots = graphWithEdges.nodes.filter((item) => item.type === "paper");
  for (const node of graphWithEdges.nodes) {
    if (!incoming.has(node.id) && !roots.some((root) => root.id === node.id)) roots.push(node);
  }
  if (roots.length === 0 && graphWithEdges.nodes[0]) roots.push(graphWithEdges.nodes[0]);

  for (const [parentID, childIDs] of children) {
    children.set(parentID, childIDs.sort((a, b) => (order.get(a) || 0) - (order.get(b) || 0)));
  }

  const measured = new Map<string, number>();
  const positions = new Map<string, { x: number; y: number }>();

  const measure = (id: string, trail: Set<string>): number => {
    const cached = measured.get(id);
    if (cached != null) return cached;
    const node = nodeByID.get(id);
    if (!node || trail.has(id)) return 0;
    const size = nodeSize(node.type, node.data);
    const nextTrail = new Set(trail).add(id);
    const childIDs = (children.get(id) || []).filter((child) => !trail.has(child));
    const structural = childIDs.filter((child) => isStructuralNode(nodeByID.get(child)));
    const inline = childIDs.filter((child) => !isStructuralNode(nodeByID.get(child)));
    const inlineHeight = Math.max(size.height, ...inline.map((child) => measure(child, nextTrail)));
    const structuralHeight = stackHeight(structural.map((child) => measure(child, nextTrail)));
    const height = structural.length > 0 && inline.length > 0
      ? structuralHeight + LAYOUT_NODE_GAP + inlineHeight
      : Math.max(inlineHeight, structuralHeight);
    const finalHeight = Math.max(size.height, height);
    measured.set(id, finalHeight);
    return finalHeight;
  };

  const place = (id: string, x: number, top: number, trail: Set<string>) => {
    const node = nodeByID.get(id);
    if (!node || trail.has(id)) return;
    const size = nodeSize(node.type, node.data);
    const nextTrail = new Set(trail).add(id);
    const childIDs = (children.get(id) || []).filter((child) => !trail.has(child));
    const structural = childIDs.filter((child) => isStructuralNode(nodeByID.get(child)));
    const inline = childIDs.filter((child) => !isStructuralNode(nodeByID.get(child)));
    const structuralHeight = stackHeight(structural.map((child) => measure(child, nextTrail)));
    const inlineHeight = Math.max(size.height, ...inline.map((child) => measure(child, nextTrail)));
    const height = measure(id, trail);
    const xNext = x + size.width + LAYOUT_RANK_GAP;
    const hasSplitRows = structural.length > 0 && inline.length > 0;
    const structuralTop = top + Math.max(0, (height - (hasSplitRows ? structuralHeight + LAYOUT_NODE_GAP + inlineHeight : structuralHeight)) / 2);
    const rowTop = hasSplitRows
      ? structuralTop + structuralHeight + LAYOUT_NODE_GAP
      : top + Math.max(0, (height - inlineHeight) / 2);
    const nodeY = node.type === "paper" && childIDs.length > 0
      ? top + height / 2 - size.height / 2
      : structural.length > 0 && !hasSplitRows
        ? structuralTop + structuralHeight / 2 - size.height / 2
        : rowTop + inlineHeight / 2 - size.height / 2;

    positions.set(id, { x, y: nodeY });

    let childTop = structuralTop;
    for (const child of structural) {
      const childHeight = measure(child, nextTrail);
      place(child, xNext, childTop, nextTrail);
      childTop += childHeight + LAYOUT_NODE_GAP;
    }

    let inlineX = xNext;
    for (const child of inline) {
      const childNode = nodeByID.get(child);
      const childHeight = measure(child, nextTrail);
      place(child, inlineX, rowTop + (inlineHeight - childHeight) / 2, nextTrail);
      inlineX += nodeSize(childNode ? childNode.type : "manual", childNode?.data).width + LAYOUT_RANK_GAP;
    }
  };

  let cursor = LAYOUT_MARGIN_Y;
  for (const root of roots) {
    const height = measure(root.id, new Set());
    place(root.id, LAYOUT_MARGIN_X, cursor, new Set());
    cursor += height + LAYOUT_ROOT_GAP;
  }
  for (const node of graphWithEdges.nodes) {
    if (positions.has(node.id)) continue;
    const height = measure(node.id, new Set());
    place(node.id, LAYOUT_MARGIN_X, cursor, new Set());
    cursor += height + LAYOUT_ROOT_GAP;
  }

  return {
    ...graphWithEdges,
    nodes: graphWithEdges.nodes.map((item) => ({
      ...item,
      position: positions.get(item.id) || item.position,
    })),
  };
}

function ensureParentEdges(graph: MindMapGraph): MindMapGraph {
  const nodeIDs = new Set(graph.nodes.map((item) => item.id));
  const edgeKeys = new Set(graph.edges.map((item) => `${item.source}->${item.target}`));
  const edges = [...graph.edges];

  for (const node of graph.nodes) {
    const parentID = node.data.parentId;
    if (!parentID || parentID === node.id || !nodeIDs.has(parentID)) continue;
    const key = `${parentID}->${node.id}`;
    if (edgeKeys.has(key)) continue;
    edges.push({
      id: key,
      source: parentID,
      target: node.id,
      data: node.data.manual ? { manual: true } : undefined,
    });
    edgeKeys.add(key);
  }

  return edges.length === graph.edges.length ? graph : { ...graph, edges };
}

function isManualNode(node?: MindMapNode) {
  return Boolean(node && (node.type === "manual" || node.data.manual));
}

function removableManualNodeIDs(graph: MindMapGraph, id: string) {
  const nodeByID = new Map(graph.nodes.map((item) => [item.id, item]));
  const root = nodeByID.get(id);
  const removable = new Set<string>();
  if (!isManualNode(root)) return removable;

  const children = new Map<string, string[]>();
  for (const edge of graph.edges) {
    children.set(edge.source, [...(children.get(edge.source) || []), edge.target]);
  }

  const visit = (nodeID: string) => {
    const node = nodeByID.get(nodeID);
    if (!isManualNode(node)) return;
    removable.add(nodeID);
    for (const child of children.get(nodeID) || []) visit(child);
  };
  visit(id);
  return removable;
}

function descendantNodeIDs(graph: MindMapGraph, id: string) {
  const descendants = new Set<string>();
  const children = new Map<string, string[]>();
  for (const edge of graph.edges) {
    children.set(edge.source, [...(children.get(edge.source) || []), edge.target]);
  }

  const visit = (nodeID: string) => {
    for (const child of children.get(nodeID) || []) {
      if (descendants.has(child)) continue;
      descendants.add(child);
      visit(child);
    }
  };
  visit(id);
  return descendants;
}

function manualNodePosition(graph: MindMapGraph, parentID: string) {
  const parent = graph.nodes.find((item) => item.id === parentID);
  const parentSize = parent ? nodeSize(parent.type, parent.data) : nodeSize("manual");
  const childCount = graph.edges.filter((item) => item.source === parentID).length;
  return {
    x: (parent?.position.x || LAYOUT_MARGIN_X) + parentSize.width + LAYOUT_RANK_GAP,
    y: (parent?.position.y || LAYOUT_MARGIN_Y) + childCount * (nodeSize("manual").height + LAYOUT_NODE_GAP),
  };
}

function nodeOptionLabel(node: MindMapNode) {
  const text = node.data.label || node.data.text || node.data.note || node.id;
  return `${(TYPE_META[node.type] || TYPE_META.manual).label} - ${text}`.slice(0, 80);
}

function nodeSize(type: MindMapNodeType, data?: Partial<MindMapNodeData>) {
  const base = NODE_SIZE[type] || NODE_SIZE.manual;
  if (!data) return { width: base.width, height: base.minHeight };

  const text = displayTextForNode(type, data) || "未命名节点";
  const bodyFontSize = type === "paper" ? 15 : type === "highlight" || type === "annotation" ? 13 : 14;
  const bodyX = type === "highlight" || type === "annotation" ? 32 : 16;
  const desiredWidth = Math.ceil(estimatePreferredTextWidth(text, bodyFontSize) + bodyX + 24);
  const width = clamp(desiredWidth, base.width, base.maxWidth);
  const bodyWidth = width - bodyX - 18;
  const bodyLines = wrapSVGText(text, bodyWidth, 13, bodyLineLimit(type));
  const bodyTop = data.pageNumber ? 69 : 61;
  const bodyBottom = bodyTop + Math.max(1, bodyLines.length) * SVG_BODY_LINE_HEIGHT;
  const height = Math.max(base.minHeight, Math.ceil(bodyBottom + 18));

  return { width, height };
}

function displayTextForNode(type: MindMapNodeType, data: Partial<MindMapNodeData>) {
  return type === "annotation"
    ? data.note || data.text || data.label
    : data.text || data.label;
}

function estimatePreferredTextWidth(text: string, fontSize: number) {
  const normalized = text.replace(/\s+/g, " ").trim();
  if (!normalized) return 0;
  const chunks = normalized.split(" ").filter(Boolean);
  const longestChunk = chunks.reduce((longest, chunk) => Math.max(longest, estimateTextWidth(chunk, fontSize)), 0);
  const balanced = Math.sqrt(Math.max(1, estimateTextWidth(normalized, fontSize)) * fontSize * 19);
  return Math.max(longestChunk, balanced);
}

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value));
}
