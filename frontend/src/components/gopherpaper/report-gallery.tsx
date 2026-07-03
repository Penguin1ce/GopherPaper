"use client";

import {
  ArrowLeft,
  BarChart3,
  CheckSquare2,
  FileText,
  ListFilter,
  Loader2,
  Search,
  Square,
  X,
} from "lucide-react";
import Link from "next/link";
import { useMemo, useState } from "react";

import { Button, buttonVariants } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Popover,
  PopoverContent,
  PopoverDescription,
  PopoverTitle,
  PopoverTrigger,
} from "@/components/ui/popover";
import { ScrollArea } from "@/components/ui/scroll-area";
import {
  CompareReportContent,
  CompareReportsBox,
} from "@/components/gopherpaper/compare-report-panel";
import * as api from "@/lib/gopherpaper/api";
import { useApp } from "@/lib/gopherpaper/store";
import type { PaperCompareReport } from "@/lib/gopherpaper/types";
import { paperTitle } from "@/lib/gopherpaper/utils";
import { cn } from "@/lib/utils";
import { Empty } from "./app-ui";
import { AgentIntro } from "./agent-intro";
import { ReportPanel } from "./report-panel";
import { WorkspaceFrame, WorkspacePanel } from "./workspace-frame";

const MAX_COMPARE_PAPERS = 6;

interface KeywordItem {
  name: string;
  count: number;
}

function KeywordChip({
  item,
  active,
  onClick,
}: {
  item: KeywordItem;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      aria-pressed={active}
      title={item.name}
      onClick={onClick}
      className={cn(
        "inline-flex h-7 max-w-full min-w-0 items-center gap-1 rounded-full border px-2.5 text-xs transition-colors focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none",
        active
          ? "border-sienna/40 bg-sienna/10 text-sienna"
          : "border-border bg-card text-muted-foreground hover:bg-accent/60 hover:text-foreground",
      )}
    >
      <span className="min-w-0 truncate">{item.name}</span>
      <span
        className={cn(
          "shrink-0 font-mono text-[10px]",
          active ? "text-sienna/70" : "text-muted-foreground/60",
        )}
      >
        {item.count}
      </span>
    </button>
  );
}

function KeywordFilter({
  keywords,
  activeKw,
  onToggle,
  onClear,
}: {
  keywords: KeywordItem[];
  activeKw: string[];
  onToggle: (name: string) => void;
  onClear: () => void;
}) {
  if (keywords.length === 0) return null;

  return (
    <Popover>
      <PopoverTrigger
        className={cn(
          "inline-flex h-8 shrink-0 items-center gap-1 rounded-full border px-2.5 text-xs font-medium transition-colors focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none",
          activeKw.length > 0
            ? "border-sienna/40 bg-sienna/10 text-sienna hover:bg-sienna/15"
            : "border-border bg-card text-muted-foreground hover:bg-accent/60 hover:text-foreground",
        )}
      >
        <ListFilter className="size-3.5" />
        {activeKw.length > 0
          ? `筛选 ${activeKw.length}`
          : `关键词 ${keywords.length}`}
      </PopoverTrigger>
      <PopoverContent className="w-80 space-y-3 p-3" align="start">
        <div className="flex items-start justify-between gap-3">
          <div className="space-y-1">
            <PopoverTitle>关键词筛选</PopoverTitle>
            <PopoverDescription>{keywords.length} 个关键词</PopoverDescription>
          </div>
          {activeKw.length > 0 && (
            <Button type="button" variant="ghost" size="xs" onClick={onClear}>
              清除
            </Button>
          )}
        </div>
        <div className="max-h-64 overflow-y-auto pr-1">
          <div className="flex flex-wrap gap-1.5">
            {keywords.map((item) => (
              <KeywordChip
                key={item.name}
                item={item}
                active={activeKw.includes(item.name)}
                onClick={() => onToggle(item.name)}
              />
            ))}
          </div>
        </div>
      </PopoverContent>
    </Popover>
  );
}

export function ReportGallery() {
  const { authed, papers, activePaperID, selectPaper, refreshPapers, toast } =
    useApp();
  const [query, setQuery] = useState("");
  const [activeKw, setActiveKw] = useState<string[]>([]);
  const [compareMode, setCompareMode] = useState(false);
  const [compareIDs, setCompareIDs] = useState<string[]>([]);
  const [compareLoading, setCompareLoading] = useState(false);
  const [latestCompareReport, setLatestCompareReport] =
    useState<PaperCompareReport | null>(null);
  const [activeCompareReport, setActiveCompareReport] =
    useState<PaperCompareReport | null>(null);

  const keywords = useMemo(() => {
    const counts = new Map<string, number>();
    for (const paper of papers) {
      for (const keyword of paper.keywords ?? []) {
        counts.set(keyword, (counts.get(keyword) ?? 0) + 1);
      }
    }
    return [...counts.entries()]
      .sort((a, b) => b[1] - a[1])
      .map(([name, count]) => ({ name, count }));
  }, [papers]);

  const shownPapers = useMemo(() => {
    if (activeKw.length === 0) return papers;
    return papers.filter((paper) =>
      paper.keywords?.some((keyword) => activeKw.includes(keyword)),
    );
  }, [papers, activeKw]);

  const toggleKw = (name: string) =>
    setActiveKw((prev) =>
      prev.includes(name)
        ? prev.filter((keyword) => keyword !== name)
        : [...prev, name],
    );

  const toggleCompareMode = () => {
    setCompareMode((value) => {
      if (value) setCompareIDs([]);
      return !value;
    });
  };

  const toggleComparePaper = (id: string) => {
    setCompareIDs((prev) => {
      if (prev.includes(id)) return prev.filter((item) => item !== id);
      if (prev.length >= MAX_COMPARE_PAPERS) {
        toast(`单次最多选择 ${MAX_COMPARE_PAPERS} 篇论文`, "error");
        return prev;
      }
      return [...prev, id];
    });
  };

  const generateCompare = async () => {
    if (compareIDs.length < 2 || compareLoading) return;
    setCompareLoading(true);
    try {
      const report = await api.comparePapers(compareIDs);
      setLatestCompareReport(report);
      setActiveCompareReport(report);
      setCompareMode(false);
      setCompareIDs([]);
    } catch (err) {
      toast((err as Error)?.message || "生成对比报告失败", "error");
    } finally {
      setCompareLoading(false);
    }
  };

  if (!authed) {
    return (
      <main className="flex h-dvh items-center justify-center bg-muted/50 p-6">
        <div className="text-center">
          <Empty title="请先登录" text="登录后即可查看与生成研读报告。" />
          <Link
            href="/"
            className={cn(buttonVariants({ variant: "outline" }), "mt-4")}
          >
            返回首页
          </Link>
        </div>
      </main>
    );
  }

  return (
    <WorkspaceFrame>
      <WorkspacePanel as="aside" className="hidden w-96 flex-col lg:flex">
        <div className="space-y-3 border-b p-4">
          <div className="flex items-center gap-3">
            <Link
              href="/"
              className={buttonVariants({ variant: "ghost", size: "icon" })}
              title="返回工作台"
              aria-label="返回工作台"
            >
              <ArrowLeft className="size-4" />
            </Link>
            <AgentIntro kind="reports" />
          </div>
        </div>

        <ScrollArea className="min-h-0 flex-1">
          <div className="space-y-3 px-3 pb-3 pt-3">
            <CompareReportsBox
              activeReportID={activeCompareReport?.id}
              latestReport={latestCompareReport}
              onOpenReport={setActiveCompareReport}
              onDeleteReport={(reportID) => {
                setActiveCompareReport((current) =>
                  current?.id === reportID ? null : current,
                );
              }}
            />

            <section className="overflow-hidden rounded-lg border bg-background">
              <div className="p-3">
            <div className="flex items-center justify-between gap-2 px-1 pb-2">
              <div className="flex min-w-0 items-baseline gap-2">
                <div className="text-sm font-medium">论文库</div>
                <div className="font-mono text-xs text-muted-foreground">
                  {papers.length > 0 ? `${papers.length}` : "等待上传"}
                </div>
              </div>
              <div className="flex shrink-0 items-center gap-1">
                <Button
                  type="button"
                  variant={compareMode ? "secondary" : "ghost"}
                  size="sm"
                  onClick={toggleCompareMode}
                  title="对比分析"
                >
                  <BarChart3 className="size-3.5" />
                  对比分析
                </Button>
                <KeywordFilter
                  keywords={keywords}
                  activeKw={activeKw}
                  onToggle={toggleKw}
                  onClear={() => setActiveKw([])}
                />
              </div>
            </div>

            <form
              className="relative mb-2"
              onSubmit={(event) => {
                event.preventDefault();
                refreshPapers(query);
              }}
            >
              <Search className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
              <Input
                value={query}
                className="h-8 rounded-full bg-card px-8"
                placeholder="搜索标题、关键词、作者"
                onChange={(event) => setQuery(event.target.value)}
              />
              {query && (
                <button
                  type="button"
                  aria-label="清除搜索"
                  className="absolute right-2 top-1/2 -translate-y-1/2 rounded-sm p-0.5 text-muted-foreground transition-colors hover:text-foreground"
                  onClick={() => {
                    setQuery("");
                    refreshPapers();
                  }}
                >
                  <X className="size-3.5" />
                </button>
              )}
            </form>

            {compareMode && (
              <div className="mb-2 flex items-center justify-between gap-2 rounded-md border bg-card px-3 py-2">
                <span className="text-xs text-muted-foreground">
                  已选择 {compareIDs.length} 篇论文
                </span>
                <div className="flex items-center gap-1.5">
                  <Button
                    type="button"
                    variant="ghost"
                    size="xs"
                    disabled={compareIDs.length === 0 || compareLoading}
                    onClick={() => setCompareIDs([])}
                  >
                    清空
                  </Button>
                  <Button
                    type="button"
                    size="xs"
                    disabled={compareIDs.length < 2 || compareLoading}
                    onClick={generateCompare}
                  >
                    {compareLoading && (
                      <Loader2 className="size-3 animate-spin" />
                    )}
                    生成报告
                  </Button>
                </div>
              </div>
            )}

            <div className="space-y-1">
              {papers.length === 0 ? (
                <Empty
                  title="还没有论文"
                  text="先在工作台上传并解析论文。"
                  compact
                />
              ) : shownPapers.length === 0 ? (
                <Empty
                  title="没有匹配的论文"
                  text="换一个关键词或清除筛选。"
                  compact
                />
              ) : (
                shownPapers.map((paper) => {
                  const selectedForCompare = compareIDs.includes(paper.id);
                  const active = paper.id === activePaperID && !activeCompareReport;
                  return (
                    <div
                      key={paper.id}
                      role="button"
                      tabIndex={0}
                      aria-label={`${compareMode ? "选择对比" : "选择论文"} ${paperTitle(paper)}`}
                      className={cn(
                        "relative flex w-full cursor-pointer select-none items-start gap-2 rounded-md px-2.5 py-2 text-left transition-colors focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none",
                        active
                          ? "bg-card shadow-sm before:absolute before:inset-y-1.5 before:left-0 before:w-0.5 before:rounded-full before:bg-sienna"
                          : "hover:bg-accent/60",
                        selectedForCompare && "ring-1 ring-sienna/35",
                      )}
                      onClick={() => {
                        if (compareMode) {
                          toggleComparePaper(paper.id);
                          return;
                        }
                        setActiveCompareReport(null);
                        selectPaper(paper.id);
                      }}
                      onKeyDown={(event) => {
                        if (event.key === "Enter" || event.key === " ") {
                          event.preventDefault();
                          if (compareMode) {
                            toggleComparePaper(paper.id);
                            return;
                          }
                          setActiveCompareReport(null);
                          selectPaper(paper.id);
                        }
                      }}
                    >
                      {compareMode ? (
                        <span className="relative z-10 mt-0.5 inline-flex size-5 shrink-0 items-center justify-center rounded-md text-sienna">
                          {selectedForCompare ? (
                            <CheckSquare2 className="size-4" />
                          ) : (
                            <Square className="size-4" />
                          )}
                        </span>
                      ) : (
                        <FileText
                          className={cn(
                            "relative z-10 mt-0.5 size-3.5 shrink-0",
                            active ? "text-sienna" : "text-muted-foreground",
                          )}
                        />
                      )}
                      <span className="pointer-events-none relative z-10 min-w-0 flex-1">
                        <span className="block truncate text-sm font-medium">
                          {paperTitle(paper)}
                        </span>
                        {paper.status !== "ready" && (
                          <span className="text-xs text-muted-foreground">
                            {paper.status}
                          </span>
                        )}
                      </span>
                    </div>
                  );
                })
              )}
            </div>
              </div>
            </section>
          </div>
        </ScrollArea>
      </WorkspacePanel>

      <WorkspacePanel className="flex min-w-0 flex-1 flex-col">
        <section className="min-h-0 flex-1 overflow-hidden">
          {activeCompareReport ? (
            <CompareReportContent
              report={activeCompareReport}
              papers={papers}
            />
          ) : (
            <ReportPanel />
          )}
        </section>
      </WorkspacePanel>
    </WorkspaceFrame>
  );
}
