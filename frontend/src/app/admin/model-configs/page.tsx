"use client";

import { ArrowLeft, Loader2, LogOut, ShieldCheck } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";

import {
  ADMIN_AUTH_KEY,
  readSavedAuth,
  type AdminAuth,
} from "@/components/gopherpaper/admin-auth";
import { AdminModelConfigPanel } from "@/components/gopherpaper/admin-model-config-panel";
import { Button } from "@/components/ui/button";

export default function AdminModelConfigsPage() {
  const [auth, setAuth] = useState<AdminAuth | null>(null);
  const [hydrated, setHydrated] = useState(false);

  useEffect(() => {
    setAuth(readSavedAuth());
    setHydrated(true);
  }, []);

  const onLogout = () => {
    window.localStorage.removeItem(ADMIN_AUTH_KEY);
    setAuth(null);
  };

  return (
    <main className="min-h-dvh bg-background text-foreground">
      <div className="mx-auto flex min-h-dvh w-full max-w-[92rem] flex-col px-4 py-5 sm:px-6 lg:px-8">
        <header className="flex flex-wrap items-center justify-between gap-3 border-b border-border pb-4">
          <Link
            href="/admin"
            className="group flex items-center gap-2.5 rounded-md outline-none focus-visible:ring-2 focus-visible:ring-ring/60"
          >
            <span className="flex size-9 items-center justify-center rounded-lg bg-primary/10 text-primary">
              <ShieldCheck className="size-5" />
            </span>
            <span>
              <span className="block text-sm font-semibold tracking-tight">
                GopherPaper Admin
              </span>
              <span className="block text-xs text-muted-foreground">模型配置中心</span>
            </span>
          </Link>
          <div className="flex items-center gap-2">
            <Link
              href="/admin"
              className="inline-flex h-8 items-center justify-center gap-1.5 rounded-lg border border-border bg-background px-2.5 text-sm font-medium outline-none transition hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring"
            >
              <ArrowLeft className="size-4" />
              返回后台
            </Link>
            {auth ? (
              <Button variant="ghost" size="sm" onClick={onLogout}>
                <LogOut className="size-4" />
                退出
              </Button>
            ) : null}
          </div>
        </header>

        <section className="flex flex-1 flex-col gap-5 py-5">
          {!hydrated ? (
            <div className="flex items-center justify-center gap-2 rounded-lg border border-border bg-card py-12 text-sm text-muted-foreground">
              <Loader2 className="size-4 animate-spin" />
              正在读取管理员登录态
            </div>
          ) : auth ? (
            <AdminModelConfigPanel token={auth.token} />
          ) : (
            <div className="rounded-lg border border-border bg-card p-6 text-center">
              <h1 className="text-base font-semibold">需要管理员登录</h1>
              <p className="mt-2 text-sm text-muted-foreground">
                模型配置涉及密钥和运行时配置，请先回到管理员后台登录。
              </p>
              <Link
                href="/admin"
                className="mt-4 inline-flex h-9 items-center justify-center rounded-lg bg-primary px-3 text-sm font-medium text-primary-foreground outline-none transition hover:bg-primary/80 focus-visible:ring-2 focus-visible:ring-ring"
              >
                去登录
              </Link>
            </div>
          )}
        </section>
      </div>
    </main>
  );
}
