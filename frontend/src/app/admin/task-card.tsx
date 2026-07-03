"use client";

import { ChevronLeft, ChevronRight, MessageSquare } from "lucide-react";

import { cn } from "@/lib/utils";
import type { TaskItem } from "./console-api";
import { Avatar, DueBadge, LabelChip, PriorityBadge, priorityMeta } from "./task-shared";

type TaskCardProps = {
  task: TaskItem;
  selected: boolean;
  onToggleSelect: (id: number) => void;
  onOpen: (id: number) => void;
  onMove: (id: number, dir: -1 | 1) => void;
  canLeft: boolean;
  canRight: boolean;
  busy?: boolean;
};

export function TaskCard({
  task,
  selected,
  onToggleSelect,
  onOpen,
  onMove,
  canLeft,
  canRight,
  busy,
}: TaskCardProps) {
  const p = priorityMeta(task.priority);
  const labels = task.labels ?? [];

  return (
    <div
      className={cn(
        "group relative overflow-hidden rounded-lg border bg-card p-3 shadow-sm transition-all hover:shadow-md",
        selected ? "border-primary ring-1 ring-primary/40" : "border-border",
        busy && "pointer-events-none opacity-60",
      )}
    >
      {/* 左侧优先级色条 */}
      <span className={cn("absolute inset-y-0 left-0 w-1", p.bar)} aria-hidden />

      <div className="flex items-start gap-2 pl-1.5">
        <input
          type="checkbox"
          checked={selected}
          onChange={() => onToggleSelect(task.id)}
          onClick={(e) => e.stopPropagation()}
          className="mt-0.5 shrink-0"
          aria-label="选择任务"
        />
        <button
          type="button"
          onClick={() => onOpen(task.id)}
          className="flex-1 text-left"
        >
          <p className="line-clamp-2 text-sm font-medium leading-snug">{task.title}</p>
        </button>
        <PriorityBadge priority={task.priority} />
      </div>

      {task.description && (
        <p className="mt-1.5 line-clamp-2 pl-1.5 text-xs text-muted-foreground">{task.description}</p>
      )}

      {labels.length > 0 && (
        <div className="mt-2 flex flex-wrap gap-1 pl-1.5">
          {labels.slice(0, 4).map((l) => (
            <LabelChip key={l}>{l}</LabelChip>
          ))}
          {labels.length > 4 && <LabelChip>+{labels.length - 4}</LabelChip>}
        </div>
      )}

      <div className="mt-2.5 flex items-center justify-between gap-2 pl-1.5">
        <div className="flex min-w-0 items-center gap-2">
          <Avatar name={task.assignee} />
          <DueBadge due={task.due_at} status={task.status} />
          {task.comment_count > 0 && (
            <span className="inline-flex items-center gap-0.5 text-[11px] text-muted-foreground">
              <MessageSquare className="size-3" />
              {task.comment_count}
            </span>
          )}
        </div>

        <div className="flex items-center gap-0.5 opacity-0 transition-opacity group-hover:opacity-100">
          <button
            type="button"
            disabled={!canLeft}
            onClick={(e) => {
              e.stopPropagation();
              onMove(task.id, -1);
            }}
            className="rounded p-1 text-muted-foreground hover:bg-muted disabled:opacity-30"
            aria-label="左移一列"
          >
            <ChevronLeft className="size-4" />
          </button>
          <button
            type="button"
            disabled={!canRight}
            onClick={(e) => {
              e.stopPropagation();
              onMove(task.id, 1);
            }}
            className="rounded p-1 text-muted-foreground hover:bg-muted disabled:opacity-30"
            aria-label="右移一列"
          >
            <ChevronRight className="size-4" />
          </button>
        </div>
      </div>
    </div>
  );
}
