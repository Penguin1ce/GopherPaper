"use client";

import { useCallback, useEffect, useState } from "react";
import { Loader2, ShieldCheck } from "lucide-react";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { type AdminAccount, fetchAdmins, formatDateTime, relativeTime, setAdminStatus } from "./console-api";

export function AdminsPanel({ token }: { token: string }) {
  const [items, setItems] = useState<AdminAccount[]>([]);
  const [loading, setLoading] = useState(false);
  const [busyId, setBusyId] = useState<number | null>(null);

  const load = useCallback(async () => {
    if (!token) return;
    setLoading(true);
    try {
      const res = await fetchAdmins(token);
      setItems(res.items ?? []);
    } catch {
      setItems([]);
    } finally {
      setLoading(false);
    }
  }, [token]);

  useEffect(() => {
    void load();
  }, [load]);

  const onToggle = async (a: AdminAccount) => {
    setBusyId(a.id);
    try {
      await setAdminStatus(token, a.id, a.status === "active" ? "disabled" : "active");
      await load();
    } catch {
      // ignore
    } finally {
      setBusyId(null);
    }
  };

  return (
    <div className="rounded-xl border border-border bg-card p-4 shadow-sm">
      <div className="mb-3 flex items-center gap-2">
        <ShieldCheck className="size-4 text-primary" />
        <h3 className="text-sm font-semibold">管理员账号</h3>
        <span className="text-xs text-muted-foreground">共 {items.length} 个</span>
      </div>

      <div className="overflow-x-auto">
        <table className="w-full min-w-[40rem] text-left text-sm">
          <thead className="bg-muted/40 text-xs uppercase text-muted-foreground">
            <tr>
              <th className="px-3 py-2 font-medium">用户名</th>
              <th className="px-3 py-2 font-medium">邮箱</th>
              <th className="px-3 py-2 font-medium">状态</th>
              <th className="px-3 py-2 font-medium">最近登录</th>
              <th className="px-3 py-2 text-right font-medium">操作</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={5} className="px-3 py-10 text-center text-muted-foreground">
                  <Loader2 className="mx-auto mb-2 size-5 animate-spin" />
                  加载中
                </td>
              </tr>
            ) : (
              items.map((a) => {
                const active = a.status === "active";
                return (
                  <tr key={a.id} className="border-t border-border">
                    <td className="px-3 py-2.5">
                      <p className="font-medium">{a.username}</p>
                      <p className="text-xs text-muted-foreground">{a.name || "—"}</p>
                    </td>
                    <td className="px-3 py-2.5 text-muted-foreground">{a.email}</td>
                    <td className="px-3 py-2.5">
                      <span
                        className={cn(
                          "rounded px-2 py-0.5 text-xs font-medium",
                          active
                            ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400"
                            : "bg-muted text-muted-foreground",
                        )}
                      >
                        {active ? "启用" : "停用"}
                      </span>
                    </td>
                    <td className="px-3 py-2.5 text-xs text-muted-foreground">
                      {a.last_login_at ? relativeTime(a.last_login_at) : "从未"}
                      <span className="ml-1 opacity-60">
                        {a.last_login_at ? `(${formatDateTime(a.last_login_at)})` : ""}
                      </span>
                    </td>
                    <td className="px-3 py-2.5 text-right">
                      <Button size="sm" variant="outline" disabled={busyId === a.id} onClick={() => void onToggle(a)}>
                        {busyId === a.id ? <Loader2 className="size-4 animate-spin" /> : active ? "停用" : "启用"}
                      </Button>
                    </td>
                  </tr>
                );
              })
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
