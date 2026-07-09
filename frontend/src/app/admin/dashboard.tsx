"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Database, Gauge, Loader2, Sparkles, Trash2 } from "lucide-react";
import { Bar, BarChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";

import { Button } from "@/components/ui/button";
import { type Analytics, dashboardRequest } from "./dashboard-api";
import { type AdvancedAnalytics, fetchAdvancedAnalytics } from "./console-api";
import { PipelineFunnel, StatusDonut, SummaryStrip, TopActors } from "./analytics-charts";

const tooltipStyle = {
  background: "var(--popover)",
  border: "1px solid var(--border)",
  borderRadius: "0.6rem",
  color: "var(--popover-foreground)",
  fontSize: "12px",
  boxShadow: "0 8px 30px -12px rgba(0,0,0,0.35)",
};

function mmdd(date: string) {
  const [, m, d] = date.split("-");
  return m && d ? `${m}-${d}` : date;
}

export function AdminDashboard({
  token,
  onChanged,
}: {
  token: string;
  onChanged?: () => void;
}) {
  const [analytics, setAnalytics] = useState<Analytics | null>(null);
  const [advanced, setAdvanced] = useState<AdvancedAnalytics | null>(null);
  const [seeding, setSeeding] = useState(false);
  const [clearing, setClearing] = useState(false);
  const [refreshKey, setRefreshKey] = useState(0);
  const [showDataMgmt, setShowDataMgmt] = useState(false);

  const loadMain = useCallback(async () => {
    if (!token) return;
    const [a, adv] = await Promise.all([
      dashboardRequest<Analytics>("/admin/analytics?days=30", token),
      fetchAdvancedAnalytics(token),
    ]);
    setAnalytics(a);
    setAdvanced(adv);
  }, [token]);

  useEffect(() => {
    void loadMain().catch(() => {});
  }, [loadMain, refreshKey]);

  const seed = async () => {
    setSeeding(true);
    try {
      await dashboardRequest("/admin/demo/seed", token, { method: "POST" });
      setRefreshKey((k) => k + 1);
      onChanged?.();
    } finally {
      setSeeding(false);
    }
  };

  const clear = async () => {
    setClearing(true);
    try {
      await dashboardRequest("/admin/demo/clear", token, { method: "POST" });
      setRefreshKey((k) => k + 1);
      onChanged?.();
    } finally {
      setClearing(false);
    }
  };

  const trendData = useMemo(
    () => (analytics?.trend ?? []).map((p) => ({ ...p, label: mmdd(p.date) })),
    [analytics],
  );

  const successRate = useMemo(() => {
    const total = trendData.reduce((sum, item) => sum + item.calls, 0);
    const success = trendData.reduce((sum, item) => sum + item.success, 0);
    return total > 0 ? success / total : null;
  }, [trendData]);

  return (
    <section className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <span className="flex size-8 items-center justify-center rounded-lg bg-primary/10 text-primary">
            <Gauge className="size-4" />
          </span>
          <div>
            <h2 className="text-base font-semibold tracking-tight">数据看板</h2>
            <p className="text-xs text-muted-foreground">关键指标 · 论文状态 · 流水线 · 活跃度 · 调用趋势</p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant={showDataMgmt ? "secondary" : "outline"}
            size="sm"
            onClick={() => setShowDataMgmt((v) => !v)}
          >
            <Database className="size-4" />
            数据管理
          </Button>
          {showDataMgmt && (
            <>
              <Button
                variant="outline"
                size="sm"
                onClick={() => void clear()}
                disabled={clearing || seeding}
              >
                {clearing ? <Loader2 className="size-4 animate-spin" /> : <Trash2 className="size-4" />}
                清除演示数据
              </Button>
              <Button size="sm" onClick={() => void seed()} disabled={seeding || clearing}>
                {seeding ? <Loader2 className="size-4 animate-spin" /> : <Sparkles className="size-4" />}
                灌入演示数据
              </Button>
            </>
          )}
        </div>
      </div>

      {/* KPI 概览 */}
      <SummaryStrip basic={analytics} />

      {/* 数据分析核心图表 */}
      <div className="grid gap-4 lg:grid-cols-2">
        <StatusDonut basic={analytics} />
        <PipelineFunnel advanced={advanced} />
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <TopActors advanced={advanced} />

        {/* 近 30 天调用趋势 */}
        <div className="rounded-xl border border-border bg-card p-4 shadow-sm">
          <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
            <div>
              <h3 className="text-sm font-semibold">服务调用趋势</h3>
              <p className="text-xs text-muted-foreground">成功 / 失败调用量按日统计</p>
            </div>
            {successRate != null && (
              <span className="rounded-md bg-emerald-500/10 px-2.5 py-1 text-xs font-medium text-emerald-600 dark:text-emerald-400">
                成功率 {(successRate * 100).toFixed(1)}%
              </span>
            )}
          </div>
          <div className="h-56 w-full">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={trendData} margin={{ top: 6, right: 8, left: -18, bottom: 0 }} barCategoryGap="16%">
                <XAxis
                  dataKey="label"
                  tick={{ fontSize: 11, fill: "var(--muted-foreground)" }}
                  tickLine={false}
                  axisLine={false}
                  interval={4}
                />
                <YAxis
                  tick={{ fontSize: 11, fill: "var(--muted-foreground)" }}
                  tickLine={false}
                  axisLine={false}
                  width={34}
                  allowDecimals={false}
                />
                <Tooltip contentStyle={tooltipStyle} cursor={{ fill: "var(--muted)", opacity: 0.35 }} />
                <Bar dataKey="success" name="成功" stackId="calls" fill="#10b981" />
                <Bar dataKey="failed" name="失败" stackId="calls" fill="#ef4444" radius={[3, 3, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </div>
      </div>
    </section>
  );
}
