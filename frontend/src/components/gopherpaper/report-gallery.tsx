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
    <main className="flex h-dvh min-h-[640px] flex-col overflow-hidden bg-muted/50">
      <header className="flex h-14 shrink-0 items-center gap-3 border-b bg-background px-4">
        <Link
          href="/"
          className={buttonVariants({ variant: "ghost", size: "icon", className: "shrink-0" })}
          title="返回工作台"
        >
          <ArrowLeft className="size-4" />
        </Link>
        <div className="min-w-0">
          <h1 className="font-serif text-base font-semibold tracking-tight">研读报告 · 小囊鼠</h1>
          <p className="truncate text-xs text-muted-foreground">
            选一篇论文,让小囊鼠按需生成结构化研读报告并导出 PDF
          </p>
        </div>
      </header>

      <div className="grid min-h-0 flex-1 grid-cols-1 lg:grid-cols-[18rem_minmax(0,1fr)]">
        <aside className="hidden min-h-0 flex-col border-r bg-background lg:flex">
          <div className="px-4 py-3 text-xs font-medium uppercase tracking-wide text-muted-foreground">
            论文
          </div>
          <ScrollArea className="min-h-0 flex-1">
            <div className="space-y-1 px-3 pb-4">
              {papers.length === 0 ? (
                <Empty title="还没有论文" text="先在工作台上传并解析论文。" compact />
              ) : (
                papers.map((p) => (
                  <button
                    key={p.id}
                    type="button"
                    onClick={() => selectPaper(p.id)}
                    className={cn(
                      "flex w-full items-start gap-2 rounded-md px-2.5 py-2 text-left transition-colors",
                      p.id === activePaperID
                        ? "bg-card shadow-sm ring-1 ring-sienna/20"
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
          </ScrollArea>
        </aside>

        <section className="min-h-0 overflow-hidden bg-background">
          <ReportPanel />
        </section>
      </div>
    </main>
  );
}
