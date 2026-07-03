"use client";

import {
  AlertTriangle,
  FileUp,
  ListFilter,
  Loader2,
  MessagesSquare,
  MoreHorizontal,
  PanelLeftOpen,
  RefreshCw,
  RotateCcw,
  Search,
  Trash2,
  X,
} from "lucide-react";
import { motion, useReducedMotion } from "motion/react";
import { useMemo, useRef, useState } from "react";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import {
  Popover,
  PopoverContent,
  PopoverDescription,
  PopoverTitle,
  PopoverTrigger,
} from "@/components/ui/popover";
import { ScrollArea } from "@/components/ui/scroll-area";
import { cn } from "@/lib/utils";
import { useApp } from "@/lib/gopherpaper/store";
import type { Paper, Session } from "@/lib/gopherpaper/types";
import {
  formatSize,
  formatTime,
  isSettled,
  paperTitle,
  sessionsForPaper,
  sessionTitle,
  statusLabel,
  statusTone,
} from "@/lib/gopherpaper/utils";
import { Empty, useGuard } from "./app-ui";

const PARSE_TIMELINE = [
  { status: "uploaded", label: "上传" },
  { status: "parsing", label: "解析" },
  { status: "extracted", label: "抽取" },
  { status: "indexed", label: "索引" },
  { status: "ready", label: "完成" },
] as const;

function timelineIndex(status: Paper["status"]): number {
  if (status === "failed") return 0;
  return Math.max(
    0,
    PARSE_TIMELINE.findIndex((step) => step.status === status),
  );
}

function currentPhaseProgress(paper: Paper): number {
  if (paper.status === "ready") return 1;
  if (paper.status === "failed") return 0;
  if (paper.status === "uploaded") return 0.12;
  if (paper.status === "parsing") {
    return Math.min(0.96, Math.max(0.08, (paper.parse_progress ?? 16) / 100));
  }
  return 0.52;
}

function pipelineProgress(paper: Paper): number {
  if (paper.status === "failed") return 0;
  if (paper.status === "ready") return 100;
  const current = timelineIndex(paper.status);
  const last = PARSE_TIMELINE.length - 1;
  // 横线表达当前流水推进,细节仍由后端状态文案说明。
  return Math.min(
    100,
    Math.max(0, ((current + currentPhaseProgress(paper)) / last) * 100),
  );
}

function parseProgressText(paper: Paper): string {
  if (paper.status !== "parsing") return "";
  if ((paper.parsed_pages ?? 0) > 0 && (paper.total_pages ?? 0) > 0) {
    return `已解析 ${paper.parsed_pages}/${paper.total_pages} 页`;
  }
  if ((paper.parse_progress ?? 0) > 0) {
    return `解析进度 ${paper.parse_progress}%`;
  }
  return "";
}

function statusDetailText(paper: Paper): string {
  if (paper.status_detail) return paper.status_detail;
  if (paper.status === "parsing") {
    return parseProgressText(paper) || "MinerU 正在解析 PDF 页面";
  }
  if (paper.status === "extracted") {
    return "抽取标题、摘要、作者等结构化信息";
  }
  if (paper.status === "indexed") {
    return "构建知识片段并写入向量索引";
  }
  if (paper.status === "uploaded") {
    return "等待解析任务启动";
  }
  return "";
}

function InlinePaperStatus({ status }: { status: Paper["status"] }) {
  const tone = statusTone(status);
  return (
    <span
      className={cn(
        "inline-flex shrink-0 items-center gap-1.5 whitespace-nowrap text-[11px] font-medium",
        tone === "failed" && "text-destructive",
        tone === "ready" && "text-muted-foreground",
        tone === "working" && "text-amber-700 dark:text-amber-300",
      )}
    >
      <span
        className={cn(
          "size-1.5 rounded-full bg-current",
          tone === "working" && "animate-pulse",
        )}
        aria-hidden
      />
      {statusLabel(status)}
    </span>
  );
}

function PipelineStepDot({
  active,
  done,
}: {
  active: boolean;
  done: boolean;
}) {
  return (
    <span
      className={cn(
        "relative z-10 flex size-5 items-center justify-center rounded-full border bg-card transition-[background-color,border-color,box-shadow]",
        done && "border-sienna bg-card",
        active &&
          "border-sienna bg-accent shadow-[0_0_0_3px_color-mix(in_oklch,var(--sienna)_10%,transparent)]",
        !done && !active && "border-border",
      )}
    >
      <span
        className={cn(
          "rounded-full transition-[width,height,background-color,opacity]",
          done
            ? "size-1.5 bg-sienna"
            : active
              ? "size-2 bg-sienna"
              : "size-1.5 bg-muted-foreground/35 opacity-70",
        )}
      />
    </span>
  );
}

function ParsePipeline({ paper }: { paper: Paper }) {
  const current = timelineIndex(paper.status);
  const lineProgress = pipelineProgress(paper);
  const reduceMotion = useReducedMotion();
  const flowing = paper.status !== "failed" && paper.status !== "ready";

  return (
    <div
      className="mt-3"
      role="progressbar"
      aria-label="论文解析流水线"
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={Math.round(lineProgress)}
      aria-valuetext={statusDetailText(paper)}
    >
      <div className="relative px-1">
        <div className="absolute left-5 right-5 top-2.5 h-0.5 rounded-full bg-border" />
        <div className="absolute left-5 right-5 top-2.5 h-0.5 overflow-hidden rounded-full">
          <motion.div
            className={cn(
              "h-full w-full origin-left rounded-full",
              flowing && !reduceMotion
                ? "bg-[linear-gradient(90deg,var(--sienna)_0%,color-mix(in_oklch,var(--sienna)_45%,white)_48%,var(--sienna)_100%)] bg-[length:220%_100%]"
                : "bg-sienna/85",
            )}
            initial={false}
            animate={{
              scaleX: lineProgress / 100,
              backgroundPosition:
                flowing && !reduceMotion ? ["160% 0", "-60% 0"] : "0% 0",
            }}
            transition={
              reduceMotion
                ? { duration: 0 }
                : {
                    scaleX: {
                      type: "spring",
                      stiffness: 260,
                      damping: 34,
                      mass: 0.7,
                    },
                    backgroundPosition: {
                      duration: 1.1,
                      ease: "linear",
                      repeat: Number.POSITIVE_INFINITY,
                    },
                  }
            }
          />
        </div>
        <ol className="relative flex items-start justify-between">
          {PARSE_TIMELINE.map((step, index) => {
            const active =
              paper.status === step.status && paper.status !== "ready";
            const done = index < current || paper.status === "ready";
            return (
              <li
                key={step.status}
                className="flex w-10 flex-col items-center gap-1.5"
              >
                <PipelineStepDot active={active} done={done} />
                <span
                  className={cn(
                    "text-[11px] leading-none transition-colors",
                    active || done
                      ? "font-medium text-sienna"
                      : "text-muted-foreground/70",
                  )}
                >
                  {step.label}
                </span>
              </li>
            );
          })}
        </ol>
      </div>
    </div>
  );
}

// 解析中/失败时的细状态条 - 就绪后隐藏, 不与右栏头部重复
function ActiveStatusStrip({ paper }: { paper: Paper }) {
  const detailText = statusDetailText(paper);

  return (
    <div className="rounded-md border bg-card px-3 py-2">
      <div className="flex items-center justify-between gap-2">
        <span className="min-w-0 truncate font-serif text-xs font-semibold">
          {paperTitle(paper)}
        </span>
        <InlinePaperStatus status={paper.status} />
      </div>
      {paper.status === "failed" ? (
        <p className="mt-1.5 text-xs text-destructive">
          {paper.fail_reason || "解析失败"}
        </p>
      ) : (
        <>
          <ParsePipeline paper={paper} />
          {detailText && (
            <p className="mt-2 text-xs text-muted-foreground">{detailText}</p>
          )}
        </>
      )}
    </div>
  );
}

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
        "inline-flex h-7 min-w-0 items-center gap-1 rounded-full border px-2.5 text-xs transition-colors focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none",
        "max-w-full",
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
  return (
    <Popover>
      <PopoverTrigger
        aria-label={
          activeKw.length > 0
            ? `已筛选 ${activeKw.length} 个关键词`
            : `关键词筛选：${keywords.length} 个关键词`
        }
        title={
          activeKw.length > 0
            ? `已筛选 ${activeKw.length} 个关键词`
            : `关键词筛选：${keywords.length} 个关键词`
        }
        className={cn(
          "inline-flex h-7 min-w-10 shrink-0 items-center justify-center gap-1 rounded-md border px-1.5 text-xs font-medium transition-colors focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none",
          activeKw.length > 0
            ? "border-sienna/40 bg-sienna/10 text-sienna hover:bg-sienna/15"
            : "border-border bg-card text-muted-foreground hover:bg-accent/60 hover:text-foreground",
        )}
      >
        <ListFilter className="size-3.5" />
        <span className="font-mono text-[11px] leading-none">
          {activeKw.length > 0 ? activeKw.length : keywords.length}
        </span>
      </PopoverTrigger>
      <PopoverContent className="w-80 space-y-3 p-3" align="start">
        <div className="flex items-start justify-between gap-3">
          <div className="space-y-1">
            <PopoverTitle>关键词筛选</PopoverTitle>
            <PopoverDescription>
              {keywords.length > 0
                ? `${keywords.length} 个关键词`
                : "当前论文还没有可筛选关键词"}
            </PopoverDescription>
          </div>
          {activeKw.length > 0 && (
            <Button type="button" variant="ghost" size="xs" onClick={onClear}>
              清除
            </Button>
          )}
        </div>
        {keywords.length > 0 ? (
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
        ) : (
          <div className="rounded-lg bg-muted/45 px-3 py-2 text-xs leading-5 text-muted-foreground">
            论文解析完成并抽取关键词后，这里会自动出现筛选项。
          </div>
        )}
      </PopoverContent>
    </Popover>
  );
}

function SessionPopover({
  paperSessions,
  activeSessionID,
  onOpenSession,
  onRemoveSession,
}: {
  paperSessions: Session[];
  activeSessionID: string | null;
  onOpenSession: (id: string) => void;
  onRemoveSession: (id: string) => void;
}) {
  return (
    <Popover>
      <PopoverTrigger
        aria-label={`论文会话：${paperSessions.length} 个`}
        title="论文会话"
        className="inline-flex h-6 min-w-8 shrink-0 items-center justify-center gap-1 rounded-md px-1.5 text-[11px] font-medium whitespace-nowrap text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none data-[popup-open]:bg-muted data-[popup-open]:text-foreground"
        onClick={(e) => e.stopPropagation()}
      >
        <MessagesSquare className="size-3" />
        <span className="font-mono">{paperSessions.length}</span>
      </PopoverTrigger>
      <PopoverContent
        className="w-64 rounded-xl p-1.5 shadow-md"
        align="end"
        sideOffset={4}
      >
        <div className="px-2 pb-1.5 pt-1">
          <PopoverTitle className="text-xs font-medium text-foreground">
            继续阅读
          </PopoverTitle>
          <PopoverDescription className="mt-0.5 text-[11px] leading-4">
            {paperSessions.length > 0
              ? `${paperSessions.length} 个论文会话`
              : "这篇论文还没有会话"}
          </PopoverDescription>
        </div>
        {paperSessions.length > 0 && (
          <div className="max-h-56 space-y-0.5 overflow-y-auto">
            {paperSessions.map((s) => (
              <div
                key={s.id}
                className={cn(
                  "group/s flex min-h-11 items-center gap-1 rounded-lg transition-colors",
                  s.id === activeSessionID
                    ? "bg-muted"
                    : "hover:bg-muted/70",
                )}
              >
                <button
                  type="button"
                  className="min-w-0 flex-1 px-2 py-2 text-left"
                  onClick={(e) => {
                    e.stopPropagation();
                    onOpenSession(s.id);
                  }}
                >
                  <span className="block truncate text-xs font-medium leading-4">
                    {sessionTitle(s)}
                  </span>
                  <span className="mt-0.5 block truncate font-mono text-[11px] text-muted-foreground">
                    {formatTime(s.updated_at || s.created_at)}
                  </span>
                </button>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-sm"
                  className="mr-1 size-6 opacity-0 transition-opacity group-focus-within/s:opacity-100 group-hover/s:opacity-100"
                  onClick={(e) => {
                    e.stopPropagation();
                    onRemoveSession(s.id);
                  }}
                  title="删除会话"
                >
                  <Trash2 className="size-3" />
                </Button>
              </div>
            ))}
          </div>
        )}
      </PopoverContent>
    </Popover>
  );
}

function PaperActionsMenu({
  paper,
  canReparse,
  busy,
  onReparse,
  onDelete,
}: {
  paper: Paper;
  canReparse: boolean;
  busy: boolean;
  onReparse: () => void;
  onDelete: () => void;
}) {
  return (
    <div
      className="pointer-events-auto"
      onClick={(e) => e.stopPropagation()}
      onKeyDown={(e) => e.stopPropagation()}
    >
      <DropdownMenu>
        <DropdownMenuTrigger
          render={
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              className="text-muted-foreground opacity-70 transition-opacity hover:opacity-100 aria-expanded:opacity-100 sm:opacity-0 sm:group-focus-within/paper:opacity-100 sm:group-hover/paper:opacity-100"
              aria-label={`论文操作：${paperTitle(paper)}`}
              title="论文操作"
            >
              {busy ? (
                <Loader2 className="size-3.5 animate-spin" />
              ) : (
                <MoreHorizontal className="size-3.5" />
              )}
            </Button>
          }
        />
        <DropdownMenuContent align="end" className="w-44">
          <DropdownMenuItem disabled={!canReparse || busy} onClick={onReparse}>
            <RotateCcw className="size-4" />
            重新解析
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem disabled={busy} variant="destructive" onClick={onDelete}>
            <Trash2 className="size-4" />
            删除论文
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}

export function PaperPane({
  onExpandSidebar,
}: {
  onExpandSidebar?: () => void;
}) {
  const {
    papers,
    activePaper,
    activePaperID,
    selectPaper,
    uploadPaper,
    reparsePaper,
    removePaper,
    refreshPapers,
    sessions,
    activeSessionID,
    openSession,
    removeSession,
  } = useApp();
  const guard = useGuard();
  const fileRef = useRef<HTMLInputElement>(null);
  const [picked, setPicked] = useState<File | null>(null);
  const [uploading, setUploading] = useState(false);
  const [uploadOpen, setUploadOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<Paper | null>(null);
  const [reparseTarget, setReparseTarget] = useState<Paper | null>(null);
  const [deletingID, setDeletingID] = useState("");
  const [reparsingID, setReparsingID] = useState("");
  const [query, setQuery] = useState("");
  const [activeKw, setActiveKw] = useState<string[]>([]);

  // 当前论文集合里的关键词去重 + 计数, 按出现频次倒序铺成可点标签。
  const keywords = useMemo(() => {
    const counts = new Map<string, number>();
    for (const p of papers)
      for (const k of p.keywords ?? []) counts.set(k, (counts.get(k) ?? 0) + 1);
    return [...counts.entries()]
      .sort((a, b) => b[1] - a[1])
      .map(([name, count]) => ({ name, count }));
  }, [papers]);

  // 选中标签做 OR 过滤: 命中任一选中关键词即显示; 未选则全列。
  const shownPapers = useMemo(() => {
    if (activeKw.length === 0) return papers;
    return papers.filter((p) => p.keywords?.some((k) => activeKw.includes(k)));
  }, [papers, activeKw]);

  const toggleKw = (name: string) =>
    setActiveKw((prev) =>
      prev.includes(name) ? prev.filter((k) => k !== name) : [...prev, name],
    );

  const onUpload = (e: React.FormEvent) => {
    e.preventDefault();
    if (!picked) return;
    setUploading(true);
    guard(async () => {
      await uploadPaper(picked);
      setPicked(null);
      if (fileRef.current) fileRef.current.value = "";
      setUploadOpen(false);
    }).finally(() => setUploading(false));
  };

  const onDelete = () => {
    if (!deleteTarget || deletingID) return;
    const id = deleteTarget.id;
    setDeletingID(id);
    guard(async () => {
      await removePaper(id);
      setDeleteTarget(null);
    }).finally(() => setDeletingID(""));
  };

  const onReparse = () => {
    if (!reparseTarget || reparsingID) return;
    const paper = reparseTarget;
    setReparsingID(paper.id);
    guard(async () => {
      await reparsePaper(paper.id);
      setReparseTarget(null);
    }).finally(() => setReparsingID(""));
  };

  const showStrip = activePaper && activePaper.status !== "ready";
  const deletingTarget = Boolean(
    deleteTarget && deletingID === deleteTarget.id,
  );
  const reparsingTarget = Boolean(
    reparseTarget && reparsingID === reparseTarget.id,
  );

  return (
    <aside className="flex min-h-0 flex-1 flex-col bg-background">
      <div className="space-y-2 px-3 pb-1 pt-2">
        <div className="flex min-w-0 items-center gap-1.5">
          {onExpandSidebar && (
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              onClick={onExpandSidebar}
              title="展开侧栏"
              className="hidden shrink-0 lg:inline-flex"
            >
              <PanelLeftOpen className="size-4" />
            </Button>
          )}
          <div className="flex shrink-0 items-baseline gap-1.5 pr-1">
            <h2 className="text-sm font-bold tracking-tight">论文库</h2>
            {papers.length > 0 && (
              <span className="font-mono text-[11px] text-muted-foreground">
                {papers.length}
              </span>
            )}
          </div>
          <form
            className="relative min-w-0 flex-1"
            onSubmit={(e) => {
              e.preventDefault();
              guard(() => refreshPapers(query));
            }}
          >
            <Search className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-foreground/55" />
            <Input
              value={query}
              className="h-7 rounded-md border-border/70 bg-card pl-7 pr-7 text-sm shadow-none hover:border-border focus-visible:border-primary/60 focus-visible:ring-1 focus-visible:ring-primary/20"
              placeholder=""
              onChange={(e) => setQuery(e.target.value)}
            />
            {query && (
              <button
                type="button"
                aria-label="清除搜索"
                className="absolute right-2 top-1/2 -translate-y-1/2 rounded-sm p-0.5 text-muted-foreground transition-colors hover:text-foreground"
                onClick={() => {
                  setQuery("");
                  guard(() => refreshPapers());
                }}
              >
                <X className="size-3.5" />
              </button>
            )}
          </form>
          <KeywordFilter
            keywords={keywords}
            activeKw={activeKw}
            onToggle={toggleKw}
            onClear={() => setActiveKw([])}
          />
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            onClick={() => guard(() => refreshPapers())}
            title="刷新"
            aria-label="刷新论文库"
          >
            <RefreshCw className="size-4" />
          </Button>
          <Button
            type="button"
            size="icon-sm"
            onClick={() => setUploadOpen(true)}
            title="上传论文"
            aria-label="上传论文"
          >
            <FileUp className="size-3.5" />
          </Button>
        </div>

        {showStrip && <ActiveStatusStrip paper={activePaper} />}
      </div>

      <ScrollArea className="min-h-0 flex-1">
        <div className="space-y-0.5 px-2 pb-3 pt-0">
          {papers.length === 0 ? (
            <Empty
              title="还没有论文"
              text="点击右上角「上传」加入第一篇 PDF。"
              compact
            />
          ) : shownPapers.length === 0 ? (
            <Empty
              title="没有匹配的论文"
              text="当前关键词筛选下没有论文，换个标签或清除筛选。"
              compact
            />
          ) : (
            shownPapers.map((p) => {
              const active = p.id === activePaperID;
              const paperSessions = sessionsForPaper(sessions, p.id);
              const canReparse = isSettled(p.status) || p.status === "uploaded";
              const reparsing = reparsingID === p.id;
              const detailText =
                p.status === "failed"
                  ? p.fail_reason || "解析失败,可重新解析"
                  : [
                      p.page_count > 0 ? `${p.page_count} 页` : "",
                      formatSize(p.size),
                      formatTime(p.updated_at || p.created_at),
                    ]
                      .filter(Boolean)
                      .join(" · ");
              return (
                <div
                  key={p.id}
                  role="button"
                  tabIndex={0}
                  aria-label={`选择论文：${paperTitle(p)}`}
                  className={cn(
                    "group/paper relative min-h-[4.75rem] w-full cursor-pointer select-none rounded-lg px-3 py-2.5 transition-colors focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none",
                    active
                      ? "bg-accent/75 ring-1 ring-primary/25 before:absolute before:inset-y-2.5 before:left-0 before:w-0.5 before:rounded-full before:bg-primary dark:bg-accent/35 dark:ring-primary/30"
                      : "hover:bg-muted/55",
                  )}
                  onClick={() => selectPaper(p.id)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault();
                      selectPaper(p.id);
                    }
                  }}
                >
                  <div className="relative z-10 min-w-0 pr-8">
                    <strong className="pointer-events-none line-clamp-2 min-w-0 text-sm font-medium leading-snug">
                      {paperTitle(p)}
                    </strong>
                  </div>
                  <div className="pointer-events-none relative z-10 mt-2 flex min-w-0 items-center gap-2 pr-16 text-xs text-muted-foreground">
                    <InlinePaperStatus status={p.status} />
                    <span
                      className="h-3 w-px shrink-0 bg-border/70"
                      aria-hidden
                    />
                    <span
                      className={cn(
                        "pointer-events-none line-clamp-1 min-w-0",
                        p.status === "failed" && "text-destructive",
                      )}
                    >
                      {detailText}
                    </span>
                  </div>
                  <div className="absolute bottom-2.5 right-2 z-10 flex items-center gap-0.5">
                    {active && (
                      <div className="pointer-events-auto">
                        <SessionPopover
                          paperSessions={paperSessions}
                          activeSessionID={activeSessionID}
                          onOpenSession={(id) => guard(() => openSession(id))}
                          onRemoveSession={(id) =>
                            guard(() => removeSession(id))
                          }
                        />
                      </div>
                    )}
                    <PaperActionsMenu
                      paper={p}
                      canReparse={canReparse}
                      busy={reparsing || deletingID === p.id}
                      onReparse={() => setReparseTarget(p)}
                      onDelete={() => setDeleteTarget(p)}
                    />
                  </div>
                </div>
              );
            })
          )}
        </div>
      </ScrollArea>

      <Dialog
        open={Boolean(deleteTarget)}
        onOpenChange={(open) => {
          if (!open && !deletingID) setDeleteTarget(null);
        }}
      >
        <DialogContent className="sm:max-w-md">
          <DialogHeader className="min-w-0">
            <div className="flex min-w-0 items-center gap-3">
              <span className="flex size-9 shrink-0 items-center justify-center rounded-md bg-destructive/10 text-destructive">
                <Trash2 className="size-4" />
              </span>
              <div className="min-w-0">
                <DialogTitle>删除论文</DialogTitle>
                <DialogDescription className="mt-1">
                  这会移除绑定会话、报告、图片和向量索引。
                </DialogDescription>
              </div>
            </div>
          </DialogHeader>
          <DialogDescription className="rounded-lg border bg-muted/35 px-3 py-2 text-foreground">
            {deleteTarget ? paperTitle(deleteTarget) : ""}
          </DialogDescription>
          <DialogFooter className="sm:flex-nowrap">
            <Button
              type="button"
              variant="outline"
              className="w-full sm:w-auto"
              disabled={deletingTarget}
              onClick={() => setDeleteTarget(null)}
            >
              取消
            </Button>
            <Button
              type="button"
              variant="destructive"
              className="w-full sm:w-auto"
              disabled={deletingTarget}
              onClick={onDelete}
            >
              {deletingTarget && <Loader2 className="size-4 animate-spin" />}
              确认删除
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={Boolean(reparseTarget)}
        onOpenChange={(open) => {
          if (!open && !reparsingID) setReparseTarget(null);
        }}
      >
        <DialogContent className="sm:max-w-md">
          <DialogHeader className="min-w-0">
            <div className="flex min-w-0 items-center gap-3">
              <span className="flex size-9 shrink-0 items-center justify-center rounded-md bg-sienna/10 text-sienna">
                <AlertTriangle className="size-4" />
              </span>
              <div className="min-w-0">
                <DialogTitle>重新解析论文</DialogTitle>
                <DialogDescription className="mt-1">
                  会重新投递解析任务，期间旧结果可能被新解析结果覆盖。
                </DialogDescription>
              </div>
            </div>
          </DialogHeader>
          <DialogDescription className="rounded-lg border bg-muted/35 px-3 py-2 text-foreground">
            {reparseTarget ? paperTitle(reparseTarget) : ""}
          </DialogDescription>
          <div className="rounded-lg bg-muted/35 px-3 py-2 text-xs leading-5 text-muted-foreground">
            适合在解析失败、元数据异常或 PDF 更新后使用。已经就绪的论文不建议频繁重解析。
          </div>
          <DialogFooter className="sm:flex-nowrap">
            <Button
              type="button"
              variant="outline"
              className="w-full sm:w-auto"
              disabled={reparsingTarget}
              onClick={() => setReparseTarget(null)}
            >
              取消
            </Button>
            <Button
              type="button"
              className="w-full sm:w-auto"
              disabled={reparsingTarget}
              onClick={onReparse}
            >
              {reparsingTarget && <Loader2 className="size-4 animate-spin" />}
              确认重新解析
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={uploadOpen} onOpenChange={setUploadOpen}>
        <DialogContent>
          <DialogHeader className="min-w-0">
            <DialogTitle>上传论文</DialogTitle>
            <DialogDescription>
              上传 PDF 后自动解析、抽取并进入知识库。
            </DialogDescription>
          </DialogHeader>
          <form onSubmit={onUpload} className="min-w-0 space-y-3">
            <label
              className={cn(
                "flex min-w-0 cursor-pointer flex-col items-center justify-center overflow-hidden rounded-lg border border-dashed bg-muted/30 px-4 py-10 text-center transition hover:border-sienna/50 hover:bg-muted/50",
                picked && "border-sienna/60 bg-sienna/5",
              )}
              onDragOver={(e) => e.preventDefault()}
              onDrop={(e) => {
                e.preventDefault();
                const f = e.dataTransfer.files?.[0];
                if (f) setPicked(f);
              }}
            >
              <input
                ref={fileRef}
                type="file"
                accept="application/pdf,.pdf"
                className="sr-only"
                onChange={(e) => setPicked(e.target.files?.[0] ?? null)}
              />
              <FileUp className="mb-3 size-6 text-muted-foreground" />
              <strong
                className="max-w-full break-words text-sm [overflow-wrap:anywhere]"
                title={picked?.name}
              >
                {picked ? picked.name : "选择或拖入 PDF"}
              </strong>
              <small className="mt-1 max-w-full break-words text-xs text-muted-foreground [overflow-wrap:anywhere]">
                {picked
                  ? `${formatSize(picked.size)} · 等待上传`
                  : "支持 50MB 以内 PDF"}
              </small>
            </label>
            <Button
              type="submit"
              className="w-full"
              disabled={uploading || !picked}
            >
              {uploading && <Loader2 className="size-4 animate-spin" />}
              上传论文
            </Button>
          </form>
        </DialogContent>
      </Dialog>
    </aside>
  );
}
