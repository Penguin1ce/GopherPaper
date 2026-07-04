"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  Activity as ActivityIcon,
  Database,
  FileText,
  Gauge,
  Loader2,
  MessagesSquare,
  Sparkles,
  Trash2,
  Users,
} from "lucide-react";
import {
  Bar,
  BarChart,
  Cell,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { CountUp } from "./count-up";
import {
  type Activity,
  type Analytics,
  type Health,
  dashboardRequest,
} from "./dashboard-api";

const STATUS_COLORS: Record<string, string> = {
  ready: "#10b981",
  indexed: "#0ea5e9",
  extracted: "#8b5cf6",
  parsing: "#f59e0b",
  uploaded: "#94a3b8",
  failed: "#ef4444",
};

const SERVICE_COLORS = ["#6366f1", "#8b5cf6", "#f59e0b", "#0ea5e9", "#ec4899"];

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
  const [health, setHealth] = useState<Health | null>(null);
  const [activity, setActivity] = useState<Activity | null>(null);
  const [seeding, setSeeding] = useState(false);
  const [clearing, setClearing] = useState(false);
  const [refreshKey, setRefreshKey] = useState(0);
  // 演示数据操作默认收起,点「数据管理」才展开,避免演示时直接暴露灌入/清除按钮
  const [showDataMgmt, setShowDataMgmt] = useState(false);

  const loadMain = useCallback(async () => {
    if (!token) return;
    const [a, act] = await Promise.all([
      dashboardRequest<Analytics>("/admin/analytics?days=30", token),
      dashboardRequest<Activity>("/admin/activity?limit=14", token),
    ]);
    setAnalytics(a);
    setActivity(act);
  }, [token]);

  useEffect(() => {
    void loadMain().catch(() => {});
  }, [loadMain, refreshKey]);

  // 健康监控独立轮询,每 5 秒刷新,呈现"实时大盘"观感
  useEffect(() => {
    if (!token) return;
    let alive = true;
    const tick = () =>
      dashboardRequest<Health>("/admin/health", token)
        .then((h) => alive && setHealth(h))
        .catch(() => {});
    void tick();
    const id = setInterval(tick, 5000);
    return () => {
      alive = false;
      clearInterval(id);
    };
  }, [token, refreshKey]);

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

  const successRate = useMemo(() => {
    if (!analytics) return null;
    // 后端空切片序列化成 JSON null,空库时 trend 为 null,兜底成空数组避免 reduce 崩溃
    const trend = analytics.trend ?? [];
    const total = trend.reduce((s, p) => s + p.calls, 0);
    const ok = trend.reduce((s, p) => s + p.success, 0);
    return total > 0 ? ok / total : null;
  }, [analytics]);

  const trendData = useMemo(
    () => (analytics?.trend ?? []).map((p) => ({ ...p, label: mmdd(p.date) })),
    [analytics],
  );

  return (
    <section className="flex flex-col gap-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <span className="flex size-8 items-center justify-center rounded-lg bg-primary/10 text-primary">
            <Gauge className="size-4" />
          </span>
          <div>
            <h2 className="text-base font-semibold tracking-tight">运营数据看板</h2>
            <p className="text-xs text-muted-foreground">近 30 天趋势 · 服务健康 · 实时活动</p>
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
              <Button variant="outline" size="sm" onClick={() => void clear()} disabled={clearing || seeding}>
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

      {/* 统计卡:数字滚动动画 */}
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <StatCard icon={FileText} label="论文总量" value={analytics?.total_papers ?? 0} tint="#6366f1" />
        <StatCard icon={Users} label="注册用户" value={analytics?.total_users ?? 0} tint="#10b981" />
        <StatCard icon={ActivityIcon} label="服务调用" value={analytics?.total_calls ?? 0} tint="#f59e0b" />
        <StatCard icon={MessagesSquare} label="问答会话" value={analytics?.total_sessions ?? 0} tint="#0ea5e9" />
      </div>

      {/* 主趋势图:每日服务调用(成功/失败堆叠) */}
      <div className="rounded-xl border border-border bg-card p-4 shadow-sm">
        <div className="mb-3 flex items-center justify-between">
          <div>
            <h3 className="text-sm font-semibold">服务调用趋势</h3>
            <p className="text-xs text-muted-foreground">近 30 天每日调用量(成功 / 失败)</p>
          </div>
          {successRate != null && (
            <span className="rounded-md bg-emerald-500/10 px-2.5 py-1 text-xs font-medium text-emerald-600 dark:text-emerald-400">
              成功率 {(successRate * 100).toFixed(1)}%
            </span>
          )}
        </div>
        <div className="h-64 w-full">
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
              {/* 堆叠柱:柱高严格等于当天调用数,无插值,视觉与真实数量一一对应 */}
              <Bar dataKey="success" name="成功" stackId="calls" fill="#10b981" />
              <Bar dataKey="failed" name="失败" stackId="calls" fill="#ef4444" radius={[3, 3, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </div>
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        {/* 论文状态分布环形图 */}
        <div className="rounded-xl border border-border bg-card p-4 shadow-sm">
          <h3 className="mb-1 text-sm font-semibold">论文状态分布</h3>
          <p className="mb-2 text-xs text-muted-foreground">全库论文按解析流水线状态</p>
          <div className="flex items-center gap-4">
            <div className="h-52 w-1/2">
              <ResponsiveContainer width="100%" height="100%">
                <PieChart>
                  <Pie
                    data={analytics?.status_distribution ?? []}
                    dataKey="count"
                    nameKey="status"
                    innerRadius="58%"
                    outerRadius="90%"
                    paddingAngle={2}
                    stroke="var(--card)"
                    strokeWidth={2}
                  >
                    {(analytics?.status_distribution ?? []).map((s) => (
                      <Cell key={s.status} fill={STATUS_COLORS[s.status] ?? "#94a3b8"} />
                    ))}
                  </Pie>
                  <Tooltip contentStyle={tooltipStyle} />
                </PieChart>
              </ResponsiveContainer>
            </div>
            <ul className="flex-1 space-y-1.5">
              {(analytics?.status_distribution ?? []).map((s) => (
                <li key={s.status} className="flex items-center justify-between gap-2 text-sm">
                  <span className="flex items-center gap-2">
                    <span
                      className="size-2.5 rounded-full"
                      style={{ background: STATUS_COLORS[s.status] ?? "#94a3b8" }}
                    />
                    <span className="text-muted-foreground">{s.status}</span>
                  </span>
                  <span className="font-medium tabular-nums">{s.count}</span>
                </li>
              ))}
            </ul>
          </div>
        </div>

        {/* 各服务调用量柱状图 */}
        <div className="rounded-xl border border-border bg-card p-4 shadow-sm">
          <h3 className="mb-1 text-sm font-semibold">各服务调用统计</h3>
          <p className="mb-2 text-xs text-muted-foreground">按服务类型的调用量与平均延迟</p>
          <div className="h-52 w-full">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart
                data={analytics?.services ?? []}
                layout="vertical"
                margin={{ top: 4, right: 12, left: 8, bottom: 0 }}
              >
                <XAxis type="number" tick={{ fontSize: 11, fill: "var(--muted-foreground)" }} tickLine={false} axisLine={false} allowDecimals={false} />
                <YAxis
                  type="category"
                  dataKey="service_type"
                  tick={{ fontSize: 12, fill: "var(--foreground)" }}
                  tickLine={false}
                  axisLine={false}
                  width={56}
                />
                <Tooltip contentStyle={tooltipStyle} cursor={{ fill: "var(--muted)", opacity: 0.4 }} />
                <Bar dataKey="success" name="成功" stackId="s" radius={[0, 0, 0, 0]}>
                  {(analytics?.services ?? []).map((_, i) => (
                    <Cell key={i} fill={SERVICE_COLORS[i % SERVICE_COLORS.length]} />
                  ))}
                </Bar>
                <Bar dataKey="failed" name="失败" stackId="s" fill="#ef4444" radius={[0, 4, 4, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </div>
          <ul className="mt-2 space-y-1 text-xs text-muted-foreground">
            {(analytics?.services ?? []).map((s) => (
              <li key={s.service_type} className="flex justify-between">
                <span>{s.service_type}</span>
                <span className="tabular-nums">平均 {Math.round(s.avg_ms)}ms · 峰值 {s.max_ms}ms</span>
              </li>
            ))}
          </ul>
        </div>
      </div>

      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
        <HealthPanel health={health} />
        <ActivityFeed activity={activity} />
      </div>
    </section>
  );
}

function StatCard({
  icon: Icon,
  label,
  value,
  tint,
}: {
  icon: typeof FileText;
  label: string;
  value: number;
  tint: string;
}) {
  return (
    <div className="relative overflow-hidden rounded-xl border border-border bg-card p-4 shadow-sm">
      <div
        className="absolute -right-6 -top-6 size-20 rounded-full opacity-15 blur-xl"
        style={{ background: tint }}
      />
      <div className="flex items-center justify-between">
        <span className="text-sm text-muted-foreground">{label}</span>
        <Icon className="size-4" style={{ color: tint }} />
      </div>
      <CountUp value={value} className="mt-2 block text-3xl font-semibold tracking-tight tabular-nums" />
    </div>
  );
}

function HealthPanel({ health }: { health: Health | null }) {
  return (
    <div className="rounded-xl border border-border bg-card p-4 shadow-sm">
      <div className="mb-3 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Database className="size-4 text-primary" />
          <h3 className="text-sm font-semibold">系统健康监控</h3>
        </div>
        {health && (
          <span className="flex items-center gap-1.5 text-xs text-muted-foreground">
            <span className="relative flex size-2">
              <span className={cn("absolute inline-flex size-2 animate-ping rounded-full opacity-75", health.healthy === health.total ? "bg-emerald-500" : "bg-amber-500")} />
              <span className={cn("inline-flex size-2 rounded-full", health.healthy === health.total ? "bg-emerald-500" : "bg-amber-500")} />
            </span>
            {health.healthy}/{health.total} 在线 · 每 5s 刷新
          </span>
        )}
      </div>
      <div className="grid gap-2 sm:grid-cols-2">
        {(health?.items ?? []).map((item) => {
          const up = item.status === "up";
          return (
            <div
              key={item.name}
              className={cn(
                "flex items-center justify-between rounded-lg border px-3 py-2.5",
                up ? "border-emerald-500/25 bg-emerald-500/5" : "border-red-500/30 bg-red-500/5",
              )}
            >
              <div className="flex items-center gap-2">
                <span className={cn("size-2.5 rounded-full", up ? "bg-emerald-500" : "bg-red-500")} />
                <span className="text-sm font-medium">{item.name}</span>
              </div>
              <div className="text-right">
                <div className={cn("text-xs font-medium", up ? "text-emerald-600 dark:text-emerald-400" : "text-red-600 dark:text-red-400")}>
                  {up ? "在线" : "离线"} · {item.latency_ms}ms
                </div>
                {item.detail && <div className="max-w-[10rem] truncate text-[11px] text-muted-foreground">{item.detail}</div>}
              </div>
            </div>
          );
        })}
        {!health && <p className="col-span-2 py-6 text-center text-sm text-muted-foreground">正在探测依赖...</p>}
      </div>
    </div>
  );
}

function ActivityFeed({ activity }: { activity: Activity | null }) {
  const iconFor = (type: string) => (type === "paper" ? FileText : type === "user" ? Users : ActivityIcon);
  return (
    <div className="rounded-xl border border-border bg-card p-4 shadow-sm">
      <div className="mb-3 flex items-center gap-2">
        <ActivityIcon className="size-4 text-primary" />
        <h3 className="text-sm font-semibold">实时活动流</h3>
      </div>
      <ul className="max-h-[19rem] space-y-1 overflow-y-auto pr-1">
        {(activity?.items ?? []).map((item, i) => {
          const Icon = iconFor(item.type);
          const failed = item.status === "failed";
          return (
            <li key={i} className="flex items-center gap-3 rounded-lg px-2 py-1.5 hover:bg-muted/50">
              <span className={cn("flex size-7 shrink-0 items-center justify-center rounded-lg", failed ? "bg-red-500/10 text-red-500" : "bg-muted text-muted-foreground")}>
                <Icon className="size-3.5" />
              </span>
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm">{item.title}</p>
                {item.subtitle && <p className="truncate text-xs text-muted-foreground">{item.subtitle}</p>}
              </div>
              <span className="shrink-0 text-[11px] text-muted-foreground">{relTime(item.created_at)}</span>
            </li>
          );
        })}
        {!activity?.items?.length && <p className="py-6 text-center text-sm text-muted-foreground">暂无活动</p>}
      </ul>
    </div>
  );
}

function relTime(iso: string) {
  const t = new Date(iso).getTime();
  if (Number.isNaN(t)) return "";
  const diff = Date.now() - t;
  const min = Math.floor(diff / 60000);
  if (min < 1) return "刚刚";
  if (min < 60) return `${min}分钟前`;
  const h = Math.floor(min / 60);
  if (h < 24) return `${h}小时前`;
  return `${Math.floor(h / 24)}天前`;
}
