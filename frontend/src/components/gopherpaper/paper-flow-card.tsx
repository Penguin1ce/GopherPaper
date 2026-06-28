"use client";

// PaperFlowCard 在助教气泡内渲染 generate_paper_flow 的论文思路图谱,两阶段渐进构建:
//  1 骨架(paper_flow 事件):节点先以「待补充」占位出现,dagre 一次性算好布局、位置自此固定。
//  2 逐节点 detail(paper_flow_node 事件):节点按顺序「点亮」并填入结合原文写的细节——Claude 式逐个冒出。
// 节点高度固定预留,故 detail 填入不引起重排、位置稳定;提供缩放控件与全屏展开便于大图浏览。

import dagre from "@dagrejs/dagre";
import {
  Background,
  BackgroundVariant,
  Controls,
  Handle,
  MarkerType,
  Position,
  ReactFlow,
  useEdgesState,
  useNodesState,
  type Edge,
  type Node,
  type NodeProps,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import type { ReactFlowInstance } from "@xyflow/react";
import { Loader2, Maximize2, Workflow, X } from "lucide-react";
import { motion } from "motion/react";
import { useEffect, useMemo, useRef, useState } from "react";

import { Button } from "@/components/ui/button";
import { figureUrl } from "@/lib/gopherpaper/api";
import type { PaperFlow } from "@/lib/gopherpaper/types";
import { cn } from "@/lib/utils";

// 节点类型配色与中文名,与知识图谱(graph-view)的色系保持一致。
const TYPE_META: Record<string, { label: string; color: string }> = {
  problem: { label: "研究问题", color: "#dc2626" },
  gap: { label: "现有不足", color: "#d97706" },
  idea: { label: "核心思路", color: "#7c3aed" },
  method: { label: "方法设计", color: "#2563eb" },
  experiment: { label: "实验验证", color: "#0891b2" },
  result: { label: "关键结果", color: "#16a34a" },
  conclusion: { label: "结论贡献", color: "#4f46e5" },
};

function typeMeta(type: string) {
  return TYPE_META[type] || { label: "节点", color: "#64748b" };
}

// 节点尺寸固定预留:detail 后填,固定高度让布局稳定、不因填字重排。
const NODE_W = 340;
const NODE_H = 208;
// 配图旁注节点尺寸与离主节点的横向间距。
const FIG_W = 220;
const FIG_H = 172;
const FIG_GAP = 40;

// FlowNodeData 注入 React Flow 的节点数据,携带原始字段、入场顺序与是否已补 detail。
type FlowNodeData = {
  label: string;
  detail?: string;
  type: string;
  order: number;
};

// FigureNodeData 配图旁注节点数据:论文真实插图(doc_id+img_name 取图)+ 简短说明。
type FigureNodeData = {
  docId: string;
  imgName: string;
  caption?: string;
};

// FlowNode 自定义节点:卡片式,顶部入边、底部出边、两侧出边(挂旁注)。
// 待补充时为虚线占位 + 加载提示;detail 到达后实线点亮并淡入正文。
function FlowNode({ data }: NodeProps) {
  const d = data as FlowNodeData;
  const meta = typeMeta(d.type);
  const filled = !!d.detail?.trim();
  return (
    <motion.div
      initial={{ opacity: 0, y: 8 }}
      animate={{
        opacity: 1,
        y: 0,
        borderColor: filled
          ? `color-mix(in srgb, ${meta.color} 48%, transparent)`
          : "var(--border)",
      }}
      transition={{ opacity: { duration: 0.3 }, y: { duration: 0.3 }, borderColor: { duration: 0.4 } }}
      className={cn(
        "flex flex-col gap-2 rounded-xl border px-4 py-3",
        filled ? "bg-card shadow-sm" : "border-dashed bg-muted/30",
      )}
      style={{ width: NODE_W, height: NODE_H }}
    >
      <Handle id="t" type="target" position={Position.Top} className="!size-2 !border-0 !bg-muted-foreground/40" />
      <Handle id="b" type="source" position={Position.Bottom} className="!size-2 !border-0 !bg-muted-foreground/40" />
      <Handle id="sl" type="source" position={Position.Left} className="!size-1 !border-0 !bg-transparent" />
      <Handle id="sr" type="source" position={Position.Right} className="!size-1 !border-0 !bg-transparent" />
      <div className="flex items-center gap-2">
        <span
          className="inline-flex shrink-0 items-center rounded-full px-2 py-0.5 text-[11px] font-medium"
          style={{ color: meta.color, background: `color-mix(in srgb, ${meta.color} 12%, transparent)` }}
        >
          {meta.label}
        </span>
        <span className="min-w-0 flex-1 truncate text-sm font-semibold text-foreground">{d.label}</span>
      </div>
      {filled ? (
        <motion.p
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ duration: 0.3 }}
          className="line-clamp-6 text-[13px] leading-6 text-muted-foreground"
        >
          {d.detail}
        </motion.p>
      ) : (
        <span className="mt-0.5 inline-flex items-center gap-1.5 text-xs text-muted-foreground/70">
          <Loader2 className="size-3.5 animate-spin" />
          正在结合原文补充…
        </span>
      )}
    </motion.div>
  );
}

// FigureNode 配图旁注节点:论文真实插图缩略图 + 说明,两侧各一入边接父节点。
// 点击在新标签打开原图;caption 走 title 悬停看全文。
function FigureNode({ data }: NodeProps) {
  const d = data as FigureNodeData;
  const src = figureUrl(d.docId, d.imgName);
  return (
    <motion.a
      href={src}
      target="_blank"
      rel="noreferrer"
      initial={{ opacity: 0, scale: 0.92 }}
      animate={{ opacity: 1, scale: 1 }}
      transition={{ duration: 0.3 }}
      className="flex flex-col overflow-hidden rounded-lg border bg-card shadow-sm transition-shadow hover:shadow-md"
      style={{ width: FIG_W, height: FIG_H }}
      title={d.caption || "论文配图"}
    >
      <Handle id="tl" type="target" position={Position.Left} className="!size-1 !border-0 !bg-transparent" />
      <Handle id="tr" type="target" position={Position.Right} className="!size-1 !border-0 !bg-transparent" />
      <img
        src={src}
        alt={d.caption || "论文配图"}
        loading="lazy"
        className="min-h-0 w-full flex-1 bg-muted/20 object-contain p-1"
      />
      {d.caption && (
        <span className="shrink-0 truncate border-t bg-card px-2 py-1 text-[11px] text-muted-foreground">
          {d.caption}
        </span>
      )}
    </motion.a>
  );
}

const nodeTypes = { flow: FlowNode, figure: FigureNode };

// LayoutResult:渲染用的节点/边 + focus(最新主节点中心,供构建时固定缩放跟随)。
type LayoutResult = { nodes: Node[]; edges: Edge[]; focus: { x: number; y: number } | null };

// layout 主线走 dagre(自顶向下,固定高度),旁注不进 dagre、手动摆在父节点左右两侧,故主线不受影响。
function layout(flow: PaperFlow): LayoutResult {
  const g = new dagre.graphlib.Graph();
  g.setGraph({ rankdir: "TB", nodesep: 34, ranksep: 58, marginx: 16, marginy: 16 });
  g.setDefaultEdgeLabel(() => ({}));

  const validIDs = new Set(flow.nodes.map((n) => n.id));
  for (const n of flow.nodes) g.setNode(n.id, { width: NODE_W, height: NODE_H });
  const edges = flow.edges.filter((e) => validIDs.has(e.from) && validIDs.has(e.to));
  for (const e of edges) g.setEdge(e.from, e.to);
  dagre.layout(g);

  // 只渲染「已开始补充(detail 非空)」的主节点:后端顺序逐节点生成,故节点天然按序逐个冒出。
  // 位置取自完整骨架,稳定不跳。focus 记最新冒出主节点中心,供构建时固定缩放跟随。
  const revealed = flow.nodes.filter((n) => !!n.detail?.trim());
  const revealedIDs = new Set(revealed.map((n) => n.id));
  let focus: { x: number; y: number } | null = null;
  const nodes: Node[] = revealed.map((n) => {
    const pos = g.node(n.id);
    focus = { x: pos?.x ?? 0, y: pos?.y ?? 0 };
    return {
      id: n.id,
      type: "flow",
      position: { x: (pos?.x ?? 0) - NODE_W / 2, y: (pos?.y ?? 0) - NODE_H / 2 },
      data: { label: n.label, detail: n.detail, type: n.type, order: 0 },
    };
  });

  const edgeColor = "color-mix(in srgb, var(--primary) 55%, transparent)";
  const rfEdges: Edge[] = edges
    .filter((e) => revealedIDs.has(e.from) && revealedIDs.has(e.to))
    .map((e, i) => ({
      id: `e-${e.from}-${e.to}-${i}`,
      source: e.from,
      target: e.to,
      sourceHandle: "b",
      targetHandle: "t",
      label: e.label,
      animated: true,
      style: { stroke: edgeColor, strokeWidth: 1.6 },
      labelStyle: { fontSize: 10, fill: "var(--muted-foreground)" },
      labelBgStyle: { fill: "var(--background)", fillOpacity: 0.85 },
      markerEnd: { type: MarkerType.ArrowClosed, color: edgeColor },
    }));

  // 配图旁注:只在父节点已冒出时挂,每个主节点最多一张。碰撞检测放置——优先右侧,
  // 与任一主节点或已放配图重叠则换左/更外侧,绝不压主线。不进 dagre。
  const figures = (flow.figures ?? []).filter((f) => revealedIDs.has(f.parent) && f.img_name);
  const mainBoxes = revealed.map((n) => {
    const pos = g.node(n.id);
    return { x: (pos?.x ?? 0) - NODE_W / 2, y: (pos?.y ?? 0) - NODE_H / 2, w: NODE_W, h: NODE_H };
  });
  const placedBoxes: { x: number; y: number; w: number; h: number }[] = [];
  const overlaps = (a: { x: number; y: number; w: number; h: number }) =>
    [...mainBoxes, ...placedBoxes].some(
      (b) => a.x < b.x + b.w + 12 && a.x + a.w + 12 > b.x && a.y < b.y + b.h + 12 && a.y + a.h + 12 > b.y,
    );
  const auxEdgeColor = "color-mix(in srgb, var(--muted-foreground) 45%, transparent)";
  // 候选位:右、左、更右、更左,逐个试,取第一个不重叠的;都不行兜底放最右。
  const cands: { side: 1 | -1; mult: number }[] = [
    { side: 1, mult: 1 },
    { side: -1, mult: 1 },
    { side: 1, mult: 1.8 },
    { side: -1, mult: 1.8 },
  ];
  for (const f of figures) {
    const pos = g.node(f.parent);
    const px = pos?.x ?? 0;
    const py = pos?.y ?? 0;
    let chosen = cands[0];
    for (const c of cands) {
      const cx = px + c.side * (NODE_W / 2 + FIG_GAP + FIG_W / 2) * c.mult;
      const box = { x: cx - FIG_W / 2, y: py - FIG_H / 2, w: FIG_W, h: FIG_H };
      if (!overlaps(box)) {
        chosen = c;
        break;
      }
    }
    const cx = px + chosen.side * (NODE_W / 2 + FIG_GAP + FIG_W / 2) * chosen.mult;
    const box = { x: cx - FIG_W / 2, y: py - FIG_H / 2, w: FIG_W, h: FIG_H };
    placedBoxes.push(box);
    nodes.push({
      id: f.id,
      type: "figure",
      position: { x: box.x, y: box.y },
      data: { docId: f.doc_id, imgName: f.img_name, caption: f.caption },
    });
    rfEdges.push({
      id: `fig-${f.id}`,
      source: f.parent,
      target: f.id,
      sourceHandle: chosen.side === 1 ? "sr" : "sl",
      targetHandle: chosen.side === 1 ? "tl" : "tr",
      style: { stroke: auxEdgeColor, strokeWidth: 1.2, strokeDasharray: "4 3" },
    });
  }

  return { nodes, edges: rfEdges, focus };
}

// FLOW_ZOOM 是实时构建时固定的缩放,节点显示大小恒定不随新节点跳变。
const FLOW_ZOOM = 0.9;

// FlowCanvas 是可复用画布,内嵌与全屏共用;expanded 时开启滚轮缩放。
// 用 useNodesState/useEdgesState 受控:React Flow 挂载后不会自动跟随 nodes prop 变化,
// 故 layout(detail 逐节点补齐)更新时经 useEffect 主动 setNodes 同步,节点方能逐个点亮。
// 视图策略:实时构建中固定缩放、仅平移跟随最新节点(节点大小恒定不跳);
// 全屏或刷新还原(挂载即完整图)时一次性 fitView 看全貌。
function FlowCanvas({
  layouted,
  building,
  expanded,
}: {
  layouted: LayoutResult;
  building: boolean;
  expanded?: boolean;
}) {
  const [nodes, setNodes, onNodesChange] = useNodesState(layouted.nodes);
  const [edges, setEdges, onEdgesChange] = useEdgesState(layouted.edges);
  const rfRef = useRef<ReactFlowInstance | null>(null);
  // 视图模式在首帧定下:全屏或挂载即完整(刷新还原)走 fit 全图,实时构建走固定缩放跟随。
  const modeRef = useRef<"fit" | "follow" | null>(null);
  const focusX = layouted.focus?.x ?? null;
  const focusY = layouted.focus?.y ?? null;
  useEffect(() => {
    setNodes(layouted.nodes);
  }, [layouted.nodes, setNodes]);
  useEffect(() => {
    setEdges(layouted.edges);
  }, [layouted.edges, setEdges]);
  useEffect(() => {
    const inst = rfRef.current;
    if (!inst || focusX === null || focusY === null) return;
    if (modeRef.current === null) {
      modeRef.current = expanded || !building ? "fit" : "follow";
    }
    const id = requestAnimationFrame(() => {
      if (modeRef.current === "follow") {
        // 固定缩放,平移到最新冒出的主节点中心,节点大小恒定。
        inst.setCenter(focusX, focusY, { zoom: FLOW_ZOOM, duration: 450 });
      } else {
        inst.fitView({ padding: 0.14, minZoom: 0.35, maxZoom: FLOW_ZOOM, duration: 300 });
      }
    });
    return () => cancelAnimationFrame(id);
    // 只在最新主节点(focus)变化时调整视图,不随流式 detail 抖动(focus 不随 detail 变)。
  }, [focusX, focusY, building, expanded]);
  return (
    <ReactFlow
      nodes={nodes}
      edges={edges}
      onNodesChange={onNodesChange}
      onEdgesChange={onEdgesChange}
      onInit={(inst) => {
        rfRef.current = inst;
      }}
      nodeTypes={nodeTypes}
      minZoom={0.3}
      maxZoom={2}
      nodesConnectable={false}
      edgesFocusable={false}
      zoomOnScroll={!!expanded}
      panOnScroll={false}
      proOptions={{ hideAttribution: true }}
    >
      <Background variant={BackgroundVariant.Dots} gap={20} size={1} className="!bg-transparent" />
      <Controls showInteractive={false} className="!shadow-sm" />
    </ReactFlow>
  );
}

export function PaperFlowCard({ flow, className }: { flow: PaperFlow; className?: string }) {
  const layouted = useMemo(() => layout(flow), [flow]);
  const [expanded, setExpanded] = useState(false);
  if (flow.nodes.length === 0) return null;

  const total = flow.nodes.length; // 主节点总数(旁注不计)
  const revealedCount = flow.nodes.filter((n) => n.detail?.trim()).length; // 已冒出的主节点数
  const building = revealedCount < total;

  const header = (onClose?: () => void) => (
    <div className="flex items-center gap-2 border-b bg-card/60 px-3 py-2">
      <Workflow className="size-3.5 shrink-0 text-primary" aria-hidden />
      <span className="min-w-0 flex-1 truncate text-xs font-semibold text-foreground/85">
        研究思路图 · {flow.title}
      </span>
      <span className="shrink-0 text-[11px] text-muted-foreground">
        {building ? `补充中 ${revealedCount}/${total}` : `${total} 节点`}
      </span>
      {onClose ? (
        <Button variant="ghost" size="icon-xs" title="关闭" onClick={onClose}>
          <X className="size-3.5" />
        </Button>
      ) : (
        <Button variant="ghost" size="icon-xs" title="全屏查看" onClick={() => setExpanded(true)}>
          <Maximize2 className="size-3.5" />
        </Button>
      )}
    </div>
  );

  return (
    <>
      <div className={cn("mt-3 overflow-hidden rounded-xl border bg-muted/20", className)}>
        {header()}
        <div className="h-[500px] w-full">
          <FlowCanvas layouted={layouted} building={building} />
        </div>
      </div>

      {expanded && (
        <div className="fixed inset-0 z-50 flex flex-col bg-background/95 backdrop-blur-sm">
          {header(() => setExpanded(false))}
          <div className="min-h-0 flex-1">
            <FlowCanvas layouted={layouted} building={building} expanded />
          </div>
        </div>
      )}
    </>
  );
}
