"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { AnimatePresence, motion } from "motion/react";
import {
  FileText,
  Loader2,
  MessagesSquare,
  Search,
  Users,
  X,
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
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import {
  type ClassStat,
  type UserDetail,
  type UserItem,
  fetchClassStats,
  fetchUserDetail,
  fetchUsers,
  formatBytes,
  formatDateTime,
  relativeTime,
} from "./console-api";

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

const PAGE_SIZE = 10;

export function UsersPanel({ token }: { token: string }) {
  const [items, setItems] = useState<UserItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [query, setQuery] = useState("");
  const [classFilter, setClassFilter] = useState("");
  const [classes, setClasses] = useState<ClassStat[]>([]);
  const [loading, setLoading] = useState(false);
  const [selectedId, setSelectedId] = useState<number | null>(null);

  const load = useCallback(
    async (nextPage = page) => {
      if (!token) return;
      setLoading(true);
      try {
        const res = await fetchUsers(token, {
          page: nextPage,
          page_size: PAGE_SIZE,
          query,
          class_id: classFilter,
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
    [classFilter, page, query, token],
  );

  useEffect(() => {
    void load(1);
    void fetchClassStats(token)
      .then((r) => setClasses(r.items ?? []))
      .catch(() => {});
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token]);

  const pageCount = useMemo(() => Math.max(1, Math.ceil(total / PAGE_SIZE)), [total]);

  return (
    <div className="rounded-xl border border-border bg-card p-4 shadow-sm">
      <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <Users className="size-4 text-primary" />
          <h3 className="text-sm font-semibold">用户管理</h3>
          <span className="text-xs text-muted-foreground">共 {total} 人</span>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <label className="relative">
            <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              className="h-8 w-48 pl-8"
              placeholder="姓名 / 学号 / 邮箱"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") void load(1);
              }}
            />
          </label>
          <select
            className="h-8 rounded-lg border border-input bg-background px-2 text-sm outline-none"
            value={classFilter}
            onChange={(e) => {
              setClassFilter(e.target.value);
            }}
          >
            <option value="">全部班级</option>
            {classes.map((c) => (
              <option key={c.class_id} value={c.class_id === "未分班" ? "" : c.class_id}>
                {c.class_id}({c.user_count})
              </option>
            ))}
          </select>
          <Button size="sm" variant="outline" onClick={() => void load(1)} disabled={loading}>
            {loading ? <Loader2 className="size-4 animate-spin" /> : <Search className="size-4" />}
            查询
          </Button>
        </div>
      </div>

      <div className="overflow-x-auto">
        <table className="w-full min-w-[46rem] text-left text-sm">
          <thead className="bg-muted/40 text-xs uppercase text-muted-foreground">
            <tr>
              <th className="px-3 py-2 font-medium">用户</th>
              <th className="px-3 py-2 font-medium">班级</th>
              <th className="px-3 py-2 font-medium">论文</th>
              <th className="px-3 py-2 font-medium">会话</th>
              <th className="px-3 py-2 font-medium">调用</th>
              <th className="px-3 py-2 font-medium">最近活跃</th>
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
                  暂无用户
                </td>
              </tr>
            ) : (
              items.map((u) => (
                <tr
                  key={u.id}
                  className="cursor-pointer border-t border-border hover:bg-muted/40"
                  onClick={() => setSelectedId(u.id)}
                >
                  <td className="px-3 py-2.5">
                    <div className="font-medium">{u.name || u.student_id}</div>
                    <div className="text-xs text-muted-foreground">{u.email || u.student_id}</div>
                  </td>
                  <td className="px-3 py-2.5 text-muted-foreground">{u.class_id || "—"}</td>
                  <td className="px-3 py-2.5 tabular-nums">{u.paper_count}</td>
                  <td className="px-3 py-2.5 tabular-nums">{u.session_count}</td>
                  <td className="px-3 py-2.5 tabular-nums">{u.call_count}</td>
                  <td className="px-3 py-2.5 text-xs text-muted-foreground">
                    {u.last_active_at ? relativeTime(u.last_active_at) : "—"}
                  </td>
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
          <Button
            size="sm"
            variant="outline"
            disabled={page <= 1 || loading}
            onClick={() => void load(page - 1)}
          >
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

      <AnimatePresence>
        {selectedId != null && (
          <UserDetailDrawer token={token} id={selectedId} onClose={() => setSelectedId(null)} />
        )}
      </AnimatePresence>
    </div>
  );
}

function UserDetailDrawer({
  token,
  id,
  onClose,
}: {
  token: string;
  id: number;
  onClose: () => void;
}) {
  const [detail, setDetail] = useState<UserDetail | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setLoading(true);
    fetchUserDetail(token, id)
      .then((d) => setDetail(d))
      .catch(() => setDetail(null))
      .finally(() => setLoading(false));
  }, [token, id]);

  const activity = useMemo(
    () => (detail?.activity ?? []).map((p) => ({ ...p, label: p.date.slice(5) })),
    [detail],
  );
  // 后端把空切片序列化成 JSON null,真实用户(无论文)会返回 null,这里兜底成空数组
  const statusDist = detail?.paper_status_distribution ?? [];
  const recentPapers = detail?.recent_papers ?? [];

  return (
    <motion.div
      className="fixed inset-0 z-50 flex justify-end bg-black/30 backdrop-blur-sm"
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      onClick={onClose}
    >
      <motion.aside
        className="h-full w-full max-w-md overflow-y-auto border-l border-border bg-background p-5 shadow-xl"
        initial={{ x: 40 }}
        animate={{ x: 0 }}
        exit={{ x: 40 }}
        transition={{ type: "spring", stiffness: 320, damping: 32 }}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="mb-4 flex items-center justify-between">
          <h3 className="text-base font-semibold">用户画像</h3>
          <button type="button" onClick={onClose} className="rounded-md p-1 hover:bg-muted">
            <X className="size-4" />
          </button>
        </div>

        {loading ? (
          <div className="py-20 text-center text-muted-foreground">
            <Loader2 className="mx-auto mb-2 size-5 animate-spin" />
            加载中
          </div>
        ) : !detail ? (
          <p className="py-20 text-center text-muted-foreground">加载失败</p>
        ) : (
          <div className="space-y-5">
            <div className="rounded-xl border border-border bg-card p-4">
              <div className="text-lg font-semibold">{detail.user.name || detail.user.student_id}</div>
              <div className="mt-1 text-sm text-muted-foreground">{detail.user.email}</div>
              <div className="mt-3 grid grid-cols-3 gap-2 text-center">
                <MiniStat icon={FileText} label="论文" value={detail.user.paper_count} />
                <MiniStat icon={MessagesSquare} label="会话" value={detail.user.session_count} />
                <MiniStat icon={Users} label="调用" value={detail.user.call_count} />
              </div>
              <div className="mt-3 text-xs text-muted-foreground">
                学号 {detail.user.student_id} · 班级 {detail.user.class_id || "—"} · 注册于{" "}
                {formatDateTime(detail.user.created_at)}
              </div>
            </div>

            {statusDist.length > 0 && (
              <div className="rounded-xl border border-border bg-card p-4">
                <h4 className="mb-2 text-sm font-semibold">论文状态分布</h4>
                <div className="h-40">
                  <ResponsiveContainer width="100%" height="100%">
                    <PieChart>
                      <Pie
                        data={statusDist}
                        dataKey="count"
                        nameKey="status"
                        innerRadius="55%"
                        outerRadius="90%"
                        paddingAngle={2}
                        stroke="var(--card)"
                      >
                        {statusDist.map((s) => (
                          <Cell key={s.status} fill={STATUS_COLORS[s.status] ?? "#94a3b8"} />
                        ))}
                      </Pie>
                      <Tooltip contentStyle={tooltipStyle} />
                    </PieChart>
                  </ResponsiveContainer>
                </div>
              </div>
            )}

            <div className="rounded-xl border border-border bg-card p-4">
              <h4 className="mb-2 text-sm font-semibold">近 14 天活动</h4>
              <div className="h-36">
                <ResponsiveContainer width="100%" height="100%">
                  <BarChart data={activity} margin={{ top: 4, right: 4, left: -22, bottom: 0 }}>
                    <XAxis dataKey="label" tick={{ fontSize: 10, fill: "var(--muted-foreground)" }} tickLine={false} axisLine={false} interval={2} />
                    <YAxis tick={{ fontSize: 10, fill: "var(--muted-foreground)" }} tickLine={false} axisLine={false} width={28} allowDecimals={false} />
                    <Tooltip contentStyle={tooltipStyle} cursor={{ fill: "var(--muted)", opacity: 0.35 }} />
                    <Bar dataKey="calls" name="调用" fill="#6366f1" radius={[2, 2, 0, 0]} />
                  </BarChart>
                </ResponsiveContainer>
              </div>
            </div>

            {recentPapers.length > 0 && (
              <div className="rounded-xl border border-border bg-card p-4">
                <h4 className="mb-2 text-sm font-semibold">最近论文</h4>
                <ul className="space-y-2">
                  {recentPapers.map((p) => (
                    <li key={p.id} className="flex items-center justify-between gap-2 text-sm">
                      <span className="min-w-0 flex-1 truncate">{p.title || p.file_name}</span>
                      <span className="shrink-0 text-xs text-muted-foreground">
                        {p.status} · {formatBytes(p.size)}
                      </span>
                    </li>
                  ))}
                </ul>
              </div>
            )}
          </div>
        )}
      </motion.aside>
    </motion.div>
  );
}

function MiniStat({
  icon: Icon,
  label,
  value,
}: {
  icon: typeof FileText;
  label: string;
  value: number;
}) {
  return (
    <div className="rounded-lg bg-muted/50 p-2">
      <Icon className="mx-auto size-4 text-muted-foreground" />
      <div className="mt-1 text-lg font-semibold tabular-nums">{value}</div>
      <div className={cn("text-[11px] text-muted-foreground")}>{label}</div>
    </div>
  );
}
