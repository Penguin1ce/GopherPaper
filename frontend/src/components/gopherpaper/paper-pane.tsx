"use client";

import { FileUp, Loader2, PanelLeftOpen, RefreshCw, Search, X } from "lucide-react";
import { useRef, useState } from "react";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Progress } from "@/components/ui/progress";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Separator } from "@/components/ui/separator";
import { cn } from "@/lib/utils";
import { useApp } from "@/lib/gopherpaper/store";
import type { Paper } from "@/lib/gopherpaper/types";
import { formatSize, formatTime, paperTitle, statusStep } from "@/lib/gopherpaper/utils";
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

export function PaperPane({ onExpandSidebar }: { onExpandSidebar?: () => void }) {
  const { papers, activePaper, activePaperID, selectPaper, uploadPaper, refreshPapers } = useApp();
  const guard = useGuard();
  const fileRef = useRef<HTMLInputElement>(null);
  const [picked, setPicked] = useState<File | null>(null);
  const [uploading, setUploading] = useState(false);
  const [uploadOpen, setUploadOpen] = useState(false);
  const [query, setQuery] = useState("");

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
    <aside className="flex min-h-0 flex-col border-r bg-muted/30">
      <div className="space-y-4 p-5 pb-4">
        <div className="flex items-center justify-between gap-2">
          <div className="flex items-center gap-1.5">
            {onExpandSidebar && (
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                onClick={onExpandSidebar}
                title="展开侧栏"
                className="hidden lg:inline-flex"
              >
                <PanelLeftOpen className="size-4" />
              </Button>
            )}
            <div className="flex items-baseline gap-2">
              <h2 className="text-[17px] font-bold tracking-tight">论文库</h2>
              {papers.length > 0 && (
                <span className="font-mono text-xs text-muted-foreground">{papers.length}</span>
              )}
            </div>
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
        <div className="space-y-1 p-2.5">
          {papers.length === 0 ? (
            <Empty title="还没有论文" text="点击右上角「上传」加入第一篇 PDF。" compact />
          ) : (
            papers.map((p) => (
              <button
                key={p.id}
                type="button"
                className={cn(
                  "w-full rounded-xl px-3.5 py-3 text-left transition-colors",
                  p.id === activePaperID
                    ? "bg-card shadow-sm ring-1 ring-border"
                    : "hover:bg-card/70",
                )}
                onClick={() => selectPaper(p.id)}
              >
                <div className="flex items-start justify-between gap-2.5">
                  <strong className="line-clamp-2 text-[13.5px] font-semibold leading-snug">
                    {paperTitle(p)}
                  </strong>
                  <StatusBadge status={p.status} />
                </div>
                <p className="mt-2 line-clamp-1 text-xs text-muted-foreground">
                  {[p.file_name, formatSize(p.size), formatTime(p.updated_at || p.created_at)]
                    .filter(Boolean)
                    .join(" · ")}
                </p>
              </button>
            ))
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
