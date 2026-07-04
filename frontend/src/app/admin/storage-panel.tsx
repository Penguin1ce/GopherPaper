"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { HardDrive, Loader2, RefreshCw } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  type StorageOverview,
  fetchStorageOverview,
  formatBytes,
  formatDateTime,
} from "./console-api";

const STATUS_LABEL: Record<string, string> = {
  uploaded: "待解析",
  parsing: "解析中",
  extracted: "已抽取",
  indexed: "已入库",
  ready: "就绪",
  failed: "失败",
};

const STATUS_COLOR: Record<string, string> = {
  uploaded: "bg-slate-400",
  parsing: "bg-amber-400",
  extracted: "bg-sky-400",
  indexed: "bg-violet-400",
  ready: "bg-emerald-500",
  failed: "bg-rose-500",
};

function statusLabel(s: string) {
  return STATUS_LABEL[s] ?? s;
}

export function StoragePanel({ token }: { token: string }) {
  const [data, setData] = useState<StorageOverview | null>(null);
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    if (!token) return;
    setLoading(true);
    try {
      const res = await fetchStorageOverview(token, 10);
      // 后端空切片会序列化成 JSON null,空库时兜底成空数组,避免渲染处 .length/.map 崩溃
      setData({
        ...res,
        by_status: res.by_status ?? [],
        buckets: res.buckets ?? [],
        trend: res.trend ?? [],
        top_papers: res.top_papers ?? [],
      });
    } catch {
      setData(null);
    } finally {
      setLoading(false);
    }
  }, [token]);

  useEffect(() => {
    void load();
  }, [load]);

  const maxStatusBytes = useMemo(
    () => Math.max(1, ...(data?.by_status ?? []).map((s) => s.bytes)),
    [data],
  );
  const maxBucketCount = useMemo(
    () => Math.max(1, ...(data?.buckets ?? []).map((b) => b.count)),
    [data],
  );
  const maxTrendBytes = useMemo(
    () => Math.max(1, ...(data?.trend ?? []).map((t) => t.bytes)),
    [data],
  );

  return (
    <div className="rounded-xl border border-border bg-card p-4 shadow-sm">
      <div className="mb-3 flex items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          <HardDrive className="size-4 text-primary" />
          <h3 className="text-sm font-semibold">存储管理</h3>
          {data && (
            <span className="text-xs text-muted-foreground">
              {formatBytes(data.total_bytes)} · {data.total_papers} 篇
            </span>
          )}
        </div>
        <Button size="sm" variant="outline" disabled={loading} onClick={() => void load()}>
          {loading ? <Loader2 className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
          刷新
        </Button>
      </div>

      {loading && !data ? (
        <div className="py-10 text-center text-muted-foreground">
          <Loader2 className="mx-auto mb-2 size-5 animate-spin" />
          加载中
        </div>
      ) : !data ? (
        <p className="py-10 text-center text-sm text-muted-foreground">暂无存储数据</p>
      ) : (
        <div className="flex flex-col gap-5">
          {/* 汇总指标 */}
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
            <StatTile label="总占用" value={formatBytes(data.total_bytes)} />
            <StatTile label="论文文件数" value={String(data.total_papers)} />
            <StatTile label="平均体积" value={formatBytes(data.avg_bytes)} />
            <StatTile label="总页数" value={data.total_pages.toLocaleString("zh-CN")} />
          </div>

          {data.missing_size > 0 && (
            <p className="rounded-lg bg-amber-500/10 px-3 py-1.5 text-xs text-amber-600 dark:text-amber-400">
              有 {data.missing_size} 篇论文未记录文件体积,未计入体积统计。
            </p>
          )}

          <div className="grid gap-5 lg:grid-cols-2">
            {/* 按状态占用 */}
            <section>
              <h4 className="mb-2 text-xs font-medium text-muted-foreground">按解析状态占用</h4>
              <div className="flex flex-col gap-2">
                {data.by_status.length === 0 ? (
                  <p className="text-xs text-muted-foreground">暂无数据</p>
                ) : (
                  data.by_status.map((s) => (
                    <div key={s.status} className="flex items-center gap-2 text-xs">
                      <span className="w-14 shrink-0 text-muted-foreground">{statusLabel(s.status)}</span>
                      <div className="h-3 flex-1 overflow-hidden rounded-full bg-muted/50">
                        <div
                          className={`h-full rounded-full ${STATUS_COLOR[s.status] ?? "bg-primary"}`}
                          style={{ width: `${(s.bytes / maxStatusBytes) * 100}%` }}
                        />
                      </div>
                      <span className="w-16 shrink-0 text-right tabular-nums">{formatBytes(s.bytes)}</span>
                      <span className="w-10 shrink-0 text-right tabular-nums text-muted-foreground">{s.count}</span>
                    </div>
                  ))
                )}
              </div>
            </section>

            {/* 体积分档 */}
            <section>
              <h4 className="mb-2 text-xs font-medium text-muted-foreground">单文件体积分布</h4>
              <div className="flex flex-col gap-2">
                {data.buckets.map((b) => (
                  <div key={b.label} className="flex items-center gap-2 text-xs">
                    <span className="w-14 shrink-0 text-muted-foreground">{b.label}</span>
                    <div className="h-3 flex-1 overflow-hidden rounded-full bg-muted/50">
                      <div
                        className="h-full rounded-full bg-primary/70"
                        style={{ width: `${(b.count / maxBucketCount) * 100}%` }}
                      />
                    </div>
                    <span className="w-10 shrink-0 text-right tabular-nums">{b.count}</span>
                    <span className="w-16 shrink-0 text-right tabular-nums text-muted-foreground">
                      {formatBytes(b.bytes)}
                    </span>
                  </div>
                ))}
              </div>
            </section>
          </div>

          {/* 近12月上传趋势 */}
          <section>
            <h4 className="mb-2 text-xs font-medium text-muted-foreground">近 12 个月上传体积</h4>
            <div className="flex items-end gap-1.5">
              {data.trend.map((t) => (
                <div key={t.month} className="group flex flex-1 flex-col items-center gap-1">
                  <div className="relative flex h-24 w-full items-end">
                    <div
                      className="w-full rounded-t bg-primary/60 transition-colors group-hover:bg-primary"
                      style={{ height: `${Math.max(t.bytes > 0 ? 4 : 0, (t.bytes / maxTrendBytes) * 100)}%` }}
                    />
                    <div className="pointer-events-none absolute -top-8 left-1/2 z-10 hidden -translate-x-1/2 whitespace-nowrap rounded bg-foreground px-2 py-1 text-[10px] text-background group-hover:block">
                      {formatBytes(t.bytes)} · {t.count} 篇
                    </div>
                  </div>
                  <span className="text-[9px] text-muted-foreground">{t.month.slice(5)}</span>
                </div>
              ))}
            </div>
          </section>

          {/* 体积 Top 榜 */}
          <section>
            <h4 className="mb-2 text-xs font-medium text-muted-foreground">体积占用 Top 10</h4>
            <div className="overflow-x-auto">
              <table className="w-full min-w-[40rem] text-left text-sm">
                <thead className="bg-muted/40 text-xs uppercase text-muted-foreground">
                  <tr>
                    <th className="px-3 py-2 font-medium">论文</th>
                    <th className="px-3 py-2 font-medium">Owner</th>
                    <th className="px-3 py-2 font-medium">状态</th>
                    <th className="px-3 py-2 text-right font-medium">页数</th>
                    <th className="px-3 py-2 text-right font-medium">体积</th>
                    <th className="px-3 py-2 font-medium">上传时间</th>
                  </tr>
                </thead>
                <tbody>
                  {data.top_papers.length === 0 ? (
                    <tr>
                      <td colSpan={6} className="px-3 py-8 text-center text-muted-foreground">
                        暂无论文
                      </td>
                    </tr>
                  ) : (
                    data.top_papers.map((p) => (
                      <tr key={p.id} className="border-t border-border">
                        <td className="max-w-[20rem] px-3 py-2.5">
                          <p className="truncate">{p.title || p.file_name}</p>
                          <p className="truncate font-mono text-[11px] text-muted-foreground">{p.file_name}</p>
                        </td>
                        <td className="px-3 py-2.5 text-muted-foreground">{p.owner_id}</td>
                        <td className="px-3 py-2.5">
                          <span className="rounded bg-muted px-2 py-0.5 text-xs">{statusLabel(p.status)}</span>
                        </td>
                        <td className="px-3 py-2.5 text-right tabular-nums text-muted-foreground">
                          {p.page_count || "—"}
                        </td>
                        <td className="px-3 py-2.5 text-right font-medium tabular-nums">{formatBytes(p.size)}</td>
                        <td className="px-3 py-2.5 text-muted-foreground">{formatDateTime(p.created_at)}</td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </section>
        </div>
      )}
    </div>
  );
}

function StatTile({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-border bg-muted/20 px-3 py-2.5">
      <p className="text-[11px] text-muted-foreground">{label}</p>
      <p className="mt-0.5 text-lg font-semibold tabular-nums">{value}</p>
    </div>
  );
}
