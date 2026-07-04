"use client";

import {
  AlertTriangle,
  ArrowLeft,
  FileText,
  FileUp,
  Loader2,
  MessagesSquare,
  MoreHorizontal,
  NotebookText,
  RotateCcw,
  Search,
  Trash2,
  X,
} from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useMemo, useRef, useState } from "react";

import { Button, buttonVariants } from "@/components/ui/button";
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
import { ScrollArea } from "@/components/ui/scroll-area";
import { useApp } from "@/lib/gopherpaper/store";
import type { Paper } from "@/lib/gopherpaper/types";
import {
  formatSize,
  formatTime,
  isSettled,
  paperTitle,
  statusLabel,
  statusTone,
} from "@/lib/gopherpaper/utils";
import { cn } from "@/lib/utils";
import { Empty, useGuard } from "./app-ui";
import { PaperCover } from "./paper-cover";
import { WorkspaceFrame, WorkspacePanel } from "./workspace-frame";

type StatusGroup = "all" | "ready" | "working" | "failed";

const STATUS_FILTERS: { value: StatusGroup; label: string; dot?: string }[] = [
  { value: "all", label: "全部" },
  { value: "ready", label: "就绪", dot: "bg-muted-foreground/45" },
  { value: "working", label: "处理中", dot: "bg-amber-500" },
  { value: "failed", label: "失败", dot: "bg-destructive" },
];

const KEYWORD_RAIL_LIMIT = 14;

function statusGroupOf(paper: Paper): StatusGroup {
  const tone = statusTone(paper.status);
  if (tone === "ready") return "ready";
  if (tone === "failed") return "failed";
  return "working";
}

function paperMetaText(paper: Paper): string {
  return [
    "PDF",
    paper.page_count > 0 ? `${paper.page_count} 页` : "",
    formatSize(paper.size),
  ]
    .filter(Boolean)
    .join(" · ");
}

function RailSectionLabel({ children }: { children: React.ReactNode }) {
  return (
    <h3 className="px-2.5 pb-1 text-xs font-semibold text-muted-foreground">
      {children}
    </h3>
  );
}

function RailFilterRow({
  label,
  count,
  dot,
  active,
  onClick,
}: {
  label: string;
  count: number;
  dot?: string;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      aria-pressed={active}
      onClick={onClick}
      className={cn(
        "flex w-full items-center gap-2 rounded-md px-2.5 py-1.5 text-left text-[13px] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50",
        active
          ? "bg-accent text-foreground"
          : "text-muted-foreground hover:bg-accent/50 hover:text-foreground",
      )}
    >
      {dot && (
        <span className={cn("size-1.5 shrink-0 rounded-full", dot)} aria-hidden />
      )}
      <span className="min-w-0 flex-1 truncate">{label}</span>
      <span
        className={cn(
          "shrink-0 font-mono text-[11px]",
          active ? "text-muted-foreground/80" : "text-muted-foreground/50",
        )}
      >
        {count}
      </span>
    </button>
  );
}

function KeywordRailChip({
  name,
  count,
  active,
  onClick,
}: {
  name: string;
  count: number;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      aria-pressed={active}
      title={name}
      onClick={onClick}
      className={cn(
        "inline-flex min-w-0 max-w-full items-center gap-1 rounded-md px-2 py-1 text-xs transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50",
        active
          ? "bg-sienna/10 text-sienna hover:bg-sienna/15"
          : "bg-muted/60 text-muted-foreground hover:bg-muted hover:text-foreground",
      )}
    >
      <span className="min-w-0 truncate">{name}</span>
      <span
        className={cn(
          "shrink-0 font-mono text-[10px]",
          active ? "text-sienna/70" : "text-muted-foreground/50",
        )}
      >
        {count}
      </span>
    </button>
  );
}

// 卡片左上角类型徽标:就绪 sienna 文档,处理中琥珀转圈,失败红色警示。
function CardGlyph({ paper }: { paper: Paper }) {
  const tone = statusTone(paper.status);
  return (
    <span
      className={cn(
        "flex size-8 shrink-0 items-center justify-center rounded-lg",
        tone === "ready" && "bg-sienna/[0.08] text-sienna",
        tone === "working" &&
          "bg-amber-500/10 text-amber-700 dark:text-amber-300",
        tone === "failed" && "bg-destructive/10 text-destructive",
      )}
      aria-hidden
    >
      {tone === "working" ? (
        <Loader2 className="size-4 animate-spin" />
      ) : tone === "failed" ? (
        <AlertTriangle className="size-4" />
      ) : (
        <FileText className="size-4" />
      )}
    </span>
  );
}

function CardStatusLine({ paper }: { paper: Paper }) {
  const tone = statusTone(paper.status);
  if (tone === "ready") return null;
  return (
    <span
      className={cn(
        "inline-flex min-w-0 items-center gap-1.5 text-[11px] font-medium",
        tone === "failed"
          ? "text-destructive"
          : "text-amber-700 dark:text-amber-300",
      )}
    >
      <span
        className={cn(
          "size-1.5 shrink-0 rounded-full bg-current",
          tone === "working" && "animate-pulse",
        )}
        aria-hidden
      />
      <span className="min-w-0 truncate">
        {tone === "failed"
          ? paper.fail_reason || "解析失败"
          : statusLabel(paper.status)}
      </span>
    </span>
  );
}

function PaperCard({
  paper,
  onOpen,
  onOpenChat,
  onOpenReport,
  onReparse,
  onDelete,
  busy,
}: {
  paper: Paper;
  onOpen: () => void;
  onOpenChat: () => void;
  onOpenReport: () => void;
  onReparse: () => void;
  onDelete: () => void;
  busy: boolean;
}) {
  const keywords = paper.keywords ?? [];
  const canReparse = isSettled(paper.status) || paper.status === "uploaded";
  return (
    <article
      role="button"
      tabIndex={0}
      aria-label={`精读论文 ${paperTitle(paper)}`}
      onClick={onOpen}
      onKeyDown={(event) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          onOpen();
        }
      }}
      className="group flex h-full cursor-pointer select-none flex-col overflow-hidden rounded-xl border bg-card text-left transition-[border-color,box-shadow,transform] duration-200 ease-out hover:-translate-y-0.5 hover:shadow-[0_2px_10px_rgba(15,23,42,0.07)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50 active:translate-y-0 active:scale-[0.995] motion-reduce:transition-none motion-reduce:hover:translate-y-0 dark:hover:shadow-[0_2px_10px_rgba(0,0,0,0.35)]"
    >
      <div className="relative aspect-[16/10] w-full shrink-0 overflow-hidden border-b border-border/60 bg-muted/40">
        <PaperCover paperID={paper.id} fallback={<CardGlyph paper={paper} />} />
      </div>

      <div className="flex min-h-0 flex-1 flex-col p-3.5">
      <div className="flex items-start justify-between gap-2">
        <h3 className="line-clamp-2 min-w-0 flex-1 font-serif text-[15px] font-semibold leading-snug">
          {paperTitle(paper)}
        </h3>
        <div
          onClick={(event) => event.stopPropagation()}
          onKeyDown={(event) => event.stopPropagation()}
        >
          <DropdownMenu>
            <DropdownMenuTrigger
              render={
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-sm"
                  className="-mr-1 -mt-1 size-6 text-muted-foreground opacity-70 transition-opacity hover:opacity-100 aria-expanded:opacity-100 sm:opacity-0 sm:group-focus-within:opacity-100 sm:group-hover:opacity-100"
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
              <DropdownMenuItem onClick={onOpenChat}>
                <MessagesSquare className="size-4" />
                打开对话
              </DropdownMenuItem>
              <DropdownMenuItem onClick={onOpenReport}>
                <NotebookText className="size-4" />
                研读报告
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem disabled={!canReparse || busy} onClick={onReparse}>
                <RotateCcw className="size-4" />
                重新解析
              </DropdownMenuItem>
              <DropdownMenuItem disabled={busy} variant="destructive" onClick={onDelete}>
                <Trash2 className="size-4" />
                删除论文
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>

      <p className="mt-1 pb-3 font-mono text-[11px] text-muted-foreground/70">
        {paperMetaText(paper)}
      </p>

      <div className="mt-auto flex min-w-0 items-center justify-between gap-2 border-t border-border/60 pt-2.5">
        {keywords.length > 0 ? (
          <div className="flex min-w-0 flex-nowrap gap-1 overflow-hidden">
            {keywords.slice(0, 2).map((keyword) => (
              <span
                key={keyword}
                className="max-w-32 truncate rounded-full bg-muted px-2 py-0.5 text-[10px] text-muted-foreground"
              >
                {keyword}
              </span>
            ))}
            {keywords.length > 2 && (
              <span className="shrink-0 px-0.5 py-0.5 font-mono text-[10px] text-muted-foreground/60">
                +{keywords.length - 2}
              </span>
            )}
          </div>
        ) : (
          <span aria-hidden />
        )}
        {statusTone(paper.status) === "ready" ? (
          <time className="shrink-0 font-mono text-[10px] text-muted-foreground/60">
            {formatTime(paper.updated_at || paper.created_at)}
          </time>
        ) : (
          <CardStatusLine paper={paper} />
        )}
      </div>
      </div>
    </article>
  );
}

export function PaperLibrary() {
  const {
    authed,
    papers,
    selectPaper,
    uploadPaper,
    reparsePaper,
    removePaper,
  } = useApp();
  const router = useRouter();
  const guard = useGuard();
  const fileRef = useRef<HTMLInputElement>(null);
  const [query, setQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState<StatusGroup>("all");
  const [activeKw, setActiveKw] = useState<string[]>([]);
  const [kwExpanded, setKwExpanded] = useState(false);
  const [uploadOpen, setUploadOpen] = useState(false);
  const [picked, setPicked] = useState<File | null>(null);
  const [uploading, setUploading] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<Paper | null>(null);
  const [busyID, setBusyID] = useState("");

  const statusCounts = useMemo(() => {
    const counts: Record<StatusGroup, number> = {
      all: papers.length,
      ready: 0,
      working: 0,
      failed: 0,
    };
    for (const paper of papers) counts[statusGroupOf(paper)] += 1;
    return counts;
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
    const q = query.trim().toLowerCase();
    return papers.filter((paper) => {
      if (statusFilter !== "all" && statusGroupOf(paper) !== statusFilter) {
        return false;
      }
      if (
        activeKw.length > 0 &&
        !paper.keywords?.some((keyword) => activeKw.includes(keyword))
      ) {
        return false;
      }
      if (!q) return true;
      const haystack = [
        paperTitle(paper),
        paper.file_name,
        ...(paper.keywords ?? []),
      ]
        .join(" ")
        .toLowerCase();
      return haystack.includes(q);
    });
  }, [papers, query, statusFilter, activeKw]);

  const filtering =
    Boolean(query.trim()) || statusFilter !== "all" || activeKw.length > 0;

  const toggleKw = (name: string) =>
    setActiveKw((prev) =>
      prev.includes(name)
        ? prev.filter((keyword) => keyword !== name)
        : [...prev, name],
    );

  // 卡片点击直达精读页;对话入口保留在卡片菜单里。
  const openReader = (paper: Paper) => {
    router.push(`/reader?id=${encodeURIComponent(paper.id)}`);
  };

  const openChat = (paper: Paper) => {
    selectPaper(paper.id);
    router.push("/");
  };

  const openReport = (paper: Paper) => {
    router.push(`/reports?paper_id=${paper.id}`);
  };

  const onReparse = (paper: Paper) => {
    if (busyID) return;
    setBusyID(paper.id);
    guard(() => reparsePaper(paper.id)).finally(() => setBusyID(""));
  };

  const onDelete = () => {
    if (!deleteTarget || busyID) return;
    const id = deleteTarget.id;
    setBusyID(id);
    guard(async () => {
      await removePaper(id);
      setDeleteTarget(null);
    }).finally(() => setBusyID(""));
  };

  const onUpload = (event: React.FormEvent) => {
    event.preventDefault();
    if (!picked) return;
    setUploading(true);
    guard(async () => {
      await uploadPaper(picked);
      setPicked(null);
      if (fileRef.current) fileRef.current.value = "";
      setUploadOpen(false);
    }).finally(() => setUploading(false));
  };

  if (!authed) {
    return (
      <main className="flex h-dvh items-center justify-center bg-muted/50 p-6">
        <div className="text-center">
          <Empty title="请先登录" text="登录后即可浏览与管理论文库。" />
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
      <WorkspacePanel className="flex min-w-0 flex-1">
        <aside className="hidden w-60 shrink-0 flex-col border-r md:flex">
          <div className="flex items-center gap-2 border-b px-3 py-3">
            <Link
              href="/"
              className={buttonVariants({ variant: "ghost", size: "icon-sm" })}
              title="返回工作台"
              aria-label="返回工作台"
            >
              <ArrowLeft className="size-4" />
            </Link>
            <div className="flex min-w-0 items-baseline gap-1.5">
              <h2 className="truncate text-sm font-semibold">论文库</h2>
              {papers.length > 0 && (
                <span className="font-mono text-[11px] text-muted-foreground/60">
                  {papers.length}
                </span>
              )}
            </div>
          </div>

          <div className="space-y-2 px-2.5 pt-3">
            <Button
              type="button"
              className="w-full"
              onClick={() => setUploadOpen(true)}
            >
              <FileUp className="size-3.5" />
              上传论文
            </Button>

            <div className="relative">
              <Search className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
              <Input
                value={query}
                className="h-8 rounded-lg border-transparent bg-muted/60 px-8 shadow-none transition-colors hover:bg-muted/80 focus-visible:border-ring/40 focus-visible:bg-background focus-visible:ring-2 focus-visible:ring-ring/25 dark:bg-muted/40 dark:hover:bg-muted/50 dark:focus-visible:bg-background"
                placeholder="搜索标题、关键词"
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
          </div>

          <ScrollArea className="min-h-0 flex-1">
            <div className="space-y-4 px-2.5 pb-4 pt-4">
              <section>
                <RailSectionLabel>状态</RailSectionLabel>
                <div className="space-y-px">
                  {STATUS_FILTERS.filter(
                    (filter) =>
                      filter.value === "all" ||
                      filter.value === "ready" ||
                      statusCounts[filter.value] > 0 ||
                      statusFilter === filter.value,
                  ).map((filter) => (
                    <RailFilterRow
                      key={filter.value}
                      label={filter.label}
                      count={statusCounts[filter.value]}
                      dot={filter.dot}
                      active={statusFilter === filter.value}
                      onClick={() => setStatusFilter(filter.value)}
                    />
                  ))}
                </div>
              </section>

              {keywords.length > 0 && (
                <section>
                  <div className="flex items-baseline justify-between gap-2 pr-1">
                    <RailSectionLabel>关键词</RailSectionLabel>
                    {activeKw.length > 0 && (
                      <button
                        type="button"
                        className="pb-1 text-[11px] text-muted-foreground transition-colors hover:text-foreground"
                        onClick={() => setActiveKw([])}
                      >
                        清除
                      </button>
                    )}
                  </div>
                  <div className="flex flex-wrap gap-1 px-1">
                    {(kwExpanded
                      ? keywords
                      : keywords.slice(0, KEYWORD_RAIL_LIMIT)
                    ).map((item) => (
                      <KeywordRailChip
                        key={item.name}
                        name={item.name}
                        count={item.count}
                        active={activeKw.includes(item.name)}
                        onClick={() => toggleKw(item.name)}
                      />
                    ))}
                    {keywords.length > KEYWORD_RAIL_LIMIT && (
                      <button
                        type="button"
                        aria-expanded={kwExpanded}
                        className="inline-flex items-center rounded-md px-2 py-1 text-[11px] text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50"
                        onClick={() => setKwExpanded((value) => !value)}
                      >
                        {kwExpanded
                          ? "收起"
                          : `展开全部 ${keywords.length} 个`}
                      </button>
                    )}
                  </div>
                </section>
              )}
            </div>
          </ScrollArea>
        </aside>

        <ScrollArea className="min-h-0 min-w-0 flex-1">
          <div className="p-4 md:p-6">
            <div className="mb-3 flex items-center justify-between gap-2 md:hidden">
              <h2 className="text-sm font-semibold">论文库</h2>
              <Button
                type="button"
                size="sm"
                onClick={() => setUploadOpen(true)}
              >
                <FileUp className="size-3.5" />
                上传
              </Button>
            </div>

            {papers.length > 0 && (
              <header className="mb-5 hidden md:block">
                <div className="flex items-baseline gap-2">
                  <h1 className="font-serif text-xl font-semibold tracking-tight">
                    论文库
                  </h1>
                  <span className="font-mono text-xs text-muted-foreground/70">
                    {filtering
                      ? `${shownPapers.length} / ${papers.length}`
                      : papers.length}
                  </span>
                </div>
                <p className="mt-1 text-sm text-muted-foreground">
                  点击卡片进入精读，对话、报告与管理在卡片菜单里。
                </p>
              </header>
            )}

            {papers.length === 0 ? (
              <div className="flex min-h-[60vh] items-center justify-center">
                <div className="flex max-w-sm flex-col items-center text-center">
                  <span className="flex size-12 items-center justify-center rounded-xl bg-sienna/10 text-sienna">
                    <FileUp className="size-5" />
                  </span>
                  <h3 className="mt-4 font-serif text-lg font-semibold tracking-tight">
                    还没有论文
                  </h3>
                  <p className="mt-2 text-sm leading-6 text-muted-foreground">
                    上传第一篇 PDF，解析完成后即可在这里浏览、
                    向小文鸮提问或让小囊鼠生成研读报告。
                  </p>
                  <Button
                    type="button"
                    size="sm"
                    className="mt-5"
                    onClick={() => setUploadOpen(true)}
                  >
                    <FileUp className="size-3.5" />
                    上传论文
                  </Button>
                </div>
              </div>
            ) : shownPapers.length === 0 ? (
              <div className="flex min-h-[40vh] items-center justify-center">
                <Empty
                  title="没有匹配的论文"
                  text={
                    filtering
                      ? "换一个关键词或清除左侧筛选。"
                      : "先在左侧上传论文。"
                  }
                  compact
                />
              </div>
            ) : (
              <div className="grid gap-3.5 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
                {shownPapers.map((paper, index) => (
                  <div
                    key={paper.id}
                    className="h-full animate-in fade-in slide-in-from-bottom-1 duration-300 motion-reduce:animate-none"
                    style={{
                      animationDelay: `${Math.min(index, 8) * 40}ms`,
                      animationFillMode: "backwards",
                    }}
                  >
                    <PaperCard
                      paper={paper}
                      busy={busyID === paper.id}
                      onOpen={() => openReader(paper)}
                      onOpenChat={() => openChat(paper)}
                      onOpenReport={() => openReport(paper)}
                      onReparse={() => onReparse(paper)}
                      onDelete={() => setDeleteTarget(paper)}
                    />
                  </div>
                ))}
              </div>
            )}
          </div>
        </ScrollArea>
      </WorkspacePanel>

      <Dialog
        open={Boolean(deleteTarget)}
        onOpenChange={(open) => {
          if (!open && !busyID) setDeleteTarget(null);
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
              disabled={Boolean(busyID)}
              onClick={() => setDeleteTarget(null)}
            >
              取消
            </Button>
            <Button
              type="button"
              variant="destructive"
              className="w-full sm:w-auto"
              disabled={Boolean(busyID)}
              onClick={onDelete}
            >
              {Boolean(busyID) && <Loader2 className="size-4 animate-spin" />}
              确认删除
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
              onDragOver={(event) => event.preventDefault()}
              onDrop={(event) => {
                event.preventDefault();
                const file = event.dataTransfer.files?.[0];
                if (file) setPicked(file);
              }}
            >
              <input
                ref={fileRef}
                type="file"
                accept="application/pdf,.pdf"
                className="sr-only"
                onChange={(event) => setPicked(event.target.files?.[0] ?? null)}
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
    </WorkspaceFrame>
  );
}
