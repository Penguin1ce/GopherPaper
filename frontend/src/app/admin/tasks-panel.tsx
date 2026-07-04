"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  Database,
  Download,
  KanbanSquare,
  Loader2,
  Plus,
  RefreshCw,
  Search,
  Trash2,
  X,
} from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import {
  bulkTasks,
  clearDemoTasks,
  downloadTasksCsv,
  fetchTaskBoard,
  fetchTaskStats,
  moveTask,
  seedDemoTasks,
  type TaskBoard,
  type TaskItem,
  type TaskStats,
} from "./console-api";
import { TaskCard } from "./task-card";
import { TaskDetailDrawer } from "./task-detail-drawer";
import { TaskEditorDialog } from "./task-editor-dialog";
import { TASK_PRIORITIES, TASK_STATUSES, priorityMeta, statusMeta } from "./task-shared";

const selectClass =
  "h-8 rounded-md border border-input bg-transparent px-2 text-xs shadow-sm outline-none focus-visible:ring-1 focus-visible:ring-ring";

export function TasksPanel({ token }: { token: string }) {
  const [board, setBoard] = useState<TaskBoard | null>(null);
  const [stats, setStats] = useState<TaskStats | null>(null);
  const [loading, setLoading] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);

  const [query, setQuery] = useState("");
  const [assigneeFilter, setAssigneeFilter] = useState("");
  const [priorityFilter, setPriorityFilter] = useState("");

  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [movingIds, setMovingIds] = useState<Set<number>>(new Set());
  const [bulkStatus, setBulkStatus] = useState("doing");

  const [editorOpen, setEditorOpen] = useState(false);
  const [editingTask, setEditingTask] = useState<TaskItem | null>(null);
  const [editorDefaultStatus, setEditorDefaultStatus] = useState("todo");

  const [detailOpen, setDetailOpen] = useState(false);
  const [detailId, setDetailId] = useState<number | null>(null);

  const load = useCallback(async () => {
    if (!token) return;
    setLoading(true);
    try {
      const [b, s] = await Promise.all([
        fetchTaskBoard(token, { assignee: assigneeFilter, priority: priorityFilter, query }),
        fetchTaskStats(token),
      ]);
      setBoard(b);
      setStats(s);
    } catch {
      setBoard(null);
    } finally {
      setLoading(false);
    }
  }, [token, assigneeFilter, priorityFilter, query]);

  useEffect(() => {
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token]);

  // 后端空切片可能被序列化为 null,这里统一归一化为可遍历数组。
  const columns = useMemo(() => {
    const rawColumns = Array.isArray(board?.columns) ? board.columns : [];
    return rawColumns.map((col) => ({
      ...col,
      items: Array.isArray(col.items) ? col.items : [],
    }));
  }, [board]);

  // id → 当前状态,便于左右移动时计算目标列。
  const statusById = useMemo(() => {
    const m = new Map<number, string>();
    columns.forEach((col) => col.items.forEach((t) => m.set(t.id, col.status)));
    return m;
  }, [columns]);

  const toggleSelect = (id: number) => {
    setSelected((s) => {
      const next = new Set(s);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const onMove = async (id: number, dir: -1 | 1) => {
    const cur = statusById.get(id);
    if (!cur) return;
    const idx = TASK_STATUSES.findIndex((s) => s.value === cur);
    const target = TASK_STATUSES[idx + dir];
    if (!target) return;
    setMovingIds((s) => new Set(s).add(id));
    try {
      await moveTask(token, id, target.value);
      await load();
    } finally {
      setMovingIds((s) => {
        const next = new Set(s);
        next.delete(id);
        return next;
      });
    }
  };

  const runBulk = async (action: "move" | "delete", extra?: { status?: string }) => {
    if (selected.size === 0) return;
    setNotice(null);
    try {
      const res = await bulkTasks(token, {
        ids: Array.from(selected),
        action,
        status: extra?.status,
      });
      setNotice(`已处理 ${res.succeeded} 项,失败 ${res.failed} 项`);
      setSelected(new Set());
      await load();
    } catch (e) {
      setNotice(e instanceof Error ? e.message : "批量操作失败");
    }
  };

  const onSeed = async () => {
    setLoading(true);
    try {
      const r = await seedDemoTasks(token);
      setNotice(r.message);
      await load();
    } finally {
      setLoading(false);
    }
  };

  const onClearDemo = async () => {
    setLoading(true);
    try {
      const r = await clearDemoTasks(token);
      setNotice(r.message);
      await load();
    } finally {
      setLoading(false);
    }
  };

  const openCreate = (status: string) => {
    setEditingTask(null);
    setEditorDefaultStatus(status);
    setEditorOpen(true);
  };

  const openDetail = (id: number) => {
    setDetailId(id);
    setDetailOpen(true);
  };

  return (
    <div className="rounded-xl border border-border bg-card p-4 shadow-sm">
      {/* 头部 */}
      <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <KanbanSquare className="size-4 text-primary" />
          <h3 className="text-sm font-semibold">运维任务看板</h3>
          {stats && <span className="text-xs text-muted-foreground">共 {stats.total} 项</span>}
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <label className="relative">
            <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              className="h-8 w-40 pl-8"
              placeholder="搜索标题/描述"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") void load();
              }}
            />
          </label>
          <Input
            className="h-8 w-28"
            placeholder="负责人"
            value={assigneeFilter}
            onChange={(e) => setAssigneeFilter(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") void load();
            }}
          />
          <select
            className={selectClass}
            value={priorityFilter}
            onChange={(e) => setPriorityFilter(e.target.value)}
          >
            <option value="">全部优先级</option>
            {TASK_PRIORITIES.map((p) => (
              <option key={p.value} value={p.value}>
                {p.label}
              </option>
            ))}
          </select>
          <Button size="sm" variant="outline" onClick={() => void load()} disabled={loading}>
            {loading ? <Loader2 className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
          </Button>
          <Button size="sm" onClick={() => openCreate("todo")}>
            <Plus className="size-4" />
            新建
          </Button>
        </div>
      </div>

      {/* 统计头 */}
      {stats && <StatsHeader stats={stats} />}

      {/* 工具行 */}
      <div className="mb-3 flex flex-wrap items-center gap-2 text-xs">
        <Button size="sm" variant="ghost" className="h-7 gap-1 px-2 text-muted-foreground" onClick={() => void onSeed()}>
          <Database className="size-3.5" />
          填充演示
        </Button>
        <Button size="sm" variant="ghost" className="h-7 gap-1 px-2 text-muted-foreground" onClick={() => void onClearDemo()}>
          <Trash2 className="size-3.5" />
          清除演示
        </Button>
        <Button
          size="sm"
          variant="ghost"
          className="h-7 gap-1 px-2 text-muted-foreground"
          onClick={() => void downloadTasksCsv(token).catch(() => setNotice("导出失败"))}
        >
          <Download className="size-3.5" />
          导出 CSV
        </Button>
        {notice && <span className="rounded bg-muted px-2 py-1 text-muted-foreground">{notice}</span>}
      </div>

      {/* 批量操作条 */}
      {selected.size > 0 && (
        <div className="mb-3 flex flex-wrap items-center gap-2 rounded-lg border border-primary/30 bg-primary/5 px-3 py-2 text-xs">
          <span className="font-medium">已选 {selected.size} 项</span>
          <span className="text-muted-foreground">移动到</span>
          <select className={selectClass} value={bulkStatus} onChange={(e) => setBulkStatus(e.target.value)}>
            {TASK_STATUSES.map((s) => (
              <option key={s.value} value={s.value}>
                {s.label}
              </option>
            ))}
          </select>
          <Button size="sm" variant="outline" className="h-7" onClick={() => void runBulk("move", { status: bulkStatus })}>
            应用
          </Button>
          <Button
            size="sm"
            variant="outline"
            className="h-7 text-destructive"
            onClick={() => void runBulk("delete")}
          >
            <Trash2 className="size-3.5" />
            删除
          </Button>
          <Button size="sm" variant="ghost" className="h-7" onClick={() => setSelected(new Set())}>
            <X className="size-3.5" />
            取消
          </Button>
        </div>
      )}

      {/* 看板列 */}
      {!board ? (
        <div className="py-12 text-center text-muted-foreground">
          <Loader2 className="mx-auto mb-2 size-5 animate-spin" />
          加载中
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4">
          {columns.map((col) => {
            const meta = statusMeta(col.status);
            const colIdx = TASK_STATUSES.findIndex((s) => s.value === col.status);
            return (
              <div key={col.status} className="flex flex-col rounded-lg bg-muted/30 p-2">
                <div className="mb-2 flex items-center gap-2 px-1">
                  <span className={cn("h-3 w-1 rounded-full", meta.headerRing)} />
                  <span className="text-sm font-medium">{meta.label}</span>
                  <span className="rounded-full bg-background px-1.5 text-[11px] text-muted-foreground">
                    {col.count}
                  </span>
                  <button
                    type="button"
                    onClick={() => openCreate(col.status)}
                    className="ml-auto rounded p-1 text-muted-foreground hover:bg-background hover:text-foreground"
                    aria-label="在此列新建"
                  >
                    <Plus className="size-3.5" />
                  </button>
                </div>
                <div className="flex flex-1 flex-col gap-2">
                  {col.items.length === 0 ? (
                    <p className="rounded-lg border border-dashed border-border py-6 text-center text-xs text-muted-foreground">
                      暂无任务
                    </p>
                  ) : (
                    col.items.map((t) => (
                      <TaskCard
                        key={t.id}
                        task={t}
                        selected={selected.has(t.id)}
                        onToggleSelect={toggleSelect}
                        onOpen={openDetail}
                        onMove={onMove}
                        canLeft={colIdx > 0}
                        canRight={colIdx < TASK_STATUSES.length - 1}
                        busy={movingIds.has(t.id)}
                      />
                    ))
                  )}
                </div>
              </div>
            );
          })}
        </div>
      )}

      <TaskEditorDialog
        token={token}
        open={editorOpen}
        onOpenChange={setEditorOpen}
        task={editingTask}
        defaultStatus={editorDefaultStatus}
        onSaved={() => void load()}
      />
      <TaskDetailDrawer
        token={token}
        taskId={detailId}
        open={detailOpen}
        onOpenChange={setDetailOpen}
        onEdit={(t) => {
          setEditingTask(t);
          setEditorOpen(true);
        }}
        onChanged={() => void load()}
      />
    </div>
  );
}

// 统计头:关键指标 + 完成率 + 优先级分布 + 负责人负载。
function StatsHeader({ stats }: { stats: TaskStats }) {
  const pct = Math.round((stats.completed_rate || 0) * 100);
  const byPriority = Array.isArray(stats.by_priority) ? stats.by_priority : [];
  const byAssignee = Array.isArray(stats.by_assignee) ? stats.by_assignee : [];
  const maxPrio = Math.max(1, ...byPriority.map((p) => p.count));
  return (
    <div className="mb-4 grid gap-3 rounded-lg border border-border bg-muted/20 p-3 lg:grid-cols-3">
      {/* 指标 */}
      <div className="grid grid-cols-4 gap-2 lg:col-span-1">
        <Kpi label="总数" value={stats.total} />
        <Kpi label="进行中" value={stats.open} tone="text-sky-600 dark:text-sky-400" />
        <Kpi label="已完成" value={stats.done} tone="text-emerald-600 dark:text-emerald-400" />
        <Kpi label="逾期" value={stats.overdue} tone="text-rose-600 dark:text-rose-400" />
      </div>

      {/* 完成率 */}
      <div className="flex flex-col justify-center">
        <div className="mb-1 flex items-center justify-between text-[11px] text-muted-foreground">
          <span>完成率</span>
          <span className="tabular-nums">{pct}%</span>
        </div>
        <div className="h-2 overflow-hidden rounded-full bg-muted">
          <div className="h-full rounded-full bg-emerald-500 transition-all" style={{ width: `${pct}%` }} />
        </div>
        <div className="mt-2 flex flex-wrap gap-1">
          {byAssignee.slice(0, 5).map((a) => (
            <span key={a.assignee} className="rounded-full bg-background px-2 py-0.5 text-[11px] text-muted-foreground">
              {a.assignee} · {a.count}
            </span>
          ))}
        </div>
      </div>

      {/* 优先级分布 */}
      <div className="flex flex-col justify-center gap-1">
        <span className="mb-0.5 text-[11px] text-muted-foreground">优先级分布</span>
        {byPriority.map((p) => {
          const meta = priorityMeta(p.priority);
          return (
            <div key={p.priority} className="flex items-center gap-2 text-[11px]">
              <span className="w-8 text-muted-foreground">{meta.label}</span>
              <div className="h-2 flex-1 overflow-hidden rounded-full bg-muted">
                <div className={cn("h-full rounded-full", meta.bar)} style={{ width: `${(p.count / maxPrio) * 100}%` }} />
              </div>
              <span className="w-6 text-right tabular-nums text-muted-foreground">{p.count}</span>
            </div>
          );
        })}
      </div>
    </div>
  );
}

function Kpi({ label, value, tone }: { label: string; value: number; tone?: string }) {
  return (
    <div className="rounded-lg border border-border bg-card px-2 py-1.5 text-center">
      <p className={cn("text-lg font-semibold tabular-nums", tone)}>{value}</p>
      <p className="text-[10px] text-muted-foreground">{label}</p>
    </div>
  );
}
