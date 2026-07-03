"use client";

// ArchitectureGraph 用 React Flow 画出 GopherPaper 多智能体系统的实时架构拓扑:
// 浏览器 → Go 网关 → 三位智能体(小文鸮/小云雀/小囊鼠)→ MinerU 解析、向量库、
// 知识图谱、大模型等基础设施。纯展示,用于后台架构讲解。

import {
  Background,
  BackgroundVariant,
  Controls,
  Handle,
  MarkerType,
  Position,
  ReactFlow,
  type Edge,
  type Node,
  type NodeProps,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import {
  Bot,
  Boxes,
  Database,
  FileSearch,
  Globe,
  MessagesSquare,
  Network,
  Server,
  Sparkles,
  Workflow,
} from "lucide-react";
import { useMemo } from "react";

type NodeCategory = "client" | "gateway" | "agent" | "infra" | "model";

type ArchNodeData = {
  label: string;
  sub: string;
  icon: keyof typeof ICONS;
  category: NodeCategory;
};

const ICONS = {
  globe: Globe,
  server: Server,
  owl: MessagesSquare,
  lark: Sparkles,
  gopher: FileSearch,
  bot: Bot,
  database: Database,
  network: Network,
  boxes: Boxes,
  workflow: Workflow,
} as const;

const CATEGORY_STYLE: Record<NodeCategory, { ring: string; bg: string; text: string }> = {
  client: { ring: "#0ea5e9", bg: "rgba(14,165,233,0.10)", text: "#0ea5e9" },
  gateway: { ring: "#6366f1", bg: "rgba(99,102,241,0.10)", text: "#6366f1" },
  agent: { ring: "#10b981", bg: "rgba(16,185,129,0.10)", text: "#10b981" },
  infra: { ring: "#f59e0b", bg: "rgba(245,158,11,0.10)", text: "#f59e0b" },
  model: { ring: "#8b5cf6", bg: "rgba(139,92,246,0.10)", text: "#8b5cf6" },
};

function ArchNode({ data }: NodeProps) {
  const d = data as ArchNodeData;
  const Icon = ICONS[d.icon];
  const style = CATEGORY_STYLE[d.category];
  return (
    <div
      className="flex w-44 items-center gap-2.5 rounded-xl border-2 bg-card px-3 py-2.5 shadow-sm"
      style={{ borderColor: style.ring, background: style.bg }}
    >
      <Handle type="target" position={Position.Left} style={{ background: style.ring }} />
      <span
        className="flex size-8 shrink-0 items-center justify-center rounded-lg"
        style={{ background: style.bg, color: style.text }}
      >
        <Icon className="size-4" />
      </span>
      <div className="min-w-0">
        <div className="truncate text-sm font-semibold">{d.label}</div>
        <div className="truncate text-[11px] text-muted-foreground">{d.sub}</div>
      </div>
      <Handle type="source" position={Position.Right} style={{ background: style.ring }} />
    </div>
  );
}

const nodeTypes = { arch: ArchNode };

const NODES: Node<ArchNodeData>[] = [
  { id: "client", type: "arch", position: { x: 0, y: 180 }, data: { label: "浏览器", sub: "Next.js 前端", icon: "globe", category: "client" } },
  { id: "gateway", type: "arch", position: { x: 240, y: 180 }, data: { label: "API 网关", sub: "Go / Gin", icon: "server", category: "gateway" } },
  { id: "owl", type: "arch", position: { x: 500, y: 40 }, data: { label: "小文鸮", sub: "论文精读 Agent", icon: "owl", category: "agent" } },
  { id: "lark", type: "arch", position: { x: 500, y: 180 }, data: { label: "小云雀", sub: "先锋工具 Agent", icon: "lark", category: "agent" } },
  { id: "gopher", type: "arch", position: { x: 500, y: 320 }, data: { label: "小囊鼠", sub: "知识管理 Agent", icon: "gopher", category: "agent" } },
  { id: "model", type: "arch", position: { x: 760, y: 40 }, data: { label: "大模型", sub: "Chat / Embedding", icon: "bot", category: "model" } },
  { id: "mineru", type: "arch", position: { x: 760, y: 140 }, data: { label: "MinerU", sub: "PDF 解析", icon: "workflow", category: "infra" } },
  { id: "milvus", type: "arch", position: { x: 760, y: 240 }, data: { label: "Milvus", sub: "向量检索库", icon: "boxes", category: "infra" } },
  { id: "neo4j", type: "arch", position: { x: 760, y: 340 }, data: { label: "Neo4j", sub: "知识图谱", icon: "network", category: "infra" } },
  { id: "mysql", type: "arch", position: { x: 1000, y: 120 }, data: { label: "MySQL", sub: "业务数据", icon: "database", category: "infra" } },
  { id: "redis", type: "arch", position: { x: 1000, y: 220 }, data: { label: "Redis", sub: "缓存 / 会话", icon: "database", category: "infra" } },
  { id: "mq", type: "arch", position: { x: 1000, y: 320 }, data: { label: "RabbitMQ", sub: "解析任务队列", icon: "boxes", category: "infra" } },
];

function edge(id: string, source: string, target: string, animated = false): Edge {
  return {
    id,
    source,
    target,
    animated,
    markerEnd: { type: MarkerType.ArrowClosed },
    style: { stroke: "var(--border)", strokeWidth: 1.5 },
  };
}

const EDGES: Edge[] = [
  edge("e1", "client", "gateway", true),
  edge("e2", "gateway", "owl", true),
  edge("e3", "gateway", "lark", true),
  edge("e4", "gateway", "gopher", true),
  edge("e5", "owl", "model"),
  edge("e6", "owl", "milvus"),
  edge("e7", "lark", "model"),
  edge("e8", "lark", "mineru"),
  edge("e9", "gopher", "neo4j"),
  edge("e10", "gopher", "milvus"),
  edge("e11", "gateway", "mysql"),
  edge("e12", "gateway", "redis"),
  edge("e13", "gateway", "mq"),
  edge("e14", "mq", "mineru", true),
];

const LEGEND: { label: string; category: NodeCategory }[] = [
  { label: "客户端", category: "client" },
  { label: "网关", category: "gateway" },
  { label: "智能体", category: "agent" },
  { label: "基础设施", category: "infra" },
  { label: "大模型", category: "model" },
];

export function ArchitectureGraph() {
  const nodes = useMemo(() => NODES, []);
  const edges = useMemo(() => EDGES, []);

  return (
    <div className="rounded-xl border border-border bg-card p-4 shadow-sm">
      <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <Workflow className="size-4 text-primary" />
          <h3 className="text-sm font-semibold">多智能体系统架构</h3>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          {LEGEND.map((l) => (
            <span key={l.category} className="flex items-center gap-1.5 text-xs text-muted-foreground">
              <span className="size-2.5 rounded-full" style={{ background: CATEGORY_STYLE[l.category].ring }} />
              {l.label}
            </span>
          ))}
        </div>
      </div>
      <div className="h-[26rem] w-full overflow-hidden rounded-lg border border-border/60">
        <ReactFlow
          nodes={nodes}
          edges={edges}
          nodeTypes={nodeTypes}
          fitView
          proOptions={{ hideAttribution: true }}
          nodesDraggable={false}
          nodesConnectable={false}
          elementsSelectable={false}
        >
          <Background variant={BackgroundVariant.Dots} gap={18} size={1} color="var(--border)" />
          <Controls showInteractive={false} />
        </ReactFlow>
      </div>
    </div>
  );
}
