"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Loader2, ShieldAlert } from "lucide-react";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { type AuditItem, fetchAudit, formatDateTime } from "./console-api";

const PAGE_SIZE = 20;

const METHOD_STYLE: Record<string, string> = {
  POST: "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400",
  PUT: "bg-amber-500/10 text-amber-600 dark:text-amber-400",
  PATCH: "bg-sky-500/10 text-sky-600 dark:text-sky-400",
  DELETE: "bg-red-500/10 text-red-600 dark:text-red-400",
};

export function AuditPanel({ token }: { token: string }) {
  const [items, setItems] = useState<AuditItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(false);

  const load = useCallback(
    async (nextPage = page) => {
      if (!token) return;
      setLoading(true);
      try {
        const res = await fetchAudit(token, { page: nextPage, page_size: PAGE_SIZE });
        setItems(res.items ?? []);
        setTotal(res.total ?? 0);
        setPage(res.page ?? nextPage);
      } catch {
        setItems([]);
      } finally {
        setLoading(false);
      }
    },
    [page, token],
  );

  useEffect(() => {
    void load(1);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token]);

  const pageCount = useMemo(() => Math.max(1, Math.ceil(total / PAGE_SIZE)), [total]);

  return (
    <div className="rounded-xl border border-border bg-card p-4 shadow-sm">
      <div className="mb-3 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <ShieldAlert className="size-4 text-primary" />
          <h3 className="text-sm font-semibold">操作审计</h3>
          <span className="text-xs text-muted-foreground">共 {total} 条</span>
        </div>
        <Button size="sm" variant="outline" onClick={() => void load(page)} disabled={loading}>
          {loading ? <Loader2 className="size-4 animate-spin" /> : null}
          刷新
        </Button>
      </div>

      <div className="overflow-x-auto">
        <table className="w-full min-w-[48rem] text-left text-sm">
          <thead className="bg-muted/40 text-xs uppercase text-muted-foreground">
            <tr>
              <th className="px-3 py-2 font-medium">方法</th>
              <th className="px-3 py-2 font-medium">路径</th>
              <th className="px-3 py-2 font-medium">管理员</th>
              <th className="px-3 py-2 font-medium">状态</th>
              <th className="px-3 py-2 font-medium">IP</th>
              <th className="px-3 py-2 font-medium">时间</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={6} className="px-3 py-10 text-center text-muted-foreground">
                  <Loader2 className="mx-auto mb-2 size-5 animate-spin" />
                  加载中
                </td>
              </tr>
            ) : items.length === 0 ? (
              <tr>
                <td colSpan={6} className="px-3 py-10 text-center text-muted-foreground">
                  暂无审计记录
                </td>
              </tr>
            ) : (
              items.map((a) => (
                <tr key={a.id} className="border-t border-border">
                  <td className="px-3 py-2.5">
                    <span
                      className={cn(
                        "rounded px-2 py-0.5 text-xs font-medium",
                        METHOD_STYLE[a.method] ?? "bg-muted text-muted-foreground",
                      )}
                    >
                      {a.method}
                    </span>
                  </td>
                  <td className="px-3 py-2.5 font-mono text-xs">{a.path}</td>
                  <td className="px-3 py-2.5 text-muted-foreground">{a.admin_name || `#${a.admin_id}`}</td>
                  <td className="px-3 py-2.5">
                    <span
                      className={cn(
                        "tabular-nums",
                        a.status >= 200 && a.status < 300
                          ? "text-emerald-600 dark:text-emerald-400"
                          : "text-red-600 dark:text-red-400",
                      )}
                    >
                      {a.status}
                    </span>
                  </td>
                  <td className="px-3 py-2.5 text-xs text-muted-foreground">{a.ip}</td>
                  <td className="px-3 py-2.5 text-xs text-muted-foreground">{formatDateTime(a.created_at)}</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      <div className="mt-3 flex items-center justify-between text-sm text-muted-foreground">
        <span>
          第 {page} / {pageCount} 页
        </span>
        <div className="flex gap-2">
          <Button size="sm" variant="outline" disabled={page <= 1 || loading} onClick={() => void load(page - 1)}>
            上一页
          </Button>
          <Button size="sm" variant="outline" disabled={page >= pageCount || loading} onClick={() => void load(page + 1)}>
            下一页
          </Button>
        </div>
      </div>
    </div>
  );
}
