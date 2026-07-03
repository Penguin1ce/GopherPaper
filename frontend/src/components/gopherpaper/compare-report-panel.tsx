"use client";

import {
  ChevronDown,
  Download,
  History,
  Loader2,
  Search,
  Trash2,
  X,
} from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { ScrollArea } from "@/components/ui/scroll-area";
import * as api from "@/lib/gopherpaper/api";
import { printReport } from "@/lib/gopherpaper/print";
import { useApp } from "@/lib/gopherpaper/store";
import type { Paper, PaperCompareReport } from "@/lib/gopherpaper/types";
import { formatTime, paperTitle } from "@/lib/gopherpaper/utils";
import { cn } from "@/lib/utils";
import { Empty } from "./app-ui";
import { Markdown } from "./markdown";

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
    <section className="overflow-hidden rounded-lg border bg-card shadow-sm">
      <button
        type="button"
        className="flex w-full items-center justify-between gap-3 px-3 py-2.5 text-left transition-colors hover:bg-accent/50"
        onClick={() => setOpen((value) => !value)}
        aria-expanded={open}
      >
        <span className="flex min-w-0 items-center gap-2">
          <History className="size-4 shrink-0 text-muted-foreground" />
          <span className="min-w-0">
            <span className="block text-sm font-medium">对比报告</span>
            <span className="block truncate text-xs text-muted-foreground">
              历史报告与检索
            </span>
          </span>
        </span>
        <span className="flex shrink-0 items-center gap-2">
          <span className="font-mono text-xs text-muted-foreground">
            {reports.length}
          </span>
          <ChevronDown
            className={cn(
              "size-4 text-muted-foreground transition-transform",
              open && "rotate-180",
            )}
          />
        </span>
      </button>

      {open && (
        <div className="border-t bg-background/60 p-2">
          <div className="relative mb-2">
            <Search className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
            <Input
              value={query}
              className="h-8 rounded-full bg-background px-8"
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

          <ScrollArea className="h-52 rounded-md border bg-background">
            <div className="space-y-1 p-2 pr-3">
              {loading ? (
                <p className="flex items-center gap-2 rounded-md px-2 py-2 text-sm text-muted-foreground">
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
                        "group flex items-center gap-2 rounded-md px-2 py-1.5 transition-colors hover:bg-accent/60",
                        activeReportID === report.id &&
                          "bg-sienna/[0.06] text-sienna",
                      )}
                    >
                      <button
                        type="button"
                        className="min-w-0 flex-1 text-left"
                        onClick={() => onOpenReport(report)}
                      >
                        <span className="block truncate text-sm font-medium">
                          {report.title}
                        </span>
                        <span className="mt-0.5 block truncate text-[11px] text-muted-foreground">
                          {names.join(" / ")}
                        </span>
                        <span className="mt-0.5 block text-[11px] text-muted-foreground">
                          {formatTime(report.updated_at || report.created_at)}
                        </span>
                      </button>
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon-sm"
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
          </ScrollArea>
        </div>
      )}
    </section>
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

  const downloadPDF = () => {
    if (!articleRef.current) return;
    printReport(`多论文对比报告 - ${report.title}`, articleRef.current.innerHTML);
  };

  return (
    <ScrollArea className="h-full">
      <div className="space-y-5 p-4">
        <div>
          <h2 className="font-serif text-lg font-semibold tracking-tight">
            多论文对比报告
          </h2>
          <p className="mt-1 text-sm text-muted-foreground">
            {formatTime(report.updated_at || report.created_at)}
          </p>
        </div>

        <article className="rounded-xl border bg-card p-6">
          <header className="mb-4 flex items-center justify-between gap-2 border-b pb-3">
            <div className="min-w-0">
              <h3 className="truncate font-serif text-xl font-semibold">
                {report.title}
              </h3>
              <p className="mt-1 text-sm text-muted-foreground">
                {comparedPapers.length} 篇论文 ·{" "}
                {formatTime(report.updated_at || report.created_at)}
              </p>
            </div>
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="shrink-0 gap-1.5"
              onClick={downloadPDF}
            >
              <Download className="size-3.5" />
              下载 PDF
            </Button>
          </header>

          <div ref={articleRef}>
            <section className="mb-5 rounded-lg border bg-muted/30 p-3">
              <div className="mb-2 text-sm font-medium">选中对比论文</div>
              <ol className="space-y-1 text-sm text-muted-foreground">
                {comparedPapers.map((name, index) => (
                  <li key={`${name}-${index}`} className="flex gap-2">
                    <span className="font-mono text-xs text-muted-foreground/70">
                      {index + 1}.
                    </span>
                    <span className="min-w-0 break-words">{name}</span>
                  </li>
                ))}
              </ol>
            </section>

            {parsed.table ? (
              <CompareMatrix table={parsed.table} />
            ) : (
              <Markdown>{normalizeCompareMarkdown(report.content)}</Markdown>
            )}

            {parsed.rest.trim() && (
              <div className="mt-5">
                <Markdown>{parsed.rest}</Markdown>
              </div>
            )}
          </div>
        </article>
      </div>
    </ScrollArea>
  );
}

function CompareMatrix({ table }: { table: CompareTable }) {
  const columns = TABLE_HEADERS.slice(1);
  return (
    <section className="space-y-4">
      <div>
        <h4 className="font-serif text-base font-semibold">方法对比矩阵</h4>
        <p className="mt-1 text-sm text-muted-foreground">
          按对比维度拆分长表格，便于横向扫描差异。
        </p>
      </div>
      <div className="grid gap-3">
        {columns.map((column) => {
          const index = table.headers.findIndex((header) =>
            header.includes(column),
          );
          if (index < 0) return null;
          return (
            <div key={column} className="rounded-lg border bg-background">
              <div className="border-b px-3 py-2 text-sm font-medium">
                {column}
              </div>
              <div className="grid gap-0 md:grid-cols-2">
                {table.rows.map((row, rowIndex) => (
                  <div
                    key={`${column}-${rowIndex}`}
                    className="min-w-0 border-b p-3 last:border-b-0 md:border-b-0 md:border-r md:last:border-r-0"
                  >
                    <div className="mb-2 line-clamp-2 text-sm font-semibold">
                      {cleanCell(row[0] || `论文 ${rowIndex + 1}`)}
                    </div>
                    <CellList value={row[index] || "未抽取"} />
                  </div>
                ))}
              </div>
            </div>
          );
        })}
      </div>
    </section>
  );
}

function CellList({ value }: { value: string }) {
  const parts = cellParts(value);
  if (parts.length <= 1) {
    return (
      <p className="text-sm leading-relaxed text-muted-foreground">
        <InlineStrong>{parts[0] || "未抽取"}</InlineStrong>
      </p>
    );
  }
  return (
    <ul className="space-y-1.5 text-sm leading-relaxed text-muted-foreground">
      {parts.map((part, index) => (
        <li key={`${part}-${index}`} className="flex gap-2">
          <span className="mt-2 size-1 shrink-0 rounded-full bg-sienna/60" />
          <span className="min-w-0">
            <InlineStrong>{part}</InlineStrong>
          </span>
        </li>
      ))}
    </ul>
  );
}

function InlineStrong({ children }: { children: string }) {
  const parts = children.split(/(\*\*[^*]+\*\*)/g).filter(Boolean);
  return (
    <>
      {parts.map((part, index) => {
        if (part.startsWith("**") && part.endsWith("**")) {
          return (
            <strong key={`${part}-${index}`} className="font-semibold text-foreground">
              {part.slice(2, -2)}
            </strong>
          );
        }
        return <span key={`${part}-${index}`}>{part}</span>;
      })}
    </>
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
  rest: string;
} {
  const lines = content.split(/\r?\n/);
  const tableStart = lines.findIndex((line, index) => {
    if (!line.trim().startsWith("|")) return false;
    const next = lines[index + 1]?.trim() || "";
    return /^\|?\s*:?-{3,}:?\s*(\|\s*:?-{3,}:?\s*)+\|?$/.test(next);
  });
  if (tableStart < 0) {
    return { table: null, rest: normalizeCompareMarkdown(content) };
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
  const before = lines
    .slice(0, tableStart)
    .filter((line) => !/方法对比表/.test(line))
    .join("\n");
  const after = lines.slice(tableEnd).join("\n");
  return {
    table: rows.length > 0 ? { headers, rows } : null,
    rest: normalizeCompareMarkdown([before, after].filter(Boolean).join("\n\n")),
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
