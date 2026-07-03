"use client";

import type { ReactNode } from "react";

import { cn } from "@/lib/utils";
import type { TaskItem } from "./console-api";

// ── 元数据 ───────────────────────────────────────────────────

export type StatusMeta = {
  value: string;
  label: string;
  dot: string; // 圆点色
  headerRing: string; // 列头强调条
  soft: string; // 软背景
};

export const TASK_STATUSES: StatusMeta[] = [
  { value: "todo", label: "待办", dot: "bg-slate-400", headerRing: "bg-slate-400", soft: "bg-slate-500/10 text-slate-600 dark:text-slate-300" },
  { value: "doing", label: "进行中", dot: "bg-sky-500", headerRing: "bg-sky-500", soft: "bg-sky-500/10 text-sky-600 dark:text-sky-400" },
  { value: "review", label: "待验收", dot: "bg-violet-500", headerRing: "bg-violet-500", soft: "bg-violet-500/10 text-violet-600 dark:text-violet-400" },
  { value: "done", label: "已完成", dot: "bg-emerald-500", headerRing: "bg-emerald-500", soft: "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400" },
];

export type PriorityMeta = {
  value: string;
  label: string;
  chip: string;
  bar: string;
};

export const TASK_PRIORITIES: PriorityMeta[] = [
  { value: "urgent", label: "紧急", chip: "bg-rose-500/12 text-rose-600 dark:text-rose-400 ring-1 ring-inset ring-rose-500/30", bar: "bg-rose-500" },
  { value: "high", label: "高", chip: "bg-amber-500/12 text-amber-600 dark:text-amber-400 ring-1 ring-inset ring-amber-500/30", bar: "bg-amber-500" },
  { value: "medium", label: "中", chip: "bg-sky-500/12 text-sky-600 dark:text-sky-400 ring-1 ring-inset ring-sky-500/30", bar: "bg-sky-500" },
  { value: "low", label: "低", chip: "bg-slate-500/12 text-slate-600 dark:text-slate-300 ring-1 ring-inset ring-slate-500/25", bar: "bg-slate-400" },
];

export function statusMeta(value: string): StatusMeta {
  return TASK_STATUSES.find((s) => s.value === value) ?? TASK_STATUSES[0];
}

export function priorityMeta(value: string): PriorityMeta {
  return TASK_PRIORITIES.find((p) => p.value === value) ?? TASK_PRIORITIES[2];
}

// 活动类型 → 中文动词。
export const ACTION_LABEL: Record<string, string> = {
  created: "创建",
  moved: "移动",
  edited: "编辑",
  commented: "评论",
  checklist: "子任务",
  assigned: "指派",
  due: "截止",
};

// ── 时间工具 ─────────────────────────────────────────────────

export function isOverdue(task: Pick<TaskItem, "due_at" | "status">): boolean {
  if (!task.due_at || task.status === "done") return false;
  const t = new Date(task.due_at).getTime();
  return !Number.isNaN(t) && t < Date.now();
}

// dueLabel 把截止时间转成「今天 / 明天 / n天后 / 逾期 n天」的短标签。
export function dueLabel(due?: string | null): string {
  if (!due) return "";
  const t = new Date(due).getTime();
  if (Number.isNaN(t)) return "";
  const dayMs = 86400000;
  const startOfToday = new Date();
  startOfToday.setHours(0, 0, 0, 0);
  const diffDays = Math.round((t - startOfToday.getTime()) / dayMs);
  if (diffDays === 0) return "今天";
  if (diffDays === 1) return "明天";
  if (diffDays === -1) return "昨天";
  if (diffDays < 0) return `逾期${-diffDays}天`;
  return `${diffDays}天后`;
}

// toDateInput 把 ISO 时间转成 <input type="date"> 需要的 YYYY-MM-DD。
export function toDateInput(due?: string | null): string {
  if (!due) return "";
  const d = new Date(due);
  if (Number.isNaN(d.getTime())) return "";
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

// dateInputToISO 把 YYYY-MM-DD 转成当天 23:59 的 ISO 串(截止到当天末)。
export function dateInputToISO(value: string): string | null {
  if (!value) return null;
  const d = new Date(`${value}T23:59:59`);
  if (Number.isNaN(d.getTime())) return null;
  return d.toISOString();
}

// ── 小组件 ───────────────────────────────────────────────────

export function PriorityBadge({ priority, className }: { priority: string; className?: string }) {
  const m = priorityMeta(priority);
  return (
    <span className={cn("inline-flex items-center rounded px-1.5 py-0.5 text-[11px] font-medium", m.chip, className)}>
      {m.label}
    </span>
  );
}

export function StatusPill({ status }: { status: string }) {
  const m = statusMeta(status);
  return (
    <span className={cn("inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium", m.soft)}>
      <span className={cn("size-1.5 rounded-full", m.dot)} />
      {m.label}
    </span>
  );
}

export function LabelChip({ children }: { children: ReactNode }) {
  return (
    <span className="inline-flex items-center rounded bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground">
      {children}
    </span>
  );
}

// Avatar 用负责人名字首字生成一个稳定配色的小圆头像。
const AVATAR_COLORS = [
  "bg-rose-500",
  "bg-orange-500",
  "bg-amber-500",
  "bg-emerald-500",
  "bg-sky-500",
  "bg-indigo-500",
  "bg-violet-500",
  "bg-pink-500",
];

export function Avatar({ name, className }: { name?: string; className?: string }) {
  const label = (name ?? "").trim();
  if (!label) {
    return (
      <span className={cn("inline-flex size-5 items-center justify-center rounded-full bg-muted text-[10px] text-muted-foreground", className)}>
        ?
      </span>
    );
  }
  let hash = 0;
  for (let i = 0; i < label.length; i += 1) hash = (hash * 31 + label.charCodeAt(i)) >>> 0;
  const color = AVATAR_COLORS[hash % AVATAR_COLORS.length];
  return (
    <span
      title={label}
      className={cn("inline-flex size-5 items-center justify-center rounded-full text-[10px] font-medium text-white", color, className)}
    >
      {label.slice(0, 1)}
    </span>
  );
}

export function DueBadge({ due, status }: { due?: string | null; status: string }) {
  if (!due) return null;
  const overdue = isOverdue({ due_at: due, status });
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px] font-medium",
        overdue
          ? "bg-rose-500/12 text-rose-600 dark:text-rose-400"
          : "bg-muted text-muted-foreground",
      )}
    >
      {dueLabel(due)}
    </span>
  );
}
