"use client";

import {
  ArrowRight,
  BarChart3,
  BookOpenText,
  Compass,
  Download,
  type LucideIcon,
  Lightbulb,
  Loader2,
  Workflow,
} from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import * as api from "@/lib/gopherpaper/api";
import { printReport } from "@/lib/gopherpaper/print";
import { useApp } from "@/lib/gopherpaper/store";
import type { ChatResponse, Reference, ReportType } from "@/lib/gopherpaper/types";
import { paperTitle } from "@/lib/gopherpaper/utils";
import { Empty, SkeletonLines } from "./app-ui";
import { Markdown } from "./markdown";
import { PaperOverview } from "./paper-overview";
import { ProcessTrace } from "./process-trace";

// buildFigureMap 从报告 meta.sources 收图块出处,拼成 markdown 解析 figure://文件名 用的 名->docID 表。
function buildFigureMap(meta?: Record<string, unknown>): Record<string, string> {
  const map: Record<string, string> = {};
  const refs = (meta?.sources as Reference[] | undefined) ?? [];
  for (const r of refs) {
    if (r.block_type === "image" && r.img_name && r.doc_id) map[r.img_name] = r.doc_id;
  }
  return map;
}

const REPORTS: { type: ReportType; label: string; desc: string; icon: LucideIcon }[] = [
  { type: "quickread", label: "论文速读", desc: "全文要点与主线", icon: BookOpenText },
  { type: "method", label: "研究方法", desc: "方法流程与设计", icon: Workflow },
  { type: "result", label: "实验结果", desc: "指标、现象与结论", icon: BarChart3 },
  { type: "innovation", label: "创新与不足", desc: "贡献点与局限", icon: Lightbulb },
  { type: "future", label: "未来建议", desc: "可延展研究方向", icon: Compass },
];

export function ReportPanel() {
  const { activePaper, activePaperID, reportReady, reportProgress, beginReport, toast } =
    useApp();
  const [active, setActive] = useState<ReportType | null>(null);
  const [loading, setLoading] = useState(false);
  const [report, setReport] = useState<ChatResponse | null>(null);
  const [awaiting, setAwaiting] = useState<ReportType | null>(null);
  const autoLoadedRef = useRef<string | null>(null);
  const articleRef = useRef<HTMLDivElement | null>(null);
  // 当前论文 ID 的 ref 镜像:异步结果回来时据此校验仍是这篇论文才落地,避免切走后覆盖,
  // 也避免把 active/loading 放进 effect 依赖(那会让 setActive/setLoading 触发 effect 自我清理)。
  const activePaperRef = useRef(activePaperID);
  useEffect(() => {
    activePaperRef.current = activePaperID;
  }, [activePaperID]);
  const readySet = useMemo(
    () => reportReady[activePaperID] || {},
    [reportReady, activePaperID],
  );
  // 当前查看/生成中报告的实时进度(执行计划/进行中/失败),由 report_progress 事件累积。
  const run = active ? reportProgress[activePaperID]?.[active] : undefined;

  useEffect(() => {
    setActive(null);
    setReport(null);
    setLoading(false);
    setAwaiting(null);
  }, [activePaperID]);

  // 进入论文时,若已有就绪报告则自动展示第一篇,免用户点击;每篇只自动一次(autoLoadedRef 守门)。
  // 依赖只放 activePaperID 与 readySet:不放 active/loading,否则本 effect 内的 setActive/setLoading
  // 会改变依赖触发清理、丢弃在飞请求,导致永远停在「生成中」。新鲜度改由 activePaperRef 校验。
  useEffect(() => {
    if (!activePaperID || autoLoadedRef.current === activePaperID) return;
    const firstReady = REPORTS.find((r) => readySet[r.type]);
    if (!firstReady) return;
    const type = firstReady.type;
    const paper = activePaperID;
    autoLoadedRef.current = paper;
    setActive(type);
    setLoading(true);
    api
      .generateReport(paper, type)
      .then((res) => {
        if (activePaperRef.current !== paper) return;
        setReport(res);
        setLoading(false);
      })
      .catch((err) => {
        if (activePaperRef.current !== paper) return;
        if (err instanceof api.ApiError && err.status === 202) {
          setAwaiting(type);
          return;
        }
        setLoading(false);
      });
  }, [activePaperID, readySet]);

  // awaiting:点了未就绪报告拿到 202 后,等 report_ready 让 readySet 更新,再拉一次缓存落地。
  useEffect(() => {
    if (!awaiting || !activePaperID || !readySet[awaiting]) return;
    const paper = activePaperID;
    const type = awaiting;
    api
      .generateReport(paper, type)
      .then((res) => {
        if (activePaperRef.current !== paper) return;
        setReport(res);
        setAwaiting(null);
        setLoading(false);
      })
      .catch((err) => {
        if (activePaperRef.current !== paper) return;
        toast((err as Error)?.message || "生成失败", "error");
        setAwaiting(null);
        setLoading(false);
      });
  }, [awaiting, activePaperID, readySet, toast]);

  const generate = async (type: ReportType) => {
    if (!activePaperID || loading) return;
    // 标记本篇已被用户主动操作,避免随后 auto-load 再覆盖。
    autoLoadedRef.current = activePaperID;
    setActive(type);
    setReport(null);
    setAwaiting(null);
    setLoading(true);
    // 未就绪的报告点了即起新一轮长任务,重置执行计划进度;命中缓存则下方直接返回内容、进度为空。
    if (!readySet[type]) beginReport(activePaperID, type);
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

  // 把当前报告正文按打印样式导出 PDF:取已渲染的正文 HTML 写进隔离 iframe 触发打印。
  const downloadPDF = () => {
    if (!articleRef.current || !active || !report) return;
    const label = REPORTS.find((r) => r.type === active)?.label || "研读报告";
    const title = `${paperTitle(activePaper!)} · ${label}`;
    printReport(title, articleRef.current.innerHTML);
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
        <PaperOverview />
        <div className="grid gap-2 grid-cols-2 sm:grid-cols-3 lg:grid-cols-5">
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
                title={ready ? "点击查看" : "点击生成"}
                onClick={() => generate(r.type)}
                className={`group relative flex items-center gap-2.5 rounded-lg border px-3 py-2.5 text-left transition-colors duration-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sienna/40 disabled:cursor-not-allowed disabled:opacity-60 ${
                  isActive
                    ? "border-sienna/50 bg-sienna/[0.05] ring-1 ring-sienna/15"
                    : "border-border bg-card hover:border-sienna/40 hover:bg-accent/40"
                }`}
              >
                <span
                  className={`flex size-7 shrink-0 items-center justify-center rounded-md border transition-colors ${
                    isActive
                      ? "border-sienna/30 bg-sienna/10 text-sienna"
                      : "border-border bg-muted/50 text-muted-foreground group-hover:text-sienna"
                  }`}
                >
                  {isBusy ? <Loader2 className="size-3.5 animate-spin" /> : <Icon className="size-3.5" />}
                </span>
                <span className="min-w-0 flex-1">
                  <span className="flex items-center gap-1.5">
                    <span className="truncate font-serif text-sm font-semibold text-foreground">
                      {r.label}
                    </span>
                    {ready && (
                      <span
                        className="size-1.5 shrink-0 rounded-full bg-emerald-500"
                        title="就绪"
                      />
                    )}
                  </span>
                  <span className="block truncate text-xs text-muted-foreground">{r.desc}</span>
                </span>
                <ArrowRight
                  className={`size-3.5 shrink-0 transition-all group-hover:translate-x-0.5 ${
                    isActive ? "text-sienna" : "text-muted-foreground/50 group-hover:text-sienna"
                  } ${isBusy ? "opacity-0" : ""}`}
                />
              </button>
            );
          })}
        </div>

        <div className="rounded-xl border bg-card">
          <div className="p-6">
            {loading ? (
              <div className="space-y-4">
                {/* 小囊鼠多 agent 长任务,生成要一分多钟,实时执行计划填补等待:规划→检索→思考。 */}
                {run && run.steps.length > 0 && (
                  <ProcessTrace steps={run.steps} live={run.live} />
                )}
                <p className="flex items-center gap-2 text-sm text-muted-foreground">
                  <Loader2 className="size-4 animate-spin" />
                  小囊鼠正在研读论文、检索证据并撰写报告，约需一分钟…
                </p>
                <SkeletonLines lines={6} />
                <SkeletonLines lines={4} />
              </div>
            ) : run?.failed ? (
              <div className="space-y-3">
                <p className="text-sm text-destructive">报告生成失败，请重试。</p>
                {run.steps.length > 0 && <ProcessTrace steps={run.steps} live={false} />}
              </div>
            ) : report ? (
              <article className="animate-in fade-in-50 slide-in-from-bottom-2 duration-300">
                <header className="mb-4 flex items-center justify-between gap-2 border-b pb-3">
                  <div className="flex min-w-0 items-center gap-2">
                    <h3 className="truncate font-serif text-xl font-semibold">
                      {REPORTS.find((r) => r.type === active)?.label}
                    </h3>
                    {report.intent && (
                      <Badge variant="secondary" className="rounded-full font-normal">
                        {report.intent}
                      </Badge>
                    )}
                  </div>
                  <Button
                    variant="outline"
                    size="sm"
                    className="shrink-0 gap-1.5"
                    onClick={downloadPDF}
                  >
                    <Download className="size-3.5" />
                    下载 PDF
                  </Button>
                </header>
                {/* 生成过程的执行计划答后可折叠回看;打印只取下方 articleRef 的正文,不含此条。 */}
                {run && run.steps.length > 0 && (
                  <ProcessTrace steps={run.steps} live={false} />
                )}
                <div ref={articleRef}>
                  <Markdown figures={buildFigureMap(report.meta)}>{report.content}</Markdown>
                </div>
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
