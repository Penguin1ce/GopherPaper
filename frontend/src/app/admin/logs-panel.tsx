"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { AlertCircle, CheckCircle2, Loader2, ScrollText, XCircle } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import {
  type LogItem,
  type LogStats,
  fetchLogStats,
  fetchLogs,
  formatDateTime,
} from "./console-api";

const PAGE_SIZE = 20;

export function LogsPanel({ token }: { token: string }) {
  const [items, setItems] = useState<LogItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [serviceType, setServiceType] = useState("");
  const [result, setResult] = useState("");
  const [actor, setActor] = useState("");
  const [stats, setStats] = useState<LogStats | null>(null);
  const [loading, setLoading] = useState(false);
  const [expanded, setExpanded] = useState<number | null>(null);

  const load = useCallback(
    async (nextPage = page) => {
      if (!token) return;
      setLoading(true);
      try {
        const res = await fetchLogs(token, {
          page: nextPage,
          page_size: PAGE_SIZE,
          service_type: serviceType,
          result,
          actor,
        });
        setItems(res.items ?? []);
        setTotal(res.total ?? 0);
        setPage(res.page ?? nextPage);
      } catch {
        setItems([]);
      } finally {
        setLoading(false);
      }
    },
    [actor, page, result, serviceType, token],
  );

  useEffect(() => {
    void load(1);
    void fetchLogStats(token)
      .then((s) => setStats(s))
      .catch(() => {});
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token]);

  const pageCount = useMemo(() => Math.max(1, Math.ceil(total / PAGE_SIZE)), [total]);

  return (
    <div className="rounded-xl border border-border bg-card p-4 shadow-sm">
      <div className="mb-3 flex items-center gap-2">
        <ScrollText className="size-4 text-primary" />
        <h3 className="text-sm font-semibold">服务调用日志</h3>
        {stats && (
          <span className="text-xs text-muted-foreground">
            共 {stats.total} 条 · 成功 {stats.success} · 失败 {stats.failed} · 平均{" "}
            {Math.round(stats.avg_ms)}ms
          </span>
        )}
      </div>

      <div className="mb-3 flex flex-wrap items-center gap-2">
        <select
          className="h-8 rounded-lg border border-input bg-background px-2 text-sm outline-none"
          value={serviceType}
          onChange={(e) => setServiceType(e.target.value)}
        >
          <option value="">全部服务</option>
          {(stats?.service_types ?? []).map((t) => (
            <option key={t} value={t}>
              {t}
            </option>
          ))}
        </select>
        <select
          className="h-8 rounded-lg border border-input bg-background px-2 text-sm outline-none"
          value={result}
          onChange={(e) => setResult(e.target.value)}
        >
          <option value="">全部结果</option>
          <option value="success">仅成功</option>
          <option value="failed">仅失败</option>
        </select>
        <Input
          className="h-8 w-40"
          placeholder="调用者 actor"
          value={actor}
          onChange={(e) => setActor(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") void load(1);
          }}
        />
        <Button size="sm" variant="outline" onClick={() => void load(1)} disabled={loading}>
          {loading ? <Loader2 className="size-4 animate-spin" /> : null}
          筛选
        </Button>
      </div>

      <div className="overflow-x-auto">
        <table className="w-full min-w-[52rem] text-left text-sm">
          <thead className="bg-muted/40 text-xs uppercase text-muted-foreground">
            <tr>
              <th className="px-3 py-2 font-medium">结果</th>
              <th className="px-3 py-2 font-medium">服务</th>
              <th className="px-3 py-2 font-medium">调用者</th>
              <th className="px-3 py-2 font-medium">耗时</th>
              <th className="px-3 py-2 font-medium">时间</th>
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
            ) : items.length === 0 ? (
              <tr>
                <td colSpan={5} className="px-3 py-10 text-center text-muted-foreground">
                  暂无日志
                </td>
              </tr>
            ) : (
              items.map((log) => (
                <FragmentRow
                  key={log.id}
                  log={log}
                  expanded={expanded === log.id}
                  onToggle={() => setExpanded(expanded === log.id ? null : log.id)}
                />
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
          <Button
            size="sm"
            variant="outline"
            disabled={page >= pageCount || loading}
            onClick={() => void load(page + 1)}
          >
            下一页
          </Button>
        </div>
      </div>
    </div>
  );
}

function FragmentRow({
  log,
  expanded,
  onToggle,
}: {
  log: LogItem;
  expanded: boolean;
  onToggle: () => void;
}) {
  const slow = log.duration_ms >= 3000;
  return (
    <>
      <tr
        className={cn("cursor-pointer border-t border-border hover:bg-muted/40", !log.success && "bg-red-500/5")}
        onClick={onToggle}
      >
        <td className="px-3 py-2.5">
          {log.success ? (
            <span className="inline-flex items-center gap-1 text-xs text-emerald-600 dark:text-emerald-400">
              <CheckCircle2 className="size-3.5" /> 成功
            </span>
          ) : (
            <span className="inline-flex items-center gap-1 text-xs text-red-600 dark:text-red-400">
              <XCircle className="size-3.5" /> 失败
            </span>
          )}
        </td>
        <td className="px-3 py-2.5">
          <span className="rounded bg-muted px-2 py-0.5 text-xs">{log.service_type}</span>
        </td>
        <td className="px-3 py-2.5 text-xs text-muted-foreground">{log.actor_id || "—"}</td>
        <td className={cn("px-3 py-2.5 tabular-nums", slow && "font-medium text-amber-600 dark:text-amber-400")}>
          {log.duration_ms}ms
        </td>
        <td className="px-3 py-2.5 text-xs text-muted-foreground">{formatDateTime(log.created_at)}</td>
      </tr>
      {expanded && (
        <tr className="border-t border-border/60 bg-muted/20">
          <td colSpan={5} className="px-3 py-2.5">
            <div className="grid gap-1 text-xs text-muted-foreground sm:grid-cols-2">
              <div>日志 ID:{log.id}</div>
              <div>论文:{log.paper_id || "—"}</div>
              <div>会话:{log.session_id || "—"}</div>
              <div>耗时:{log.duration_ms}ms</div>
            </div>
            {log.error_message && (
              <div className="mt-2 flex items-start gap-1.5 rounded-lg bg-red-500/10 p-2 text-xs text-red-700 dark:text-red-300">
                <AlertCircle className="mt-0.5 size-3.5 shrink-0" />
                <span className="break-all font-mono">{log.error_message}</span>
              </div>
            )}
          </td>
        </tr>
      )}
    </>
  );
}
