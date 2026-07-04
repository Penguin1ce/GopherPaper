"use client";

import {
  ArrowLeft,
  BarChart3,
  CheckSquare2,
  FileUp,
  ListFilter,
  Loader2,
  Search,
  Square,
  X,
} from "lucide-react";
import Link from "next/link";
import { useEffect, useMemo, useRef, useState } from "react";

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
  CompareGenerationPanel,
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
          "inline-flex h-7 shrink-0 items-center gap-1 rounded-md px-2 text-xs font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50",
          activeKw.length > 0
            ? "bg-sienna/10 text-sienna hover:bg-sienna/15"
            : "text-muted-foreground hover:bg-accent/60 hover:text-foreground",
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
  const {
    authed,
    papers,
    activePaperID,
    selectPaper,
    refreshPapers,
    compareProgress,
    beginCompare,
    toast,
  } = useApp();
  const [query, setQuery] = useState("");
  const [activeKw, setActiveKw] = useState<string[]>([]);
  const [compareMode, setCompareMode] = useState(false);
  const [compareIDs, setCompareIDs] = useState<string[]>([]);
  const [compareLoading, setCompareLoading] = useState(false);
  const [latestCompareReport, setLatestCompareReport] =
    useState<PaperCompareReport | null>(null);
  const [activeCompareReport, setActiveCompareReport] =
    useState<PaperCompareReport | null>(null);
  const routedPaperRef = useRef("");
  const routedCompareRef = useRef("");

  useEffect(() => {
    const paperID = new URLSearchParams(window.location.search)
      .get("paper_id")
      ?.trim();
    if (
      !paperID ||
      routedPaperRef.current === paperID ||
      activePaperID === paperID ||
      !papers.some((paper) => paper.id === paperID)
    ) {
      return;
    }
    routedPaperRef.current = paperID;
    setActiveCompareReport(null);
    selectPaper(paperID);
  }, [activePaperID, papers, selectPaper]);

  useEffect(() => {
    const raw = new URLSearchParams(window.location.search)
      .get("compare_ids")
      ?.trim();
    if (!raw) return;
    const requestedIDs = [...new Set(raw.split(",").map((id) => id.trim()))]
      .filter(Boolean)
      .slice(0, MAX_COMPARE_PAPERS);
    if (requestedIDs.length < 2) return;
    const routeKey = requestedIDs.join(",");
    if (routedCompareRef.current === routeKey) return;
    if (!requestedIDs.every((id) => papers.some((paper) => paper.id === id))) {
      return;
    }
    routedCompareRef.current = routeKey;
    setActiveCompareReport(null);
    setCompareMode(true);
    setCompareIDs(requestedIDs);
  }, [papers]);

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
  const selectedComparePapers = useMemo(
    () => papers.filter((paper) => compareIDs.includes(paper.id)),
    [compareIDs, papers],
  );

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
    const selectedIDs = [...compareIDs];
    setCompareLoading(true);
    setActiveCompareReport(null);
    beginCompare(selectedIDs);
    try {
      const report = await api.comparePapers(selectedIDs);
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
        <div className="flex items-center gap-2 border-b px-3 py-3">
          <Link
            href="/"
            className={buttonVariants({ variant: "ghost", size: "icon-sm" })}
            title="返回工作台"
            aria-label="返回工作台"
          >
            <ArrowLeft className="size-4" />
          </Link>
          <AgentIntro kind="reports" />
        </div>

        <ScrollArea className="min-h-0 flex-1">
          <div className="space-y-4 px-2.5 pb-4 pt-2">
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

            <section>
              <div className="flex items-center justify-between gap-2 px-2.5 pb-1.5">
                <div className="flex min-w-0 items-baseline gap-1.5">
                  <h3 className="text-xs font-semibold text-muted-foreground">
                    论文库
                  </h3>
                  {papers.length > 0 && (
                    <span className="font-mono text-[11px] text-muted-foreground/60">
                      {papers.length}
                    </span>
                  )}
                </div>
                {papers.length > 0 && (
                  <div className="flex shrink-0 items-center gap-0.5">
                    <button
                      type="button"
                      aria-pressed={compareMode}
                      title="选择多篇论文生成对比报告"
                      onClick={toggleCompareMode}
                      className={cn(
                        "inline-flex h-7 items-center gap-1 rounded-md px-2 text-xs font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50",
                        compareMode
                          ? "bg-sienna/10 text-sienna"
                          : "text-muted-foreground hover:bg-accent/60 hover:text-foreground",
                      )}
                    >
                      <BarChart3 className="size-3.5" />
                      对比分析
                    </button>
                    <KeywordFilter
                      keywords={keywords}
                      activeKw={activeKw}
                      onToggle={toggleKw}
                      onClear={() => setActiveKw([])}
                    />
                  </div>
                )}
              </div>

              {papers.length > 0 && (
              <form
                className="relative mb-1.5"
                onSubmit={(event) => {
                  event.preventDefault();
                  refreshPapers(query);
                }}
              >
                <Search className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
                <Input
                  value={query}
                  className="h-8 rounded-lg border-transparent bg-muted/60 px-8 shadow-none transition-colors hover:bg-muted/80 focus-visible:border-ring/40 focus-visible:bg-background focus-visible:ring-2 focus-visible:ring-ring/25 dark:bg-muted/40 dark:hover:bg-muted/50 dark:focus-visible:bg-background"
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
              )}

              {compareMode && (
                <div className="mb-1.5 flex items-center justify-between gap-2 rounded-md bg-sienna/[0.07] px-2.5 py-1.5">
                  <span className="text-xs text-sienna">
                    已选 {compareIDs.length} 篇论文
                  </span>
                  <div className="flex items-center gap-1">
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
                      {compareLoading ? "小囊鼠分析中" : "生成报告"}
                    </Button>
                  </div>
                </div>
              )}

              <div className="space-y-px">
                {papers.length === 0 ? (
                  <div className="flex flex-col items-center rounded-lg border border-dashed bg-muted/30 px-4 py-6 text-center">
                    <span className="flex size-9 items-center justify-center rounded-lg bg-sienna/10 text-sienna">
                      <FileUp className="size-4" />
                    </span>
                    <h4 className="mt-3 text-sm font-medium">还没有论文</h4>
                    <p className="mt-1 text-xs leading-5 text-muted-foreground">
                      在小文鸮工作台上传 PDF，解析完成后
                      <br />
                      小囊鼠就能生成研读报告。
                    </p>
                    <Link
                      href="/"
                      className={cn(
                        buttonVariants({ variant: "outline", size: "sm" }),
                        "mt-3",
                      )}
                    >
                      去工作台上传
                    </Link>
                  </div>
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
                          "flex w-full cursor-pointer select-none items-start gap-2 rounded-md px-2.5 py-1.5 text-left transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50",
                          active ? "bg-accent" : "hover:bg-accent/50",
                          selectedForCompare && "bg-sienna/[0.08]",
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
                        {compareMode && (
                          <span className="mt-0.5 inline-flex size-4 shrink-0 items-center justify-center">
                            {selectedForCompare ? (
                              <CheckSquare2 className="size-4 text-sienna" />
                            ) : (
                              <Square className="size-4 text-muted-foreground/60" />
                            )}
                          </span>
                        )}
                        <span className="min-w-0 flex-1">
                          <span className="block truncate text-sm leading-6">
                            {paperTitle(paper)}
                          </span>
                          {paper.status !== "ready" && (
                            <span className="block text-[11px] text-muted-foreground">
                              {paper.status}
                            </span>
                          )}
                        </span>
                      </div>
                    );
                  })
                )}
              </div>
            </section>
          </div>
        </ScrollArea>
      </WorkspacePanel>

      <WorkspacePanel className="flex min-w-0 flex-1 flex-col">
        <section className="min-h-0 flex-1 overflow-hidden">
          {compareLoading ? (
            <CompareGenerationPanel
              papers={selectedComparePapers}
              paperIDs={compareIDs}
              progress={compareProgress}
            />
          ) : activeCompareReport ? (
            <CompareReportContent
              report={activeCompareReport}
              papers={papers}
            />
          ) : papers.length === 0 ? (
            <div className="flex h-full items-center justify-center p-8">
              <div className="flex max-w-md flex-col items-center text-center">
                <span className="flex size-12 items-center justify-center rounded-xl bg-sienna/10 text-sienna">
                  <FileUp className="size-5" />
                </span>
                <h2 className="mt-4 font-serif text-lg font-semibold tracking-tight">
                  先上传一篇论文
                </h2>
                <p className="mt-2 text-sm leading-6 text-muted-foreground">
                  小囊鼠基于已解析的论文生成速读、方法、结果等研读报告。
                  论文的上传入口在小文鸮工作台，上传后会自动解析入库，
                  完成后回到这里即可生成报告。
                </p>
                <Link
                  href="/"
                  className={cn(buttonVariants({ size: "sm" }), "mt-5")}
                >
                  <FileUp className="size-3.5" />
                  去工作台上传
                </Link>
              </div>
            </div>
          ) : (
            <ReportPanel />
          )}
        </section>
      </WorkspacePanel>
    </WorkspaceFrame>
  );
}
