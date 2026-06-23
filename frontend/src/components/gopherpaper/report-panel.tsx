"use client";

import {
  ArrowRight,
  BarChart3,
  BookOpenText,
  Compass,
  type LucideIcon,
  Lightbulb,
  Loader2,
  Scale,
  Workflow,
} from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";

import { Badge } from "@/components/ui/badge";
import { ScrollArea } from "@/components/ui/scroll-area";
import * as api from "@/lib/gopherpaper/api";
import { useApp } from "@/lib/gopherpaper/store";
import type { ChatResponse, ReportType } from "@/lib/gopherpaper/types";
import { paperTitle } from "@/lib/gopherpaper/utils";
import { Empty, SkeletonLines } from "./app-ui";
import { Markdown } from "./markdown";

const REPORTS: { type: ReportType; label: string; desc: string; icon: LucideIcon }[] = [
  { type: "quickread", label: "论文速读", desc: "全文要点与主线", icon: BookOpenText },
  { type: "method", label: "研究方法", desc: "方法流程与设计", icon: Workflow },
  { type: "result", label: "实验结果", desc: "指标、现象与结论", icon: BarChart3 },
  { type: "innovation", label: "创新与不足", desc: "贡献点与局限", icon: Lightbulb },
  { type: "compare", label: "同类对比", desc: "与相关工作的异同", icon: Scale },
  { type: "future", label: "未来建议", desc: "可延展研究方向", icon: Compass },
];

export function ReportPanel() {
  const { activePaper, activePaperID, reportReady, toast } = useApp();
  const [active, setActive] = useState<ReportType | null>(null);
  const [loading, setLoading] = useState(false);
  const [report, setReport] = useState<ChatResponse | null>(null);
  const [awaiting, setAwaiting] = useState<ReportType | null>(null);
  const autoLoadedRef = useRef<string | null>(null);
  const readySet = useMemo(
    () => reportReady[activePaperID] || {},
    [reportReady, activePaperID],
  );

  useEffect(() => {
    setActive(null);
    setReport(null);
    setLoading(false);
    setAwaiting(null);
  }, [activePaperID]);

  // 进入论文时,若已有就绪报告则自动展示第一篇,免用户点击;每篇只自动一次且不打断手动操作。
  useEffect(() => {
    if (!activePaperID || autoLoadedRef.current === activePaperID) return;
    if (active || report || loading || awaiting) return;
    const firstReady = REPORTS.find((r) => readySet[r.type]);
    if (!firstReady) return;
    const type = firstReady.type;
    autoLoadedRef.current = activePaperID;
    setActive(type);
    setLoading(true);
    let cancelled = false;
    api
      .generateReport(activePaperID, type)
      .then((res) => {
        if (cancelled) return;
        setReport(res);
        setLoading(false);
      })
      .catch((err) => {
        if (cancelled) return;
        if (err instanceof api.ApiError && err.status === 202) {
          setAwaiting(type);
          return;
        }
        setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [activePaperID, readySet, active, report, loading, awaiting]);

  useEffect(() => {
    if (!awaiting || !activePaperID || !readySet[awaiting]) return;
    let cancelled = false;
    api
      .generateReport(activePaperID, awaiting)
      .then((res) => {
        if (cancelled) return;
        setReport(res);
        setAwaiting(null);
        setLoading(false);
      })
      .catch((err) => {
        if (cancelled) return;
        toast((err as Error)?.message || "生成失败", "error");
        setAwaiting(null);
        setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [awaiting, activePaperID, readySet, toast]);

  const run = async (type: ReportType) => {
    if (!activePaperID || loading) return;
    setActive(type);
    setReport(null);
    setAwaiting(null);
    setLoading(true);
    try {
      const res = await api.generateReport(activePaperID, type);
      setReport(res);
      setLoading(false);
    } catch (err) {
      if (err instanceof api.ApiError && err.status === 202) {
        setAwaiting(type);
        return;
      }
      toast((err as Error)?.message || "生成失败", "error");
      setLoading(false);
    }
  };

  if (!activePaper) {
    return (
      <div className="p-4">
        <Empty title="尚未选择论文" text="在左侧选择一篇论文后生成研读报告。" />
      </div>
    );
  }

  return (
    <ScrollArea className="h-full">
      <div className="space-y-5 p-4">
        <div>
          <h2 className="font-serif text-lg font-semibold tracking-tight">研读报告</h2>
          <p className="mt-1 text-sm text-muted-foreground">
            基于《{paperTitle(activePaper)}》生成结构化报告，点击卡片即可查看。
          </p>
        </div>
        {activePaper.status !== "ready" && (
          <div className="rounded-lg border bg-muted/40 p-3 text-sm text-muted-foreground">
            论文尚未完全就绪，当前状态：{activePaper.status}
          </div>
        )}
        <div className="grid gap-2.5 md:grid-cols-2 xl:grid-cols-3">
          {REPORTS.map((r) => {
            const Icon = r.icon;
            const isActive = active === r.type;
            const ready = Boolean(readySet[r.type]);
            const isBusy = loading && isActive;
            return (
              <button
                key={r.type}
                type="button"
                disabled={loading}
                aria-pressed={isActive}
                onClick={() => run(r.type)}
                className={`group relative flex flex-col gap-1.5 rounded-xl border p-4 text-left transition-all duration-200 ease-out focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sienna/40 disabled:cursor-not-allowed disabled:opacity-60 ${
                  isActive
                    ? "border-sienna/60 bg-sienna/[0.06] shadow-sm ring-1 ring-sienna/20"
                    : "border-border bg-card hover:-translate-y-0.5 hover:border-sienna/40 hover:shadow-sm active:translate-y-0 active:scale-[0.99]"
                }`}
              >
                <div className="flex items-center justify-between gap-2">
                  <span
                    className={`flex size-7 items-center justify-center rounded-lg border transition-colors ${
                      isActive
                        ? "border-sienna/30 bg-sienna/10 text-sienna"
                        : "border-border bg-muted/50 text-muted-foreground group-hover:text-sienna"
                    }`}
                  >
                    {isBusy ? <Loader2 className="size-3.5 animate-spin" /> : <Icon className="size-3.5" />}
                  </span>
                  {ready && (
                    <span className="inline-flex items-center gap-1 text-[11px] font-medium text-emerald-600 dark:text-emerald-400">
                      <span className="size-1.5 rounded-full bg-emerald-500" />
                      就绪
                    </span>
                  )}
                </div>
                <span className="mt-1 font-serif text-sm font-semibold text-foreground">
                  {r.label}
                </span>
                <span className="text-xs leading-snug text-muted-foreground">{r.desc}</span>
                <span
                  className={`mt-1.5 inline-flex items-center gap-1 text-[11px] font-medium transition-colors ${
                    isActive ? "text-sienna" : "text-muted-foreground/70 group-hover:text-sienna"
                  }`}
                >
                  {isBusy
                    ? "生成中…"
                    : isActive && report
                      ? "正在查看"
                      : ready
                        ? "点击查看"
                        : "点击生成"}
                  {!isBusy && (
                    <ArrowRight className="size-3 transition-transform group-hover:translate-x-0.5" />
                  )}
                </span>
              </button>
            );
          })}
        </div>

        <div className="rounded-xl border bg-card">
          <div className="p-6">
            {loading ? (
              <div className="space-y-4">
                <p className="flex items-center gap-2 text-sm text-muted-foreground">
                  <Loader2 className="size-4 animate-spin" />
                  {awaiting ? "报告正在后台生成，就绪后自动展示…" : "正在生成报告，稍候…"}
                </p>
                <SkeletonLines lines={6} />
                <SkeletonLines lines={4} />
              </div>
            ) : report ? (
              <article className="animate-in fade-in-50 slide-in-from-bottom-2 duration-300">
                <header className="mb-4 flex items-center gap-2 border-b pb-3">
                  <h3 className="font-serif text-xl font-semibold">
                    {REPORTS.find((r) => r.type === active)?.label}
                  </h3>
                  {report.intent && (
                    <Badge variant="secondary" className="rounded-full font-normal">
                      {report.intent}
                    </Badge>
                  )}
                </header>
                <Markdown>{report.content}</Markdown>
              </article>
            ) : (
              <Empty title="还没有打开报告" text="点击上方任一卡片，查看或生成对应研读报告。" compact />
            )}
          </div>
        </div>
      </div>
    </ScrollArea>
  );
}
