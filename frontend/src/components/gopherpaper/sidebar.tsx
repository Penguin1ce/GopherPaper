"use client";

import { Coffee, LogOut, NotebookText, PanelLeftClose, Plus, Trash2 } from "lucide-react";
import Link from "next/link";

import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Button, buttonVariants } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Separator } from "@/components/ui/separator";
import { cn } from "@/lib/utils";
import { useApp } from "@/lib/gopherpaper/store";
import { formatTime, sessionsForPaper, sessionTitle } from "@/lib/gopherpaper/utils";
import { Empty, useGuard } from "./app-ui";

export function Sidebar({ onCollapse }: { onCollapse?: () => void }) {
  const {
    user,
    sessions,
    activePaperID,
    activeSessionID,
    logout,
    openSession,
    createSession,
    removeSession,
  } = useApp();
  const guard = useGuard();
  // 会话归属当前论文,只列出选中论文的会话,与右栏论文选择保持一致。
  const visibleSessions = sessionsForPaper(sessions, activePaperID);

  return (
    <div className="flex h-full flex-col bg-background">
      <div className="p-4">
        <div className="flex items-center gap-3">
          <Avatar className="size-9 rounded-lg">
            <AvatarFallback className="rounded-md bg-primary font-serif font-semibold text-primary-foreground">
              {(user?.name || user?.student_id || "G").slice(0, 1).toUpperCase()}
            </AvatarFallback>
          </Avatar>
          <div className="min-w-0 flex-1">
            <div className="truncate text-sm font-medium">{user?.name || user?.student_id}</div>
            <div className="truncate text-xs text-muted-foreground">{user?.email || "已登录"}</div>
          </div>
          <Button type="button" variant="ghost" size="icon" onClick={logout} title="退出登录">
            <LogOut className="size-4" />
          </Button>
          {onCollapse && (
            <Button
              type="button"
              variant="ghost"
              size="icon"
              onClick={onCollapse}
              title="收起侧栏"
              className="hidden lg:inline-flex"
            >
              <PanelLeftClose className="size-4" />
            </Button>
          )}
        </div>
      </div>
      <Separator />
      <div className="space-y-2 p-4">
        <Button
          type="button"
          className="h-10 w-full justify-start"
          variant="secondary"
          onClick={() => guard(async () => {
            await createSession("新会话", activePaperID || undefined);
          })}
        >
          <Plus className="size-4" />
          新建会话
        </Button>
        <Link
          className={buttonVariants({ variant: "outline", className: "h-10 w-full justify-start" })}
          href="/reports"
        >
          <NotebookText className="size-4" />
          研读报告 · 小囊鼠
        </Link>
        <Link
          className={buttonVariants({ variant: "outline", className: "h-10 w-full justify-start" })}
          href="/pioneer"
        >
          <Coffee className="size-4" />
          工具助手 · 小云雀
        </Link>
      </div>
      <Separator />
      <div className="px-4 py-3">
        <div className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
          会话
        </div>
      </div>
      <ScrollArea className="min-h-0 flex-1">
        <div className="space-y-1 px-3 pb-4">
          {visibleSessions.length === 0 ? (
            <Empty title="还没有会话" text="向当前论文提问会自动创建会话。" compact />
          ) : (
            visibleSessions.map((s) => (
              <div
                key={s.id}
                className={cn(
                  "group relative flex items-center gap-1 rounded-md p-1",
                  s.id === activeSessionID
                    ? "bg-card shadow-sm before:absolute before:inset-y-1.5 before:left-0 before:w-0.5 before:rounded-full before:bg-sienna"
                    : "hover:bg-accent/60",
                )}
              >
                <button
                  type="button"
                  className="min-w-0 flex-1 rounded-md px-2 py-2 text-left"
                  onClick={() => guard(() => openSession(s.id))}
                >
                  <div className="truncate text-sm font-medium">{sessionTitle(s)}</div>
                  <div className="mt-0.5 text-xs text-muted-foreground">
                    {formatTime(s.updated_at || s.created_at)}
                  </div>
                </button>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-sm"
                  className="opacity-0 group-hover:opacity-100"
                  onClick={() => guard(() => removeSession(s.id))}
                  title="删除会话"
                >
                  <Trash2 className="size-3.5" />
                </Button>
              </div>
            ))
          )}
        </div>
      </ScrollArea>
    </div>
  );
}
