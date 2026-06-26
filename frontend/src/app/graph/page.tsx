"use client";

import { GraphView } from "@/components/gopherpaper/graph-view";

// 知识图谱独立页:复用工作台 store(papers/选中论文/SSE),展示论文间关系与研究趋势。
export default function GraphPage() {
  return <GraphView />;
}
