"use client";

import {
  Library,
  LogOut,
  Network,
  NotebookText,
  PanelLeftClose,
  Search,
  type LucideIcon,
} from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";

import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { useApp } from "@/lib/gopherpaper/store";
import { cn } from "@/lib/utils";
import { AGENT_INTROS } from "./agent-intro";

interface NavItem {
  href: string;
  icon: LucideIcon;
  label: string;
  desc: string;
  agent?: string;
}

const NAV_ITEMS: NavItem[] = [
  {
    href: "/library",
    icon: Library,
    label: "论文库",
    desc: "浏览与管理全部论文",
  },
  {
    href: "/reports",
    icon: NotebookText,
    ...AGENT_INTROS.reports,
  },
  {
    href: "/graph",
    icon: Network,
    ...AGENT_INTROS.graph,
  },
  {
    href: "/pioneer",
    icon: Search,
    ...AGENT_INTROS.pioneer,
  },
];

export function Sidebar() {
  const pathname = usePathname();

  return (
    <div className="shrink-0 bg-background">
      <nav className="px-2.5 py-2" aria-label="主要入口">
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
                    {item.agent && (
                      <span className="shrink-0 rounded border border-border/70 px-1.5 py-0.5 text-[10px] leading-none text-muted-foreground">
                        {item.agent}
                      </span>
                    )}
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

export function SidebarUserCard({ onCollapse }: { onCollapse?: () => void }) {
  const { user, logout } = useApp();
  const fallback = (user?.name || user?.student_id || "G").slice(0, 1).toUpperCase();
  const displayName = user?.name || user?.student_id || "GopherPaper 用户";

  return (
    <div className="shrink-0 border-t border-border/60 bg-background px-3 py-2">
      <div className="flex min-h-9 items-center gap-1.5">
        <Link
          href="/profile"
          className="flex min-w-0 flex-1 items-center gap-2 rounded-md px-1.5 py-1 outline-none ring-ring/50 transition-colors hover:bg-muted/45 focus-visible:ring-3"
          title="个人中心"
        >
          <Avatar key={user?.avatar_url || fallback} className="size-7 rounded-full">
            {user?.avatar_url && (
              <AvatarImage src={user.avatar_url} alt="用户头像" className="rounded-full" />
            )}
            <AvatarFallback className="rounded-full bg-primary/10 font-serif font-semibold text-primary">
              {fallback}
            </AvatarFallback>
          </Avatar>
          <span className="block min-w-0 flex-1 truncate text-sm font-medium leading-5">
            {displayName}
          </span>
        </Link>
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          onClick={() => void logout()}
          title="退出登录"
          className="text-muted-foreground hover:text-foreground"
        >
          <LogOut className="size-4" />
        </Button>
        {onCollapse && (
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            onClick={onCollapse}
            title="收起侧栏"
            className="hidden text-muted-foreground hover:text-foreground lg:inline-flex"
          >
            <PanelLeftClose className="size-4" />
          </Button>
        )}
      </div>
    </div>
  );
}
