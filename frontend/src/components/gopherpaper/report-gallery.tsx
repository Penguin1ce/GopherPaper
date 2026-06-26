"use client";

import { ArrowLeft, FileText } from "lucide-react";
import Link from "next/link";

import { buttonVariants } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { useApp } from "@/lib/gopherpaper/store";
import { paperTitle } from "@/lib/gopherpaper/utils";
import { cn } from "@/lib/utils";
import { Empty } from "./app-ui";
import { ReportPanel } from "./report-panel";
import { WorkspaceFrame, WorkspacePanel } from "./workspace-frame";

// ReportGallery 是小囊鼠研读报告的独立画廊页:左侧论文列表,右侧选中论文的报告卡与正文。
// 复用工作台的 store(papers/报告进度/SSE 全在 AppProvider 里),右栏直接复用 ReportPanel。
export function ReportGallery() {
  const { authed, papers, activePaperID, selectPaper } = useApp();

  if (!authed) {
    return (
      <main className="flex h-dvh items-center justify-center bg-muted/50 p-6">
        <div className="text-center">
          <Empty title="请先登录" text="登录后即可查看与生成研读报告。" />
          <Link href="/" className={cn(buttonVariants({ variant: "outline" }), "mt-4")}>
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
            <div className="min-w-0">
              <div className="truncate text-sm font-medium">小囊鼠</div>
              <div className="truncate text-xs text-muted-foreground">
                生成结构化报告 · 导出 PDF
              </div>
            </div>
          </div>
        </div>
        <ScrollArea className="min-h-0 flex-1">
          <div className="px-3 pb-2 pt-3">
            <div className="flex items-baseline justify-between gap-2 px-1 pb-2">
              <div className="text-sm font-medium">论文库</div>
              <div className="font-mono text-xs text-muted-foreground">
                {papers.length > 0 ? `${papers.length}` : "等待上传"}
              </div>
            </div>
            <div className="space-y-1">
              {papers.length === 0 ? (
                <Empty title="还没有论文" text="先在工作台上传并解析论文。" compact />
              ) : (
                papers.map((p) => (
                  <button
                    key={p.id}
                    type="button"
                    onClick={() => selectPaper(p.id)}
                    className={cn(
                      "relative flex w-full items-start gap-2 rounded-md px-2.5 py-2 text-left transition-colors",
                      p.id === activePaperID
                        ? "bg-card shadow-sm before:absolute before:inset-y-1.5 before:left-0 before:w-0.5 before:rounded-full before:bg-sienna"
                        : "hover:bg-accent/60",
                    )}
                  >
                    <FileText
                      className={cn(
                        "mt-0.5 size-3.5 shrink-0",
                        p.id === activePaperID ? "text-sienna" : "text-muted-foreground",
                      )}
                    />
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-sm font-medium">{paperTitle(p)}</span>
                      {p.status !== "ready" && (
                        <span className="text-xs text-muted-foreground">{p.status}</span>
                      )}
                    </span>
                  </button>
                ))
              )}
            </div>
          </div>
        </ScrollArea>
      </WorkspacePanel>

      <WorkspacePanel className="flex min-w-0 flex-1 flex-col">
        <section className="min-h-0 flex-1 overflow-hidden">
          <ReportPanel />
        </section>
      </WorkspacePanel>
    </WorkspaceFrame>
  );
}
