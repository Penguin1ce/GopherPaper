"use client";

import { Coffee, LogOut, Network, NotebookText, PanelLeftClose, Plus } from "lucide-react";
import Link from "next/link";

import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Button, buttonVariants } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { useApp } from "@/lib/gopherpaper/store";
import { useGuard } from "./app-ui";

export function Sidebar({ onCollapse }: { onCollapse?: () => void }) {
  const { user, logout, activePaperID, createSession } = useApp();
  const guard = useGuard();
  const fallback = (user?.name || user?.student_id || "G").slice(0, 1).toUpperCase();

  return (
    <div className="shrink-0 bg-background">
      <div className="p-4">
        <div className="flex items-center gap-3">
          <Link
            href="/profile"
            className="rounded-lg outline-none ring-ring/50 transition hover:opacity-90 focus-visible:ring-3"
            title="个人中心"
          >
            <Avatar key={user?.avatar_url || fallback} className="size-9 rounded-lg">
              {user?.avatar_url && (
                <AvatarImage src={user.avatar_url} alt="用户头像" className="rounded-md" />
              )}
              <AvatarFallback className="rounded-md bg-primary font-serif font-semibold text-primary-foreground">
                {fallback}
              </AvatarFallback>
            </Avatar>
          </Link>
          <div className="min-w-0 flex-1">
            <div className="truncate text-sm font-medium">{user?.name || user?.student_id}</div>
            <div className="truncate text-xs text-muted-foreground">{user?.email || "已登录"}</div>
          </div>
          <Button type="button" variant="ghost" size="icon" onClick={() => logout()} title="退出登录">
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
          variant="default"
          className="h-10 w-full justify-start"
          onClick={() =>
            guard(async () => {
              await createSession("新会话", activePaperID || undefined);
            })
          }
        >
          <Plus className="size-4" />
          新建对话
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
          href="/graph"
        >
          <Network className="size-4" />
          知识图谱 · 论文关系
        </Link>
        <Link
          className={buttonVariants({ variant: "outline", className: "h-10 w-full justify-start" })}
          href="/pioneer"
        >
          <Coffee className="size-4" />
          工具助手 · 小云雀
        </Link>
      </div>
    </div>
  );
}
