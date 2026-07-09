"use client";

import { useEffect, useState } from "react";
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

import { readSavedAuth } from "@/components/gopherpaper/admin-auth";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { type Analytics, dashboardRequest } from "../dashboard-api";
import { type AdvancedAnalytics, fetchAdvancedAnalytics } from "../console-api";
import {
  HourlyDistribution,
  LatencyHistogram,
  PipelineFunnel,
  StatusDonut,
  SummaryStrip,
  TopActors,
} from "../analytics-charts";

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
              <h1 className="text-base font-semibold tracking-tight">数据分析</h1>
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
