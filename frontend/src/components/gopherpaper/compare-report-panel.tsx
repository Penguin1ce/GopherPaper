"use client";

import {
  AlertTriangle,
  BookOpenCheck,
  ChevronDown,
  Download,
  GitCompareArrows,
  History,
  Loader2,
  Search,
  ShieldCheck,
  Trash2,
  X,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { ScrollArea } from "@/components/ui/scroll-area";
import * as api from "@/lib/gopherpaper/api";
import { printReport } from "@/lib/gopherpaper/print";
import {
  sourceReferenceForLabel,
  sourceTagReaderHref,
} from "@/lib/gopherpaper/source-links";
import { useApp } from "@/lib/gopherpaper/store";
import type {
  CompareRun,
  Paper,
  PaperCompareReport,
  Reference,
} from "@/lib/gopherpaper/types";
import { formatTime, paperTitle } from "@/lib/gopherpaper/utils";
import { cn } from "@/lib/utils";
import { Empty } from "./app-ui";
import { Markdown } from "./markdown";
import { ProcessTrace } from "./process-trace";

const TABLE_HEADERS = [
  "论文",
  "研究问题",
  "方法路线",
  "数据集",
  "评价指标",
  "实验结论",
];

interface CompareReportsBoxProps {
  activeReportID?: number | null;
  latestReport?: PaperCompareReport | null;
  onOpenReport: (report: PaperCompareReport) => void;
  onDeleteReport?: (reportID: number) => void;
}

interface CompareReportContentProps {
  report: PaperCompareReport;
  papers: Paper[];
}

interface ComparePaperMeta {
  id?: string;
  title?: string;
  file_name?: string;
}

interface CompareTable {
  headers: string[];
  rows: string[][];
}

const PAPER_PALETTE = [
  {
    header: "border-t-sky-400",
    badge:
      "border-sky-300 bg-sky-50 text-sky-700 dark:border-sky-700 dark:bg-sky-950/50 dark:text-sky-300",
  },
  {
    header: "border-t-emerald-400",
    badge:
      "border-emerald-300 bg-emerald-50 text-emerald-700 dark:border-emerald-700 dark:bg-emerald-950/50 dark:text-emerald-300",
  },
  {
    header: "border-t-amber-400",
    badge:
      "border-amber-300 bg-amber-50 text-amber-800 dark:border-amber-700 dark:bg-amber-950/50 dark:text-amber-300",
  },
  {
    header: "border-t-rose-400",
    badge:
      "border-rose-300 bg-rose-50 text-rose-700 dark:border-rose-700 dark:bg-rose-950/50 dark:text-rose-300",
  },
  {
    header: "border-t-violet-400",
    badge:
      "border-violet-300 bg-violet-50 text-violet-700 dark:border-violet-700 dark:bg-violet-950/50 dark:text-violet-300",
  },
  {
    header: "border-t-cyan-400",
    badge:
      "border-cyan-300 bg-cyan-50 text-cyan-700 dark:border-cyan-700 dark:bg-cyan-950/50 dark:text-cyan-300",
  },
];

const COMPARE_PHASE_LABELS: Record<string, string> = {
  preparing: "准备论文",
  researching: "逐篇检索",
  planning: "制定检索计划",
  action: "检索正文",
  reasoning: "整理证据",
  replanning: "补充检索",
  writing: "对齐写作",
  reviewing: "交叉审校",
  completed: "生成完成",
  failed: "生成失败",
};

const COMPARE_LOADING_STEPS = [
  {
    phase: "preparing",
    text: "正在准备所选论文与对比范围。",
  },
];

export function CompareReportsBox({
  activeReportID,
  latestReport,
  onOpenReport,
  onDeleteReport,
}: CompareReportsBoxProps) {
  const { papers, toast } = useApp();
  const [open, setOpen] = useState(false);
  const [reports, setReports] = useState<PaperCompareReport[]>([]);
  const [query, setQuery] = useState("");
  const [loading, setLoading] = useState(false);
  const [deletingID, setDeletingID] = useState<number | null>(null);

  useEffect(() => {
    let alive = true;
    setLoading(true);
    api
      .listCompareReports()
      .then((items) => {
        if (alive) setReports(items ?? []);
      })
      .catch((err) => {
        if (alive) toast((err as Error)?.message || "查询对比报告失败", "error");
      })
      .finally(() => {
        if (alive) setLoading(false);
      });
    return () => {
      alive = false;
    };
  }, [toast]);

  useEffect(() => {
    if (!latestReport) return;
    setReports((prev) => [
      latestReport,
      ...prev.filter((report) => report.id !== latestReport.id),
    ]);
    setOpen(true);
  }, [latestReport]);

  const filteredReports = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return reports;
    return reports.filter((report) => {
      const names = comparePaperNames(report, papers).join(" ");
      return `${report.title} ${names}`.toLowerCase().includes(q);
    });
  }, [papers, query, reports]);

  const deleteReport = async (report: PaperCompareReport) => {
    if (deletingID) return;
    setDeletingID(report.id);
    try {
      await api.deleteCompareReport(report.id);
      setReports((prev) => prev.filter((item) => item.id !== report.id));
      onDeleteReport?.(report.id);
    } catch (err) {
      toast((err as Error)?.message || "删除对比报告失败", "error");
    } finally {
      setDeletingID(null);
    }
  };

  return (
    <section>
      <button
        type="button"
        className="flex w-full items-center justify-between gap-2 rounded-md px-2.5 py-1.5 text-left transition-colors hover:bg-accent/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50"
        onClick={() => setOpen((value) => !value)}
        aria-expanded={open}
      >
        <span className="flex min-w-0 items-center gap-1.5 text-xs font-semibold text-muted-foreground">
          <History className="size-3.5 shrink-0" />
          对比报告
        </span>
        <span className="flex shrink-0 items-center gap-1 text-muted-foreground/60">
          <span className="font-mono text-[11px]">{reports.length}</span>
          <ChevronDown
            className={cn(
              "size-3.5 transition-transform duration-200 ease-out",
              open && "rotate-180",
            )}
          />
        </span>
      </button>

      {open && (
        <div className="mt-1">
          <div className="relative mb-1.5">
            <Search className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
            <Input
              value={query}
              className="h-8 rounded-lg border-transparent bg-muted/60 px-8 shadow-none transition-colors hover:bg-muted/80 focus-visible:border-ring/40 focus-visible:bg-background focus-visible:ring-2 focus-visible:ring-ring/25 dark:bg-muted/40 dark:hover:bg-muted/50 dark:focus-visible:bg-background"
              placeholder="搜索报告或论文"
              onChange={(event) => setQuery(event.target.value)}
            />
            {query && (
              <button
                type="button"
                aria-label="清除搜索"
                className="absolute right-2 top-1/2 -translate-y-1/2 rounded-sm p-0.5 text-muted-foreground transition-colors hover:text-foreground"
                onClick={() => setQuery("")}
              >
                <X className="size-3.5" />
              </button>
            )}
          </div>

          <div className="max-h-56 space-y-px overflow-y-auto">
            {loading ? (
              <p className="flex items-center gap-2 rounded-md px-2.5 py-2 text-sm text-muted-foreground">
                <Loader2 className="size-4 animate-spin" />
                正在加载
              </p>
            ) : reports.length === 0 ? (
              <Empty title="还没有对比报告" text="生成后会保存在这里。" compact />
            ) : filteredReports.length === 0 ? (
              <Empty title="没有匹配报告" text="换一个关键词试试。" compact />
            ) : (
              filteredReports.map((report) => {
                const names = comparePaperNames(report, papers);
                return (
                  <div
                    key={report.id}
                    className={cn(
                      "group flex items-center gap-1 rounded-md px-2.5 py-1.5 transition-colors",
                      activeReportID === report.id
                        ? "bg-accent"
                        : "hover:bg-accent/50",
                    )}
                  >
                    <button
                      type="button"
                      className="min-w-0 flex-1 text-left focus-visible:outline-none"
                      onClick={() => onOpenReport(report)}
                    >
                      <span className="block truncate text-sm leading-6">
                        {report.title}
                      </span>
                      <span className="block truncate text-[11px] text-muted-foreground">
                        {formatTime(report.updated_at || report.created_at)}
                        {names.length > 0 && ` · ${names.join(" / ")}`}
                      </span>
                    </button>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon-sm"
                      className="text-muted-foreground/50 hover:text-destructive"
                      title="删除对比报告"
                      disabled={Boolean(deletingID)}
                      onClick={() => deleteReport(report)}
                    >
                      {deletingID === report.id ? (
                        <Loader2 className="size-3.5 animate-spin" />
                      ) : (
                        <Trash2 className="size-3.5" />
                      )}
                    </Button>
                  </div>
                );
              })
            )}
          </div>
        </div>
      )}
    </section>
  );
}

export function CompareGenerationPanel({
  papers,
  paperIDs,
  progress,
}: {
  papers: Paper[];
  paperIDs: string[];
  progress: CompareRun | null;
}) {
  const names = paperIDs.map((id) => {
    const paper = papers.find((item) => item.id === id);
    return paper ? paperTitle(paper) : id;
  });
  const steps =
    progress && progress.steps.length > 0
      ? progress.steps
      : COMPARE_LOADING_STEPS;

  return (
    <ScrollArea className="h-full">
      <div className="mx-auto flex min-h-full w-full max-w-5xl items-center p-5 md:p-8">
        <section className="w-full overflow-hidden rounded-lg border bg-card shadow-sm">
          <header className="flex items-start gap-3 border-b px-5 py-4">
            <span className="mt-0.5 flex size-9 shrink-0 items-center justify-center rounded-md bg-emerald-50 text-emerald-700 dark:bg-emerald-950/50 dark:text-emerald-300">
              <BookOpenCheck className="size-4.5" />
            </span>
            <div className="min-w-0">
              <h2 className="font-serif text-lg font-semibold">
                小囊鼠正在生成对比报告
              </h2>
              <p className="mt-1 text-sm leading-relaxed text-muted-foreground">
                正在逐篇读取正文证据，而不是只比较元数据。所需时间会随论文数量增加。
              </p>
            </div>
          </header>

          <div className="grid gap-0 lg:grid-cols-[minmax(0,1fr)_18rem]">
            <div className="space-y-5 p-5">
              <div className="flex items-center gap-2 text-xs text-muted-foreground">
                <span className="font-medium text-foreground">逐篇检索</span>
                <span aria-hidden>→</span>
                <span className="font-medium text-foreground">对齐写作</span>
                <span aria-hidden>→</span>
                <span className="font-medium text-foreground">交叉审校</span>
              </div>

              <ProcessTrace
                steps={steps}
                live={progress?.live ?? true}
                phaseLabels={COMPARE_PHASE_LABELS}
              />

              <p className="border-t pt-4 text-xs leading-relaxed text-muted-foreground">
                对比过程仅检索所选论文，并保留研究问题、方法、实验与结论的原文页码。
              </p>
            </div>

            <aside className="border-t bg-muted/20 p-5 lg:border-l lg:border-t-0">
              <div className="mb-3 flex items-center justify-between">
                <span className="text-sm font-medium">对比论文</span>
                <span className="font-mono text-xs text-muted-foreground">
                  {names.length}
                </span>
              </div>
              <ol className="space-y-2.5">
                {names.map((name, index) => (
                  <li key={`${paperIDs[index]}-${index}`} className="flex gap-2.5">
                    <span
                      className={cn(
                        "flex size-5 shrink-0 items-center justify-center rounded-full border font-mono text-[10px]",
                        paperPalette(index).badge,
                      )}
                    >
                      {paperLetter(index)}
                    </span>
                    <span className="min-w-0 text-xs leading-relaxed text-muted-foreground">
                      {name}
                    </span>
                  </li>
                ))}
              </ol>
            </aside>
          </div>
        </section>
      </div>
    </ScrollArea>
  );
}

export function CompareReportContent({
  report,
  papers,
}: CompareReportContentProps) {
  const articleRef = useRef<HTMLDivElement | null>(null);
  const comparedPapers = useMemo(
    () => comparePaperNames(report, papers),
    [report, papers],
  );
  const parsed = useMemo(() => parseCompareReport(report.content), [report]);
  const sources = useMemo(
    () => parseCompareSources(report.meta?.sources),
    [report.meta],
  );
  const metaPapers = useMemo(
    () => parseComparePaperMeta(report.meta?.compare_papers),
    [report.meta],
  );
  const allowedPaperIDs = useMemo(
    () => new Set(report.paper_ids),
    [report.paper_ids],
  );
  const formatWarnings = parseStringList(report.meta?.format_warnings);
  const resolveSourceLink = useCallback(
    (label: string) => {
      const ref = sourceReferenceForLabel(label, sources);
      let paperIndex = ref?.doc_id
        ? report.paper_ids.indexOf(ref.doc_id)
        : -1;
      if (paperIndex < 0) {
        paperIndex = metaPapers.findIndex((paper) => {
          const fileName = paper.file_name?.split(/[\\/]/).pop();
          return Boolean(fileName && label.includes(fileName));
        });
      }
      if (paperIndex < 0) paperIndex = 0;
      const fallbackPaperID = report.paper_ids[paperIndex] || "";
      const page = sourcePageFromLabel(label);
      return {
        href: sourceTagReaderHref(
          label,
          sources,
          fallbackPaperID,
          allowedPaperIDs,
        ),
        label: `原文${paperLetter(paperIndex)}${page ? ` · 第 ${page} 页` : ""}`,
        className: paperPalette(paperIndex).badge,
      };
    },
    [allowedPaperIDs, metaPapers, report.paper_ids, sources],
  );

  const downloadPDF = () => {
    if (!articleRef.current) return;
    printReport(
      `多论文对比报告 - ${report.title}`,
      articleRef.current,
      { mirrorStyles: true, landscape: true },
    );
  };

  return (
    <ScrollArea className="h-full">
      <div className="mx-auto w-full max-w-[1600px] p-4 md:p-6">
        <article ref={articleRef} data-compare-report>
          <header className="flex flex-col gap-4 border-b pb-5 md:flex-row md:items-start md:justify-between">
            <div className="min-w-0">
              <div className="mb-2 flex items-center gap-2 text-sm font-medium text-emerald-700 dark:text-emerald-300">
                <GitCompareArrows className="size-4" />
                小囊鼠多论文对比
              </div>
              <h2 className="font-serif text-2xl font-semibold leading-tight">
                {report.title}
              </h2>
              <p className="mt-2 text-sm text-muted-foreground">
                {comparedPapers.length} 篇论文 ·{" "}
                {formatTime(report.updated_at || report.created_at)}
              </p>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <span className="inline-flex h-8 items-center gap-1.5 rounded-md border bg-card px-2.5 text-xs text-muted-foreground">
                <ShieldCheck className="size-3.5" />
                小囊鼠三阶段审校
              </span>
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="gap-1.5"
                onClick={downloadPDF}
                data-print-hidden
              >
                <Download className="size-3.5" />
                下载 PDF
              </Button>
            </div>
          </header>

          <section className="grid gap-2 border-b py-4 sm:grid-cols-2 xl:grid-cols-3">
            {comparedPapers.map((name, index) => (
              <div
                key={`${name}-${index}`}
                className={cn(
                  "min-w-0 border-t-2 bg-muted/20 px-3 py-2.5",
                  paperPalette(index).header,
                )}
              >
                <div className="mb-1 font-mono text-[10px] text-muted-foreground">
                  PAPER {paperLetter(index)}
                </div>
                <div className="line-clamp-2 text-sm font-medium leading-snug">
                  {name}
                </div>
              </div>
            ))}
          </section>

          {formatWarnings.length > 0 && (
            <div className="mt-4 flex items-start gap-2 border-l-2 border-amber-400 bg-amber-50/60 px-3 py-2.5 text-sm text-amber-900 dark:bg-amber-950/20 dark:text-amber-200">
              <AlertTriangle className="mt-0.5 size-4 shrink-0" />
              <div>
                <div className="font-medium">报告格式需要留意</div>
                <div className="mt-1 text-xs leading-relaxed opacity-80">
                  {formatWarnings.join("；")}
                </div>
              </div>
            </div>
          )}

          {parsed.intro.trim() && (
            <section className="border-b py-5">
              <div className="mb-2 text-xs font-medium uppercase text-muted-foreground">
                总体判断
              </div>
              <Markdown sourceLink={resolveSourceLink}>{parsed.intro}</Markdown>
            </section>
          )}

          {parsed.table ? (
            <CompareMatrix
              table={parsed.table}
              sourceLink={resolveSourceLink}
            />
          ) : (
            <div className="py-5">
              <Markdown sourceLink={resolveSourceLink}>
                {normalizeCompareMarkdown(report.content)}
              </Markdown>
            </div>
          )}

          {parsed.analysis.trim() && (
            <section className="border-t py-6">
              <Markdown sourceLink={resolveSourceLink}>
                {parsed.analysis}
              </Markdown>
            </section>
          )}
        </article>
      </div>
    </ScrollArea>
  );
}

function CompareMatrix({
  table,
  sourceLink,
}: {
  table: CompareTable;
  sourceLink: (label: string) => {
    href?: string | null;
    label?: string;
    className?: string;
  } | null;
}) {
  const columns = TABLE_HEADERS.slice(1);
  return (
    <section className="py-6">
      <div className="mb-4 flex items-end justify-between gap-3">
        <div>
          <h3 className="font-serif text-lg font-semibold">横向对比矩阵</h3>
          <p className="mt-1 text-sm text-muted-foreground">
            横向滚动查看同一维度下各论文的证据与差异。
          </p>
        </div>
        <span className="shrink-0 font-mono text-xs text-muted-foreground">
          {table.rows.length} × {columns.length}
        </span>
      </div>
      <div className="overflow-x-auto rounded-lg border">
        <table className="w-max min-w-full border-collapse text-left">
          <thead>
            <tr className="border-b bg-muted/40">
              <th className="sticky left-0 z-20 w-36 min-w-36 border-r bg-muted px-3 py-3 text-xs font-medium text-muted-foreground">
                对比维度
              </th>
              {table.rows.map((row, rowIndex) => (
                <th
                  key={`paper-${rowIndex}`}
                  className={cn(
                    "min-w-64 max-w-80 border-r border-t-2 bg-card px-4 py-3 align-top last:border-r-0",
                    paperPalette(rowIndex).header,
                  )}
                >
                  <span className="line-clamp-2 text-sm font-semibold leading-snug">
                    {cleanCell(row[0] || `论文 ${paperLetter(rowIndex)}`)}
                  </span>
                  <span className="mt-1 block font-mono text-[10px] font-medium text-muted-foreground">
                    PAPER {paperLetter(rowIndex)}
                  </span>
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {columns.map((column) => {
              const valueIndex = table.headers.findIndex((header) =>
                header.includes(column),
              );
              if (valueIndex < 0) return null;
              return (
                <tr key={column} className="border-b last:border-b-0">
                  <th className="sticky left-0 z-10 border-r bg-background px-3 py-4 align-top text-sm font-medium">
                    {column}
                  </th>
                  {table.rows.map((row, rowIndex) => (
                    <td
                      key={`${column}-${rowIndex}`}
                      className="min-w-64 max-w-80 border-r bg-card/40 px-4 py-4 align-top last:border-r-0"
                    >
                      <CellList
                        value={row[valueIndex] || "未抽取"}
                        sourceLink={sourceLink}
                      />
                    </td>
                  ))}
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </section>
  );
}

function CellList({
  value,
  sourceLink,
}: {
  value: string;
  sourceLink: (label: string) => {
    href?: string | null;
    label?: string;
    className?: string;
  } | null;
}) {
  const parts = cellParts(value);
  if (parts.length <= 1) {
    return (
      <div className="text-sm leading-relaxed text-muted-foreground">
        <Markdown compact sourceLink={sourceLink}>
          {parts[0] || "未抽取"}
        </Markdown>
      </div>
    );
  }
  return (
    <ul className="space-y-1.5 text-sm leading-relaxed text-muted-foreground">
      {parts.map((part, index) => (
        <li key={`${part}-${index}`} className="flex gap-2">
          <span className="mt-2 size-1 shrink-0 rounded-full bg-sienna/60" />
          <div className="min-w-0">
            <Markdown compact sourceLink={sourceLink}>
              {part}
            </Markdown>
          </div>
        </li>
      ))}
    </ul>
  );
}

function comparePaperNames(report: PaperCompareReport, papers: Paper[]) {
  const knownPapers = new Map(
    papers.map((paper) => [paper.id, paperTitle(paper)]),
  );
  const metaPapers = parseComparePaperMeta(report.meta?.compare_papers);
  const metaNames = new Map(
    metaPapers
      .filter((item) => item.id)
      .map((item) => [
        item.id as string,
        item.title || item.file_name || (item.id as string),
      ]),
  );
  return report.paper_ids.map(
    (id) => metaNames.get(id) || knownPapers.get(id) || id,
  );
}

function parseComparePaperMeta(value: unknown): ComparePaperMeta[] {
  if (!Array.isArray(value)) return [];
  return value.filter((item): item is ComparePaperMeta => {
    return Boolean(item && typeof item === "object");
  });
}

function parseCompareReport(content: string): {
  table: CompareTable | null;
  intro: string;
  analysis: string;
} {
  const lines = content.split(/\r?\n/);
  const tableStart = lines.findIndex((line, index) => {
    if (!line.trim().startsWith("|")) return false;
    const next = lines[index + 1]?.trim() || "";
    return /^\|?\s*:?-{3,}:?\s*(\|\s*:?-{3,}:?\s*)+\|?$/.test(next);
  });
  if (tableStart < 0) {
    return {
      table: null,
      intro: "",
      analysis: normalizeCompareMarkdown(content),
    };
  }

  let tableEnd = tableStart;
  while (tableEnd < lines.length && lines[tableEnd].trim().startsWith("|")) {
    tableEnd += 1;
  }
  const tableLines = lines.slice(tableStart, tableEnd);
  const headers = splitTableRow(tableLines[0]);
  const rows = tableLines
    .slice(2)
    .map(splitTableRow)
    .filter((row) => row.length > 0);
  const intro = lines
    .slice(0, tableStart)
    .filter(
      (line) =>
        !/^#\s+多论文对比分析\s*$/.test(line.trim()) &&
        !/方法对比表/.test(line),
    )
    .join("\n");
  return {
    table: rows.length > 0 ? { headers, rows } : null,
    intro: normalizeCompareMarkdown(intro),
    analysis: normalizeCompareMarkdown(lines.slice(tableEnd).join("\n")),
  };
}

function splitTableRow(line: string): string[] {
  const trimmed = line.trim().replace(/^\|/, "").replace(/\|$/, "");
  return trimmed.split("|").map((cell) => cleanCell(cell));
}

function normalizeCompareMarkdown(value: string): string {
  return value.replace(/<br\s*\/?>/gi, "\n");
}

function cleanCell(value: string): string {
  return normalizeCompareMarkdown(value)
    .replace(/<[^>]+>/g, "")
    .replace(/&nbsp;/g, " ")
    .replace(/&amp;/g, "&")
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .replace(/\s+/g, " ")
    .trim();
}

function cellParts(value: string): string[] {
  const normalized = normalizeCompareMarkdown(value)
    .replace(/；\s*(?=\d+[.、])/g, "\n")
    .replace(/。\s*(?=\d+[.、])/g, "。\n")
    .replace(/\s*(?=\d+[.、]\s)/g, "\n");
  return normalized
    .split(/\n+/)
    .map((item) => cleanCell(item).replace(/^\d+[.、]\s*/, ""))
    .filter(Boolean);
}

function parseCompareSources(value: unknown): Reference[] {
  if (!Array.isArray(value)) return [];
  return value.filter(
    (item): item is Reference => Boolean(item && typeof item === "object"),
  );
}

function parseStringList(value: unknown): string[] {
  if (!Array.isArray(value)) return [];
  return value.filter(
    (item): item is string => typeof item === "string" && item.trim().length > 0,
  );
}

function paperLetter(index: number): string {
  return String.fromCharCode(65 + Math.max(0, index));
}

function paperPalette(index: number) {
  return PAPER_PALETTE[Math.max(0, index) % PAPER_PALETTE.length];
}

function sourcePageFromLabel(label: string): number | null {
  const match = label.match(/第\s*(\d+)\s*页|p\.?\s*(\d+)/i);
  const page = Number.parseInt(match?.[1] || match?.[2] || "", 10);
  return Number.isFinite(page) && page > 0 ? page : null;
}
