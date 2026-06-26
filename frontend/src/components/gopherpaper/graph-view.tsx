"use client";

import { ArrowLeft, FileText, Loader2, Network } from "lucide-react";
import Link from "next/link";
import { useEffect, useMemo, useState } from "react";

import { buttonVariants } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import * as api from "@/lib/gopherpaper/api";
import { useApp } from "@/lib/gopherpaper/store";
import type {
  GraphStats,
  GraphTrends,
  NameCount,
  RelatedPaper,
} from "@/lib/gopherpaper/types";
import { paperTitle } from "@/lib/gopherpaper/utils";
import { cn } from "@/lib/utils";
import { Empty } from "./app-ui";

// 关系类型的展示元信息:配色与中文名,边按存在的最强关系上色。
const VIA_META: Record<string, { label: string; color: string }> = {
  cites: { label: "引用", color: "#8b5cf6" },
  cocitation: { label: "共被引", color: "#f59e0b" },
  author: { label: "共享作者", color: "#10b981" },
  keyword: { label: "共享主题", color: "#0ea5e9" },
  similar: { label: "语义相似", color: "#ec4899" },
};
// 边配色优先级:引用关系最强,语义相似兜底最弱。
const VIA_PRIORITY = ["cites", "cocitation", "author", "keyword", "similar"];

function primaryVia(vias: string[]): string {
  for (const v of VIA_PRIORITY) if (vias.includes(v)) return v;
  return vias[0] || "similar";
}

function shortTitle(s: string, n = 16): string {
  const t = s.trim() || "(未命名)";
  return [...t].length > n ? `${[...t].slice(0, n).join("")}…` : t;
}

// RelationGraph 用 SVG 画一张以选中论文为中心的辐射关系图,周围是相关论文,边色标关系类型。
// 点击相关论文节点可把它设为新的中心,沿关系链浏览。
function RelationGraph({
  centerTitle,
  related,
  onPick,
}: {
  centerTitle: string;
  related: RelatedPaper[];
  onPick: (id: string) => void;
}) {
  const cx = 400;
  const cy = 270;
  const rx = 268;
  const ry = 196;
  const maxScore = Math.max(1, ...related.map((r) => r.score));

  const nodes = related.map((r, i) => {
    const angle = (-90 + (i * 360) / related.length) * (Math.PI / 180);
    const x = cx + rx * Math.cos(angle);
    const y = cy + ry * Math.sin(angle);
    const radius = 15 + 13 * (r.score / maxScore);
    const via = primaryVia(r.vias);
    return { ...r, x, y, radius, color: VIA_META[via]?.color || "#94a3b8" };
  });

  return (
    <svg viewBox="0 0 800 540" className="h-auto w-full" role="img" aria-label="论文关系图">
      {/* 关系边 */}
      {nodes.map((n) => (
        <line
          key={`e-${n.id}`}
          x1={cx}
          y1={cy}
          x2={n.x}
          y2={n.y}
          stroke={n.color}
          strokeWidth={1.4 + 2.6 * (n.score / maxScore)}
          strokeOpacity={0.55}
        />
      ))}

      {/* 相关论文节点 */}
      {nodes.map((n) => (
        <g
          key={n.id}
          className="cursor-pointer"
          onClick={() => onPick(n.id)}
          style={{ transition: "opacity .2s" }}
        >
          <title>
            {`${n.title || "(未命名)"}${n.year ? ` · ${n.year}` : ""}\n关系: ${n.vias
              .map((v) => VIA_META[v]?.label || v)
              .join("、")} · 强度 ${n.score}`}
          </title>
          <circle cx={n.x} cy={n.y} r={n.radius} fill={n.color} fillOpacity={0.16} stroke={n.color} strokeWidth={2} />
          <text x={n.x} y={n.y + 4} textAnchor="middle" className="fill-foreground text-[12px] font-medium">
            {n.year || ""}
          </text>
          <text
            x={n.x}
            y={n.y + n.radius + 15}
            textAnchor="middle"
            className="fill-muted-foreground text-[11px]"
          >
            {shortTitle(n.title, 14)}
          </text>
        </g>
      ))}

      {/* 中心论文节点 */}
      <circle cx={cx} cy={cy} r={46} className="fill-primary" />
      <foreignObject x={cx - 64} y={cy - 30} width={128} height={60}>
        <div className="flex h-[60px] items-center justify-center px-1 text-center text-[12px] font-semibold leading-tight text-primary-foreground">
          {shortTitle(centerTitle, 22)}
        </div>
      </foreignObject>
    </svg>
  );
}

// StatChip 是总览里的一格数字指标。
function StatChip({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="rounded-lg border bg-card px-4 py-2.5 shadow-sm">
      <div className="text-xl font-semibold tabular-nums">{value}</div>
      <div className="text-xs text-muted-foreground">{label}</div>
    </div>
  );
}

// YearTrend 是年度论文数的小柱状图,直观看研究随时间的分布。
function YearTrend({ data }: { data: GraphTrends["by_year"] }) {
  if (data.length === 0) {
    return <p className="text-sm text-muted-foreground">暂无带年份的论文,解析新论文后会自动抽取发表年份。</p>;
  }
  const max = Math.max(...data.map((d) => d.count));
  return (
    <div className="flex items-end gap-2 overflow-x-auto pb-1">
      {data.map((d) => (
        <div key={d.year} className="flex min-w-[2.2rem] flex-col items-center gap-1">
          <span className="text-xs tabular-nums text-muted-foreground">{d.count}</span>
          <div
            className="w-7 rounded-t bg-primary/80"
            style={{ height: `${8 + 96 * (d.count / max)}px` }}
            title={`${d.year}: ${d.count} 篇`}
          />
          <span className="text-xs tabular-nums text-muted-foreground">{d.year}</span>
        </div>
      ))}
    </div>
  );
}

export function GraphView() {
  const { authed, papers, activePaperID, selectPaper } = useApp();

  const [overview, setOverview] = useState<GraphStats | null>(null);
  const [trends, setTrends] = useState<GraphTrends | null>(null);
  const [keywords, setKeywords] = useState<NameCount[]>([]);
  const [related, setRelated] = useState<RelatedPaper[]>([]);
  const [loadingRelated, setLoadingRelated] = useState(false);

  // 总览/趋势/关键词随登录态加载一次。
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
      .graphTrends(8)
      .then((value) => {
        if (!cancelled) setTrends(value);
      })
      .catch(() => {
        if (!cancelled) setTrends(null);
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

  // 相关论文随选中论文变化加载。
  useEffect(() => {
    if (!authed || !activePaperID) {
      setRelated([]);
      return;
    }
    let cancelled = false;
    setLoadingRelated(true);
    api
      .relatedPapers(activePaperID, 12)
      .then((r) => {
        if (!cancelled) setRelated(Array.isArray(r) ? r : []);
      })
      .catch(() => {
        if (!cancelled) setRelated([]);
      })
      .finally(() => {
        if (!cancelled) setLoadingRelated(false);
      });
    return () => {
      cancelled = true;
    };
  }, [authed, activePaperID]);

  const centerPaper = useMemo(
    () => papers.find((p) => p.id === activePaperID) || null,
    [papers, activePaperID],
  );
  const maxKw = Math.max(1, ...keywords.map((k) => k.count));

  if (!authed) {
    return (
      <main className="flex h-dvh items-center justify-center bg-muted/50 p-6">
        <div className="text-center">
          <Empty title="请先登录" text="登录后即可查看论文关系图谱。" />
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
          <h1 className="font-serif text-base font-semibold tracking-tight">知识图谱 · 论文关系</h1>
          <p className="truncate text-xs text-muted-foreground">
            选一篇论文,看它与其他论文的引用、共享作者与共享主题关系,以及整体研究趋势
          </p>
        </div>
      </header>

      <div className="grid min-h-0 flex-1 grid-cols-1 lg:grid-cols-[18rem_minmax(0,1fr)]">
        {/* 论文列表 */}
        <aside className="hidden min-h-0 flex-col border-r bg-background lg:flex">
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
                    onClick={() => selectPaper(p.id)}
                    className={cn(
                      "flex w-full items-start gap-2 rounded-md px-2.5 py-2 text-left transition-colors",
                      p.id === activePaperID
                        ? "bg-card shadow-sm ring-1 ring-sienna/20"
                        : "hover:bg-accent/60",
                    )}
                  >
                    <FileText
                      className={cn(
                        "mt-0.5 size-3.5 shrink-0",
                        p.id === activePaperID ? "text-sienna" : "text-muted-foreground",
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

        {/* 图谱主区 */}
        <ScrollArea className="min-h-0 bg-background">
          <div className="space-y-6 p-5">
            {/* 总览 */}
            <section className="grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-6">
              <StatChip label="论文" value={overview?.papers ?? "—"} />
              <StatChip label="作者" value={overview?.authors ?? "—"} />
              <StatChip label="关键词" value={overview?.keywords ?? "—"} />
              <StatChip label="引用关系" value={overview?.citations ?? "—"} />
              <StatChip
                label="年份跨度"
                value={
                  overview && overview.min_year > 0
                    ? `${overview.min_year}–${overview.max_year}`
                    : "—"
                }
              />
              <StatChip label="相关论文" value={related.length} />
            </section>

            {/* 关系图 */}
            <section className="rounded-xl border bg-card p-4 shadow-sm">
              <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
                <div className="flex items-center gap-2 text-sm font-medium">
                  <Network className="size-4 text-primary" />
                  论文关系图
                </div>
                <div className="flex flex-wrap gap-3">
                  {VIA_PRIORITY.map((v) => (
                    <span key={v} className="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
                      <span className="size-2.5 rounded-full" style={{ background: VIA_META[v].color }} />
                      {VIA_META[v].label}
                    </span>
                  ))}
                </div>
              </div>

              {!centerPaper ? (
                <Empty title="选一篇论文" text="在左侧选中论文,这里会画出它的关系网络。" compact />
              ) : loadingRelated ? (
                <div className="flex h-72 items-center justify-center text-muted-foreground">
                  <Loader2 className="size-5 animate-spin" />
                </div>
              ) : related.length === 0 ? (
                <Empty
                  title="暂无相关论文"
                  text="该论文还没有与其他论文形成可见关系。多解析几篇相关领域的论文后,共享作者/主题与引用关系会自动浮现。"
                  compact
                />
              ) : (
                <RelationGraph
                  centerTitle={paperTitle(centerPaper)}
                  related={related}
                  onPick={selectPaper}
                />
              )}
            </section>

            {/* 趋势与关键词 */}
            <section className="grid grid-cols-1 gap-4 xl:grid-cols-2">
              <div className="rounded-xl border bg-card p-4 shadow-sm">
                <div className="mb-3 text-sm font-medium">年度发表趋势</div>
                <YearTrend data={trends?.by_year ?? []} />
              </div>
              <div className="rounded-xl border bg-card p-4 shadow-sm">
                <div className="mb-3 text-sm font-medium">热门研究主题</div>
                {keywords.length === 0 ? (
                  <p className="text-sm text-muted-foreground">暂无关键词数据。</p>
                ) : (
                  <div className="flex flex-wrap gap-2">
                    {keywords.map((k) => (
                      <span
                        key={k.name}
                        className="inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-xs"
                        style={{
                          background: `color-mix(in srgb, var(--primary) ${8 + 22 * (k.count / maxKw)}%, transparent)`,
                        }}
                        title={`${k.count} 篇`}
                      >
                        {k.name}
                        <span className="tabular-nums text-muted-foreground">{k.count}</span>
                      </span>
                    ))}
                  </div>
                )}
              </div>
            </section>
          </div>
        </ScrollArea>
      </div>
    </main>
  );
}
