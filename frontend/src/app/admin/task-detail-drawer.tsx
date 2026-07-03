"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import {
  CalendarClock,
  Check,
  Loader2,
  Pencil,
  Plus,
  Send,
  Trash2,
  User,
} from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { cn } from "@/lib/utils";
import {
  addChecklistItem,
  addTaskComment,
  deleteChecklistItem,
  deleteTaskComment,
  fetchTaskDetail,
  formatDateTime,
  relativeTime,
  updateChecklistItem,
  type TaskDetail,
  type TaskItem,
} from "./console-api";
import {
  ACTION_LABEL,
  Avatar,
  DueBadge,
  LabelChip,
  PriorityBadge,
  StatusPill,
} from "./task-shared";

type TaskDetailDrawerProps = {
  token: string;
  taskId: number | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onEdit: (task: TaskItem) => void;
  onChanged: () => void;
};

export function TaskDetailDrawer({
  token,
  taskId,
  open,
  onOpenChange,
  onEdit,
  onChanged,
}: TaskDetailDrawerProps) {
  const [detail, setDetail] = useState<TaskDetail | null>(null);
  const [loading, setLoading] = useState(false);
  const [newCheck, setNewCheck] = useState("");
  const [newComment, setNewComment] = useState("");
  const [busy, setBusy] = useState(false);
  const changedRef = useRef(false);

  const load = useCallback(async () => {
    if (!token || !taskId) return;
    setLoading(true);
    try {
      const res = await fetchTaskDetail(token, taskId);
      setDetail(res);
    } catch {
      setDetail(null);
    } finally {
      setLoading(false);
    }
  }, [token, taskId]);

  useEffect(() => {
    if (open && taskId) {
      changedRef.current = false;
      void load();
    } else {
      setDetail(null);
      setNewCheck("");
      setNewComment("");
    }
  }, [open, taskId, load]);

  // 关闭时若发生过改动,通知父组件刷新看板。
  const handleOpenChange = (next: boolean) => {
    if (!next && changedRef.current) onChanged();
    onOpenChange(next);
  };

  const markChanged = () => {
    changedRef.current = true;
  };

  const onAddCheck = async () => {
    if (!taskId || !newCheck.trim()) return;
    setBusy(true);
    try {
      const res = await addChecklistItem(token, taskId, newCheck.trim());
      setDetail((d) => (d ? { ...d, checklist: res } : d));
      setNewCheck("");
      markChanged();
    } finally {
      setBusy(false);
    }
  };

  const onToggleCheck = async (itemId: number, done: boolean) => {
    if (!taskId) return;
    const res = await updateChecklistItem(token, taskId, itemId, { done }).catch(() => null);
    if (res) {
      setDetail((d) => (d ? { ...d, checklist: res } : d));
      markChanged();
    }
  };

  const onDeleteCheck = async (itemId: number) => {
    if (!taskId) return;
    const res = await deleteChecklistItem(token, taskId, itemId).catch(() => null);
    if (res) {
      setDetail((d) => (d ? { ...d, checklist: res } : d));
      markChanged();
    }
  };

  const onAddComment = async () => {
    if (!taskId || !newComment.trim()) return;
    setBusy(true);
    try {
      const c = await addTaskComment(token, taskId, newComment.trim());
      setDetail((d) => (d ? { ...d, comments: [...d.comments, c] } : d));
      setNewComment("");
      markChanged();
      void load(); // 刷新活动时间线
    } finally {
      setBusy(false);
    }
  };

  const onDeleteComment = async (commentId: number) => {
    if (!taskId) return;
    await deleteTaskComment(token, taskId, commentId).catch(() => {});
    setDetail((d) =>
      d ? { ...d, comments: d.comments.filter((c) => c.id !== commentId) } : d,
    );
    markChanged();
  };

  const task = detail?.task;
  const checklist = detail?.checklist;
  const checkPct = checklist && checklist.total > 0 ? Math.round((checklist.done / checklist.total) * 100) : 0;

  return (
    <Sheet open={open} onOpenChange={handleOpenChange}>
      <SheetContent side="right" className="w-full gap-0 p-0 sm:max-w-xl">
        <SheetHeader className="border-b border-border px-5 py-4">
          <SheetTitle className="pr-8 text-base leading-snug">
            {loading ? "加载中…" : task?.title ?? "任务详情"}
          </SheetTitle>
          {task && (
            <div className="mt-1.5 flex flex-wrap items-center gap-2">
              <StatusPill status={task.status} />
              <PriorityBadge priority={task.priority} />
              <DueBadge due={task.due_at} status={task.status} />
              <div className="ml-auto flex items-center gap-1">
                <Button size="sm" variant="outline" onClick={() => onEdit(task)}>
                  <Pencil className="size-3.5" />
                  编辑
                </Button>
              </div>
            </div>
          )}
        </SheetHeader>

        {loading ? (
          <div className="grid flex-1 place-items-center text-muted-foreground">
            <Loader2 className="size-6 animate-spin" />
          </div>
        ) : !detail || !task ? (
          <div className="grid flex-1 place-items-center text-sm text-muted-foreground">未找到任务</div>
        ) : (
          <div className="flex-1 space-y-6 overflow-y-auto px-5 py-4">
            {/* 元信息 */}
            <section className="grid grid-cols-2 gap-3 text-sm">
              <Meta icon={<User className="size-3.5" />} label="负责人">
                <span className="flex items-center gap-1.5">
                  <Avatar name={task.assignee} />
                  {task.assignee || "未指派"}
                </span>
              </Meta>
              <Meta icon={<CalendarClock className="size-3.5" />} label="截止">
                {task.due_at ? formatDateTime(task.due_at) : "—"}
              </Meta>
              <Meta label="创建人">{task.creator_name || "—"}</Meta>
              <Meta label="创建时间">{formatDateTime(task.created_at)}</Meta>
            </section>

            {task.labels && task.labels.length > 0 && (
              <div className="flex flex-wrap gap-1.5">
                {task.labels.map((l) => (
                  <LabelChip key={l}>{l}</LabelChip>
                ))}
              </div>
            )}

            {task.description && (
              <section>
                <h4 className="mb-1.5 text-xs font-semibold text-muted-foreground">描述</h4>
                <p className="whitespace-pre-wrap rounded-lg bg-muted/40 px-3 py-2 text-sm leading-relaxed">
                  {task.description}
                </p>
              </section>
            )}

            {/* 子清单 */}
            <section>
              <div className="mb-2 flex items-center justify-between">
                <h4 className="text-xs font-semibold text-muted-foreground">
                  子任务 {checklist ? `${checklist.done}/${checklist.total}` : ""}
                </h4>
                <span className="text-[11px] text-muted-foreground">{checkPct}%</span>
              </div>
              <div className="mb-2 h-1.5 overflow-hidden rounded-full bg-muted">
                <div className="h-full rounded-full bg-emerald-500 transition-all" style={{ width: `${checkPct}%` }} />
              </div>
              <ul className="space-y-1">
                {checklist?.items.map((item) => (
                  <li key={item.id} className="group flex items-center gap-2 rounded px-1 py-1 hover:bg-muted/50">
                    <button
                      type="button"
                      onClick={() => void onToggleCheck(item.id, !item.done)}
                      className={cn(
                        "flex size-4 shrink-0 items-center justify-center rounded border",
                        item.done ? "border-emerald-500 bg-emerald-500 text-white" : "border-muted-foreground/40",
                      )}
                      aria-label="切换完成"
                    >
                      {item.done && <Check className="size-3" />}
                    </button>
                    <span className={cn("flex-1 text-sm", item.done && "text-muted-foreground line-through")}>
                      {item.content}
                    </span>
                    <button
                      type="button"
                      onClick={() => void onDeleteCheck(item.id)}
                      className="text-muted-foreground opacity-0 transition-opacity hover:text-destructive group-hover:opacity-100"
                      aria-label="删除子任务"
                    >
                      <Trash2 className="size-3.5" />
                    </button>
                  </li>
                ))}
              </ul>
              <div className="mt-2 flex items-center gap-2">
                <Input
                  className="h-8"
                  placeholder="添加子任务"
                  value={newCheck}
                  onChange={(e) => setNewCheck(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") void onAddCheck();
                  }}
                />
                <Button size="sm" variant="outline" disabled={busy || !newCheck.trim()} onClick={() => void onAddCheck()}>
                  <Plus className="size-4" />
                </Button>
              </div>
            </section>

            {/* 评论 */}
            <section>
              <h4 className="mb-2 text-xs font-semibold text-muted-foreground">评论 {detail.comments.length}</h4>
              <ul className="space-y-2.5">
                {detail.comments.map((c) => (
                  <li key={c.id} className="group flex gap-2">
                    <Avatar name={c.author_name} className="mt-0.5" />
                    <div className="flex-1 rounded-lg bg-muted/40 px-3 py-2">
                      <div className="flex items-center gap-2">
                        <span className="text-xs font-medium">{c.author_name || "匿名"}</span>
                        <span className="text-[10px] text-muted-foreground">{relativeTime(c.created_at)}</span>
                        <button
                          type="button"
                          onClick={() => void onDeleteComment(c.id)}
                          className="ml-auto text-muted-foreground opacity-0 transition-opacity hover:text-destructive group-hover:opacity-100"
                          aria-label="删除评论"
                        >
                          <Trash2 className="size-3" />
                        </button>
                      </div>
                      <p className="mt-0.5 whitespace-pre-wrap text-sm">{c.content}</p>
                    </div>
                  </li>
                ))}
                {detail.comments.length === 0 && (
                  <li className="text-xs text-muted-foreground">还没有评论</li>
                )}
              </ul>
              <div className="mt-2 flex items-end gap-2">
                <Textarea
                  className="min-h-[2.25rem]"
                  rows={2}
                  placeholder="写下进展或讨论…"
                  value={newComment}
                  onChange={(e) => setNewComment(e.target.value)}
                />
                <Button size="sm" disabled={busy || !newComment.trim()} onClick={() => void onAddComment()}>
                  <Send className="size-4" />
                </Button>
              </div>
            </section>

            {/* 活动时间线 */}
            <section>
              <h4 className="mb-2 text-xs font-semibold text-muted-foreground">活动</h4>
              <ol className="relative space-y-3 border-l border-border pl-4">
                {detail.activities.map((a) => (
                  <li key={a.id} className="relative">
                    <span className="absolute -left-[1.28rem] top-1 size-2 rounded-full bg-primary/70 ring-2 ring-background" />
                    <p className="text-xs">
                      <span className="font-medium">{a.actor_name || "系统"}</span>{" "}
                      <span className="text-muted-foreground">
                        {ACTION_LABEL[a.action] ?? a.action} · {a.detail}
                      </span>
                    </p>
                    <p className="text-[10px] text-muted-foreground">{relativeTime(a.created_at)}</p>
                  </li>
                ))}
                {detail.activities.length === 0 && <li className="text-xs text-muted-foreground">暂无活动</li>}
              </ol>
            </section>
          </div>
        )}
      </SheetContent>
    </Sheet>
  );
}

function Meta({
  icon,
  label,
  children,
}: {
  icon?: React.ReactNode;
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div>
      <p className="mb-0.5 flex items-center gap-1 text-[11px] text-muted-foreground">
        {icon}
        {label}
      </p>
      <div className="text-sm">{children}</div>
    </div>
  );
}
