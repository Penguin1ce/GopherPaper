"use client";

import { LogOut, Network, NotebookText, PanelLeftClose, Search } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";

import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { useApp } from "@/lib/gopherpaper/store";
import { cn } from "@/lib/utils";

const NAV_ITEMS = [
  {
    href: "/reports",
    icon: NotebookText,
    label: "研读与报告",
    desc: "速读、方法、结果、相关研究",
    agent: "小囊鼠",
  },
  {
    href: "/graph",
    icon: Network,
    label: "论文关系图谱",
    desc: "查看论文、作者、关键词关系",
    agent: "知识库",
  },
  {
    href: "/pioneer",
    icon: Search,
    label: "学术探索引擎",
    desc: "Ask me anything",
    agent: "小云雀",
  },
];

export function Sidebar({ onCollapse }: { onCollapse?: () => void }) {
  const { user, logout } = useApp();
  const pathname = usePathname();
  const fallback = (user?.name || user?.student_id || "G").slice(0, 1).toUpperCase();

  return (
    <div className="shrink-0 bg-background">
      <div className="flex h-14 items-center border-b px-3">
        <div className="flex w-full items-center gap-2.5">
          <Link
            href="/profile"
            className="rounded-lg outline-none ring-ring/50 transition hover:opacity-90 focus-visible:ring-3"
            title="个人中心"
          >
            <Avatar key={user?.avatar_url || fallback} className="size-8 rounded-lg">
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
          <Button type="button" variant="ghost" size="icon-sm" onClick={() => logout()} title="退出登录">
            <LogOut className="size-4" />
          </Button>
          {onCollapse && (
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              onClick={onCollapse}
              title="收起侧栏"
              className="hidden lg:inline-flex"
            >
              <PanelLeftClose className="size-4" />
            </Button>
          )}
        </div>
      </div>
      <nav className="border-b px-2.5 py-2" aria-label="主要入口">
        <div className="space-y-1">
          {NAV_ITEMS.map((item) => {
            const Icon = item.icon;
            const active = pathname === item.href || pathname.startsWith(`${item.href}/`);
            return (
              <Link
                key={item.href}
                href={item.href}
                aria-current={active ? "page" : undefined}
                className={cn(
                  "group flex min-h-12 items-center gap-2.5 rounded-lg px-2.5 py-2 text-left outline-none transition-colors duration-200 focus-visible:ring-2 focus-visible:ring-ring/50",
                  active
                    ? "bg-sienna/[0.08] text-foreground ring-1 ring-sienna/15"
                    : "text-muted-foreground hover:bg-accent/55 hover:text-foreground",
                )}
              >
                <span
                  className={cn(
                    "grid size-7 shrink-0 place-items-center rounded-md border bg-background transition-colors",
                    active
                      ? "border-sienna/25 text-sienna"
                      : "border-border text-muted-foreground group-hover:text-foreground",
                  )}
                >
                  <Icon className="size-3.5" />
                </span>
                <span className="min-w-0 flex-1">
                  <span className="flex items-center gap-2">
                    <span className="truncate text-sm font-semibold leading-5">{item.label}</span>
                    <span className="shrink-0 rounded border border-border/70 px-1.5 py-0.5 text-[10px] leading-none text-muted-foreground">
                      {item.agent}
                    </span>
                  </span>
                  <span className="block truncate text-xs leading-4 text-muted-foreground">
                    {item.desc}
                  </span>
                </span>
              </Link>
            );
          })}
        </div>
      </nav>
    </div>
  );
}
