"use client";

import { FileUp, ListFilter, Loader2, MessagesSquare, RefreshCw, Search, Trash2, X } from "lucide-react";
import { useMemo, useRef, useState } from "react";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import {
  Popover,
  PopoverContent,
  PopoverDescription,
  PopoverTitle,
  PopoverTrigger,
} from "@/components/ui/popover";
import { Progress } from "@/components/ui/progress";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Separator } from "@/components/ui/separator";
import { cn } from "@/lib/utils";
import { useApp } from "@/lib/gopherpaper/store";
import type { Paper, Session } from "@/lib/gopherpaper/types";
import {
  formatSize,
  formatTime,
  paperTitle,
  sessionsForPaper,
  sessionTitle,
  statusStep,
} from "@/lib/gopherpaper/utils";
import { Empty, StatusBadge, useGuard } from "./app-ui";

function paperProgress(paper: Paper): number {
  if (paper.status === "failed") return 100;
  if (paper.progress > 0) return paper.progress;
  return Math.min(100, Math.max(8, (statusStep(paper.status) + 1) * 20));
}

// 解析中/失败时的细状态条 — 就绪后隐藏, 不与右栏头部重复
function ActiveStatusStrip({ paper }: { paper: Paper }) {
  return (
    <div className="rounded-md border bg-card px-3 py-2">
      <div className="flex items-center justify-between gap-2">
        <span className="min-w-0 truncate font-serif text-xs font-semibold">
          {paperTitle(paper)}
        </span>
        <StatusBadge status={paper.status} />
      </div>
      {paper.status === "failed" ? (
        <p className="mt-1.5 text-xs text-destructive">{paper.fail_reason || "解析失败"}</p>
      ) : (
        <Progress value={paperProgress(paper)} className="mt-2 h-1" />
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
  if (keywords.length === 0) return null;

  return (
    <Popover>
      <PopoverTrigger
        className={cn(
          "inline-flex h-7 shrink-0 items-center gap-1 rounded-full border px-2.5 text-xs font-medium transition-colors focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none",
          activeKw.length > 0
            ? "border-sienna/40 bg-sienna/10 text-sienna hover:bg-sienna/15"
            : "border-border bg-card text-muted-foreground hover:bg-accent/60 hover:text-foreground",
        )}
      >
        <ListFilter className="size-3.5" />
        {activeKw.length > 0 ? `筛选 ${activeKw.length}` : `关键词 ${keywords.length}`}
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
        className="inline-flex h-6 min-w-[3.75rem] shrink-0 items-center justify-center gap-1 rounded-full border border-border bg-background px-2 text-[11px] font-medium whitespace-nowrap text-muted-foreground transition-colors hover:bg-accent/60 hover:text-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none"
        onClick={(e) => e.stopPropagation()}
      >
        <MessagesSquare className="size-3" />
        会话{paperSessions.length}
      </PopoverTrigger>
      <PopoverContent className="w-72 space-y-2 p-2" align="end">
        <div className="px-1.5 py-1">
          <PopoverTitle>论文会话</PopoverTitle>
          <PopoverDescription>
            {paperSessions.length > 0 ? "选择一个会话继续阅读" : "这篇论文还没有会话"}
          </PopoverDescription>
        </div>
        {paperSessions.length > 0 && (
          <div className="max-h-64 space-y-1 overflow-y-auto pr-1">
            {paperSessions.map((s) => (
              <div
                key={s.id}
                className={cn(
                  "group/s flex items-center gap-1 rounded-md transition-colors",
                  s.id === activeSessionID ? "bg-accent/70" : "hover:bg-accent/55",
                )}
              >
                <button
                  type="button"
                  className="min-w-0 flex-1 px-2 py-1.5 text-left"
                  onClick={(e) => {
                    e.stopPropagation();
                    onOpenSession(s.id);
                  }}
                >
                  <span className="block truncate text-xs font-medium">{sessionTitle(s)}</span>
                  <span className="mt-0.5 block truncate text-[11px] text-muted-foreground">
                    {formatTime(s.updated_at || s.created_at)}
                  </span>
                </button>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-sm"
                  className="opacity-0 group-hover/s:opacity-100"
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

export function PaperPane() {
  const {
    papers,
    activePaper,
    activePaperID,
    selectPaper,
    uploadPaper,
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
  const [query, setQuery] = useState("");
  const [activeKw, setActiveKw] = useState<string[]>([]);

  // 当前论文集合里的关键词去重 + 计数, 按出现频次倒序铺成可点标签。
  const keywords = useMemo(() => {
    const counts = new Map<string, number>();
    for (const p of papers) for (const k of p.keywords ?? []) counts.set(k, (counts.get(k) ?? 0) + 1);
    return [...counts.entries()].sort((a, b) => b[1] - a[1]).map(([name, count]) => ({ name, count }));
  }, [papers]);

  // 选中标签做 OR 过滤: 命中任一选中关键词即显示; 未选则全列。
  const shownPapers = useMemo(() => {
    if (activeKw.length === 0) return papers;
    return papers.filter((p) => p.keywords?.some((k) => activeKw.includes(k)));
  }, [papers, activeKw]);

  const toggleKw = (name: string) =>
    setActiveKw((prev) => (prev.includes(name) ? prev.filter((k) => k !== name) : [...prev, name]));

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

  const showStrip = activePaper && activePaper.status !== "ready";

  return (
    <aside className="flex min-h-0 flex-1 flex-col bg-background">
      <div className="space-y-3 px-4 pb-3 pt-4">
        <div className="flex items-center justify-between gap-2">
          <div className="flex min-w-0 items-center gap-2">
            <div className="flex items-baseline gap-2">
              <h2 className="text-[15px] font-bold tracking-tight">论文库</h2>
              {papers.length > 0 && (
                <span className="font-mono text-xs text-muted-foreground">{papers.length}</span>
              )}
            </div>
            <KeywordFilter
              keywords={keywords}
              activeKw={activeKw}
              onToggle={toggleKw}
              onClear={() => setActiveKw([])}
            />
          </div>
          <div className="flex items-center gap-1">
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              onClick={() => guard(() => refreshPapers())}
              title="刷新"
            >
              <RefreshCw className="size-4" />
            </Button>
            <Button type="button" size="sm" onClick={() => setUploadOpen(true)}>
              <FileUp className="size-3.5" />
              上传
            </Button>
          </div>
        </div>

        <form
          className="relative"
          onSubmit={(e) => {
            e.preventDefault();
            guard(() => refreshPapers(query));
          }}
        >
          <Search className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={query}
            className="rounded-full bg-card px-8"
            placeholder="搜索论文 · 回车"
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

        {showStrip && <ActiveStatusStrip paper={activePaper} />}
      </div>

      <Separator />

      <ScrollArea className="min-h-0 flex-1">
        <div className="space-y-0.5 px-2 pb-3 pt-1">
          {papers.length === 0 ? (
            <Empty title="还没有论文" text="点击右上角「上传」加入第一篇 PDF。" compact />
          ) : shownPapers.length === 0 ? (
            <Empty title="没有匹配的论文" text="当前关键词筛选下没有论文，换个标签或清除筛选。" compact />
          ) : (
            shownPapers.map((p) => {
              const active = p.id === activePaperID;
              const paperSessions = sessionsForPaper(sessions, p.id);
              return (
                <div
                  key={p.id}
                  className={cn(
                    "group relative h-[5.5rem] w-full rounded-md px-3 py-2.5 transition-colors",
                    active
                      ? "bg-card shadow-sm before:absolute before:inset-y-2 before:left-0 before:w-0.5 before:rounded-full before:bg-sienna"
                      : "hover:bg-accent/60",
                  )}
                >
                  <button
                    type="button"
                    aria-label={`选择论文：${paperTitle(p)}`}
                    className="absolute inset-0 z-0 rounded-md focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none"
                    onClick={() => selectPaper(p.id)}
                  />
                  <div className="pointer-events-none relative z-10 flex items-start justify-between gap-2.5">
                    <strong className="pointer-events-none line-clamp-2 min-w-0 text-sm font-medium leading-snug">
                      {paperTitle(p)}
                    </strong>
                    <div className="pointer-events-none">
                      <StatusBadge status={p.status} />
                    </div>
                  </div>
                  <div className="pointer-events-none relative z-10 mt-1.5 grid grid-cols-[minmax(0,1fr)_auto] items-center gap-2 text-xs text-muted-foreground">
                    <span className="pointer-events-none line-clamp-1 min-w-0">
                      {[p.file_name, formatSize(p.size), formatTime(p.updated_at || p.created_at)]
                        .filter(Boolean)
                        .join(" · ")}
                    </span>
                    {active && (
                      <div className="pointer-events-auto">
                        <SessionPopover
                          paperSessions={paperSessions}
                          activeSessionID={activeSessionID}
                          onOpenSession={(id) => guard(() => openSession(id))}
                          onRemoveSession={(id) => guard(() => removeSession(id))}
                        />
                      </div>
                    )}
                  </div>
                </div>
              );
            })
          )}
        </div>
      </ScrollArea>

      <Dialog open={uploadOpen} onOpenChange={setUploadOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>上传论文</DialogTitle>
            <DialogDescription>上传 PDF 后自动解析、抽取并进入知识库。</DialogDescription>
          </DialogHeader>
          <form onSubmit={onUpload} className="space-y-3">
            <label
              className={cn(
                "flex cursor-pointer flex-col items-center justify-center rounded-lg border border-dashed bg-muted/30 px-4 py-10 text-center transition hover:border-sienna/50 hover:bg-muted/50",
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
              <strong className="max-w-full truncate text-sm">
                {picked ? picked.name : "选择或拖入 PDF"}
              </strong>
              <small className="mt-1 text-xs text-muted-foreground">
                {picked ? `${formatSize(picked.size)} · 等待上传` : "支持 50MB 以内 PDF"}
              </small>
            </label>
            <Button type="submit" className="w-full" disabled={uploading || !picked}>
              {uploading && <Loader2 className="size-4 animate-spin" />}
              上传论文
            </Button>
          </form>
        </DialogContent>
      </Dialog>
    </aside>
  );
}
