"use client";

import {
  BarChart3,
  BookOpenText,
  Download,
  Link2,
  type LucideIcon,
  Lightbulb,
  Loader2,
  Waypoints,
  Workflow,
} from "lucide-react";
import {
  type Dispatch,
  type SetStateAction,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";

import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import * as api from "@/lib/gopherpaper/api";
import { printReport } from "@/lib/gopherpaper/print";
import { sourceTagReaderHref } from "@/lib/gopherpaper/source-links";
import { useApp } from "@/lib/gopherpaper/store";
import type {
  ChatResponse,
  PaperFlow,
  PlanStep,
  Reference,
  ReportType,
} from "@/lib/gopherpaper/types";
import { paperTitle } from "@/lib/gopherpaper/utils";
import { cn } from "@/lib/utils";
import { Empty, SkeletonLines } from "./app-ui";
import { Markdown } from "./markdown";
import { PaperFlowCard } from "./paper-flow-card";
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

const REPORTS: { type: ReportType; label: string; icon: LucideIcon }[] = [
  { type: "quickread", label: "论文速读", icon: BookOpenText },
  { type: "method", label: "研究方法", icon: Workflow },
  { type: "result", label: "实验结果", icon: BarChart3 },
  { type: "innovation", label: "创新与不足", icon: Lightbulb },
  { type: "related", label: "相关研究", icon: Link2 },
];

const REPORT_LOADING_STEPS: PlanStep[] = [
  { phase: "preparing", text: "小囊鼠已接收生成任务，正在启动研读流水线。" },
];

type ReportMap = Partial<Record<ReportType, ChatResponse>>;
type ReportFlagMap = Partial<Record<ReportType, boolean>>;

export function ReportPanel() {
  const { activePaper, activePaperID, reportReady, reportProgress, beginReport, toast } =
    useApp();
  const [active, setActive] = useState<ReportType | null>(null);
  const [reports, setReports] = useState<ReportMap>({});
  const [fetching, setFetching] = useState<ReportFlagMap>({});
  const [awaiting, setAwaiting] = useState<ReportFlagMap>({});
  // 论文思路图独立于报告状态机:按需生成,后端持久化,用小云雀同款节点图渲染。
  const [flow, setFlow] = useState<PaperFlow | null>(null);
  const [flowLoading, setFlowLoading] = useState(false);
  const [flowError, setFlowError] = useState(false);
  const autoLoadedRef = useRef<string | null>(null);
  const articleRef = useRef<HTMLDivElement | null>(null);
  const mountedRef = useRef(true);
  // 当前论文 ID 的 ref 镜像:异步结果回来时据此校验仍是这篇论文才落地,避免切走后覆盖,
  // 同一篇内的多张卡片可以并行生成,但旧论文的异步响应不能写回新论文。
  const activePaperRef = useRef(activePaperID);
  useEffect(() => {
    activePaperRef.current = activePaperID;
  }, [activePaperID]);
  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);
  const readySet = useMemo(
    () => reportReady[activePaperID] || {},
    [reportReady, activePaperID],
  );
  // 当前查看/生成中报告的实时进度(执行计划/进行中/失败),由 report_progress 事件累积。
  const run = active ? reportProgress[activePaperID]?.[active] : undefined;
  const report = active ? (reports[active] ?? null) : null;
  const activeFetching = active ? Boolean(fetching[active]) : false;
  const activeAwaiting = active ? Boolean(awaiting[active] && !run?.failed) : false;
  const activeGenerating =
    !report && !run?.failed && (Boolean(run?.live) || activeAwaiting);
  const activeLoading = activeFetching || (!report && activeGenerating);
  const reportRefs = (report?.meta?.sources as Reference[] | undefined) ?? [];
  const activeReportLabel = active
    ? REPORTS.find((r) => r.type === active)?.label || "研读报告"
    : "研读报告";

  const setTypeFlag = useCallback(
    (setter: Dispatch<SetStateAction<ReportFlagMap>>, type: ReportType, value: boolean) => {
      setter((prev) => {
        if (Boolean(prev[type]) === value) return prev;
        const next = { ...prev };
        if (value) next[type] = true;
        else delete next[type];
        return next;
      });
    },
    [],
  );

  const fetchReport = useCallback(
    async (paper: string, type: ReportType) => {
      setTypeFlag(setFetching, type, true);
      try {
        const res = await api.generateReport(paper, type);
        if (!mountedRef.current || activePaperRef.current !== paper) return;
        setReports((prev) => ({ ...prev, [type]: res }));
        setTypeFlag(setAwaiting, type, false);
      } catch (err) {
        if (!mountedRef.current || activePaperRef.current !== paper) return;
        if (err instanceof api.ApiError && err.status === 202) {
          beginReport(paper, type);
          setTypeFlag(setAwaiting, type, true);
          return;
        }
        setTypeFlag(setAwaiting, type, false);
        toast((err as Error)?.message || "生成失败", "error");
      } finally {
        if (mountedRef.current && activePaperRef.current === paper) {
          setTypeFlag(setFetching, type, false);
        }
      }
    },
    [beginReport, setTypeFlag, toast],
  );

  useEffect(() => {
    setActive(null);
    setReports({});
    setFetching({});
    setAwaiting({});
    setFlow(null);
    setFlowLoading(false);
    setFlowError(false);
  }, [activePaperID]);

  const openReport = useCallback(
    (type: ReportType) => {
      if (!activePaperID) return;
      const paper = activePaperID;
      const reportRun = reportProgress[paper]?.[type];
      autoLoadedRef.current = paper;
      setActive(type);
      if (reports[type] || fetching[type] || (awaiting[type] && !reportRun?.failed)) {
        return;
      }
      // 先问后端:命中缓存直接展示;只有 202 才进入本地生成态,避免已生成报告被误标为生成中。
      void fetchReport(paper, type);
    },
    [activePaperID, awaiting, fetchReport, fetching, reportProgress, reports],
  );

  // 进入论文时,若已有就绪报告则自动展示第一篇,免用户点击;每篇只自动一次(autoLoadedRef 守门)。
  useEffect(() => {
    if (!activePaperID || active || autoLoadedRef.current === activePaperID) return;
    const firstReady = REPORTS.find((r) => readySet[r.type]);
    if (!firstReady) return;
    openReport(firstReady.type);
  }, [active, activePaperID, openReport, readySet]);

  // awaiting:多张卡可同时处于生成中;各自收到 report_ready 后独立拉取持久化缓存。
  // 旧 ready 标记可能还在,但后端返回 202 后代表本轮仍在生成;必须等 run 收尾后再拉缓存,
  // 否则会在 readySet=true + awaiting=true 之间形成紧密 POST 重试循环。
  useEffect(() => {
    if (!activePaperID) return;
    for (const { type } of REPORTS) {
      const reportRun = reportProgress[activePaperID]?.[type];
      if (awaiting[type] && reportRun?.failed) {
        setTypeFlag(setAwaiting, type, false);
        continue;
      }
      if (
        awaiting[type] &&
        readySet[type] &&
        !fetching[type] &&
        reportRun &&
        !reportRun.live &&
        !reportRun.failed
      ) {
        void fetchReport(activePaperID, type);
      }
    }
  }, [
    activePaperID,
    awaiting,
    fetchReport,
    fetching,
    readySet,
    reportProgress,
    setTypeFlag,
  ]);

  // 生成论文思路图:复用小云雀同款思路图链路。独立动作,不进报告状态机。
  const generateFlow = async () => {
    if (!activePaperID || flowLoading) return;
    const paper = activePaperID;
    setFlow(null);
    setFlowError(false);
    setFlowLoading(true);
    try {
      const res = await api.generatePaperFlow(paper);
      if (!mountedRef.current || activePaperRef.current !== paper) return;
      const nextFlow = res.meta?.flow as PaperFlow | undefined;
      if (!nextFlow) throw new Error("思路图数据为空");
      setFlow(nextFlow);
    } catch (err) {
      if (!mountedRef.current || activePaperRef.current !== paper) return;
      setFlowError(true);
      toast((err as Error)?.message || "思路图生成失败", "error");
    } finally {
      if (mountedRef.current && activePaperRef.current === paper) setFlowLoading(false);
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
      <div className="px-6 pb-6 pt-8 lg:p-8">
        <Empty title="尚未选择论文" text="在左侧选择一篇论文后生成研读报告。" />
      </div>
    );
  }

  return (
    <ScrollArea className="h-full">
      <div className="mx-auto flex min-h-full w-full max-w-5xl flex-col px-5 py-5 lg:px-7">
        <header className="shrink-0 border-b pb-4">
          <div className="flex items-start justify-between gap-4">
            <div className="min-w-0">
              <h2 className="font-serif text-lg font-semibold tracking-tight">
                研读报告
              </h2>
              <p className="mt-1 truncate text-sm text-muted-foreground">
                {paperTitle(activePaper)}
              </p>
            </div>
            <Button
              type="button"
              variant={flow || flowLoading ? "secondary" : "outline"}
              size="sm"
              disabled={flowLoading}
              onClick={generateFlow}
              className="shrink-0 gap-1.5"
            >
              {flowLoading ? (
                <Loader2 className="size-3.5 animate-spin" />
              ) : (
                <Waypoints className="size-3.5" />
              )}
              思路图
            </Button>
          </div>
          {activePaper.status !== "ready" && (
            <p className="mt-3 rounded-md bg-muted/45 px-3 py-2 text-xs text-muted-foreground">
              论文尚未完全就绪，当前状态：{activePaper.status}
            </p>
          )}
          <div
            className="mt-4 flex flex-wrap items-center gap-1.5"
            role="tablist"
            aria-label="报告类型"
          >
            {REPORTS.map((r) => {
              const Icon = r.icon;
              const isActive = active === r.type;
              const reportRun = reportProgress[activePaperID]?.[r.type];
              const ready = Boolean(
                reports[r.type] ||
                  (readySet[r.type] && !reportRun?.live && !reportRun?.failed),
              );
              const isFetching = Boolean(fetching[r.type]);
              const isAwaiting = Boolean(awaiting[r.type] && !reportRun?.failed);
              const isGenerating =
                !ready &&
                !reportRun?.failed &&
                (Boolean(reportRun?.live) || isAwaiting);
              const isBusy = isFetching || isGenerating;
              return (
                <button
                  key={r.type}
                  type="button"
                  disabled={isFetching}
                  role="tab"
                  aria-selected={isActive}
                  title={
                    reportRun?.failed
                      ? "生成失败，点击重试"
                      : isGenerating
                      ? "生成中，点击查看进度"
                      : ready
                        ? "点击查看"
                        : "点击生成"
                  }
                  onClick={() => openReport(r.type)}
                  className={cn(
                    "inline-flex h-8 items-center gap-1.5 rounded-md px-2.5 text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sienna/40 disabled:cursor-not-allowed disabled:opacity-60",
                    isActive
                      ? "bg-sienna/10 text-sienna ring-1 ring-sienna/20"
                      : "text-muted-foreground hover:bg-accent/60 hover:text-foreground",
                  )}
                >
                  {isBusy ? (
                    <Loader2 className="size-3.5 animate-spin" />
                  ) : (
                    <Icon className="size-3.5" />
                  )}
                  <span>{r.label}</span>
                  {ready && (
                    <span
                      className="size-1.5 rounded-full bg-emerald-500"
                      title="就绪"
                    />
                  )}
                </button>
              );
            })}
          </div>
        </header>

        {(flowLoading || flow || flowError) && (
          <section className="mt-4 rounded-lg border bg-muted/20 p-3">
            {flowLoading ? (
              <p className="flex items-center gap-2 text-sm text-muted-foreground">
                <Loader2 className="size-4 animate-spin" />
                正在构建论文思路图…
              </p>
            ) : flowError ? (
              <p className="text-sm text-destructive">思路图生成失败，请重试。</p>
            ) : flow ? (
              <div className="animate-in fade-in-50 slide-in-from-bottom-2 duration-300">
                <PaperFlowCard flow={flow} />
              </div>
            ) : null}
          </section>
        )}

        <section className="min-h-0 flex-1 py-5">
          {activeLoading ? (
            <div className="mx-auto max-w-2xl space-y-4">
              <p className="flex items-center gap-2 text-sm text-muted-foreground">
                <Loader2 className="size-4 animate-spin" />
                正在生成{activeReportLabel}，约需一分钟…
              </p>
              <ProcessTrace
                steps={
                  run && run.steps.length > 0
                    ? run.steps
                    : REPORT_LOADING_STEPS
                }
                live={run?.live ?? true}
              />
              <SkeletonLines lines={5} />
            </div>
          ) : run?.failed ? (
            <div className="mx-auto max-w-2xl space-y-3">
              <p className="text-sm text-destructive">报告生成失败，请重试。</p>
              {run.steps.length > 0 && (
                <ProcessTrace steps={run.steps} live={false} />
              )}
            </div>
          ) : report ? (
            <article className="mx-auto max-w-3xl animate-in fade-in-50 slide-in-from-bottom-2 duration-300">
              <header className="mb-5 flex items-center justify-between gap-3 border-b pb-3">
                <h3 className="min-w-0 truncate font-serif text-xl font-semibold">
                  {activeReportLabel}
                </h3>
                <Button
                  variant="ghost"
                  size="sm"
                  className="shrink-0 gap-1.5"
                  onClick={downloadPDF}
                >
                  <Download className="size-3.5" />
                  下载 PDF
                </Button>
              </header>
              <div ref={articleRef}>
                <Markdown
                  figures={buildFigureMap(report.meta)}
                  sourceHref={(label) =>
                    sourceTagReaderHref(label, reportRefs, activePaperID)
                  }
                >
                  {report.content}
                </Markdown>
              </div>
            </article>
          ) : (
            <div className="mx-auto flex min-h-[18rem] max-w-xl items-center justify-center">
              <Empty
                title="选择一种报告"
                text="点击上方标签查看或生成研读报告。"
                compact
              />
            </div>
          )}
        </section>
      </div>
    </ScrollArea>
  );
}
