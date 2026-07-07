"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import {
  ArrowLeft,
  Clock,
  FileBarChart,
  Gauge,
  Layers,
  Loader2,
  PieChart as PieChartIcon,
  Printer,
  School,
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
import {
  type AdvancedAnalytics,
  type ClassStat,
  fetchAdvancedAnalytics,
  fetchClassStats,
} from "../console-api";

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

type ReportTab = "summary" | "status" | "services" | "classes" | "latency";

const REPORT_TABS: Array<{ key: ReportTab; label: string; icon: typeof Gauge }> = [
  { key: "summary", label: "摘要", icon: Gauge },
  { key: "status", label: "论文状态", icon: PieChartIcon },
  { key: "services", label: "服务调用", icon: Layers },
  { key: "classes", label: "班级概览", icon: School },
  { key: "latency", label: "延迟分布", icon: Clock },
];

export default function AdminReportsPage() {
  const [token, setToken] = useState<string | null>(null);
  const [mounted, setMounted] = useState(false);
  const [basic, setBasic] = useState<Analytics | null>(null);
  const [advanced, setAdvanced] = useState<AdvancedAnalytics | null>(null);
  const [classes, setClasses] = useState<ClassStat[]>([]);
  const [loading, setLoading] = useState(true);
  const [tab, setTab] = useState<ReportTab>("summary");

  useEffect(() => {
    setMounted(true);
    setToken(readSavedAuth()?.token ?? null);
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
      fetchClassStats(token),
    ])
      .then(([b, a, c]) => {
        setBasic(b);
        setAdvanced(a);
        setClasses(c.items ?? []);
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  }, [token]);

  const successRate = useMemo(() => {
    if (!basic) return null;
    // 后端空切片序列化成 JSON null,空库时 trend 为 null,兜底成空数组避免 reduce 崩溃
    const trend = basic.trend ?? [];
    const total = trend.reduce((s, p) => s + p.calls, 0);
    const ok = trend.reduce((s, p) => s + p.success, 0);
    return total > 0 ? ok / total : null;
  }, [basic]);

  if (!mounted) return null;

  if (!token) {
    return (
      <main className="grid min-h-dvh place-items-center bg-background p-6 text-center">
        <div>
          <p className="text-sm text-muted-foreground">请先在管理后台登录。</p>
          <Link href="/admin" className="mt-3 inline-flex items-center gap-1 text-sm font-medium text-primary hover:underline">
            <ArrowLeft className="size-4" /> 返回后台
          </Link>
        </div>
      </main>
    );
  }

  return (
    <main className="min-h-dvh bg-background text-foreground">
      <div className="mx-auto w-full max-w-5xl px-4 py-6 sm:px-6">
        <header className="mb-6 flex items-center justify-between border-b border-border pb-4 print:hidden">
          <div className="flex items-center gap-2">
            <span className="flex size-9 items-center justify-center rounded-lg bg-primary/10 text-primary">
              <FileBarChart className="size-5" />
            </span>
            <div>
              <h1 className="text-base font-semibold tracking-tight">运营报表</h1>
              <p className="text-xs text-muted-foreground">近 30 天综合运营概况</p>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => window.print()}
              className="inline-flex items-center gap-1 rounded-lg border border-border px-3 py-1.5 text-sm font-medium hover:bg-muted"
            >
              <Printer className="size-4" /> 打印 / 导出 PDF
            </button>
            <Link href="/admin" className="inline-flex items-center gap-1 text-sm font-medium text-primary hover:underline">
              <ArrowLeft className="size-4" /> 返回
            </Link>
          </div>
        </header>

        {loading ? (
          <div className="py-24 text-center text-muted-foreground">
            <Loader2 className="mx-auto mb-2 size-6 animate-spin" />
            正在生成报表
          </div>
        ) : (
          <article className="space-y-5">
            <ReportHeader />
            <Tabs value={tab} onValueChange={(value) => setTab(value as ReportTab)}>
              <TabsList variant="line" className="flex-wrap print:hidden">
                {REPORT_TABS.map(({ key, label, icon: Icon }) => (
                  <TabsTrigger key={key} value={key}>
                    <Icon className="size-4" />
                    {label}
                  </TabsTrigger>
                ))}
              </TabsList>

              <TabsContent value="summary">
                <SummarySection basic={basic} successRate={successRate} />
              </TabsContent>

              <TabsContent value="status">
                <Section title="论文状态分布">
                  <div className="flex flex-wrap items-center gap-6">
                    <div className="h-52 w-52">
                      <ResponsiveContainer width="100%" height="100%">
                        <PieChart>
                          <Pie data={basic?.status_distribution ?? []} dataKey="count" nameKey="status" innerRadius="55%" outerRadius="90%" paddingAngle={2} stroke="var(--card)" strokeWidth={2}>
                            {(basic?.status_distribution ?? []).map((s) => (
                              <Cell key={s.status} fill={STATUS_COLORS[s.status] ?? "#94a3b8"} />
                            ))}
                          </Pie>
                          <Tooltip contentStyle={tooltipStyle} />
                        </PieChart>
                      </ResponsiveContainer>
                    </div>
                    <ul className="space-y-1 text-sm">
                      {(basic?.status_distribution ?? []).map((s) => (
                        <li key={s.status} className="flex items-center gap-2">
                          <span className="size-2.5 rounded-full" style={{ background: STATUS_COLORS[s.status] ?? "#94a3b8" }} />
                          <span className="w-20 text-muted-foreground">{s.status}</span>
                          <span className="font-medium tabular-nums">{s.count}</span>
                        </li>
                      ))}
                    </ul>
                  </div>
                </Section>
              </TabsContent>

              <TabsContent value="services">
                <Section title="各服务调用统计">
                  <table className="w-full text-left text-sm">
                    <thead className="border-b border-border text-xs uppercase text-muted-foreground">
                      <tr>
                        <th className="py-2">服务</th>
                        <th className="py-2">总调用</th>
                        <th className="py-2">成功</th>
                        <th className="py-2">失败</th>
                        <th className="py-2">平均延迟</th>
                        <th className="py-2">峰值</th>
                      </tr>
                    </thead>
                    <tbody>
                      {(basic?.services ?? []).map((s) => (
                        <tr key={s.service_type} className="border-b border-border/50">
                          <td className="py-2 font-medium">{s.service_type}</td>
                          <td className="py-2 tabular-nums">{s.total}</td>
                          <td className="py-2 tabular-nums text-emerald-600 dark:text-emerald-400">{s.success}</td>
                          <td className="py-2 tabular-nums text-red-600 dark:text-red-400">{s.failed}</td>
                          <td className="py-2 tabular-nums">{Math.round(s.avg_ms)}ms</td>
                          <td className="py-2 tabular-nums">{s.max_ms}ms</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </Section>
              </TabsContent>

              <TabsContent value="classes">
                <Section title="班级维度概览">
                  <div className="h-56">
                    <ResponsiveContainer width="100%" height="100%">
                      <BarChart data={classes} margin={{ top: 6, right: 8, left: -20, bottom: 0 }}>
                        <XAxis dataKey="class_id" tick={{ fontSize: 11, fill: "var(--muted-foreground)" }} tickLine={false} axisLine={false} />
                        <YAxis tick={{ fontSize: 11, fill: "var(--muted-foreground)" }} tickLine={false} axisLine={false} width={30} allowDecimals={false} />
                        <Tooltip contentStyle={tooltipStyle} cursor={{ fill: "var(--muted)", opacity: 0.35 }} />
                        <Bar dataKey="user_count" name="用户数" fill="#6366f1" radius={[3, 3, 0, 0]} />
                        <Bar dataKey="paper_count" name="论文数" fill="#10b981" radius={[3, 3, 0, 0]} />
                      </BarChart>
                    </ResponsiveContainer>
                  </div>
                </Section>
              </TabsContent>

              <TabsContent value="latency">
                <Section title="延迟分布">
                  <div className="h-52">
                    <ResponsiveContainer width="100%" height="100%">
                      <BarChart data={advanced?.latency_histogram ?? []} margin={{ top: 6, right: 8, left: -20, bottom: 0 }}>
                        <XAxis dataKey="label" tick={{ fontSize: 11, fill: "var(--muted-foreground)" }} tickLine={false} axisLine={false} />
                        <YAxis tick={{ fontSize: 11, fill: "var(--muted-foreground)" }} tickLine={false} axisLine={false} width={30} allowDecimals={false} />
                        <Tooltip contentStyle={tooltipStyle} cursor={{ fill: "var(--muted)", opacity: 0.35 }} />
                        <Bar dataKey="count" name="调用数" fill="#0ea5e9" radius={[3, 3, 0, 0]} />
                      </BarChart>
                    </ResponsiveContainer>
                  </div>
                </Section>
              </TabsContent>
            </Tabs>

            <footer className="border-t border-border pt-4 text-xs text-muted-foreground">
              本报表由 GopherPaper 后台自动生成,数据统计口径为最近 30 天。
            </footer>
          </article>
        )}
      </div>
    </main>
  );
}

function ReportHeader() {
  return (
    <div className="text-center">
      <h2 className="font-serif text-2xl font-semibold">GopherPaper 运营报表</h2>
      <p className="mt-1 text-sm text-muted-foreground">科研文献智能解析与知识服务系统</p>
    </div>
  );
}

function SummarySection({
  basic,
  successRate,
}: {
  basic: Analytics | null;
  successRate: number | null;
}) {
  const cards = [
    { label: "论文总量", value: basic?.total_papers ?? 0 },
    { label: "注册用户", value: basic?.total_users ?? 0 },
    { label: "服务调用", value: basic?.total_calls ?? 0 },
    { label: "问答会话", value: basic?.total_sessions ?? 0 },
  ];
  return (
    <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
      {cards.map((c) => (
        <div key={c.label} className="rounded-xl border border-border bg-card p-4 text-center">
          <div className="text-2xl font-semibold tabular-nums">{c.value.toLocaleString("zh-CN")}</div>
          <div className="mt-1 text-xs text-muted-foreground">{c.label}</div>
        </div>
      ))}
      {successRate != null && (
        <div className="col-span-2 rounded-xl border border-emerald-500/25 bg-emerald-500/5 p-4 text-center sm:col-span-4">
          <span className="text-sm text-muted-foreground">30 天服务成功率</span>
          <span className="ml-2 text-lg font-semibold text-emerald-600 dark:text-emerald-400">
            {(successRate * 100).toFixed(1)}%
          </span>
        </div>
      )}
    </div>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section>
      <h3 className="mb-3 border-l-4 border-primary pl-3 text-base font-semibold">{title}</h3>
      {children}
    </section>
  );
}
