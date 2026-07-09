"use client";

import { useMemo } from "react";
import { Clock, Gauge, TrendingUp, Users } from "lucide-react";
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

import { type Analytics } from "./dashboard-api";
import { type AdvancedAnalytics } from "./console-api";

export const STATUS_COLORS: Record<string, string> = {
  ready: "#10b981",
  indexed: "#0ea5e9",
  extracted: "#8b5cf6",
  parsing: "#f59e0b",
  uploaded: "#94a3b8",
  failed: "#ef4444",
};

export const tooltipStyle = {
  background: "var(--popover)",
  border: "1px solid var(--border)",
  borderRadius: "0.6rem",
  color: "var(--popover-foreground)",
  fontSize: "12px",
};

export function SummaryStrip({ basic }: { basic: Analytics | null }) {
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

export function LatencyHistogram({ advanced }: { advanced: AdvancedAnalytics | null }) {
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

export function HourlyDistribution({ advanced }: { advanced: AdvancedAnalytics | null }) {
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

export function TopActors({ advanced }: { advanced: AdvancedAnalytics | null }) {
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
                <div className="h-full rounded-full bg-primary" style={{ width: `${(a.calls / max) * 100}%` }} />
              </div>
              <span className="w-10 shrink-0 text-right text-sm tabular-nums">{a.calls}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

export function PipelineFunnel({ advanced }: { advanced: AdvancedAnalytics | null }) {
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

export function StatusDonut({ basic }: { basic: Analytics | null }) {
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
