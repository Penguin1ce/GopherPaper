"use client";

import { ArrowLeft, Loader2, SlidersHorizontal } from "lucide-react";
import Link from "next/link";

import { AdminModelConfigPanel } from "@/components/gopherpaper/admin-model-config-panel";
import { Empty } from "@/components/gopherpaper/app-ui";
import { buttonVariants } from "@/components/ui/button";
import { userModelConfigRequest } from "@/lib/gopherpaper/api";
import { useApp } from "@/lib/gopherpaper/store";
import { cn } from "@/lib/utils";

export default function ProfileModelManagementPage() {
  const { authReady, authed } = useApp();

  if (!authReady) {
    return (
      <main className="grid min-h-dvh place-items-center bg-background text-foreground">
        <div className="inline-flex items-center gap-2 rounded-2xl border bg-card px-4 py-3 text-sm text-muted-foreground shadow-sm">
          <Loader2 className="size-4 animate-spin" />
          正在读取登录状态
        </div>
      </main>
    );
  }

  if (!authed) {
    return (
      <main className="grid min-h-dvh place-items-center bg-background text-foreground">
        <div className="text-center">
          <Empty title="请先登录" text="登录后即可进入模型管理。" />
          <Link href="/" className={cn(buttonVariants({ variant: "outline" }), "mt-4")}>
            返回登录页
          </Link>
        </div>
      </main>
    );
  }

  return (
    <main className="min-h-dvh bg-background text-foreground">
      <div className="mx-auto flex min-h-dvh w-full max-w-[92rem] flex-col px-4 py-5 sm:px-6 lg:px-8">
        <header className="flex flex-wrap items-center justify-between gap-3 border-b border-border pb-4">
          <Link
            href="/profile"
            className="group flex items-center gap-2.5 rounded-md outline-none focus-visible:ring-2 focus-visible:ring-ring/60"
          >
            <span className="flex size-9 items-center justify-center rounded-lg bg-primary/10 text-primary">
              <SlidersHorizontal className="size-5" />
            </span>
            <span>
              <span className="block text-sm font-semibold tracking-tight">
                模型管理
              </span>
              <span className="block text-xs text-muted-foreground">
                登录账号下的模型服务配置
              </span>
            </span>
          </Link>
          <Link
            href="/profile"
            className={cn(buttonVariants({ variant: "ghost", size: "sm" }), "gap-1.5")}
          >
            <ArrowLeft className="size-4" />
            返回个人中心
          </Link>
        </header>

        <section className="flex flex-1 flex-col gap-5 py-5">
          <AdminModelConfigPanel
            basePath="/user/model-configs"
            request={userModelConfigRequest}
            title="模型管理"
            description="管理当前登录账号自己的模型服务配置。个人配置只影响当前账号；未配置的角色继续使用系统默认，Embedding 与 Rerank 仍由管理员统一维护。"
            scope="user"
          />
        </section>
      </div>
    </main>
  );
}
