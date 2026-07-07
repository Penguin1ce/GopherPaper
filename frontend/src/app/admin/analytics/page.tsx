"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import {
  ArrowLeft,
  Clock,
  Gauge,
  GitBranch,
  Loader2,
  PieChart as PieChartIcon,
  TrendingUp,
  Users,
  Zap,
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

import { readSavedAuth } from "@/components/gopherpaper/admin-auth";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { type Analytics, dashboardRequest } from "../dashboard-api";
import { type AdvancedAnalytics, fetchAdvancedAnalytics } from "../console-api";

const STATUS_COLORS: Record<string, string> = {
  ready: "#10b981",
  indexed: "#0ea5e9",
  extracted: "#8b5cf6",
  parsing: "#f59e0b",
  uploaded: "#94a3b8",
  failed: "#ef4444",
};

const tooltipStyle = {
  background: "var(--popover)",
  border: "1px solid var(--border)",
  borderRadius: "0.6rem",
  color: "var(--popover-foreground)",
  fontSize: "12px",
};

type AnalyticsTab = "summary" | "latency" | "hourly" | "actors" | "pipeline" | "status";

const ANALYTICS_TABS: Array<{ key: AnalyticsTab; label: string; icon: typeof Gauge }> = [
  { key: "summary", label: "总览", icon: Gauge },
  { key: "latency", label: "延迟", icon: Zap },
  { key: "hourly", label: "时段", icon: Clock },
  { key: "actors", label: "用户排行", icon: Users },
  { key: "pipeline", label: "流水线", icon: GitBranch },
  { key: "status", label: "论文状态", icon: PieChartIcon },
];

export default function AdminAnalyticsPage() {
  const [token, setToken] = useState<string | null>(null);
  const [mounted, setMounted] = useState(false);
  const [basic, setBasic] = useState<Analytics | null>(null);
  const [advanced, setAdvanced] = useState<AdvancedAnalytics | null>(null);
  const [loading, setLoading] = useState(true);
  const [tab, setTab] = useState<AnalyticsTab>("summary");

  useEffect(() => {
    setMounted(true);
    const auth = readSavedAuth();
    setToken(auth?.token ?? null);
  }, []);

  useEffect(() => {
    if (!token) {
      setLoading(false);
      return;
    }
    setLoading(true);
    Promise.all([
      dashboardRequest<Analytics>("/admin/analytics?days=30", token),
      fetchAdvancedAnalytics(token),
    ])
      .then(([b, a]) => {
        setBasic(b);
        setAdvanced(a);
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  }, [token]);

  if (!mounted) return null;

  if (!token) {
    return (
      <main className="grid min-h-dvh place-items-center bg-background p-6 text-center">
        <div>
          <p className="text-sm text-muted-foreground">请先在管理后台登录。</p>
          <Link
            href="/admin"
            className="mt-3 inline-flex items-center gap-1 text-sm font-medium text-primary hover:underline"
          >
            <ArrowLeft className="size-4" /> 返回后台
          </Link>
        </div>
      </main>
    );
  }

  return (
    <main className="min-h-dvh bg-background text-foreground">
      <div className="mx-auto w-full max-w-[92rem] px-4 py-5 sm:px-6 lg:px-8">
        <header className="mb-5 flex items-center justify-between border-b border-border pb-4">
          <div className="flex items-center gap-2">
            <span className="flex size-9 items-center justify-center rounded-lg bg-primary/10 text-primary">
              <TrendingUp className="size-5" />
            </span>
            <div>
              <h1 className="text-base font-semibold tracking-tight">运营数据分析</h1>
              <p className="text-xs text-muted-foreground">延迟分布 · 时段热度 · 用户排行 · 流水线漏斗</p>
            </div>
          </div>
          <Link
            href="/admin"
            className="inline-flex items-center gap-1 text-sm font-medium text-primary hover:underline"
          >
            <ArrowLeft className="size-4" /> 返回后台
          </Link>
        </header>

        {loading ? (
          <div className="py-24 text-center text-muted-foreground">
            <Loader2 className="mx-auto mb-2 size-6 animate-spin" />
            正在加载分析数据
          </div>
        ) : (
          <Tabs value={tab} onValueChange={(value) => setTab(value as AnalyticsTab)}>
            <TabsList variant="line" className="flex-wrap">
              {ANALYTICS_TABS.map(({ key, label, icon: Icon }) => (
                <TabsTrigger key={key} value={key}>
                  <Icon className="size-4" />
                  {label}
                </TabsTrigger>
              ))}
            </TabsList>

            <TabsContent value="summary">
              <SummaryStrip basic={basic} />
            </TabsContent>
            <TabsContent value="latency">
              <LatencyHistogram advanced={advanced} />
            </TabsContent>
            <TabsContent value="hourly">
              <HourlyDistribution advanced={advanced} />
            </TabsContent>
            <TabsContent value="actors">
              <TopActors advanced={advanced} />
            </TabsContent>
            <TabsContent value="pipeline">
              <PipelineFunnel advanced={advanced} />
            </TabsContent>
            <TabsContent value="status">
              <StatusDonut basic={basic} />
            </TabsContent>
          </Tabs>
        )}
      </div>
    </main>
  );
}

function SummaryStrip({ basic }: { basic: Analytics | null }) {
  const cards = [
    { icon: Gauge, label: "论文总量", value: basic?.total_papers ?? 0, tint: "#6366f1" },
    { icon: Users, label: "注册用户", value: basic?.total_users ?? 0, tint: "#10b981" },
    { icon: TrendingUp, label: "服务调用", value: basic?.total_calls ?? 0, tint: "#f59e0b" },
    { icon: Clock, label: "问答会话", value: basic?.total_sessions ?? 0, tint: "#0ea5e9" },
  ];
  return (
    <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      {cards.map((c) => {
        const Icon = c.icon;
        return (
          <div key={c.label} className="rounded-xl border border-border bg-card p-4 shadow-sm">
            <div className="flex items-center justify-between">
              <span className="text-sm text-muted-foreground">{c.label}</span>
              <Icon className="size-4" style={{ color: c.tint }} />
            </div>
            <div className="mt-2 text-3xl font-semibold tabular-nums">{c.value.toLocaleString("zh-CN")}</div>
          </div>
        );
      })}
    </div>
  );
}

function LatencyHistogram({ advanced }: { advanced: AdvancedAnalytics | null }) {
  const data = advanced?.latency_histogram ?? [];
  return (
    <div className="rounded-xl border border-border bg-card p-4 shadow-sm">
      <h3 className="mb-1 text-sm font-semibold">服务延迟直方图</h3>
      <p className="mb-2 text-xs text-muted-foreground">各调用按响应耗时分档统计</p>
      <div className="h-56">
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={data} margin={{ top: 6, right: 8, left: -20, bottom: 0 }}>
            <XAxis dataKey="label" tick={{ fontSize: 11, fill: "var(--muted-foreground)" }} tickLine={false} axisLine={false} />
            <YAxis tick={{ fontSize: 11, fill: "var(--muted-foreground)" }} tickLine={false} axisLine={false} width={34} allowDecimals={false} />
            <Tooltip contentStyle={tooltipStyle} cursor={{ fill: "var(--muted)", opacity: 0.35 }} />
            <Bar dataKey="count" name="调用数" radius={[4, 4, 0, 0]}>
              {data.map((_, i) => (
                <Cell key={i} fill={i >= 4 ? "#ef4444" : i >= 3 ? "#f59e0b" : "#6366f1"} />
              ))}
            </Bar>
          </BarChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}

function HourlyDistribution({ advanced }: { advanced: AdvancedAnalytics | null }) {
  const data = useMemo(
    () => (advanced?.hourly_distribution ?? []).map((h) => ({ ...h, label: `${h.hour}` })),
    [advanced],
  );
  return (
    <div className="rounded-xl border border-border bg-card p-4 shadow-sm">
      <h3 className="mb-1 text-sm font-semibold">24 小时调用分布</h3>
      <p className="mb-2 text-xs text-muted-foreground">按一天各小时聚合的调用量</p>
      <div className="h-56">
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={data} margin={{ top: 6, right: 8, left: -20, bottom: 0 }}>
            <XAxis dataKey="label" tick={{ fontSize: 10, fill: "var(--muted-foreground)" }} tickLine={false} axisLine={false} interval={1} />
            <YAxis tick={{ fontSize: 11, fill: "var(--muted-foreground)" }} tickLine={false} axisLine={false} width={34} allowDecimals={false} />
            <Tooltip contentStyle={tooltipStyle} cursor={{ fill: "var(--muted)", opacity: 0.35 }} />
            <Bar dataKey="count" name="调用数" fill="#0ea5e9" radius={[2, 2, 0, 0]} />
          </BarChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}

function TopActors({ advanced }: { advanced: AdvancedAnalytics | null }) {
  const data = advanced?.top_actors ?? [];
  const max = Math.max(1, ...data.map((a) => a.calls));
  return (
    <div className="rounded-xl border border-border bg-card p-4 shadow-sm">
      <h3 className="mb-1 text-sm font-semibold">调用量 Top 用户</h3>
      <p className="mb-3 text-xs text-muted-foreground">调用服务最活跃的用户排行</p>
      {data.length === 0 ? (
        <p className="py-8 text-center text-sm text-muted-foreground">暂无数据</p>
      ) : (
        <ul className="space-y-2">
          {data.map((a, i) => (
            <li key={a.actor_id} className="flex items-center gap-3">
              <span className="w-5 shrink-0 text-right text-xs text-muted-foreground">{i + 1}</span>
              <span className="w-28 shrink-0 truncate text-sm">{a.actor_id}</span>
              <div className="h-4 flex-1 overflow-hidden rounded-full bg-muted">
                <div
                  className="h-full rounded-full bg-primary"
                  style={{ width: `${(a.calls / max) * 100}%` }}
                />
              </div>
              <span className="w-10 shrink-0 text-right text-sm tabular-nums">{a.calls}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

function PipelineFunnel({ advanced }: { advanced: AdvancedAnalytics | null }) {
  const data = advanced?.pipeline ?? [];
  const max = Math.max(1, ...data.map((s) => s.count));
  const stageColor: Record<string, string> = {
    uploaded: "#94a3b8",
    parsing: "#f59e0b",
    extracted: "#8b5cf6",
    indexed: "#0ea5e9",
    ready: "#10b981",
    failed: "#ef4444",
  };
  return (
    <div className="rounded-xl border border-border bg-card p-4 shadow-sm">
      <h3 className="mb-1 text-sm font-semibold">论文流水线漏斗</h3>
      <p className="mb-3 text-xs text-muted-foreground">各解析阶段的论文数量</p>
      <div className="space-y-2">
        {data.map((s) => (
          <div key={s.stage} className="flex items-center gap-3">
            <span className="w-16 shrink-0 text-xs text-muted-foreground">{s.stage}</span>
            <div className="h-6 flex-1 overflow-hidden rounded-lg bg-muted">
              <div
                className="flex h-full items-center justify-end rounded-lg px-2 text-xs font-medium text-white"
                style={{ width: `${Math.max(6, (s.count / max) * 100)}%`, background: stageColor[s.stage] ?? "#6366f1" }}
              >
                {s.count}
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function StatusDonut({ basic }: { basic: Analytics | null }) {
  const data = basic?.status_distribution ?? [];
  return (
    <div className="rounded-xl border border-border bg-card p-4 shadow-sm">
      <h3 className="mb-1 text-sm font-semibold">论文状态分布</h3>
      <p className="mb-2 text-xs text-muted-foreground">全库论文按解析流水线状态</p>
      <div className="flex flex-wrap items-center gap-6">
        <div className="h-52 w-52">
          <ResponsiveContainer width="100%" height="100%">
            <PieChart>
              <Pie
                data={data}
                dataKey="count"
                nameKey="status"
                innerRadius="55%"
                outerRadius="90%"
                paddingAngle={2}
                stroke="var(--card)"
                strokeWidth={2}
              >
                {data.map((s) => (
                  <Cell key={s.status} fill={STATUS_COLORS[s.status] ?? "#94a3b8"} />
                ))}
              </Pie>
              <Tooltip contentStyle={tooltipStyle} />
            </PieChart>
          </ResponsiveContainer>
        </div>
        <ul className="space-y-1.5">
          {data.map((s) => (
            <li key={s.status} className="flex items-center gap-2 text-sm">
              <span className="size-2.5 rounded-full" style={{ background: STATUS_COLORS[s.status] ?? "#94a3b8" }} />
              <span className="w-20 text-muted-foreground">{s.status}</span>
              <span className="font-medium tabular-nums">{s.count}</span>
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
}
